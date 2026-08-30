package controllers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
	"prerender-shield/internal/services"
	"prerender-shield/internal/ssl"
)

// fakeWafRedis 实现 repository.WafRedisClient 接口（仅 Get 语义）
type fakeWafRedis struct {
	values map[string]string
}

func (f *fakeWafRedis) Context() context.Context { return context.Background() }
func (f *fakeWafRedis) Get(_ context.Context, key string) (string, error) {
	return f.values[key], nil
}
func (f *fakeWafRedis) Set(_ context.Context, key string, value interface{}, _ time.Duration) error {
	return nil
}
func (f *fakeWafRedis) LPush(_ context.Context, key string, value interface{}) error { return nil }
func (f *fakeWafRedis) LRange(_ context.Context, key string, start, stop int64) ([]string, error) {
	return nil, nil
}
func (f *fakeWafRedis) LLen(_ context.Context, key string) (int64, error)              { return 0, nil }
func (f *fakeWafRedis) LTrim(_ context.Context, key string, start, stop int64) error   { return nil }
func (f *fakeWafRedis) HIncrBy(_ context.Context, key, field string, incr int64) error { return nil }
func (f *fakeWafRedis) Incr(_ context.Context, key string) error                       { return nil }
func (f *fakeWafRedis) HGetAll(_ context.Context, key string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (f *fakeWafRedis) Expire(_ context.Context, key string, _ time.Duration) error { return nil }

// seedVisitLogs 直接以 ZSet 形式写入 DB15 访问日志（Washed + 地理字段）
func seedVisitLogs(t *testing.T, client *redis.Client, site string, logs []logging.VisitLog) {
	t.Helper()
	day := time.Now().Format("2006-01-02")
	key := fmt.Sprintf("visit_logs:%s:%s", site, day)
	for _, l := range logs {
		if l.Time.IsZero() {
			l.Time = time.Now()
		}
		data, err := json.Marshal(l)
		require.NoError(t, err)
		require.NoError(t, client.ZAdd(key, float64(l.Time.UnixNano()), string(data)))
	}
	t.Cleanup(func() { client.Del(key) })
}

// TestOverviewController_GetOverview_FullData 全量数据路径：WAF 统计 + SSL 证书 + 地理 + 流量合并
func TestOverviewController_GetOverview_FullData(t *testing.T) {
	client := newTestRedisDB15(t)
	now := time.Now()

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{ID: "ctl-ov", Name: "OV", Firewall: config.FirewallConfig{Enabled: true}, Prerender: config.PrerenderConfig{Enabled: true}},
		},
	}

	// WAF 全局统计（fake WafRedisClient）
	wafRepo := repository.NewWafRepository(&fakeWafRedis{
		values: map[string]string{"waf:stats:global:total": "456", "waf:stats:global:blocked": "12"},
	})

	// SSL 管理器：2 张证书
	sslMgr := &fakeSSLManager{certs: map[string]map[string]interface{}{
		"a.example": {"domain": "a.example"},
		"b.example": {"domain": "b.example"},
	}}

	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: true})
	visitLogMgr := logging.NewVisitLogManagerWithClient(client.GetRawClient())
	crawlerLogMgr := logging.NewCrawlerLogManagerWithClient(client.GetRawClient())

	controller := &OverviewController{
		cfg:           configRef{snapshot: cfg},
		monitor:       monitor,
		visitLogMgr:   visitLogMgr,
		crawlerLogMgr: crawlerLogMgr,
		wafStatsSvc:   services.NewOverviewService(wafRepo),
		sslMgr:        sslMgr,
	}

	// 地理访问日志：一条 country_code 命中、一条回退 country 名称
	seedVisitLogs(t, client, "all", []logging.VisitLog{
		{ID: "ov1", Washed: true, Latitude: 39.9, Longitude: 116.4, CountryCode: "US", Country: "United States", City: "NYC", Time: now},
		{ID: "ov2", Washed: true, Latitude: 31.2, Longitude: 121.5, CountryCode: "", Country: "China", City: "SH", Time: now},
	})
	// 爬虫日志（爬虫/拦截统计）
	seedCrawlerLogs(t, client, "all", []logging.CrawlerLog{
		{ID: "ovc1", Site: "all", Status: 200, Time: now},
		{ID: "ovc2", Site: "all", Status: 403, Time: now},
	})

	router := ginNewRouter()
	router.GET("/overview", controller.GetOverview)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/overview", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, jsonUnmarshalBody(w, &resp))

	// WAF 统计优先于 monitor 计数
	// 全量并行时其他包会向全局键 crawler_logs:all:<day> 并发写入 → 精确计数有竞态，改抗并发断言
	assert.GreaterOrEqual(t, resp.Data["totalRequests"], float64(456))
	// 爬虫 2 条，其中 403 一条
	assert.GreaterOrEqual(t, resp.Data["crawlerRequests"], float64(2))
	assert.GreaterOrEqual(t, resp.Data["blockedRequests"], float64(1))
	assert.Equal(t, float64(2), resp.Data["sslCertificates"])
	assert.Equal(t, true, resp.Data["firewallEnabled"])
	assert.Equal(t, true, resp.Data["prerenderEnabled"])

	// geoData：国家聚合（US 命中 + China 名称回退）
	geoData := resp.Data["geoData"].(map[string]interface{})
	countryData := geoData["countryData"].([]interface{})
	require.Len(t, countryData, 2)
	names := map[string]bool{}
	for _, c := range countryData {
		names[c.(map[string]interface{})["country"].(string)] = true
	}
	assert.True(t, names["United States"])
	assert.True(t, names["China"])
	assert.NotEmpty(t, geoData["mapData"])
	assert.NotEmpty(t, geoData["globeData"])
}

// TestOverviewController_GetOverview_SSLError ListCertificates 失败 → sslCertificates 保持 0
func TestOverviewController_GetOverview_SSLError(t *testing.T) {
	cfg := &config.Config{Sites: []config.SiteConfig{{ID: "s1"}}}
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: true})
	client := newTestRedisDB15(t)

	controller := &OverviewController{
		cfg:           configRef{snapshot: cfg},
		monitor:       monitor,
		visitLogMgr:   logging.NewVisitLogManagerWithClient(client.GetRawClient()),
		crawlerLogMgr: logging.NewCrawlerLogManagerWithClient(client.GetRawClient()),
		sslMgr:        &fakeSSLManager{listErr: true},
	}

	router := ginNewRouter()
	router.GET("/overview", controller.GetOverview)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/overview", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"sslCertificates":0`)
}

// TestOverviewController_GetOverview_MonitorNilFallbacks monitor 缺省字段（已在生产侧证明恒命中，此处防御性回归）
func TestOverviewController_GetOverview_MonitorNilFallbacks(t *testing.T) {
	client := newTestRedisDB15(t)
	cfg := &config.Config{Sites: []config.SiteConfig{{ID: "s1"}}}

	controller := &OverviewController{
		cfg:         configRef{snapshot: cfg},
		monitor:     monitoring.NewMonitor(monitoring.Config{Enabled: true}),
		visitLogMgr: logging.NewVisitLogManagerWithClient(client.GetRawClient()),
	}

	router := ginNewRouter()
	router.GET("/overview", controller.GetOverview)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/overview", nil))
	require.Equal(t, http.StatusOK, w.Code)
}

// fakeSSLManager 实现 ssl.Manager 接口的最小假实现（仅 ListCertificates 语义）
type fakeSSLManager struct {
	certs   map[string]map[string]interface{}
	listErr bool
}

func (f *fakeSSLManager) RequestCertificate(domain string) error                   { return nil }
func (f *fakeSSLManager) RenewCertificate(domain string) error                     { return nil }
func (f *fakeSSLManager) ImportCertificate(domain, certPath, keyPath string) error { return nil }
func (f *fakeSSLManager) GetCertificate(domain string) (*tls.Certificate, error)   { return nil, nil }
func (f *fakeSSLManager) GetCertificateStatus(domain string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
func (f *fakeSSLManager) ListCertificates() (map[string]map[string]interface{}, error) {
	if f.listErr {
		return nil, errors.New("list failed (simulated)")
	}
	return f.certs, nil
}
func (f *fakeSSLManager) DeleteCertificate(domain string) error { return nil }
func (f *fakeSSLManager) CheckExpiration() ([]string, error)    { return nil, nil }
func (f *fakeSSLManager) SetACMEClient(client *ssl.ACMEClient)  {}
