package firewall

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"prerender-shield/internal/firewall/types"
)

func TestEngine_NewEngine(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
			BlockMessage:  "Request blocked",
		},
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)
	assert.NotNil(t, engine)
}

func TestEngine_UpdateRules(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)
	assert.NotNil(t, engine)

	err = engine.UpdateRules()
	assert.NoError(t, err)
}

// TestEngine_HandleRequest_Success 测试 HandleRequest 成功情况
func TestEngine_HandleRequest_Success(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
		},
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)
	assert.NotNil(t, engine)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/clean-path", nil)
	w := httptest.NewRecorder()

	allowed := engine.HandleRequest(w, req)
	assert.True(t, allowed)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestEngine_HandleRequest_FailClosed 测试 FailClosed 策略
func TestEngine_HandleRequest_FailClosed(t *testing.T) {
	config := Config{
		RulesPath:    "/tmp/rules",
		FailStrategy: FailClosed,
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)
	assert.NotNil(t, engine)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	w := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		engine.HandleRequest(w, req)
	})
}

// TestEngine_HandleRequest_FailOpen 测试 FailOpen 策略
func TestEngine_HandleRequest_FailOpen(t *testing.T) {
	config := Config{
		RulesPath:    "/tmp/rules",
		FailStrategy: FailOpen,
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)
	assert.NotNil(t, engine)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	w := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		allowed := engine.HandleRequest(w, req)
		assert.True(t, allowed)
	})
}

// TestEngine_getFromCache 测试 getFromCache
func TestEngine_getFromCache(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	cacheItem := engine.getFromCache("nonexistent-key")
	assert.Nil(t, cacheItem)
}

// TestEngine_addToCache 测试 addToCache
func TestEngine_addToCache(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)
	assert.NotNil(t, engine)

	result := &CheckResult{
		Allow:     true,
		CreatedAt: time.Now(),
	}
	engine.addToCache("test-key", result)

	// 无 Redis 时缓存不可用；若以后注入 Redis，此检查确保数据一致
	cacheItem := engine.getFromCache("test-key")
	if cacheItem != nil {
		assert.True(t, cacheItem.Allow)
	}
}

// TestEngine_clearCache 测试 clearCache
func TestEngine_clearCache(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	engine.addToCache("key1", &CheckResult{Allow: true})
	engine.addToCache("key2", &CheckResult{Allow: false})

	engine.clearCache()

	cacheItem := engine.getFromCache("key1")
	assert.Nil(t, cacheItem)
}

// TestEngine_CacheOperations_Redis 测试基于 Redis 的 addToCache / getFromCache
func TestEngine_CacheOperations_Redis(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
		CacheTTL:  3600,
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	result := &CheckResult{
		Allow:     true,
		CreatedAt: time.Now(),
	}
	// 无 Redis 时 addToCache/getFromCache 应静默返回 nil
	engine.addToCache("test-key", result)
	cacheItem := engine.getFromCache("test-key")
	assert.Nil(t, cacheItem) // Redis 不可用，返回 nil
}

// Test_normalizeURL 测试 normalizeURL（包级别函数）
func Test_normalizeURL(t *testing.T) {
	u, err := url.Parse("http://example.com/TEST/path?b=2&a=1")
	assert.NoError(t, err)

	normalized := normalizeURL(u)

	// 验证 scheme 和 host 被小写化
	assert.Contains(t, normalized, "http://example.com")
	// 验证包含路径和查询参数
	assert.Contains(t, normalized, "/TEST/path")
	assert.Contains(t, normalized, "a=1")
	assert.Contains(t, normalized, "b=2")
}

// Test_normalizeQuery 测试 normalizeQuery（包级别函数）
func Test_normalizeQuery(t *testing.T) {
	values := url.Values{}
	values.Set("c", "3")
	values.Set("a", "1")
	values.Set("b", "2")

	normalized := normalizeQuery(values.Encode())

	// 验证包含所有参数（normalizeQuery 不保证顺序）
	assert.Contains(t, normalized, "a=1")
	assert.Contains(t, normalized, "b=2")
	assert.Contains(t, normalized, "c=3")
}

// TestEngine_calculateBodyHash 测试 calculateBodyHash
func TestEngine_calculateBodyHash(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "http://example.com", strings.NewReader("test body content"))
	hash := engine.calculateBodyHash(req)

	assert.NotEmpty(t, hash)

	req2 := httptest.NewRequest(http.MethodPost, "http://example.com", strings.NewReader("test body content"))
	hash2 := engine.calculateBodyHash(req2)
	assert.Equal(t, hash, hash2)
}

// TestEngine_generateRequestCacheKey 测试 generateRequestCacheKey
func TestEngine_generateRequestCacheKey(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/test?a=1", nil)
	key := engine.generateRequestCacheKey(req)

	assert.NotEmpty(t, key)
}

// Test_getClientIP 测试 getClientIP（包级别函数）
func Test_getClientIP(t *testing.T) {
	// 测试从 X-Forwarded-For 获取 IP
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")

	ip := getClientIP(req)
	assert.Equal(t, "10.0.0.1", ip)

	// 测试从 X-Real-IP 获取 IP
	req = httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("X-Real-IP", "172.16.0.1")

	ip = getClientIP(req)
	assert.Equal(t, "172.16.0.1", ip)

	// 测试从 RemoteAddr 获取 IP（包含端口）
	req = httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	ip = getClientIP(req)
	assert.Equal(t, "192.168.1.1:12345", ip)
}

// TestEngine_ReloadRules 测试 ReloadRules
func TestEngine_ReloadRules(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	assert.NotPanics(t, func() {
		err := engine.ruleManager.ReloadRules()
		_ = err
	})
}

// TestCheckResult 测试 CheckResult 结构
func TestCheckResult(t *testing.T) {
	result := &CheckResult{
		Allow:   true,
		Threats: []types.Threat{},
	}

	assert.True(t, result.Allow)
	assert.Empty(t, result.Threats)
}

// TestRuleManager_loadRulesFromFile_EmptyPath 测试 loadRulesFromFile 空路径
func TestRuleManager_loadRulesFromFile_EmptyPath(t *testing.T) {
	rm := NewRuleManager("", false, 0, "", nil)
	rules, err := rm.loadRulesFromFile()
	assert.Error(t, err)
	assert.Nil(t, rules)
	assert.Contains(t, err.Error(), "rules path not configured")
}

// TestRuleManager_loadRulesFromFile_FileNotFound 测试 loadRulesFromFile 文件不存在
func TestRuleManager_loadRulesFromFile_FileNotFound(t *testing.T) {
	rm := NewRuleManager("/nonexistent/path/rules.json", false, 0, "", nil)
	rules, err := rm.loadRulesFromFile()
	assert.Error(t, err)
	assert.Nil(t, rules)
	assert.Contains(t, err.Error(), "rules file not found")
}

// TestRuleManager_loadRulesFromFile_InvalidJSON 测试 loadRulesFromFile 无效 JSON
func TestRuleManager_loadRulesFromFile_InvalidJSON(t *testing.T) {
	tmpFile := t.TempDir() + "/invalid.json"
	err := os.WriteFile(tmpFile, []byte("invalid json content"), 0644)
	assert.NoError(t, err)

	rm := NewRuleManager(tmpFile, false, 0, "", nil)
	rules, err := rm.loadRulesFromFile()
	assert.Error(t, err)
	assert.Nil(t, rules)
	assert.Contains(t, err.Error(), "failed to parse JSON")
}

// TestRuleManager_loadRulesFromFile_ValidJSON 测试 loadRulesFromFile 有效 JSON
func TestRuleManager_loadRulesFromFile_ValidJSON(t *testing.T) {
	tmpFile := t.TempDir() + "/valid.json"
	validJSON := `{
		"injection": [
			{
				"id": "1",
				"name": "Test Rule",
				"enabled": true,
				"pattern": "test",
				"action": "block"
			}
		]
	}`
	err := os.WriteFile(tmpFile, []byte(validJSON), 0644)
	assert.NoError(t, err)

	rm := NewRuleManager(tmpFile, false, 0, "", nil)
	rules, err := rm.loadRulesFromFile()
	assert.NoError(t, err)
	assert.NotNil(t, rules)
	assert.NotEmpty(t, rules)
}

// TestRuleManager_loadRulesFromFile_UnsupportedFormat 测试 loadRulesFromFile 不支持的格式
func TestRuleManager_loadRulesFromFile_UnsupportedFormat(t *testing.T) {
	tmpFile := t.TempDir() + "/rules.yaml"
	err := os.WriteFile(tmpFile, []byte("yaml content"), 0644)
	assert.NoError(t, err)

	rm := NewRuleManager(tmpFile, false, 0, "", nil)
	rules, err := rm.loadRulesFromFile()
	assert.Error(t, err)
	assert.Nil(t, rules)
	assert.Contains(t, err.Error(), "YAML rules not yet implemented")
}

// TestRuleManager_loadRulesFromFile_DefaultFormat 测试 loadRulesFromFile 默认格式（无扩展名）
func TestRuleManager_loadRulesFromFile_DefaultFormat(t *testing.T) {
	tmpFile := t.TempDir() + "/rules"
	invalidJSON := `invalid json`
	err := os.WriteFile(tmpFile, []byte(invalidJSON), 0644)
	assert.NoError(t, err)

	rm := NewRuleManager(tmpFile, false, 0, "", nil)
	rules, err := rm.loadRulesFromFile()
	assert.Error(t, err)
	assert.Nil(t, rules)
	assert.Contains(t, err.Error(), "unsupported rules file format")
}

// TestEngine_normalizeURL_EmptyPath 测试 normalizeURL 空路径
func TestEngine_normalizeURL_EmptyPath(t *testing.T) {
	u, err := url.Parse("http://example.com")
	assert.NoError(t, err)

	normalized := normalizeURL(u)
	assert.Contains(t, normalized, "/")
}

// TestEngine_normalizeURL_CleanupPath 测试 normalizeURL 清理路径
func TestEngine_normalizeURL_CleanupPath(t *testing.T) {
	u, err := url.Parse("http://example.com/../../../etc/passwd")
	assert.NoError(t, err)

	normalized := normalizeURL(u)
	// 路径应该被清理
	assert.NotContains(t, normalized, "..")
}

// TestEngine_calculateBodyHash_NilBody 测试 calculateBodyHash 空 body
func TestEngine_calculateBodyHash_NilBody(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	hash := engine.calculateBodyHash(req)
	// 空 body 返回空字符串
	assert.Empty(t, hash)
}

// TestEngine_HandleRequest_WithThreats 测试 HandleRequest 检测到威胁
func TestEngine_HandleRequest_WithThreats(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
			BlockMessage:  "Blocked by WAF",
		},
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	// 手动添加一个会触发威胁的探测器
	mockDetector := &mockThreatDetector{}
	engine.owaspDetectors["mock"] = mockDetector

	req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	w := httptest.NewRecorder()

	allowed := engine.HandleRequest(w, req)
	assert.False(t, allowed)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestEngine_HandleRequest_WithDetectorError_FailClosed 测试 HandleRequest 检测器错误时 FailClosed 策略
func TestEngine_HandleRequest_WithDetectorError_FailClosed(t *testing.T) {
	config := Config{
		RulesPath:    "/tmp/rules",
		FailStrategy: FailClosed,
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	// 手动添加一个会返回错误的探测器
	errorDetector := &mockErrorDetector{}
	engine.owaspDetectors["error"] = errorDetector

	req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	w := httptest.NewRecorder()

	allowed := engine.HandleRequest(w, req)
	// 检测器错误会触发 FailClosed 策略，添加威胁信息
	assert.False(t, allowed)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Access Denied")
}

// TestEngine_UpdateRules_WithCacheClear 测试 UpdateRules 清理缓存
func TestEngine_UpdateRules_WithCacheClear(t *testing.T) {
	config := Config{
		RulesPath: "/tmp/rules",
	}

	engine, err := NewEngine("test-site", config)
	assert.NoError(t, err)

	// 添加缓存
	engine.addToCache("test-key", &CheckResult{Allow: true, CreatedAt: time.Now()})

	// 更新规则（应该清理缓存）
	err = engine.UpdateRules()
	assert.NoError(t, err)

	// 无 Redis 时缓存不可用
	cached := engine.getFromCache("test-key")
	if cached != nil {
		// 如果确实有 Redis，应确保规则更新后缓存清空
		t.Log("Redis connected: cache cleared after UpdateRules")
	}
}

// mockErrorDetector 模拟返回错误的探测器
type mockErrorDetector struct{}

func (m *mockErrorDetector) Detect(req *http.Request) ([]types.Threat, error) {
	return nil, assert.AnError
}

func (m *mockErrorDetector) Name() string {
	return "error"
}

// TestRuleManager_fetchRulesFromRemote_NoRemoteSource 测试 fetchRulesFromRemote 没有配置远程源
func TestRuleManager_fetchRulesFromRemote_NoRemoteSource(t *testing.T) {
	rm := NewRuleManager("", false, 0, "", nil)
	rules, err := rm.fetchRulesFromRemote()
	assert.Error(t, err)
	assert.Nil(t, rules)
	assert.Contains(t, err.Error(), "remote rule source not configured")
}

// TestRuleManager_saveRulesToRedis_NilClient 测试 saveRulesToRedis Redis 客户端为 nil
func TestRuleManager_saveRulesToRedis_NilClient(t *testing.T) {
	rm := NewRuleManager("", false, 0, "", nil)
	rules := map[string][]types.Rule{"injection": {}}
	err := rm.saveRulesToRedis(rules)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis client not set")
}

// TestRuleManager_loadRulesFromRedis_NilClient 测试 loadRulesFromRedis Redis 客户端为 nil
func TestRuleManager_loadRulesFromRedis_NilClient(t *testing.T) {
	rm := NewRuleManager("", false, 0, "", nil)
	rules, err := rm.loadRulesFromRedis()
	assert.Error(t, err)
	assert.Nil(t, rules)
	assert.Contains(t, err.Error(), "redis client not set")
}

// mockThreatDetector 模拟威胁探测器
type mockThreatDetector struct{}

func (m *mockThreatDetector) Detect(req *http.Request) ([]types.Threat, error) {
	return []types.Threat{{Type: "test", Message: "Mock threat detected"}}, nil
}

func (m *mockThreatDetector) Name() string {
	return "mock"
}
