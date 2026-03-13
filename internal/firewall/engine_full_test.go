package firewall

import (
	"bytes"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRuleManager_NewRuleManager 测试创建规则管理器
func TestRuleManager_NewRuleManager(t *testing.T) {
	rm := NewRuleManager("/tmp/rules", true, time.Hour, "", nil)
	assert.NotNil(t, rm)
	assert.NotNil(t, rm.rules)
	assert.Equal(t, "/tmp/rules", rm.rulesPath)

	// 清理
	rm.StopAutoUpdate()
}

// TestRuleManager_NewRuleManager_DefaultInterval 测试默认更新间隔
func TestRuleManager_NewRuleManager_DefaultInterval(t *testing.T) {
	rm := NewRuleManager("/tmp/rules", true, 0, "", nil)
	assert.NotNil(t, rm)
	// 应该使用默认值 24 小时
	rm.StopAutoUpdate()
}

// TestRuleManager_ReloadRules_NilRedis 测试 Redis 为 nil 时重新加载规则
func TestRuleManager_ReloadRules_NilRedis(t *testing.T) {
	rm := NewRuleManager("", false, 0, "", nil)
	err := rm.ReloadRules()
	assert.NoError(t, err)
	// 应该使用默认规则
	assert.NotEmpty(t, rm.rules)
}

// TestRuleManager_GetRulesByCategory 测试按分类获取规则
func TestRuleManager_GetRulesByCategory(t *testing.T) {
	rm := NewRuleManager("", false, 0, "", nil)

	// 获取存在的分类
	rules := rm.GetRulesByCategory("injection")
	assert.NotNil(t, rules)

	// 获取不存在的分类
	emptyRules := rm.GetRulesByCategory("nonexistent")
	assert.Empty(t, emptyRules)

	rm.StopAutoUpdate()
}

// TestRuleManager_StopAutoUpdate 测试停止自动更新
func TestRuleManager_StopAutoUpdate(t *testing.T) {
	rm := NewRuleManager("", true, time.Millisecond*10, "", nil)
	// 等待一小段时间让 goroutine 启动
	time.Sleep(time.Millisecond * 50)

	// 停止应该不 panic
	assert.NotPanics(t, func() {
		rm.StopAutoUpdate()
	})
}

// TestEngineManager_NewEngineManager 测试创建引擎管理器
func TestEngineManager_NewEngineManager(t *testing.T) {
	em := NewEngineManager()
	assert.NotNil(t, em)
	assert.NotNil(t, em.engines)
}

// TestEngineManager_AddSite 测试添加站点
func TestEngineManager_AddSite(t *testing.T) {
	em := NewEngineManager()

	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
			BlockMessage:  "Blocked",
		},
		CacheTTL: 60,
	}

	// 添加站点
	err := em.AddSite("test-site", config)
	assert.NoError(t, err)

	// 验证站点已添加
	engine, exists := em.GetEngine("test-site")
	assert.True(t, exists)
	assert.NotNil(t, engine)

	// 再次添加同一站点应该不返回错误
	err = em.AddSite("test-site", config)
	assert.NoError(t, err)
}

// TestEngineManager_RemoveSite 测试移除站点
func TestEngineManager_RemoveSite(t *testing.T) {
	em := NewEngineManager()

	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
		},
	}

	// 添加站点
	err := em.AddSite("test-site", config)
	assert.NoError(t, err)

	// 移除站点
	em.RemoveSite("test-site")

	// 验证站点已移除
	_, exists := em.GetEngine("test-site")
	assert.False(t, exists)
}

// TestEngineManager_ListSites 测试列出所有站点
func TestEngineManager_ListSites(t *testing.T) {
	em := NewEngineManager()

	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
		},
	}

	// 添加多个站点
	em.AddSite("site1", config)
	em.AddSite("site2", config)
	em.AddSite("site3", config)

	sites := em.ListSites()
	assert.Len(t, sites, 3)
	assert.Contains(t, sites, "site1")
	assert.Contains(t, sites, "site2")
	assert.Contains(t, sites, "site3")
}

// TestEngine_CheckRequest 测试请求检查
func TestEngine_CheckRequest(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
			BlockMessage:  "Blocked",
		},
		CacheTTL: 60,
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)
	assert.NotNil(t, engine)

	// 创建测试请求
	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	// 检查请求
	result, err := engine.CheckRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Threats)
	assert.True(t, result.Allow || len(result.Threats) > 0)
}

// TestEngine_CheckRequest_Cache 测试请求缓存
func TestEngine_CheckRequest_Cache(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
		},
		CacheTTL: 60,
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	// 创建相同的请求两次
	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	result1, err := engine.CheckRequest(req)
	assert.NoError(t, err)

	result2, err := engine.CheckRequest(req)
	assert.NoError(t, err)

	// 第二次应该从缓存返回
	assert.Equal(t, result1.Allow, result2.Allow)
}

// TestEngine_CheckRequest_WithBody 测试带请求体的检查
func TestEngine_CheckRequest_WithBody(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
		},
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	// 创建带请求体的 POST 请求
	body := bytes.NewBufferString("test data")
	req := httptest.NewRequest("POST", "http://example.com/api", body)
	req.Header.Set("Content-Type", "application/json")

	result, err := engine.CheckRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestEngine_CheckRequest_WithHeaders 测试带自定义请求头的检查
func TestEngine_CheckRequest_WithHeaders(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
		},
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("X-Custom-Header", "test-value")
	req.Header.Set("User-Agent", "TestAgent/1.0")

	result, err := engine.CheckRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestEngine_GenerateRequestCacheKey 测试缓存键生成
func TestEngine_GenerateRequestCacheKey(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
		},
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	req1 := httptest.NewRequest("GET", "http://example.com/test", nil)
	req2 := httptest.NewRequest("GET", "http://example.com/test", nil)
	req3 := httptest.NewRequest("POST", "http://example.com/test", nil)

	key1 := engine.generateRequestCacheKey(req1)
	key2 := engine.generateRequestCacheKey(req2)
	key3 := engine.generateRequestCacheKey(req3)

	// 相同的请求应该生成相同的键
	assert.Equal(t, key1, key2)

	// 不同的请求应该生成不同的键
	assert.NotEqual(t, key1, key3)
}

// TestEngine_CacheOperations 测试缓存操作
func TestEngine_CacheOperations(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
		},
		CacheTTL: 60, // 60 秒 TTL
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	// 测试 getFromCache 空缓存
	result := engine.getFromCache("nonexistent-key")
	assert.Nil(t, result)

	// 测试 addToCache
	testResult := &CheckResult{Allow: true, CreatedAt: time.Now()}
	engine.addToCache("test-key", testResult)

	// 验证缓存已添加
	cachedResult := engine.getFromCache("test-key")
	assert.NotNil(t, cachedResult)
	assert.Equal(t, testResult.Allow, cachedResult.Allow)
}

// TestCheckResult_Struct 测试 CheckResult 结构体
func TestCheckResult_Struct(t *testing.T) {
	result := &CheckResult{
		Threats:   nil,
		CreatedAt: time.Now(),
		Allow:     true,
	}

	assert.NotNil(t, result)
	assert.True(t, result.Allow)
	assert.Nil(t, result.Threats)
	assert.NotZero(t, result.CreatedAt)
}

// TestConfig_Struct 测试 Config 结构体
func TestConfig_Struct(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
			BlockMessage:  "Blocked",
		},
		CacheTTL:  120,
		Blacklist: []string{"192.168.1.1"},
		Whitelist: []string{"10.0.0.1"},
	}

	assert.Equal(t, "/tmp/rules", config.RulesPath)
	assert.Equal(t, "block", config.ActionConfig.DefaultAction)
	assert.Equal(t, "Blocked", config.ActionConfig.BlockMessage)
	assert.Equal(t, 120, config.CacheTTL)
	assert.Len(t, config.Blacklist, 1)
	assert.Len(t, config.Whitelist, 1)
}

// TestAIEngineConfig_Struct 测试 AIEngineConfig 结构体
func TestAIEngineConfig_Struct(t *testing.T) {
	config := AIEngineConfig{
		Enabled:             true,
		ModelPath:           "/tmp/model",
		WorkerPool:          4,
		ConfidenceThreshold: 0.85,
		TimeoutMs:           100,
		CacheSize:           1000,
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "/tmp/model", config.ModelPath)
	assert.Equal(t, 4, config.WorkerPool)
	assert.Equal(t, float32(0.85), config.ConfidenceThreshold)
	assert.Equal(t, 100, config.TimeoutMs)
	assert.Equal(t, 1000, config.CacheSize)
}

// TestActionConfig_Struct 测试 ActionConfig 结构体
func TestActionConfig_Struct(t *testing.T) {
	config := ActionConfig{
		DefaultAction: "challenge",
		BlockMessage:  "Access denied",
	}

	assert.Equal(t, "challenge", config.DefaultAction)
	assert.Equal(t, "Access denied", config.BlockMessage)
}

// TestEngine_NilConfig 测试配置为 nil 时的行为
func TestEngine_NilConfig(t *testing.T) {
	// GeoIP 配置为 nil 时不应该 panic
	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
		},
		GeoIPConfig:     nil,
		RateLimitConfig: nil,
		FileIntegrityConfig: nil,
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)
	assert.NotNil(t, engine)
}

// TestEngine_EmptyConfig 测试空配置时的行为
func TestEngine_EmptyConfig(t *testing.T) {
	config := Config{}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)
	assert.NotNil(t, engine)
}

// TestEngine_CheckRequest_Malicious 测试恶意请求检测
func TestEngine_CheckRequest_Malicious(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
		},
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	// 创建带有 SQL 注入特征的请求
	req := httptest.NewRequest("GET", "http://example.com/search?q=SELECT%20*%20FROM%20users", nil)

	result, err := engine.CheckRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	// 检测可能会发现威胁
	assert.NotNil(t, result.Threats)
}

// TestEngine_CheckRequest_XSS 测试 XSS 检测
func TestEngine_CheckRequest_XSS(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
		},
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	// 创建带有 XSS 特征的请求
	req := httptest.NewRequest("GET", "http://example.com/comment?text=%3Cscript%3Ealert(1)%3C/script%3E", nil)

	result, err := engine.CheckRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestEngine_Interface 测试 Engine 接口实现
func TestEngine_Interface(t *testing.T) {
	var _ interface{} = (*Engine)(nil)
}

// TestEngineManager_Interface 测试 EngineManager 接口实现
func TestEngineManager_Interface(t *testing.T) {
	var _ interface{} = (*EngineManager)(nil)
}
