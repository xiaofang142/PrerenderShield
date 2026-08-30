package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"prerender-shield/internal/models"
	redisPkg "prerender-shield/internal/redis"

	goRedis "github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

// ---------- 测试辅助 ----------

const gapPrefix = "repogap:"

// newGapRepoClient 创建 DB15 上的真实客户端，熔断阈值设为极高，
// 使错误注入不会被熔断拦截（repository 各仓储方法走 wrapper 的熔断检查）
func newGapRepoClient(t *testing.T) *redisPkg.Client {
	t.Helper()
	cl, err := redisPkg.NewClientWithFullConfig("localhost:6379", "", 15, redisPkg.DefaultPoolConfig(), redisPkg.CircuitBreakerConfig{
		FailureThreshold: 1000,
		SuccessThreshold: 1,
		Timeout:          time.Hour,
	})
	if err != nil {
		t.Skipf("Redis not available at localhost:6379: %v", err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	return cl
}

// gapFailCmdHook 通过 go-redis Hook 在真实请求路径上注入命令失败：
// BeforeProcess 返回错误即短路（命令不会真正发出），其余命令正常放行。
// 用于覆盖「前一命令成功、后一命令失败」等依赖顺序的错误分支
type gapFailCmdHook struct {
	name      string // 需要失败的命令名（小写），空表示匹配全部
	keyPrefix string // 仅当首个 key 参数以该前缀开头时生效，空表示不限制
	remaining int64  // 剩余注入次数（原子），耗尽后不再拦截
}

func (h *gapFailCmdHook) match(cmd goRedis.Cmder) bool {
	if h.name != "" && cmd.Name() != h.name {
		return false
	}
	if h.keyPrefix != "" {
		args := cmd.Args()
		if len(args) < 2 {
			return false
		}
		key, _ := args[1].(string)
		if !strings.HasPrefix(key, h.keyPrefix) {
			return false
		}
	}
	return true
}

func (h *gapFailCmdHook) BeforeProcess(ctx context.Context, cmd goRedis.Cmder) (context.Context, error) {
	if h.match(cmd) && atomic.AddInt64(&h.remaining, -1) >= 0 {
		return ctx, fmt.Errorf("injected %s failure", cmd.Name())
	}
	return ctx, nil
}

func (h *gapFailCmdHook) AfterProcess(ctx context.Context, cmd goRedis.Cmder) error {
	return nil
}

func (h *gapFailCmdHook) BeforeProcessPipeline(ctx context.Context, cmds []goRedis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *gapFailCmdHook) AfterProcessPipeline(ctx context.Context, cmds []goRedis.Cmder) error {
	return nil
}

// injectGapFailure 对客户端注入失败：之后 times 次匹配的命令返回错误
func injectGapFailure(cl *redisPkg.Client, name, keyPrefix string, times int64) {
	cl.GetRawClient().AddHook(&gapFailCmdHook{name: name, keyPrefix: keyPrefix, remaining: times})
}

// ---------- AlertRepository ----------

func TestGapAlertRepository_LiveRoundtrip(t *testing.T) {
	cl := newGapRepoClient(t)
	repo := NewAlertRepository(cl)

	// 清理历史残留，确保初始为空（覆盖 getLegacyAlertHistory 的 keys 为空分支）
	assert.NoError(t, cl.Del("alert:history"))
	legacyKeys, err := cl.Keys("alert:history:gap-*")
	assert.NoError(t, err)
	if len(legacyKeys) > 0 {
		assert.NoError(t, cl.DelMultiple(legacyKeys))
	}

	// ZSet 与旧版散列键均为空 → 空历史
	assert.Empty(t, repo.GetAlertHistory(10))

	// 正常追加（带 ID 与时间戳）
	repo.AppendAlertHistory(AlertRecord{ID: "gap-a1", Level: "warn", Rule: "r", Message: "m", Value: 1, Threshold: 2, Timestamp: time.Now()})
	// 自动补 ID 与时间戳
	repo.AppendAlertHistory(AlertRecord{Level: "info"})

	hist := repo.GetAlertHistory(10)
	if !assert.NotEmpty(t, hist) {
		return
	}
	assert.Equal(t, "gap-a1", hist[0].ID) // 最新在前

	// 旧版散列键回退：ZSet 删空后读取散列键记录
	assert.NoError(t, cl.Del("alert:history"))
	newer := time.Now().Add(-time.Hour)
	older := time.Now().Add(-24 * time.Hour)
	r1, _ := json.Marshal(AlertRecord{ID: "lg-new", Level: "warn", Timestamp: newer})
	r2, _ := json.Marshal(AlertRecord{ID: "lg-old", Level: "info", Timestamp: older})
	assert.NoError(t, cl.Set("alert:history:gap-new", string(r1), 0))
	assert.NoError(t, cl.Set("alert:history:gap-old", string(r2), 0))
	assert.NoError(t, cl.Set("alert:history:gap-bad", "not-json{", 0)) // 非法 JSON → 跳过
	assert.NoError(t, cl.Set("alert:history:gap-empty", "", 0))        // 空值 → 跳过

	hist = repo.GetAlertHistory(10)
	if assert.Len(t, hist, 2) {
		assert.Equal(t, "lg-new", hist[0].ID) // 按时间降序
	}
	hist = repo.GetAlertHistory(1) // limit 截断
	assert.Len(t, hist, 1)

	// 规则 upsert / delete
	assert.NoError(t, repo.SaveAlertRule(AlertRuleData{Name: "cpu", Metric: "cpu", Operator: ">", Threshold: 90, Severity: "critical", Enabled: true, Cooldown: 5, Description: "d"}))
	rules := repo.GetAlertRules()
	if !assert.NotEmpty(t, rules) {
		return
	}
	id := rules[0].ID
	assert.NoError(t, repo.SaveAlertRule(AlertRuleData{ID: id, Name: "cpu-updated", Metric: "cpu", Operator: ">", Threshold: 95, Enabled: true})) // 按 ID upsert
	rules = repo.GetAlertRules()
	if assert.Len(t, rules, 1) {
		assert.Equal(t, "cpu-updated", rules[0].Name)
	}
	assert.NoError(t, repo.SaveAlertRule(AlertRuleData{ID: "gap-r2", Name: "mem"})) // 新增
	assert.NoError(t, repo.DeleteAlertRule("gap-r2"))
	assert.NoError(t, repo.DeleteAlertRule(id))
	assert.NoError(t, repo.DeleteAlertRule("no-such-id")) // 不存在不报错
	assert.Empty(t, repo.GetAlertRules())

	// 清理
	_ = cl.DelMultiple([]string{"alert:history", "monitoring:alert-rules"})
	keys, _ := cl.Keys("alert:history:gap-*")
	if len(keys) > 0 {
		_ = cl.DelMultiple(keys)
	}
}

func TestGapAlertRepository_NilClient(t *testing.T) {
	repo := NewAlertRepository(nil)

	repo.AppendAlertHistory(AlertRecord{Level: "info"}) // 直接返回，不 panic
	assert.Empty(t, repo.GetAlertHistory(10))
	assert.Empty(t, repo.GetAlertRules())
	assert.Error(t, repo.SaveAlertRule(AlertRuleData{Name: "x"}))
	assert.Error(t, repo.DeleteAlertRule("x"))
}

func TestGapAlertRepository_ErrorBranches(t *testing.T) {
	cl := newGapRepoClient(t)
	repo := NewAlertRepository(cl)

	// 序列化失败：NaN 无法 JSON 序列化
	repo.AppendAlertHistory(AlertRecord{ID: "nan", Value: math.NaN()})

	// ZAdd 失败
	injectGapFailure(cl, "zadd", "", 1)
	repo.AppendAlertHistory(AlertRecord{ID: "zadd-err", Timestamp: time.Now()})

	// ZAdd 成功但 Expire 失败
	injectGapFailure(cl, "expire", "", 1)
	repo.AppendAlertHistory(AlertRecord{ID: "expire-err", Timestamp: time.Now()})

	// ZAdd/Expire 成功但 ZRemRangeByScore 失败
	injectGapFailure(cl, "zremrangebyscore", "", 1)
	repo.AppendAlertHistory(AlertRecord{ID: "zrem-err", Timestamp: time.Now()})

	// GetAlertHistory：ZRevRange 失败 → 空
	injectGapFailure(cl, "zrevrange", "", 1)
	assert.Empty(t, repo.GetAlertHistory(10))

	// getLegacyAlertHistory：Keys（SCAN）失败 → 空
	// 先清空 ZSet，确保走旧版散列键回退路径
	assert.NoError(t, cl.Del("alert:history"))
	injectGapFailure(cl, "scan", "", 1)
	assert.Empty(t, repo.GetAlertHistory(10))

	// getLegacyAlertHistory：单键 Get 失败 → 跳过该键（两个键中仅首个 Get 失败）
	r, _ := json.Marshal(AlertRecord{ID: "lg-x", Timestamp: time.Now()})
	assert.NoError(t, cl.Set("alert:history:gap-err", string(r), 0))
	assert.NoError(t, cl.Set("alert:history:gap-ok", string(r), 0))
	injectGapFailure(cl, "get", "alert:history:", 1)
	hist := repo.GetAlertHistory(10)
	assert.Len(t, hist, 1) // 失败键被跳过，成功键可读出
	_ = cl.DelMultiple([]string{"alert:history:gap-err", "alert:history:gap-ok"})

	// GetAlertRules：Get 失败 → 空
	injectGapFailure(cl, "get", "monitoring:alert-rules", 1)
	assert.Empty(t, repo.GetAlertRules())

	// GetAlertRules：非法 JSON → 空
	assert.NoError(t, cl.Set("monitoring:alert-rules", "not-json{", 0))
	assert.Empty(t, repo.GetAlertRules())
	_ = cl.Del("monitoring:alert-rules")

	// saveRules：序列化失败（NaN 阈值）
	assert.Error(t, repo.SaveAlertRule(AlertRuleData{Name: "nan", Threshold: math.NaN()}))

	// saveRules：Set 失败
	injectGapFailure(cl, "set", "monitoring:alert-rules", 1)
	assert.Error(t, repo.SaveAlertRule(AlertRuleData{Name: "set-err"}))

	// DeleteAlertRule：删除后保存失败
	assert.NoError(t, repo.SaveAlertRule(AlertRuleData{ID: "d1", Name: "x"}))
	injectGapFailure(cl, "set", "monitoring:alert-rules", 1)
	assert.Error(t, repo.DeleteAlertRule("d1"))

	_ = cl.DelMultiple([]string{"alert:history", "monitoring:alert-rules"})
}

// ---------- FirewallRulesRepository ----------

func TestGapFirewallRulesRepository(t *testing.T) {
	cl := newGapRepoClient(t)
	repo := NewFirewallRulesRepository(cl)
	site := "gap-fw"
	key := "firewall:rules:" + site

	// 无数据 → nil,nil
	data, err := repo.Get(site)
	assert.NoError(t, err)
	assert.Nil(t, data)

	// 保存 + 读取往返（含非对象成员）
	rulesJSON := `{"rules":[{"id":"a","name":"r1"},{"id":"b","name":"r2"},"str-rule"]}`
	assert.NoError(t, repo.Save(site, []byte(rulesJSON)))
	data, err = repo.Get(site)
	assert.NoError(t, err)
	if assert.NotNil(t, data) {
		entries := data["rules"].([]interface{})
		assert.Len(t, entries, 3)
	}

	// 删除存在的规则
	assert.NoError(t, repo.DeleteRule(site, "a"))
	data, _ = repo.Get(site)
	entries := data["rules"].([]interface{})
	assert.Len(t, entries, 2) // b 与 str-rule 保留

	// 删除不存在的规则（无变化，不报错）
	assert.NoError(t, repo.DeleteRule(site, "no-such"))

	// "rules" 不是数组 → 直接返回
	assert.NoError(t, cl.Set(key, `{"rules":"not-array"}`, 0))
	assert.NoError(t, repo.DeleteRule(site, "a"))

	// Get 错误 → nil,nil
	injectGapFailure(cl, "get", "firewall:rules:", 1)
	data, err = repo.Get(site)
	assert.NoError(t, err)
	assert.Nil(t, data)

	// 非法 JSON → nil,nil
	assert.NoError(t, cl.Set(key, "not-json{", 0))
	data, err = repo.Get(site)
	assert.NoError(t, err)
	assert.Nil(t, data)

	// Save：Set 失败
	injectGapFailure(cl, "set", "firewall:rules:", 1)
	assert.Error(t, repo.Save(site, []byte(`{"rules":[]}`)))

	// DeleteRule：末尾 Save 失败
	assert.NoError(t, cl.Set(key, `{"rules":[{"id":"c"}]}`, 0))
	injectGapFailure(cl, "set", "firewall:rules:", 1)
	assert.Error(t, repo.DeleteRule(site, "c"))

	// nil client
	nilRepo := NewFirewallRulesRepository(nil)
	data, err = nilRepo.Get(site)
	assert.NoError(t, err)
	assert.Nil(t, data)
	assert.Error(t, nilRepo.Save(site, []byte("{}")))
	assert.NoError(t, nilRepo.DeleteRule(site, "a")) // Get → nil,nil → 返回 nil

	_ = cl.Del(key)
}

// ---------- NotificationChannelsRepository ----------

func TestGapNotificationChannelsRepository(t *testing.T) {
	cl := newGapRepoClient(t)
	repo := NewNotificationChannelsRepository(cl)
	key := "monitoring:notification-channels"

	// 无数据
	assert.Empty(t, repo.Get())

	// 保存 + 读取
	channels := []NotificationChannelData{{Type: "webhook", URL: "http://x", Enabled: true}}
	assert.NoError(t, repo.Save(channels))
	got := repo.Get()
	if assert.Len(t, got, 1) {
		assert.Equal(t, "webhook", got[0].Type)
	}

	// 非法 JSON → 空
	assert.NoError(t, cl.Set(key, "not-json{", 0))
	assert.Empty(t, repo.Get())

	// Get 失败 → 空
	injectGapFailure(cl, "get", key, 1)
	assert.Empty(t, repo.Get())

	// Save：Set 失败
	injectGapFailure(cl, "set", key, 1)
	assert.Error(t, repo.Save(channels))

	// nil client
	nilRepo := NewNotificationChannelsRepository(nil)
	assert.Empty(t, nilRepo.Get())
	assert.Error(t, nilRepo.Save(channels))

	_ = cl.Del(key)
}

// ---------- SiteRepository（真实客户端往返） ----------

func TestGapSiteRepository_LiveRoundtrip(t *testing.T) {
	cl := newGapRepoClient(t)
	repo := NewSiteRepository(cl)

	site := &models.Site{Domain: "repogap-site.com", Name: "origin", Enabled: true}
	assert.NoError(t, repo.Create(site))
	assert.NotEmpty(t, site.ID)

	got, err := repo.Get(site.ID)
	assert.NoError(t, err)
	if assert.NotNil(t, got) {
		assert.Equal(t, "repogap-site.com", got.Domain)
		assert.True(t, got.Enabled)
	}

	got.Name = "updated"
	assert.NoError(t, repo.Update(got))
	got2, _ := repo.Get(site.ID)
	assert.Equal(t, "updated", got2.Name)

	byDomain, err := repo.GetByDomain("repogap-site.com")
	assert.NoError(t, err)
	assert.NotNil(t, byDomain)

	list, err := repo.List()
	assert.NoError(t, err)
	assert.NotEmpty(t, list)

	// 不存在的域名 → nil,nil
	none, err := repo.GetByDomain("repogap-none.com")
	assert.NoError(t, err)
	assert.Nil(t, none)

	assert.NoError(t, repo.Delete(site.ID))
	gone, err := repo.Get(site.ID)
	assert.NoError(t, err)
	assert.Nil(t, gone)

	// 清理
	_ = cl.SetRemove("sites", site.ID)
}

func TestGapSiteRepository_ErrorBranches(t *testing.T) {
	cl := newGapRepoClient(t)
	repo := NewSiteRepository(cl)

	// Create：HashSetAll 失败
	siteErr1 := &models.Site{Domain: "gap-err1.com"}
	injectGapFailure(cl, "hset", "site:", 1)
	err := repo.Create(siteErr1)
	assert.Contains(t, err.Error(), "failed to save site")

	// Create：Set 域名映射失败
	siteErr2 := &models.Site{Domain: "gap-err2.com"}
	injectGapFailure(cl, "set", "domain:", 1)
	err = repo.Create(siteErr2)
	assert.Contains(t, err.Error(), "failed to save domain mapping")

	// Create：SetAdd 失败
	siteErr3 := &models.Site{Domain: "gap-err3.com"}
	injectGapFailure(cl, "sadd", "", 1)
	err = repo.Create(siteErr3)
	assert.Contains(t, err.Error(), "failed to add site to list")

	// 正常建站供后续分支使用
	site := &models.Site{Domain: "gap-err-main.com", Name: "m", Enabled: true}
	assert.NoError(t, repo.Create(site))
	siteKey := "site:" + site.ID

	// Update：HashSetAll 失败
	injectGapFailure(cl, "hset", "site:", 1)
	err = repo.Update(site)
	assert.Contains(t, err.Error(), "failed to update site")

	// Update：Set 域名映射失败
	injectGapFailure(cl, "set", "domain:", 1)
	err = repo.Update(site)
	assert.Contains(t, err.Error(), "failed to update domain mapping")

	// Delete：Get（HashGetAll）失败
	injectGapFailure(cl, "hgetall", "site:", 1)
	err = repo.Delete(site.ID)
	assert.Contains(t, err.Error(), "failed to get site")

	// Delete：Del 域名映射失败
	injectGapFailure(cl, "del", "domain:", 1)
	err = repo.Delete(site.ID)
	assert.Contains(t, err.Error(), "failed to delete domain mapping")

	// Delete：SetRemove 失败
	injectGapFailure(cl, "srem", "", 1)
	err = repo.Delete(site.ID)
	assert.Contains(t, err.Error(), "failed to remove site from list")

	// Delete：DeleteSiteData（SCAN）失败
	injectGapFailure(cl, "scan", "", 1)
	err = repo.Delete(site.ID)
	assert.Contains(t, err.Error(), "failed to delete site data")

	// List：集合成员的站点读取失败 → 跳过（continue）
	assert.NoError(t, cl.SetAdd("sites", "gap-ghost-id"))
	injectGapFailure(cl, "hgetall", "site:", 1)
	list, err := repo.List()
	assert.NoError(t, err)
	assert.Empty(t, list)
	_ = cl.SetRemove("sites", "gap-ghost-id")

	// GetByDomain：Get 失败
	injectGapFailure(cl, "get", "domain:", 1)
	_, err = repo.GetByDomain("gap-err-main.com")
	assert.Contains(t, err.Error(), "failed to get site ID by domain")

	// 清理
	_ = cl.SetRemove("sites", site.ID)
	_ = cl.DelMultiple([]string{siteKey, "site:" + siteErr2.ID, "domain:gap-err2.com"})
}

// ---------- RedisClientWrapper（WafRedisClient 真实实现） ----------

func TestGapRedisClientWrapper(t *testing.T) {
	cl := newGapRepoClient(t)
	w := &RedisClientWrapper{Client: cl}
	ctx := w.Context()
	assert.NotNil(t, ctx)

	// 字符串
	assert.NoError(t, w.Set(ctx, gapPrefix+"w", "v", time.Minute))
	v, err := w.Get(ctx, gapPrefix+"w")
	assert.NoError(t, err)
	assert.Equal(t, "v", v)

	// 列表
	assert.NoError(t, w.LPush(ctx, gapPrefix+"l", "a"))
	assert.NoError(t, w.LPush(ctx, gapPrefix+"l", "b"))
	n, err := w.LLen(ctx, gapPrefix+"l")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), n)
	vals, err := w.LRange(ctx, gapPrefix+"l", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, vals)
	assert.NoError(t, w.LTrim(ctx, gapPrefix+"l", 0, 0))
	vals, _ = w.LRange(ctx, gapPrefix+"l", 0, -1)
	assert.Equal(t, []string{"b"}, vals)

	// 哈希与计数
	assert.NoError(t, w.HIncrBy(ctx, gapPrefix+"h", "f", 2))
	assert.NoError(t, w.HIncrBy(ctx, gapPrefix+"h", "f", 3))
	m, err := w.HGetAll(ctx, gapPrefix+"h")
	assert.NoError(t, err)
	assert.Equal(t, "5", m["f"])
	assert.NoError(t, w.Incr(ctx, gapPrefix+"c"))

	// Expire
	assert.NoError(t, w.Expire(ctx, gapPrefix+"w", time.Minute))

	_ = cl.DelMultiple([]string{gapPrefix + "w", gapPrefix + "l", gapPrefix + "h", gapPrefix + "c"})
}

func TestGapRedisClientWrapper_ClosedClient(t *testing.T) {
	cl := newGapRepoClient(t)
	_ = cl.Close()
	w := &RedisClientWrapper{Client: cl}
	ctx := w.Context()

	_, err := w.Get(ctx, "k")
	assert.Error(t, err)
	assert.Error(t, w.Set(ctx, "k", "v", 0))
	assert.Error(t, w.LPush(ctx, "k", "v"))
	_, err = w.LRange(ctx, "k", 0, -1)
	assert.Error(t, err)
	_, err = w.LLen(ctx, "k")
	assert.Error(t, err)
	assert.Error(t, w.LTrim(ctx, "k", 0, -1))
	assert.Error(t, w.HIncrBy(ctx, "k", "f", 1))
	assert.Error(t, w.Incr(ctx, "k"))
	_, err = w.HGetAll(ctx, "k")
	assert.Error(t, err)
	assert.Error(t, w.Expire(ctx, "k", time.Minute))
}

// ---------- WafRepository（真实客户端） ----------

func TestGapWafRepository_ConfigAndIPLists(t *testing.T) {
	cl := newGapRepoClient(t)
	repo := NewWafRepository(&RedisClientWrapper{Client: cl})
	site := "gap-waf"
	site2 := "gap-waf2"
	site3 := "gap-waf3"
	cfgKey := "waf:config:" + site
	cfgKey2 := "waf:config:" + site2
	cfgKey3 := "waf:config:" + site3
	cfgKey4 := "waf:config:gap-waf4"
	cfgKey5 := "waf:config:gap-waf5"

	// 无配置
	cfg, err := repo.GetWafConfigBySiteID(site)
	assert.NoError(t, err)
	assert.Nil(t, cfg)

	// GetBlacklist / GetWhitelist：无配置 → 空列表
	black, err := repo.GetBlacklist(site)
	assert.NoError(t, err)
	assert.Empty(t, black)
	white, err := repo.GetWhitelist(site)
	assert.NoError(t, err)
	assert.Empty(t, white)

	// AddIPToBlacklist：无配置 → 新建配置（config==nil 分支）
	assert.NoError(t, repo.AddIPToBlacklist(site, "3.3.3.3"))
	black, _ = repo.GetBlacklist(site)
	assert.Equal(t, []string{"3.3.3.3"}, black)

	// 重复添加不生效
	assert.NoError(t, repo.AddIPToBlacklist(site, "3.3.3.3"))
	black, _ = repo.GetBlacklist(site)
	assert.Len(t, black, 1)

	// AddIPToWhitelist：已有配置 → 移入白名单并移出黑名单
	assert.NoError(t, repo.AddIPToWhitelist(site, "3.3.3.3"))
	white, _ = repo.GetWhitelist(site)
	assert.Equal(t, []string{"3.3.3.3"}, white)
	black, _ = repo.GetBlacklist(site)
	assert.Empty(t, black)

	// AddIPToWhitelist：无配置 → 新建配置（config==nil 分支）
	assert.NoError(t, repo.AddIPToWhitelist(site3, "4.4.4.4"))
	white, _ = repo.GetWhitelist(site3)
	assert.Equal(t, []string{"4.4.4.4"}, white)

	// Update 系列：无配置 → 新建配置（config==nil 分支，各自用全新 site）
	assert.NoError(t, repo.UpdateBlockedCountries(site2, []string{"CN", "RU"}))
	cfg2, err := repo.GetWafConfigBySiteID(site2)
	assert.NoError(t, err)
	assert.Len(t, cfg2.BlockedCountries, 2)
	site4 := "gap-waf4"
	site5 := "gap-waf5"
	assert.NoError(t, repo.UpdateIPWhitelist(site4, []string{"1.1.1.1"}))
	white, _ = repo.GetWhitelist(site4)
	assert.Equal(t, []string{"1.1.1.1"}, white)
	assert.NoError(t, repo.UpdateIPBlacklist(site5, []string{"9.9.9.9"}))
	black, _ = repo.GetBlacklist(site5)
	assert.Equal(t, []string{"9.9.9.9"}, black)

	// Update 系列：已有配置 → 整表替换
	assert.NoError(t, repo.UpdateBlockedCountries(site, []string{"RU"}))
	cfg, _ = repo.GetWafConfigBySiteID(site)
	assert.Len(t, cfg.BlockedCountries, 1)
	assert.NoError(t, repo.UpdateIPWhitelist(site, []string{"1.1.1.1", "2.2.2.2"}))
	white, _ = repo.GetWhitelist(site)
	assert.Len(t, white, 2)
	assert.NoError(t, repo.UpdateIPBlacklist(site, []string{"9.9.9.9"}))
	black, _ = repo.GetBlacklist(site)
	assert.Equal(t, []string{"9.9.9.9"}, black)

	// AddIPToWhitelist：把黑名单 IP 移回白名单
	assert.NoError(t, repo.AddIPToWhitelist(site, "9.9.9.9"))
	white, _ = repo.GetWhitelist(site)
	assert.Contains(t, white, "9.9.9.9")
	black, _ = repo.GetBlacklist(site)
	assert.NotContains(t, black, "9.9.9.9")

	// GetWafConfigBySiteID：非法 JSON → 错误
	assert.NoError(t, cl.Set(cfgKey, "not-json{", 0))
	_, err = repo.GetWafConfigBySiteID(site)
	assert.Error(t, err)
	_ = cl.Del(cfgKey)

	// Update 系列错误：Get 失败
	injectGapFailure(cl, "get", "waf:config:", 1)
	assert.Error(t, repo.UpdateBlockedCountries(site, []string{"US"}))
	injectGapFailure(cl, "get", "waf:config:", 1)
	assert.Error(t, repo.UpdateIPWhitelist(site, []string{"1.1.1.1"}))
	injectGapFailure(cl, "get", "waf:config:", 1)
	assert.Error(t, repo.UpdateIPBlacklist(site, []string{"1.1.1.1"}))

	// GetBlacklist / GetWhitelist 错误
	injectGapFailure(cl, "get", "waf:config:", 1)
	_, err = repo.GetBlacklist(site)
	assert.Error(t, err)
	injectGapFailure(cl, "get", "waf:config:", 1)
	_, err = repo.GetWhitelist(site)
	assert.Error(t, err)

	// AddIPToWhitelist / AddIPToBlacklist 错误
	injectGapFailure(cl, "get", "waf:config:", 1)
	assert.Error(t, repo.AddIPToWhitelist(site, "5.5.5.5"))
	injectGapFailure(cl, "get", "waf:config:", 1)
	assert.Error(t, repo.AddIPToBlacklist(site, "5.5.5.5"))

	// saveWafConfig：Set 失败（CreateWafConfig / UpdateWafConfig 往返）
	assert.NoError(t, repo.CreateWafConfig(&models.WafConfig{ID: "c1", SiteID: site, Enabled: true}))
	saved, err := repo.GetWafConfigBySiteID(site)
	assert.NoError(t, err)
	assert.NotNil(t, saved)
	injectGapFailure(cl, "set", "waf:config:", 1)
	assert.Error(t, repo.UpdateWafConfig(&models.WafConfig{ID: "c1", SiteID: site, Enabled: false}))

	_ = cl.DelMultiple([]string{cfgKey, cfgKey2, cfgKey3, cfgKey4, cfgKey5})
}

func TestGapWafRepository_LogsAndStats(t *testing.T) {
	cl := newGapRepoClient(t)
	repo := NewWafRepository(&RedisClientWrapper{Client: cl})
	site := "gap-waf-log"

	// CreateAccessLog：allow → 仅访问日志 + total 统计
	allowLog := &models.AccessLog{ID: "al1", SiteID: site, IPAddress: "1.1.1.1", Action: "allow", CreatedAt: time.Now()}
	assert.NoError(t, repo.CreateAccessLog(allowLog))

	// CreateAccessLog：block → 攻击日志 + blocked 统计（攻击日志 LTrim else 分支）
	blockLog := &models.AccessLog{ID: "bl1", SiteID: site, IPAddress: "2.2.2.2", Action: "block", CreatedAt: time.Now()}
	assert.NoError(t, repo.CreateAccessLog(blockLog))

	logs, total, err := repo.GetAccessLogs(site, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, logs, 2)
	atks, atkTotal, err := repo.GetAttackLogs(site, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), atkTotal)
	assert.Len(t, atks, 1)

	// 攻击日志推送失败：仅记录日志，主流程不受影响
	injectGapFailure(cl, "lpush", "waf:attacks:", 1)
	blockLog2 := &models.AccessLog{ID: "bl2", SiteID: site, IPAddress: "3.3.3.3", Action: "block", CreatedAt: time.Now()}
	assert.NoError(t, repo.CreateAccessLog(blockLog2))

	// 全局统计
	stats, err := repo.GetGlobalStats("", "")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, stats.TotalRequests, int64(3))
	assert.GreaterOrEqual(t, stats.BlockedRequests, int64(2))

	// GetAccessLogs：LRange 失败
	injectGapFailure(cl, "lrange", "waf:logs:", 1)
	_, _, err = repo.GetAccessLogs(site, 1, 10)
	assert.Error(t, err)

	// GetAttackLogs：LRange 失败
	injectGapFailure(cl, "lrange", "waf:attacks:", 1)
	_, _, err = repo.GetAttackLogs(site, 1, 10)
	assert.Error(t, err)

	// 日志含非法 JSON 条目 → 跳过（al1/bl1/bl2 主日志均推送成功，加 1 条非法 JSON 共 4 条）
	assert.NoError(t, cl.ListPush("waf:logs:"+site, "not-json{"))
	logs, total, err = repo.GetAccessLogs(site, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(4), total)
	assert.Len(t, logs, 3)

	// GetTrafficStats：起止时间解析错误
	_, err = repo.GetTrafficStats("bad-time", "2026-08-31T00:00:00Z")
	assert.Error(t, err)
	_, err = repo.GetTrafficStats("2026-08-31T00:00:00Z", "bad-time")
	assert.Error(t, err)

	// GetTrafficStats：HGetAll 失败 → continue（首个小时失败，次小时成功）
	injectGapFailure(cl, "hgetall", "waf:stats:hourly:", 1)
	hour10, _ := time.Parse(time.RFC3339, "2026-08-31T10:00:00Z")
	hour11, _ := time.Parse(time.RFC3339, "2026-08-31T11:00:00Z")
	hour12, _ := time.Parse(time.RFC3339, "2026-08-31T12:00:00Z")
	data, err := repo.GetTrafficStats("2026-08-31T10:00:00Z", "2026-08-31T12:00:00Z")
	assert.NoError(t, err)
	assert.Len(t, data, 2) // 10 点被跳过，11/12 点返回

	// GetTrafficStats：多小时数据 → 排序分支
	assert.NoError(t, cl.HashSetAll(fmt.Sprintf("waf:stats:hourly:%d", hour10.Unix()), map[string]interface{}{"total": "5", "blocked": "1"}))
	assert.NoError(t, cl.HashSetAll(fmt.Sprintf("waf:stats:hourly:%d", hour11.Unix()), map[string]interface{}{"total": "7", "blocked": "3"}))
	data, err = repo.GetTrafficStats("2026-08-31T10:00:00Z", "2026-08-31T12:00:00Z")
	assert.NoError(t, err)
	assert.Len(t, data, 3)
	for i := 1; i < len(data); i++ {
		assert.Less(t, data[i-1]["time"].(string), data[i]["time"].(string)) // 升序
	}
	assert.Equal(t, int64(5), data[0]["totalRequests"].(int64))

	// 清理
	_ = cl.DelMultiple([]string{
		"waf:logs:" + site, "waf:attacks:" + site,
		"waf:stats:global:total", "waf:stats:global:blocked",
		fmt.Sprintf("waf:stats:hourly:%d", hour10.Unix()),
		fmt.Sprintf("waf:stats:hourly:%d", hour11.Unix()),
		fmt.Sprintf("waf:stats:hourly:%d", hour12.Unix()),
	})
}

// ---------- WafRepositoryInMemory（剩余分支） ----------

func TestGapWafRepositoryInMemory_GapBranches(t *testing.T) {
	repo := NewWafRepositoryInMemory()

	// GetAttackLogs：起始页超出范围 → 空结果（站点存在日志但页码越界）
	repo.attackLogs["s"] = []models.AccessLog{{ID: "atk1", SiteID: "s"}, {ID: "atk2", SiteID: "s"}}
	_, total, err := repo.GetAttackLogs("s", 10, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), total)

	// AddIPToWhitelist：黑名单过滤循环（保留其他 IP）
	repo.ipBlacklists["s"] = []models.IPBlacklist{{IPAddress: "8.8.8.8"}, {IPAddress: "1.2.3.4"}}
	assert.NoError(t, repo.AddIPToWhitelist("s", "1.2.3.4"))
	black, err := repo.GetBlacklist("s")
	assert.NoError(t, err)
	assert.Equal(t, []string{"8.8.8.8"}, black)

	// AddIPToBlacklist：白名单过滤循环（保留其他 IP）
	repo.ipWhitelists["s"] = []models.IPWhitelist{{IPAddress: "7.7.7.7"}, {IPAddress: "5.6.7.8"}}
	assert.NoError(t, repo.AddIPToBlacklist("s", "5.6.7.8"))
	white, err := repo.GetWhitelist("s")
	assert.NoError(t, err)
	assert.Equal(t, []string{"7.7.7.7"}, white)

	// GetBlacklist / GetWhitelist 最终状态（黑名单：剩余 8.8.8.8 + 新加入的 5.6.7.8）
	black, err = repo.GetBlacklist("s")
	assert.NoError(t, err)
	assert.Equal(t, []string{"8.8.8.8", "5.6.7.8"}, black)
	white, err = repo.GetWhitelist("s")
	assert.NoError(t, err)
	assert.Equal(t, []string{"7.7.7.7"}, white)
}
