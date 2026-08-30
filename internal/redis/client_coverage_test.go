package redis

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------- 测试辅助 ----------

const covTestPrefix = "cov:"

// newCovClient 创建直连 DB15 的测试客户端
func newCovClient(t *testing.T) *Client {
	t.Helper()
	cl, err := NewClient("localhost:6379", "", 15)
	if err != nil {
		t.Skipf("Redis not available at localhost:6379: %v", err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	return cl
}

// newClosedClient 创建已关闭的客户端，用于触发 "client is closed" 错误分支
func newClosedClient(t *testing.T) *Client {
	t.Helper()
	cl := newCovClient(t)
	_ = cl.Close()
	return cl
}

// newCBOpenClient 创建熔断器已打开的客户端，用于覆盖各方法的熔断分支
func newCBOpenClient(t *testing.T) *Client {
	t.Helper()
	cl := newCovClient(t)
	cl.circuitBreaker = NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          time.Hour,
	})
	cl.circuitBreaker.RecordFailure()
	return cl
}

// ---------- 构造函数与辅助方法 ----------

func TestCovNewClientWithPool(t *testing.T) {
	cl, err := NewClientWithPool("localhost:6379", "", 15, PoolConfig{
		MaxActive:   5,
		MaxIdle:     2,
		IdleTimeout: time.Minute,
		PoolTimeout: time.Second,
	})
	assert.NoError(t, err)
	assert.NotNil(t, cl)
	if cl != nil {
		assert.NoError(t, cl.Set(covTestPrefix+"pool", "ok", time.Minute))
		val, err := cl.Get(covTestPrefix + "pool")
		assert.NoError(t, err)
		assert.Equal(t, "ok", val)
		_ = cl.Del(covTestPrefix + "pool")
		_ = cl.Close()
	}
}

func TestCovNewClientWithFullConfig_PingError(t *testing.T) {
	_, err := NewClientWithFullConfig("localhost:1", "", 0, DefaultPoolConfig(), DefaultCircuitBreakerConfig())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to redis")
}

func TestCovNewClientWithURLAndConfig_ParseError(t *testing.T) {
	_, err := NewClientWithURLAndConfig("://invalid-url", DefaultCircuitBreakerConfig())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse redis url")
}

func TestCovNewClientWithURLAndConfig_ConnectError(t *testing.T) {
	_, err := NewClientWithURLAndConfig("redis://localhost:1/0", DefaultCircuitBreakerConfig())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to redis")
}

func TestCovGetCircuitBreaker(t *testing.T) {
	cl := newCovClient(t)
	cb := cl.GetCircuitBreaker()
	assert.NotNil(t, cb)
	assert.Equal(t, StateClosed, cb.State())
}

// ---------- 熔断器 String / Allow 未知状态分支 ----------

func TestCovCircuitBreakerState_String(t *testing.T) {
	assert.Equal(t, "closed", StateClosed.String())
	assert.Equal(t, "open", StateOpen.String())
	assert.Equal(t, "half-open", StateHalfOpen.String())
	assert.Equal(t, "unknown", CircuitBreakerState(99).String())
}

func TestCovCircuitBreaker_AllowUnknownState(t *testing.T) {
	// 构造非法状态值，覆盖 Allow 的 default 分支（同包白盒测试）
	cb := &CircuitBreaker{state: CircuitBreakerState(42)}
	assert.False(t, cb.Allow())
}

// ---------- 熔断器打开时全方法扫描 ----------

func TestCovClient_CircuitBreakerOpen_AllMethods(t *testing.T) {
	cl := newCBOpenClient(t)

	cases := map[string]func() error{
		"Set":                       func() error { return cl.Set("k", "v", 0) },
		"Get":                       func() error { _, err := cl.Get("k"); return err },
		"Del":                       func() error { return cl.Del("k") },
		"ZAdd":                      func() error { return cl.ZAdd("k", 1, "m") },
		"ZRevRange":                 func() error { _, err := cl.ZRevRange("k", 0, -1); return err },
		"ZRemRangeByScore":          func() error { _, err := cl.ZRemRangeByScore("k", "0", "100"); return err },
		"Exists":                    func() error { _, err := cl.Exists("k"); return err },
		"HashSet":                   func() error { return cl.HashSet("k", "f", "v") },
		"HashGet":                   func() error { _, err := cl.HashGet("k", "f"); return err },
		"HashIncrBy":                func() error { _, err := cl.HashIncrBy("k", "f", 1); return err },
		"HashGetAll":                func() error { _, err := cl.HashGetAll("k"); return err },
		"HashSetAll":                func() error { return cl.HashSetAll("k", map[string]interface{}{"f": "v"}) },
		"ListPush":                  func() error { return cl.ListPush("k", "v") },
		"ListPop":                   func() error { _, err := cl.ListPop("k"); return err },
		"ListRange":                 func() error { _, err := cl.ListRange("k", 0, -1); return err },
		"ListLength":                func() error { _, err := cl.ListLength("k"); return err },
		"GetPushLogCount":           func() error { _, err := cl.GetPushLogCount("s"); return err },
		"SetAdd":                    func() error { return cl.SetAdd("k", "m") },
		"SetMembers":                func() error { _, err := cl.SetMembers("k"); return err },
		"SetContains":               func() error { _, err := cl.SetContains("k", "m"); return err },
		"SetRemove":                 func() error { return cl.SetRemove("k", "m") },
		"Incr":                      func() error { _, err := cl.Incr("k"); return err },
		"Keys":                      func() error { _, err := cl.Keys("k*"); return err },
		"DelMultiple":               func() error { return cl.DelMultiple([]string{"k"}) },
		"Decr":                      func() error { _, err := cl.Decr("k"); return err },
		"Expire":                    func() error { return cl.Expire("k", time.Minute) },
		"TTL":                       func() error { _, err := cl.TTL("k"); return err },
		"Publish":                   func() error { return cl.Publish("ch", "msg") },
		"GetURLCount":               func() error { _, err := cl.GetURLCount("s"); return err },
		"IsPreheatRunning":          func() error { _, err := cl.IsPreheatRunning("s"); return err },
		"GetCurrentPreheatTask":     func() error { _, err := cl.GetCurrentPreheatTask("s"); return err },
		"UpdatePreheatTaskProgress": func() error { return cl.UpdatePreheatTaskProgress("t", 1) },
		"GetPushOffset":             func() error { _, err := cl.GetPushOffset("s"); return err },
		"GetPushLogs":               func() error { _, err := cl.GetPushLogs("s", 10, 0); return err },
		"DeleteSiteData":            func() error { return cl.DeleteSiteData("s") },
	}

	for name, fn := range cases {
		err := fn()
		assert.Equal(t, ErrCircuitBreakerOpen, err, "method %s should fail with open circuit breaker", name)
	}

	// Subscribe 在熔断时返回 nil
	assert.Nil(t, cl.Subscribe("ch"))

	// 间接调用链同样被熔断拦截
	assert.Error(t, cl.AddURL("s", "u"))
	assert.Error(t, cl.RemoveURL("s", "u"))
	_, err := cl.GetURLs("s")
	assert.Error(t, err)
	assert.Error(t, cl.ClearURLs("s"))
	assert.Error(t, cl.SetURLPreheatStatus("s", "u", "st"))
	assert.Error(t, cl.SetPreheatRunning("s", true))
	assert.Error(t, cl.SetPushOffset("s", 1))
	assert.Error(t, cl.SetLastPushDate("s", "d"))
	assert.Error(t, cl.IncrDailyPushCount("s"))
	assert.Error(t, cl.IncrPushStats("s", "success"))
	assert.Error(t, cl.AddPushLog("s", "log"))
	_, err = cl.GetURLPreheatStatus("s", "u")
	assert.Error(t, err)
	assert.Error(t, cl.SetSiteStats("s", map[string]interface{}{"a": 1}))
	_, err = cl.GetSiteStats("s")
	assert.Error(t, err)
	_, err = cl.GetCacheCount()
	assert.Error(t, err)
	assert.Error(t, cl.ClearCache())
	assert.Error(t, cl.CreatePreheatTask("t", map[string]interface{}{}))
	assert.Error(t, cl.SetPushTask("s", map[string]interface{}{}))
	assert.Error(t, cl.SaveUser("u1", map[string]interface{}{"username": "n"}))
	assert.Error(t, cl.SaveUserWithCredentials("u1", "n", "p"))
	_, err = cl.GetUser("u1")
	assert.Error(t, err)
	_, err = cl.GetUserByUsername("n")
	assert.Error(t, err)
	assert.Error(t, cl.SaveSession("sess", map[string]interface{}{}, time.Minute))
	err = cl.GetSession("sess", &map[string]interface{}{})
	assert.Error(t, err)
	_, err = cl.CheckSessionExists("sess")
	assert.Error(t, err)
	assert.Error(t, cl.DeleteSession("sess"))
	assert.Error(t, cl.SaveSystemConfig(map[string]interface{}{"k": "v"}))
	_, err = cl.GetSystemConfig()
	assert.Error(t, err)
	// GetPushStatsWithURLCounts 内部吞掉所有错误，熔断打开时仍返回零值统计
	stats, err := cl.GetPushStatsWithURLCounts("s")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), stats["urlCount"])
	_, err = cl.GetLast15DaysPushCount("s")
	assert.NoError(t, err) // 该方法不检查熔断器，直接读原始客户端
}

// ---------- 字符串方法组 ----------

func TestCovStringMethods(t *testing.T) {
	cl := newCovClient(t)
	key := covTestPrefix + "str"

	// Set / Get
	assert.NoError(t, cl.Set(key, "v1", time.Minute))
	val, err := cl.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, "v1", val)

	// Get 不存在的键 → 返回空串且无错误（redis.Nil 分支）
	val, err = cl.Get(covTestPrefix + "missing")
	assert.NoError(t, err)
	assert.Equal(t, "", val)

	// Del
	assert.NoError(t, cl.Del(key))
	ok, err := cl.Exists(key)
	assert.NoError(t, err)
	assert.False(t, ok)

	// Exists 存在分支
	assert.NoError(t, cl.Set(key, "v2", 0))
	ok, err = cl.Exists(key)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, cl.Del(key))

	// Incr / Decr
	n, err := cl.Incr(covTestPrefix + "counter")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n)
	n, err = cl.Decr(covTestPrefix + "counter")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), n)
	assert.NoError(t, cl.Del(covTestPrefix+"counter"))

	// Expire / TTL
	assert.NoError(t, cl.Set(key, "v3", 0))
	assert.NoError(t, cl.Expire(key, 2*time.Minute))
	ttl, err := cl.TTL(key)
	assert.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0))
	assert.NoError(t, cl.Del(key))

	// Keys（SCAN 遍历）
	assert.NoError(t, cl.Set(covTestPrefix+"k1", "a", time.Minute))
	assert.NoError(t, cl.Set(covTestPrefix+"k2", "b", time.Minute))
	keys, err := cl.Keys(covTestPrefix + "k*")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(keys), 2)
	assert.NoError(t, cl.DelMultiple([]string{covTestPrefix + "k1", covTestPrefix + "k2"}))

	// DelMultiple 空列表
	assert.NoError(t, cl.DelMultiple([]string{}))

	// Publish
	assert.NoError(t, cl.Publish(covTestPrefix+"chan", "hello"))

	// Subscribe / SubscribeWithContext
	pubsub := cl.Subscribe(covTestPrefix + "chan")
	assert.NotNil(t, pubsub)
	_ = pubsub.Close()

	pubsub = cl.SubscribeWithContext(context.Background(), covTestPrefix+"chan")
	assert.NotNil(t, pubsub)
	_ = pubsub.Close()
}

func TestCovStringMethods_ClosedClient(t *testing.T) {
	cl := newClosedClient(t)

	assert.Error(t, cl.Set("k", "v", 0))
	_, err := cl.Get("k")
	assert.Error(t, err)
	assert.Error(t, cl.Del("k"))
	_, err = cl.Exists("k")
	assert.Error(t, err)
	_, err = cl.Incr("k")
	assert.Error(t, err)
	_, err = cl.Decr("k")
	assert.Error(t, err)
	assert.Error(t, cl.Expire("k", time.Minute))
	_, err = cl.TTL("k")
	assert.Error(t, err)
	_, err = cl.Keys("k*")
	assert.Error(t, err)
	assert.Error(t, cl.DelMultiple([]string{"k1", "k2"}))
	assert.Error(t, cl.Publish("ch", "msg"))
}

// ---------- JSON 方法组 ----------

func TestCovJSONMethods(t *testing.T) {
	cl := newCovClient(t)
	key := covTestPrefix + "json"

	// SaveJSON 成功 → GetJSON 反序列化
	assert.NoError(t, cl.SaveJSON(key, map[string]string{"a": "b"}, time.Minute))
	var dest map[string]string
	assert.NoError(t, cl.GetJSON(key, &dest))
	assert.Equal(t, "b", dest["a"])

	// GetJSON 空数据分支（键不存在 → Get 返回 "" → 直接返回 nil）
	var dest2 map[string]string
	assert.NoError(t, cl.GetJSON(covTestPrefix+"json-missing", &dest2))
	assert.Nil(t, dest2)

	// GetJSON 非法 JSON → 反序列化错误
	assert.NoError(t, cl.Set(covTestPrefix+"json-bad", "not-json{", 0))
	var dest3 map[string]string
	assert.Error(t, cl.GetJSON(covTestPrefix+"json-bad", &dest3))
	assert.NoError(t, cl.Del(covTestPrefix+"json-bad"))

	// SaveJSON 序列化失败（NaN）
	err := cl.SaveJSON(covTestPrefix+"json-nan", math.NaN(), 0)
	assert.Error(t, err)
}

func TestCovGetJSON_EmptyData(t *testing.T) {
	cl := newCovClient(t)
	var dest map[string]string
	assert.NoError(t, cl.GetJSON(covTestPrefix+"json-empty", &dest))
	assert.Nil(t, dest)
}

// ---------- 哈希方法组 ----------

func TestCovHashMethods(t *testing.T) {
	cl := newCovClient(t)
	key := covTestPrefix + "hash"

	assert.NoError(t, cl.HashSet(key, "f1", "v1"))
	val, err := cl.HashGet(key, "f1")
	assert.NoError(t, err)
	assert.Equal(t, "v1", val)

	// HashGet 不存在的字段 → "" 无错误
	val, err = cl.HashGet(key, "no-such-field")
	assert.NoError(t, err)
	assert.Equal(t, "", val)

	// HashIncrBy
	n, err := cl.HashIncrBy(key, "count", 3)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), n)
	n, err = cl.HashIncrBy(key, "count", 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), n)

	// HashSetAll / HashGetAll
	assert.NoError(t, cl.HashSetAll(key, map[string]interface{}{"a": "1", "b": "2"}))
	all, err := cl.HashGetAll(key)
	assert.NoError(t, err)
	assert.Equal(t, "1", all["a"])
	assert.Equal(t, "2", all["b"])

	// HashGetAll 空哈希
	empty, err := cl.HashGetAll(covTestPrefix + "hash-missing")
	assert.NoError(t, err)
	assert.Empty(t, empty)

	assert.NoError(t, cl.Del(key))
}

func TestCovHashMethods_ClosedClient(t *testing.T) {
	cl := newClosedClient(t)

	assert.Error(t, cl.HashSet("k", "f", "v"))
	_, err := cl.HashGet("k", "f")
	assert.Error(t, err)
	_, err = cl.HashIncrBy("k", "f", 1)
	assert.Error(t, err)
	_, err = cl.HashGetAll("k")
	assert.Error(t, err)
	assert.Error(t, cl.HashSetAll("k", map[string]interface{}{"f": "v"}))
}

// ---------- 列表方法组 ----------

func TestCovListMethods(t *testing.T) {
	cl := newCovClient(t)
	key := covTestPrefix + "list"

	assert.NoError(t, cl.ListPush(key, "a", "b", "c"))
	length, err := cl.ListLength(key)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), length)

	val, err := cl.ListPop(key)
	assert.NoError(t, err)
	assert.Equal(t, "a", val)

	// ListPop 空列表 → "" 无错误
	val, err = cl.ListPop(covTestPrefix + "list-missing")
	assert.NoError(t, err)
	assert.Equal(t, "", val)

	vals, err := cl.ListRange(key, 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "c"}, vals)

	// GetPushLogCount
	count, err := cl.GetPushLogCount("cov-push-site")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
	assert.NoError(t, cl.ListPush("site:cov-push-site:push:logs", "log1"))
	count, err = cl.GetPushLogCount("cov-push-site")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	assert.NoError(t, cl.Del(key))
	assert.NoError(t, cl.Del("site:cov-push-site:push:logs"))
}

func TestCovListMethods_ClosedClient(t *testing.T) {
	cl := newClosedClient(t)

	assert.Error(t, cl.ListPush("k", "v"))
	_, err := cl.ListPop("k")
	assert.Error(t, err)
	_, err = cl.ListRange("k", 0, -1)
	assert.Error(t, err)
	_, err = cl.ListLength("k")
	assert.Error(t, err)
}

// ---------- 集合方法组 ----------

func TestCovSetMethods(t *testing.T) {
	cl := newCovClient(t)
	key := covTestPrefix + "set"

	assert.NoError(t, cl.SetAdd(key, "m1", "m2"))
	members, err := cl.SetMembers(key)
	assert.NoError(t, err)
	assert.Len(t, members, 2)

	ok, err := cl.SetContains(key, "m1")
	assert.NoError(t, err)
	assert.True(t, ok)
	ok, err = cl.SetContains(key, "m3")
	assert.NoError(t, err)
	assert.False(t, ok)

	assert.NoError(t, cl.SetRemove(key, "m1"))
	ok, err = cl.SetContains(key, "m1")
	assert.NoError(t, err)
	assert.False(t, ok)

	// URL 系列方法（基于集合）
	assert.NoError(t, cl.AddURL("cov-set-site", "http://example.com/1"))
	urls, err := cl.GetURLs("cov-set-site")
	assert.NoError(t, err)
	assert.Equal(t, []string{"http://example.com/1"}, urls)
	count, err := cl.GetURLCount("cov-set-site")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
	assert.NoError(t, cl.RemoveURL("cov-set-site", "http://example.com/1"))
	count, err = cl.GetURLCount("cov-set-site")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
	assert.NoError(t, cl.ClearURLs("cov-set-site"))

	assert.NoError(t, cl.Del(key))
}

func TestCovSetMethods_ClosedClient(t *testing.T) {
	cl := newClosedClient(t)

	assert.Error(t, cl.SetAdd("k", "m"))
	_, err := cl.SetMembers("k")
	assert.Error(t, err)
	_, err = cl.SetContains("k", "m")
	assert.Error(t, err)
	assert.Error(t, cl.SetRemove("k", "m"))
	_, err = cl.GetURLCount("s")
	assert.Error(t, err)
}

// ---------- 有序集合方法组 ----------

func TestCovZSetMethods(t *testing.T) {
	cl := newCovClient(t)
	key := covTestPrefix + "zset"

	assert.NoError(t, cl.ZAdd(key, 1, "a"))
	assert.NoError(t, cl.ZAdd(key, 2, "b"))
	assert.NoError(t, cl.ZAdd(key, 3, "c"))

	vals, err := cl.ZRevRange(key, 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []string{"c", "b", "a"}, vals)

	n, err := cl.ZRemRangeByScore(key, "-inf", "1")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n)

	vals, err = cl.ZRevRange(key, 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []string{"c", "b"}, vals)

	// 空区间
	n, err = cl.ZRemRangeByScore(key, "100", "200")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), n)

	assert.NoError(t, cl.Del(key))
}

func TestCovZSetMethods_ClosedClient(t *testing.T) {
	cl := newClosedClient(t)

	assert.Error(t, cl.ZAdd("k", 1, "m"))
	_, err := cl.ZRevRange("k", 0, -1)
	assert.Error(t, err)
	_, err = cl.ZRemRangeByScore("k", "0", "1")
	assert.Error(t, err)
}

// ---------- 预热状态方法组 ----------

func TestCovPreheatMethods(t *testing.T) {
	cl := newCovClient(t)

	assert.NoError(t, cl.SetURLPreheatStatus("cov-preheat-site", "http://example.com/x", "cached"))
	status, err := cl.GetURLPreheatStatusMap("cov-preheat-site", "http://example.com/x")
	assert.NoError(t, err)
	assert.Equal(t, "cached", status["status"])

	statusStr, err := cl.GetURLPreheatStatus("cov-preheat-site", "http://example.com/x")
	assert.NoError(t, err)
	assert.Equal(t, "cached", statusStr)

	// IsPreheatRunning true / false
	assert.NoError(t, cl.SetPreheatRunning("cov-preheat-site", true))
	running, err := cl.IsPreheatRunning("cov-preheat-site")
	assert.NoError(t, err)
	assert.True(t, running)
	assert.NoError(t, cl.SetPreheatRunning("cov-preheat-site", false))
	running, err = cl.IsPreheatRunning("cov-preheat-site")
	assert.NoError(t, err)
	assert.False(t, running)

	// GetCurrentPreheatTask 不存在的任务
	task, err := cl.GetCurrentPreheatTask("cov-preheat-site")
	assert.NoError(t, err)
	assert.Equal(t, "", task)

	// UpdatePreheatTaskProgress 成功
	assert.NoError(t, cl.UpdatePreheatTaskProgress("cov-task-1", 50))
	progress, err := cl.HashGet("task:preheat:cov-task-1", "progress")
	assert.NoError(t, err)
	assert.Equal(t, "50", progress)

	_ = cl.Del("site:cov-preheat-site:preheat:http://example.com/x")
	_ = cl.Del("site:cov-preheat-site:preheat:running")
	_ = cl.Del("task:preheat:cov-task-1")
}

func TestCovPreheatMethods_ClosedClient(t *testing.T) {
	cl := newClosedClient(t)

	_, err := cl.GetURLPreheatStatusMap("s", "u")
	assert.Error(t, err)
	_, err = cl.IsPreheatRunning("s")
	assert.Error(t, err)
	assert.Error(t, cl.UpdatePreheatTaskProgress("t", 1))
}

// ---------- 推送统计方法组 ----------

func TestCovPushMethods(t *testing.T) {
	cl := newCovClient(t)

	// SetPushOffset / GetPushOffset（合法数字）
	assert.NoError(t, cl.SetPushOffset("cov-push-site", 42))
	offset, err := cl.GetPushOffset("cov-push-site")
	assert.NoError(t, err)
	assert.Equal(t, int64(42), offset)

	// GetPushOffset 不存在 → 0
	offset, err = cl.GetPushOffset("cov-push-site-missing")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), offset)

	// GetPushOffset 值非数字 → Sscanf 失败 → 0
	// 注意 Set 的键必须是 GetPushOffset 内部拼出的 site:<siteID>:push:offset
	assert.NoError(t, cl.Set("site:"+covTestPrefix+"push:offset-bad:push:offset", "not-a-number", 0))
	offset, err = cl.GetPushOffset(covTestPrefix + "push:offset-bad")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), offset)
	assert.NoError(t, cl.Del("site:"+covTestPrefix+"push:offset-bad:push:offset"))

	// SetLastPushDate
	assert.NoError(t, cl.SetLastPushDate("cov-push-site", "2026-08-30"))

	// IncrDailyPushCount / IncrPushStats
	assert.NoError(t, cl.IncrDailyPushCount("cov-push-site"))
	assert.NoError(t, cl.IncrPushStats("cov-push-site", "success"))

	// IncrDailyPushCountWithCount / IncrPushStatsWithCount
	assert.NoError(t, cl.IncrDailyPushCountWithCount("cov-push-site", 3))
	assert.NoError(t, cl.IncrPushStatsWithCount("cov-push-site", "failed", 2))

	// AddPushLog / AddPushLogStruct / GetPushLogs
	assert.NoError(t, cl.AddPushLog("cov-push-site", "plain-log"))
	assert.NoError(t, cl.AddPushLogStruct("cov-push-site", map[string]string{"level": "info", "msg": "json-log"}))
	logs, err := cl.GetPushLogs("cov-push-site", 10, 0)
	assert.NoError(t, err)
	assert.Len(t, logs, 2)
	// JSON 日志解析为 map，纯文本保留为 string
	foundMap := false
	foundStr := false
	for _, l := range logs {
		switch v := l.(type) {
		case map[string]interface{}:
			foundMap = true
			assert.Equal(t, "json-log", v["msg"])
		case string:
			foundStr = true
		}
	}
	assert.True(t, foundMap)
	assert.True(t, foundStr)

	// GetPushStatsWithURLCounts
	assert.NoError(t, cl.AddURL("cov-push-site", "http://example.com/1"))
	stats, err := cl.GetPushStatsWithURLCounts("cov-push-site")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), stats["urlCount"])
	assert.Equal(t, int64(1), stats["successCount"])
	assert.Equal(t, int64(2), stats["failedCount"])
	assert.Equal(t, int64(3), stats["totalCount"])

	// GetLast15DaysPushCount
	counts, err := cl.GetLast15DaysPushCount("cov-push-site")
	assert.NoError(t, err)
	assert.Len(t, counts, 15)

	// 清理
	_ = cl.DelMultiple([]string{
		"site:cov-push-site:push:offset",
		"site:cov-push-site:push:last_date",
		"site:cov-push-site:push:daily_count",
		"site:cov-push-site:push:stats:success",
		"site:cov-push-site:push:stats:failed",
		"site:cov-push-site:push:logs",
		"site:cov-push-site:urls",
	})
}

func TestCovPushMethods_ClosedClient(t *testing.T) {
	cl := newClosedClient(t)

	assert.Error(t, cl.SetPushOffset("s", 1))
	assert.Error(t, cl.SetLastPushDate("s", "d"))
	assert.Error(t, cl.IncrDailyPushCountWithCount("s", 1))
	assert.Error(t, cl.IncrPushStatsWithCount("s", "success", 1))
	assert.Error(t, cl.AddPushLog("s", "log"))

	// GetPushStatsWithURLCounts：GetURLCount 失败时 urlCount 回退 0，不返回错误
	stats, err := cl.GetPushStatsWithURLCounts("s")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), stats["urlCount"])
}

func TestCovAddPushLogStruct_MarshalError(t *testing.T) {
	cl := newCovClient(t)
	// channel 无法 JSON 序列化
	assert.Error(t, cl.AddPushLogStruct("cov-push-site", make(chan int)))
}

// ---------- 用户与会话方法组 ----------

func TestCovUserAndSessionMethods(t *testing.T) {
	cl := newCovClient(t)

	// SaveUser 带 username
	assert.NoError(t, cl.SaveUser("cov-user-1", map[string]interface{}{
		"id":       "cov-user-1",
		"username": "covuser",
	}))
	// SaveUser 不带 username（跳过用户名映射）
	assert.NoError(t, cl.SaveUser("cov-user-2", map[string]interface{}{"id": "cov-user-2"}))

	user, err := cl.GetUser("cov-user-1")
	assert.NoError(t, err)
	assert.Equal(t, "cov-user-1", user["id"])

	uid, err := cl.GetUserByUsername("covuser")
	assert.NoError(t, err)
	assert.Equal(t, "cov-user-1", uid)

	// GetUserByUsername 不存在的用户
	uid, err = cl.GetUserByUsername("cov-no-such-user")
	assert.NoError(t, err)
	assert.Equal(t, "", uid)

	// SaveUserWithCredentials
	assert.NoError(t, cl.SaveUserWithCredentials("cov-user-3", "covuser3", "pwd"))
	user, err = cl.GetUser("cov-user-3")
	assert.NoError(t, err)
	assert.Equal(t, "covuser3", user["username"])

	// GetAllUsers
	users, err := cl.GetAllUsers()
	assert.NoError(t, err)
	assert.Contains(t, users, "cov-user-1")

	// 会话
	assert.NoError(t, cl.SaveSession("cov-session-1", map[string]interface{}{"user": "u"}, time.Minute))
	exists, err := cl.CheckSessionExists("cov-session-1")
	assert.NoError(t, err)
	assert.True(t, exists)
	var sess map[string]interface{}
	assert.NoError(t, cl.GetSession("cov-session-1", &sess))
	assert.Equal(t, "u", sess["user"])
	assert.NoError(t, cl.DeleteSession("cov-session-1"))
	exists, err = cl.CheckSessionExists("cov-session-1")
	assert.NoError(t, err)
	assert.False(t, exists)

	_ = cl.DelMultiple([]string{
		"user:cov-user-1", "user:cov-user-2", "user:cov-user-3",
		"username:covuser", "username:covuser3",
	})
}

func TestCovUserMethods_ClosedClient(t *testing.T) {
	cl := newClosedClient(t)

	assert.Error(t, cl.SaveUser("u1", map[string]interface{}{"username": "n"}))
	_, err := cl.GetAllUsers()
	assert.Error(t, err)
}

// ---------- 站点统计 / 缓存 / 系统配置 ----------

func TestCovSiteStatsMethods(t *testing.T) {
	cl := newCovClient(t)

	assert.NoError(t, cl.SetSiteStats("cov-stats-site", map[string]interface{}{
		"total_urls": 10,
		"cached":     "5",
	}))
	stats, err := cl.GetSiteStats("cov-stats-site")
	assert.NoError(t, err)
	assert.Equal(t, "10", stats["total_urls"])
	assert.Equal(t, "5", stats["cached"])

	_ = cl.Del("site:cov-stats-site:stats")
}

func TestCovCacheMethods(t *testing.T) {
	cl := newCovClient(t)

	// 有缓存键（cache:* 前缀）
	assert.NoError(t, cl.Set("cache:covtest:a", "1", time.Minute))
	assert.NoError(t, cl.Set("cache:covtest:b", "2", time.Minute))
	count, err := cl.GetCacheCount()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(2))
	assert.NoError(t, cl.ClearCache())
	count, err = cl.GetCacheCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestCovCacheMethods_EmptyDB(t *testing.T) {
	// 在空的 DB14 上验证 "无缓存键" 分支
	cl, err := NewClient("localhost:6379", "", 14)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	t.Cleanup(func() { _ = cl.Close() })

	// 确保该测试库无 cache:* 键
	keys, err := cl.Keys("cache:*")
	assert.NoError(t, err)
	if len(keys) > 0 {
		assert.NoError(t, cl.DelMultiple(keys))
	}

	assert.NoError(t, cl.ClearCache())
	count, err := cl.GetCacheCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestCovSystemConfig(t *testing.T) {
	cl := newCovClient(t)

	// 不存在 → 空 map
	cfg, err := cl.GetSystemConfig()
	assert.NoError(t, err)
	assert.Empty(t, cfg)

	// 保存后读取：字符串与数字值
	assert.NoError(t, cl.SaveSystemConfig(map[string]interface{}{
		"site_name":      "PrerenderShield",
		"max_concurrent": 5,
	}))
	cfg, err = cl.GetSystemConfig()
	assert.NoError(t, err)
	assert.Equal(t, "PrerenderShield", cfg["site_name"])
	assert.Equal(t, "5", cfg["max_concurrent"])

	// 非法 JSON → 错误
	assert.NoError(t, cl.Set("system:config", "not-json{", 0))
	_, err = cl.GetSystemConfig()
	assert.Error(t, err)

	// 受保护键拒绝写入
	assert.Error(t, cl.SaveSystemConfig(map[string]interface{}{"jwt_secret": "x"}))
	assert.Error(t, cl.SaveSystemConfig(map[string]interface{}{"redis_url": "x"}))
	assert.Error(t, cl.SaveSystemConfig(map[string]interface{}{"admin_password": "x"}))

	// 序列化失败（NaN）
	assert.Error(t, cl.SaveSystemConfig(map[string]interface{}{"bad": math.NaN()}))

	_ = cl.Del("system:config")
}

// ---------- DeleteSiteData ----------

func TestCovDeleteSiteData(t *testing.T) {
	cl := newCovClient(t)

	// 造出会被三个模式命中的键
	assert.NoError(t, cl.Set(covTestPrefix+"site-data:s1", "x", time.Minute))
	assert.NoError(t, cl.Set(covTestPrefix+"cache-s1", "x", time.Minute))

	// 用真实站点键验证删除
	assert.NoError(t, cl.Set("site:cov-delsite:main", "x", time.Minute))
	assert.NoError(t, cl.Set("cache:cov-delsite:k", "x", time.Minute))
	assert.NoError(t, cl.Set("task:preheat:cov-delsite:t", "x", time.Minute))

	assert.NoError(t, cl.DeleteSiteData("cov-delsite"))

	ok, err := cl.Exists("site:cov-delsite:main")
	assert.NoError(t, err)
	assert.False(t, ok)
	ok, err = cl.Exists("cache:cov-delsite:k")
	assert.NoError(t, err)
	assert.False(t, ok)
	ok, err = cl.Exists("task:preheat:cov-delsite:t")
	assert.NoError(t, err)
	assert.False(t, ok)

	// 无数据站点：Keys 无命中 → 直接成功
	assert.NoError(t, cl.DeleteSiteData("cov-delsite-empty"))

	_ = cl.Del(covTestPrefix + "site-data:s1")
	_ = cl.Del(covTestPrefix + "cache-s1")
}

func TestCovDeleteSiteData_ClosedClient(t *testing.T) {
	cl := newClosedClient(t)
	assert.Error(t, cl.DeleteSiteData("s"))
}

// ---------- GetPushLogs / GetPushOffset 边界补充 ----------

func TestCovGetPushLogs_EmptyList(t *testing.T) {
	cl := newCovClient(t)
	logs, err := cl.GetPushLogs("cov-no-logs", 10, 0)
	assert.NoError(t, err)
	assert.Empty(t, logs)
}

// ---------- 原始客户端与上下文 ----------

func TestCovClient_RawAndContext(t *testing.T) {
	cl := newCovClient(t)
	assert.NotNil(t, cl.GetRawClient())
	assert.NotNil(t, cl.Context())
}

// ---------- SaveJSON / SetPushTask / CreatePreheatTask ----------

func TestCovTaskMethods(t *testing.T) {
	cl := newCovClient(t)

	assert.NoError(t, cl.SetPushTask("cov-task-site", map[string]interface{}{"status": "pending"}))
	var task map[string]interface{}
	assert.NoError(t, cl.GetJSON("site:cov-task-site:push:task", &task))
	assert.Equal(t, "pending", task["status"])

	assert.NoError(t, cl.CreatePreheatTask("cov-task-2", map[string]interface{}{"urls": 3}))
	var t2 map[string]interface{}
	assert.NoError(t, cl.GetJSON("task:preheat:cov-task-2", &t2))
	assert.Equal(t, float64(3), t2["urls"])

	_ = cl.DelMultiple([]string{"site:cov-task-site:push:task", "task:preheat:cov-task-2"})
}

// ---------- GetURLPreheatStatusMap 依赖 Get 的错误透传 ----------

func TestCovGetPushOffset_ClosedClient(t *testing.T) {
	cl := newClosedClient(t)
	offset, err := cl.GetPushOffset("s")
	assert.NoError(t, err) // Get 出错时按 0 处理
	assert.Equal(t, int64(0), offset)
}
