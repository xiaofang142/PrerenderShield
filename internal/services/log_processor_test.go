package services

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	crawlerUnwashedKey = "crawler_logs:unwashed"
	visitUnwashedKey   = "visit_logs:unwashed"
)

// setLogProcessorTestConfig 设置测试用全局站点配置
func setLogProcessorTestConfig(t *testing.T, sites ...config.SiteConfig) {
	t.Helper()
	config.GetInstance().UpdateConfig(&config.Config{Sites: sites})
}

// newTestGeoIPService 构造无网络的 GeoIPService，可选注入本机位置
func newTestGeoIPService(serverLocation *GeoLocation) *GeoIPService {
	svc := &GeoIPService{client: &http.Client{Transport: roundTripperFunc(failRT)}}
	if serverLocation != nil {
		svc.serverLocation = serverLocation
	}
	return svc
}

// pushCrawlerLog 直接向待清洗队列写入爬虫日志（绕过管理器异步通道，保证确定性）
func pushCrawlerLog(t *testing.T, client *redis.Client, l logging.CrawlerLog) {
	t.Helper()
	if l.Time.IsZero() {
		l.Time = time.Now()
	}
	data, err := json.Marshal(l)
	require.NoError(t, err)
	require.NoError(t, client.RPush(context.Background(), crawlerUnwashedKey, data).Err())
}

// pushVisitLog 直接向待清洗队列写入访问日志
func pushVisitLog(t *testing.T, client *redis.Client, l logging.VisitLog) {
	t.Helper()
	if l.Time.IsZero() {
		l.Time = time.Now()
	}
	data, err := json.Marshal(l)
	require.NoError(t, err)
	require.NoError(t, client.RPush(context.Background(), visitUnwashedKey, data).Err())
}

func TestNewLogProcessor(t *testing.T) {
	client := newTestRawRedisClient(t)

	p := NewLogProcessor(
		logging.NewCrawlerLogManagerWithClient(client),
		logging.NewVisitLogManagerWithClient(client),
		newTestGeoIPService(nil),
		config.GetInstance(),
		client,
	)

	require.NotNil(t, p)
	assert.NotNil(t, p.crawlerLogMgr)
	assert.NotNil(t, p.visitLogMgr)
	assert.NotNil(t, p.geoIP)
	assert.NotNil(t, p.configMgr)
	assert.NotNil(t, p.redisClient)
	assert.NotNil(t, p.ctx)
}

func TestLogProcessor_ProcessCrawlerLogs_EmptyQueue(t *testing.T) {
	client := newTestRawRedisClient(t)
	delTestKeys(t, client, crawlerUnwashedKey)

	p := NewLogProcessor(
		logging.NewCrawlerLogManagerWithClient(client),
		logging.NewVisitLogManagerWithClient(client),
		newTestGeoIPService(nil),
		config.GetInstance(),
		client,
	)

	// 空队列：直接返回，不应 panic
	p.processCrawlerLogs()

	// 队列恢复为空
	assert.Equal(t, int64(0), client.LLen(context.Background(), crawlerUnwashedKey).Val())
}

func TestLogProcessor_ProcessCrawlerLogs_RedisError(t *testing.T) {
	// 使用已关闭的客户端触发 GetUnwashedLogs 错误分支
	client := redis.NewClient(&redis.Options{Addr: testRedisAddr, DB: testRedisDB})
	require.NoError(t, client.Ping(context.Background()).Err())

	p := NewLogProcessor(
		logging.NewCrawlerLogManagerWithClient(client),
		logging.NewVisitLogManagerWithClient(client),
		newTestGeoIPService(nil),
		config.GetInstance(),
		client,
	)

	require.NoError(t, client.Close())
	// 不应 panic，静默返回
	p.processCrawlerLogs()
}

func TestLogProcessor_ProcessCrawlerLogs_Success(t *testing.T) {
	client := newTestRawRedisClient(t)
	delTestKeys(t, client, crawlerUnwashedKey, "crawler_logs:site-a:*", "crawler_logs:all:*", "firewall:site-a:blacklist")
	defer delTestKeys(t, client, "crawler_logs:site-a:*", "crawler_logs:all:*", "firewall:site-a:blacklist")

	setLogProcessorTestConfig(t, config.SiteConfig{
		ID:       "site-a",
		Firewall: config.FirewallConfig{GeoIPConfig: config.GeoIPConfig{BlockList: []string{"xx"}}},
	})

	geo := newTestGeoIPService(&GeoLocation{
		Country: "Testland", CountryCode: "XX", City: "Test City", Latitude: 1.5, Longitude: 2.5,
	})
	p := NewLogProcessor(
		logging.NewCrawlerLogManagerWithClient(client),
		logging.NewVisitLogManagerWithClient(client),
		geo,
		config.GetInstance(),
		client,
	)

	pushCrawlerLog(t, client, logging.CrawlerLog{Site: "site-a", IP: "127.0.0.1", Route: "/a"})
	pushCrawlerLog(t, client, logging.CrawlerLog{Site: "site-a", IP: "127.0.0.1", Route: "/b"})

	p.processCrawlerLogs()

	// 内网 IP 命中 BlockList（大小写不敏感）→ 加入黑名单
	members, err := client.SMembers(context.Background(), "firewall:site-a:blacklist").Result()
	require.NoError(t, err)
	assert.Contains(t, members, "127.0.0.1")

	// 日志应被标记 washed 并填充地理位置
	date := time.Now().Format("2006-01-02")
	entries, err := client.ZRange(context.Background(), "crawler_logs:site-a:"+date, 0, -1).Result()
	require.NoError(t, err)
	require.Len(t, entries, 2)

	var washed int
	for _, raw := range entries {
		var l logging.CrawlerLog
		require.NoError(t, json.Unmarshal([]byte(raw), &l))
		assert.Equal(t, "Testland", l.Country)
		assert.Equal(t, "XX", l.CountryCode)
		assert.Equal(t, "Test City", l.City)
		assert.Equal(t, 1.5, l.Latitude)
		assert.Equal(t, 2.5, l.Longitude)
		if l.Washed {
			washed++
		}
	}
	assert.Equal(t, 2, washed)

	// 待清洗队列已清空
	assert.Equal(t, int64(0), client.LLen(context.Background(), crawlerUnwashedKey).Val())
}

func TestLogProcessor_ProcessCrawlerLogs_GeoIPErrorSkipsBan(t *testing.T) {
	client := newTestRawRedisClient(t)
	delTestKeys(t, client, crawlerUnwashedKey, "crawler_logs:site-a:*", "crawler_logs:all:*", "firewall:site-a:blacklist")
	defer delTestKeys(t, client, "crawler_logs:site-a:*", "crawler_logs:all:*", "firewall:site-a:blacklist")

	setLogProcessorTestConfig(t, config.SiteConfig{
		ID:       "site-a",
		Firewall: config.FirewallConfig{GeoIPConfig: config.GeoIPConfig{BlockList: []string{"XX"}}},
	})

	// 无本机位置且所有 provider 失败 → GetLocation 返回错误
	p := NewLogProcessor(
		logging.NewCrawlerLogManagerWithClient(client),
		logging.NewVisitLogManagerWithClient(client),
		newTestGeoIPService(nil),
		config.GetInstance(),
		client,
	)

	pushCrawlerLog(t, client, logging.CrawlerLog{Site: "site-a", IP: "8.8.8.8"})

	p.processCrawlerLogs()

	// GeoIP 失败时不应封禁
	members, err := client.SMembers(context.Background(), "firewall:site-a:blacklist").Result()
	require.NoError(t, err)
	assert.Empty(t, members)

	// 日志仍被标记 washed，但地理位置为空
	date := time.Now().Format("2006-01-02")
	entries, err := client.ZRange(context.Background(), "crawler_logs:site-a:"+date, 0, -1).Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)

	var l logging.CrawlerLog
	require.NoError(t, json.Unmarshal([]byte(entries[0]), &l))
	assert.True(t, l.Washed)
	assert.Empty(t, l.Country)
}

func TestLogProcessor_ProcessCrawlerLogs_UpdateLogError(t *testing.T) {
	// 假 Redis：LPOP 正常返回日志，ZADD/ZREM 返回错误 → UpdateLog 失败
	fake := newFakeRedisServer(t, map[string]bool{"ZADD": true, "ZREM": true})
	client := redis.NewClient(&redis.Options{Addr: fake.addr(), DB: testRedisDB})
	t.Cleanup(func() { _ = client.Close() })

	setLogProcessorTestConfig(t, config.SiteConfig{
		ID:       "site-a",
		Firewall: config.FirewallConfig{GeoIPConfig: config.GeoIPConfig{BlockList: []string{"XX"}}},
	})

	log := logging.CrawlerLog{Site: "site-a", IP: "127.0.0.1", Route: "/x"}
	if log.Time.IsZero() {
		log.Time = time.Now()
	}
	data, err := json.Marshal(log)
	require.NoError(t, err)
	fake.push(crawlerUnwashedKey, string(data))

	p := NewLogProcessor(
		logging.NewCrawlerLogManagerWithClient(client),
		logging.NewVisitLogManagerWithClient(client),
		newTestGeoIPService(&GeoLocation{CountryCode: "XX"}),
		config.GetInstance(),
		client,
	)

	// UpdateLog 失败仅记录日志，不应 panic
	p.processCrawlerLogs()
}

func TestLogProcessor_ProcessVisitLogs_Success(t *testing.T) {
	client := newTestRawRedisClient(t)
	delTestKeys(t, client, visitUnwashedKey, "visit_logs:site-b:*", "visit_logs:all:*", "firewall:site-b:blacklist")
	defer delTestKeys(t, client, "visit_logs:site-b:*", "visit_logs:all:*", "firewall:site-b:blacklist")

	setLogProcessorTestConfig(t, config.SiteConfig{
		ID:       "site-b",
		Firewall: config.FirewallConfig{GeoIPConfig: config.GeoIPConfig{BlockList: []string{"ZZ"}}},
	})

	p := NewLogProcessor(
		logging.NewCrawlerLogManagerWithClient(client),
		logging.NewVisitLogManagerWithClient(client),
		newTestGeoIPService(&GeoLocation{Country: "Testland", CountryCode: "ZZ", City: "Test City"}),
		config.GetInstance(),
		client,
	)

	pushVisitLog(t, client, logging.VisitLog{Site: "site-b", IP: "127.0.0.1", URL: "/v"})

	p.processVisitLogs()

	members, err := client.SMembers(context.Background(), "firewall:site-b:blacklist").Result()
	require.NoError(t, err)
	assert.Contains(t, members, "127.0.0.1")

	date := time.Now().Format("2006-01-02")
	entries, err := client.ZRange(context.Background(), "visit_logs:site-b:"+date, 0, -1).Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)

	var l logging.VisitLog
	require.NoError(t, json.Unmarshal([]byte(entries[0]), &l))
	assert.True(t, l.Washed)
	assert.Equal(t, "Testland", l.Country)
	assert.Equal(t, "ZZ", l.CountryCode)

	assert.Equal(t, int64(0), client.LLen(context.Background(), visitUnwashedKey).Val())
}

func TestLogProcessor_ProcessVisitLogs_EmptyQueue(t *testing.T) {
	client := newTestRawRedisClient(t)
	delTestKeys(t, client, visitUnwashedKey)

	p := NewLogProcessor(
		logging.NewCrawlerLogManagerWithClient(client),
		logging.NewVisitLogManagerWithClient(client),
		newTestGeoIPService(nil),
		config.GetInstance(),
		client,
	)

	p.processVisitLogs()

	assert.Equal(t, int64(0), client.LLen(context.Background(), visitUnwashedKey).Val())
}

func TestLogProcessor_ProcessVisitLogs_UpdateLogError(t *testing.T) {
	fake := newFakeRedisServer(t, map[string]bool{"ZADD": true, "ZREM": true})
	client := redis.NewClient(&redis.Options{Addr: fake.addr(), DB: testRedisDB})
	t.Cleanup(func() { _ = client.Close() })

	log := logging.VisitLog{Site: "site-b", IP: "127.0.0.1", URL: "/v"}
	if log.Time.IsZero() {
		log.Time = time.Now()
	}
	data, err := json.Marshal(log)
	require.NoError(t, err)
	fake.push(visitUnwashedKey, string(data))

	p := NewLogProcessor(
		logging.NewCrawlerLogManagerWithClient(client),
		logging.NewVisitLogManagerWithClient(client),
		newTestGeoIPService(&GeoLocation{CountryCode: "ZZ"}),
		config.GetInstance(),
		client,
	)

	p.processVisitLogs()
}

func TestLogProcessor_CheckAndBan(t *testing.T) {
	client := newTestRawRedisClient(t)
	delTestKeys(t, client, "firewall:site-a:blacklist", "firewall:site-b:blacklist")
	defer delTestKeys(t, client, "firewall:site-a:blacklist", "firewall:site-b:blacklist")

	setLogProcessorTestConfig(t,
		config.SiteConfig{
			ID:       "site-a",
			Firewall: config.FirewallConfig{GeoIPConfig: config.GeoIPConfig{BlockList: []string{"CN"}}},
		},
		config.SiteConfig{
			ID:       "site-b",
			Firewall: config.FirewallConfig{GeoIPConfig: config.GeoIPConfig{BlockList: []string{}}},
		},
	)

	p := NewLogProcessor(
		logging.NewCrawlerLogManagerWithClient(client),
		logging.NewVisitLogManagerWithClient(client),
		newTestGeoIPService(nil),
		config.GetInstance(),
		client,
	)

	// 未知站点：直接返回，不封禁
	p.checkAndBan("unknown-site", "1.2.3.4", "CN")

	// 命中 BlockList（大小写不敏感）→ 封禁
	p.checkAndBan("site-a", "1.2.3.4", "cn")
	members, err := client.SMembers(context.Background(), "firewall:site-a:blacklist").Result()
	require.NoError(t, err)
	assert.Contains(t, members, "1.2.3.4")

	// 未命中 BlockList → 不封禁
	p.checkAndBan("site-a", "5.6.7.8", "US")
	members, err = client.SMembers(context.Background(), "firewall:site-a:blacklist").Result()
	require.NoError(t, err)
	assert.Len(t, members, 1)

	// BlockList 为空 → 不封禁
	p.checkAndBan("site-b", "9.9.9.9", "CN")
	members, err = client.SMembers(context.Background(), "firewall:site-b:blacklist").Result()
	require.NoError(t, err)
	assert.Empty(t, members)
}

func TestLogProcessor_AddToBlacklist(t *testing.T) {
	client := newTestRawRedisClient(t)
	delTestKeys(t, client, "firewall:add-test:blacklist")
	defer delTestKeys(t, client, "firewall:add-test:blacklist")

	p := NewLogProcessor(
		logging.NewCrawlerLogManagerWithClient(client),
		logging.NewVisitLogManagerWithClient(client),
		newTestGeoIPService(nil),
		config.GetInstance(),
		client,
	)

	p.addToBlacklist("add-test", "203.0.113.9")

	members, err := client.SMembers(context.Background(), "firewall:add-test:blacklist").Result()
	require.NoError(t, err)
	assert.Equal(t, []string{"203.0.113.9"}, members)
}

func TestLogProcessor_Start_ProcessesAndStops(t *testing.T) {
	client := newTestRawRedisClient(t)
	delTestKeys(t, client, crawlerUnwashedKey, visitUnwashedKey,
		"crawler_logs:site-tick:*", "crawler_logs:all:*",
		"visit_logs:site-tick:*", "visit_logs:all:*", "firewall:site-tick:blacklist")
	defer delTestKeys(t, client, "firewall:site-tick:blacklist")

	setLogProcessorTestConfig(t, config.SiteConfig{
		ID:       "site-tick",
		Firewall: config.FirewallConfig{GeoIPConfig: config.GeoIPConfig{BlockList: []string{"XX"}}},
	})

	p := NewLogProcessor(
		logging.NewCrawlerLogManagerWithClient(client),
		logging.NewVisitLogManagerWithClient(client),
		newTestGeoIPService(&GeoLocation{CountryCode: "XX"}),
		config.GetInstance(),
		client,
	)

	pushCrawlerLog(t, client, logging.CrawlerLog{Site: "site-tick", IP: "127.0.0.1"})
	pushVisitLog(t, client, logging.VisitLog{Site: "site-tick", IP: "127.0.0.1"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p.Start(ctx)

	// 5 秒 tick 后两类日志应被清洗并触发封禁
	waitForCondition(t, 8*time.Second, func() bool {
		members, err := client.SMembers(context.Background(), "firewall:site-tick:blacklist").Result()
		return err == nil && len(members) > 0
	}, "Start 后 8 秒内未完成日志清洗与封禁")

	// 取消 context 后循环退出，不再处理新日志
	cancel()
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, int64(0), client.LLen(context.Background(), crawlerUnwashedKey).Val())
	assert.Equal(t, int64(0), client.LLen(context.Background(), visitUnwashedKey).Val())
}

func TestLogProcessor_ProcessVisitLogs_GeoIPErrorSkipsBan(t *testing.T) {
	client := newTestRawRedisClient(t)
	delTestKeys(t, client, visitUnwashedKey, "visit_logs:site-b:*", "visit_logs:all:*", "firewall:site-b:blacklist")
	defer delTestKeys(t, client, "visit_logs:site-b:*", "visit_logs:all:*", "firewall:site-b:blacklist")

	setLogProcessorTestConfig(t, config.SiteConfig{
		ID:       "site-b",
		Firewall: config.FirewallConfig{GeoIPConfig: config.GeoIPConfig{BlockList: []string{"ZZ"}}},
	})

	// 无本机位置且所有 provider 失败 → GetLocation 返回错误
	p := NewLogProcessor(
		logging.NewCrawlerLogManagerWithClient(client),
		logging.NewVisitLogManagerWithClient(client),
		newTestGeoIPService(nil),
		config.GetInstance(),
		client,
	)

	pushVisitLog(t, client, logging.VisitLog{Site: "site-b", IP: "8.8.8.8"})

	p.processVisitLogs()

	// GeoIP 失败时不应封禁
	members, err := client.SMembers(context.Background(), "firewall:site-b:blacklist").Result()
	require.NoError(t, err)
	assert.Empty(t, members)

	// 日志仍被标记 washed，但地理位置为空
	date := time.Now().Format("2006-01-02")
	entries, err := client.ZRange(context.Background(), "visit_logs:site-b:"+date, 0, -1).Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)

	var l logging.VisitLog
	require.NoError(t, json.Unmarshal([]byte(entries[0]), &l))
	assert.True(t, l.Washed)
	assert.Empty(t, l.Country)
}
