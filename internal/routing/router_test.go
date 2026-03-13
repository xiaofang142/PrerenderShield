package routing

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestMemoryCache 测试 MemoryCache
func TestMemoryCache_New(t *testing.T) {
	cache := NewMemoryCache()
	assert.NotNil(t, cache)
	assert.NotNil(t, cache.cache)
}

func TestMemoryCache_Set(t *testing.T) {
	cache := NewMemoryCache()

	err := cache.Set("key1", "value1", 60)
	assert.NoError(t, err)
}

func TestMemoryCache_Get(t *testing.T) {
	cache := NewMemoryCache()

	// 测试不存在的键
	val := cache.Get("nonexistent")
	assert.Nil(t, val)

	// 测试存在的键
	cache.Set("key1", "value1", 60)
	val = cache.Get("key1")
	assert.Equal(t, "value1", val)
}

func TestMemoryCache_Get_Expired(t *testing.T) {
	cache := NewMemoryCache()

	// 设置一个立即过期的缓存
	cache.Set("key1", "value1", 0)
	time.Sleep(10 * time.Millisecond)

	val := cache.Get("key1")
	assert.Nil(t, val) // 应该返回 nil，因为已过期
}

func TestMemoryCache_Clear(t *testing.T) {
	cache := NewMemoryCache()

	cache.Set("key1", "value1", 60)
	cache.Set("key2", "value2", 60)

	err := cache.Clear()
	assert.NoError(t, err)

	val := cache.Get("key1")
	assert.Nil(t, val)
}

func TestMemoryCache_ConcurrentAccess(t *testing.T) {
	cache := NewMemoryCache()

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			key := "key" + string(rune(id+'0'))
			cache.Set(key, "value", 60)
			cache.Get(key)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证并发访问没有导致 panic
	assert.NotNil(t, cache.cache)
}

// TestRegexMatcher 测试 RegexMatcher
func TestRegexMatcher_New(t *testing.T) {
	matcher := &RegexMatcher{}
	assert.NotNil(t, matcher)
}

func TestRegexMatcher_Match_ExactPath(t *testing.T) {
	matcher := &RegexMatcher{}

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rule := &RouteRule{
		Pattern: "=/api/test",
	}

	matched := matcher.Match(req, rule)
	assert.True(t, matched)
}

func TestRegexMatcher_Match_ExactPath_NoMatch(t *testing.T) {
	matcher := &RegexMatcher{}

	req := httptest.NewRequest(http.MethodGet, "/api/other", nil)
	rule := &RouteRule{
		Pattern: "=/api/test",
	}

	matched := matcher.Match(req, rule)
	assert.False(t, matched)
}

func TestRegexMatcher_Match_PrefixPattern(t *testing.T) {
	matcher := &RegexMatcher{}

	req := httptest.NewRequest(http.MethodGet, "/api/users/123", nil)
	rule := &RouteRule{
		Pattern: "/api/users/*",
	}

	matched := matcher.Match(req, rule)
	assert.True(t, matched)
}

func TestRegexMatcher_Match_PrefixPattern_NoMatch(t *testing.T) {
	matcher := &RegexMatcher{}

	req := httptest.NewRequest(http.MethodGet, "/api/posts/123", nil)
	rule := &RouteRule{
		Pattern: "/api/users/*",
	}

	matched := matcher.Match(req, rule)
	assert.False(t, matched)
}

func TestRegexMatcher_Match_RegexPattern(t *testing.T) {
	matcher := &RegexMatcher{}

	req := httptest.NewRequest(http.MethodGet, "/api/users/123", nil)
	rule := &RouteRule{
		Pattern: "^/api/users/[0-9]+$",
	}

	matched := matcher.Match(req, rule)
	assert.True(t, matched)
}

func TestRegexMatcher_Match_RegexPattern_NoMatch(t *testing.T) {
	matcher := &RegexMatcher{}

	req := httptest.NewRequest(http.MethodGet, "/api/users/abc", nil)
	rule := &RouteRule{
		Pattern: "^/api/users/[0-9]+$",
	}

	matched := matcher.Match(req, rule)
	assert.False(t, matched)
}

func TestRegexMatcher_Match_Domain_Exact(t *testing.T) {
	matcher := &RegexMatcher{}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/test", nil)
	req.Host = "example.com"
	rule := &RouteRule{
		Domain:  "example.com",
		Pattern: "/api/*",
	}

	matched := matcher.Match(req, rule)
	assert.True(t, matched)
}

func TestRegexMatcher_Match_Domain_Exact_NoMatch(t *testing.T) {
	matcher := &RegexMatcher{}

	req := httptest.NewRequest(http.MethodGet, "http://other.com/api", nil)
	req.Host = "other.com"
	rule := &RouteRule{
		Domain:  "example.com",
		Pattern: "/api/*",
	}

	matched := matcher.Match(req, rule)
	assert.False(t, matched)
}

func TestRegexMatcher_Match_Domain_Wildcard(t *testing.T) {
	matcher := &RegexMatcher{}

	req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/api/test", nil)
	req.Host = "sub.example.com"
	rule := &RouteRule{
		Domain:  "*.example.com",
		Pattern: "/api/*",
	}

	matched := matcher.Match(req, rule)
	assert.True(t, matched)
}

func TestRegexMatcher_Match_Domain_Prefix(t *testing.T) {
	matcher := &RegexMatcher{}

	req := httptest.NewRequest(http.MethodGet, "http://example.co.uk/api/test", nil)
	req.Host = "example.co.uk"
	rule := &RouteRule{
		Domain:  "example.*",
		Pattern: "/api/*",
	}

	matched := matcher.Match(req, rule)
	assert.True(t, matched)
}

func TestRegexMatcher_Match_Domain_Any(t *testing.T) {
	matcher := &RegexMatcher{}

	// 使用 /api/ 路径，确保匹配 /api/*
	req := httptest.NewRequest(http.MethodGet, "http://any-domain.com/api/test", nil)
	req.Host = "any-domain.com"
	rule := &RouteRule{
		Domain:  "*",
		Pattern: "/api/*",
	}

	matched := matcher.Match(req, rule)
	// 通配符 "*" 域名匹配逻辑：如果 rule.Domain == "*" 则匹配所有
	// 但需要检查代码实现，可能 "*" 有特殊处理
	// 根据代码，rule.Domain == "*" 时直接通过
	assert.True(t, matched)
}

func TestRegexMatcher_Match_InvalidPattern(t *testing.T) {
	matcher := &RegexMatcher{}

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rule := &RouteRule{
		Pattern: "[invalid(regex", // 无效的正则表达式
	}

	matched := matcher.Match(req, rule)
	assert.False(t, matched) // 无效正则应该返回 false
}

func TestRegexMatcher_Match_WithPort(t *testing.T) {
	matcher := &RegexMatcher{}

	req := httptest.NewRequest(http.MethodGet, "http://example.com:8080/api/test", nil)
	req.Host = "example.com:8080"
	rule := &RouteRule{
		Domain:  "example.com",
		Pattern: "/api/*",
	}

	matched := matcher.Match(req, rule)
	assert.True(t, matched)
}

// TestRouter 测试 Router
func TestRouter_New(t *testing.T) {
	cache := NewMemoryCache()
	config := Config{
		Cache: cache,
	}

	router := NewRouter(config)
	assert.NotNil(t, router)
	assert.NotNil(t, router.exactRules)
	assert.NotNil(t, router.prefixRules)
	assert.NotNil(t, router.regexRules)
}

func TestRouter_ServeHTTP_WithMatchingRule(t *testing.T) {
	cache := NewMemoryCache()
	handlerCalled := false

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{
			{
				ID:       "rule1",
				Pattern:  "/api/*",
				Action:   "test",
				Priority: 10,
			},
		},
		Handlers: map[string]HandlerFunc{
			"test": func(w http.ResponseWriter, r *http.Request, rule *RouteRule) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	router := NewRouter(config)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRouter_ServeHTTP_NoMatchingRule(t *testing.T) {
	cache := NewMemoryCache()

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{
			{
				ID:       "rule1",
				Pattern:  "/api/*",
				Action:   "test",
				Priority: 10,
			},
		},
		Handlers: map[string]HandlerFunc{},
	}

	router := NewRouter(config)

	req := httptest.NewRequest(http.MethodGet, "/other/path", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRouter_ServeHTTP_NoHandler(t *testing.T) {
	cache := NewMemoryCache()

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{
			{
				ID:       "rule1",
				Pattern:  "/api/*",
				Action:   "unknown",
				Priority: 10,
			},
		},
		Handlers: map[string]HandlerFunc{},
	}

	router := NewRouter(config)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "No handler for action")
}

func TestRouter_MatchRoute_ExactMatch(t *testing.T) {
	cache := NewMemoryCache()

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{
			{
				ID:       "rule1",
				Pattern:  "=/api/exact",
				Action:   "test",
				Priority: 10,
			},
		},
		Handlers: map[string]HandlerFunc{},
	}

	router := NewRouter(config)

	req := httptest.NewRequest(http.MethodGet, "/api/exact", nil)
	rule := router.MatchRoute(req)

	assert.NotNil(t, rule)
	assert.Equal(t, "rule1", rule.ID)
}

func TestRouter_MatchRoute_PrefixMatch(t *testing.T) {
	cache := NewMemoryCache()

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{
			{
				ID:       "rule1",
				Pattern:  "/api/*",
				Action:   "test",
				Priority: 10,
			},
		},
		Handlers: map[string]HandlerFunc{},
	}

	router := NewRouter(config)

	req := httptest.NewRequest(http.MethodGet, "/api/users/123", nil)
	rule := router.MatchRoute(req)

	assert.NotNil(t, rule)
	assert.Equal(t, "rule1", rule.ID)
}

func TestRouter_MatchRoute_CacheHit(t *testing.T) {
	cache := NewMemoryCache()

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{
			{
				ID:       "rule1",
				Pattern:  "/api/*",
				Action:   "test",
				Priority: 10,
			},
		},
		Handlers: map[string]HandlerFunc{},
	}

	router := NewRouter(config)

	// 第一次请求
	req1 := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	router.MatchRoute(req1)

	// 第二次请求，应该命中缓存
	req2 := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rule := router.MatchRoute(req2)

	assert.NotNil(t, rule)
	assert.Equal(t, "rule1", rule.ID)
}

func TestRouter_UpdateRules(t *testing.T) {
	cache := NewMemoryCache()

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{
			{
				ID:       "rule1",
				Pattern:  "/api/*",
				Action:   "test",
				Priority: 10,
			},
		},
		Handlers: map[string]HandlerFunc{},
	}

	router := NewRouter(config)

	// 更新规则
	newRules := []*RouteRule{
		{
			ID:       "rule2",
			Pattern:  "/v2/*",
			Action:   "test",
			Priority: 20,
		},
	}

	err := router.UpdateRules(newRules)
	assert.NoError(t, err)

	rules := router.GetRules()
	assert.Len(t, rules, 1)
	assert.Equal(t, "rule2", rules[0].ID)
}

func TestRouter_AddRule(t *testing.T) {
	cache := NewMemoryCache()

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{},
		Handlers: map[string]HandlerFunc{},
	}

	router := NewRouter(config)

	rule := &RouteRule{
		Pattern:  "/new/*",
		Action:   "test",
		Priority: 10,
	}

	err := router.AddRule(rule)
	assert.NoError(t, err)

	rules := router.GetRules()
	assert.Len(t, rules, 1)
	assert.NotEmpty(t, rule.ID) // 应该自动生成 ID
}

func TestRouter_AddRule_WithID(t *testing.T) {
	cache := NewMemoryCache()

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{},
		Handlers: map[string]HandlerFunc{},
	}

	router := NewRouter(config)

	rule := &RouteRule{
		ID:       "custom-id",
		Pattern:  "/new/*",
		Action:   "test",
		Priority: 10,
	}

	err := router.AddRule(rule)
	assert.NoError(t, err)

	rules := router.GetRules()
	assert.Len(t, rules, 1)
	assert.Equal(t, "custom-id", rule.ID)
}

func TestRouter_DeleteRule(t *testing.T) {
	cache := NewMemoryCache()

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{
			{
				ID:       "rule1",
				Pattern:  "/api/*",
				Action:   "test",
				Priority: 10,
			},
		},
		Handlers: map[string]HandlerFunc{},
	}

	router := NewRouter(config)

	err := router.DeleteRule("rule1")
	assert.NoError(t, err)

	rules := router.GetRules()
	assert.Empty(t, rules)
}

func TestRouter_DeleteRule_NotFound(t *testing.T) {
	cache := NewMemoryCache()

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{},
		Handlers: map[string]HandlerFunc{},
	}

	router := NewRouter(config)

	err := router.DeleteRule("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rule not found")
}

func TestRouter_GetRules(t *testing.T) {
	cache := NewMemoryCache()

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{
			{
				ID:       "rule1",
				Pattern:  "/api/*",
				Action:   "test",
				Priority: 10,
			},
		},
		Handlers: map[string]HandlerFunc{},
	}

	router := NewRouter(config)

	rules := router.GetRules()
	assert.Len(t, rules, 1)
	assert.Equal(t, "rule1", rules[0].ID)
}

func TestRouter_AddHandler(t *testing.T) {
	cache := NewMemoryCache()

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{},
	}

	router := NewRouter(config)

	handler := func(w http.ResponseWriter, r *http.Request, rule *RouteRule) {}
	router.AddHandler("test", handler)

	assert.Len(t, router.handlers, 1)
}

func TestRouter_RemoveHandler(t *testing.T) {
	cache := NewMemoryCache()

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{},
		Handlers: map[string]HandlerFunc{
			"test": func(w http.ResponseWriter, r *http.Request, rule *RouteRule) {},
		},
	}

	router := NewRouter(config)

	router.RemoveHandler("test")

	assert.Empty(t, router.handlers)
}

func TestRouter_Priority(t *testing.T) {
	cache := NewMemoryCache()

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{
			{
				ID:       "rule1",
				Pattern:  "/api/*",
				Action:   "test1",
				Priority: 5,
			},
			{
				ID:       "rule2",
				Pattern:  "=/api/users/123",
				Action:   "test2",
				Priority: 10,
			},
		},
		Handlers: map[string]HandlerFunc{},
	}

	router := NewRouter(config)

	// 请求 /api/users/123 应该匹配精确匹配的 rule2（优先级更高）
	req := httptest.NewRequest(http.MethodGet, "/api/users/123", nil)
	rule := router.MatchRoute(req)

	assert.NotNil(t, rule)
	// 精确匹配优先于前缀匹配
	assert.Equal(t, "rule2", rule.ID)
}

// TestRouteRule 测试 RouteRule 结构
func TestRouteRule(t *testing.T) {
	rule := &RouteRule{
		ID:       "rule1",
		Domain:   "example.com",
		Pattern:  "/api/*",
		Action:   "proxy",
		Priority: 10,
		Params: map[string]string{
			"key": "value",
		},
	}

	assert.Equal(t, "rule1", rule.ID)
	assert.Equal(t, "example.com", rule.Domain)
	assert.Equal(t, "/api/*", rule.Pattern)
	assert.Equal(t, "proxy", rule.Action)
	assert.Equal(t, 10, rule.Priority)
	assert.NotNil(t, rule.Params)
}

// TestConfig 测试 Config 结构
func TestConfig(t *testing.T) {
	config := &Config{
		Rules: []*RouteRule{
			{ID: "rule1"},
		},
		Cache: NewMemoryCache(),
		Handlers: map[string]HandlerFunc{
			"test": func(w http.ResponseWriter, r *http.Request, rule *RouteRule) {},
		},
	}

	assert.NotNil(t, config.Rules)
	assert.NotNil(t, config.Cache)
	assert.NotNil(t, config.Handlers)
}

// TestHandlerFunc 测试 HandlerFunc 类型
func TestHandlerFunc(t *testing.T) {
	var handler HandlerFunc = func(w http.ResponseWriter, r *http.Request, rule *RouteRule) {
		w.WriteHeader(http.StatusOK)
	}

	assert.NotNil(t, handler)
}

// TestSortRules 测试 sortRules 方法
func TestSortRules(t *testing.T) {
	cache := NewMemoryCache()

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{
			{ID: "rule1", Priority: 5},
			{ID: "rule2", Priority: 10},
			{ID: "rule3", Priority: 1},
		},
		Handlers: map[string]HandlerFunc{},
	}

	router := NewRouter(config)

	// 验证规则已按优先级排序
	rules := router.GetRules()
	assert.Len(t, rules, 3)
	assert.Equal(t, "rule2", rules[0].ID) // 优先级最高
	assert.Equal(t, "rule1", rules[1].ID)
	assert.Equal(t, "rule3", rules[2].ID) // 优先级最低
}

// TestBuildRuleIndexes 测试 buildRuleIndexes 方法
func TestBuildRuleIndexes(t *testing.T) {
	cache := NewMemoryCache()

	config := Config{
		Cache: cache,
		Rules: []*RouteRule{
			{ID: "rule1", Pattern: "=/api/exact"},
			{ID: "rule2", Pattern: "/api/*"},
			{ID: "rule3", Pattern: "^/api/[0-9]+$"},
		},
		Handlers: map[string]HandlerFunc{},
	}

	router := NewRouter(config)

	// 验证索引已正确构建
	assert.Len(t, router.exactRules, 1)
	assert.Contains(t, router.exactRules, "/api/exact")

	// 前缀匹配：/api/* 会被解析为前缀 /api/
	assert.Len(t, router.prefixRules, 1)
	assert.Contains(t, router.prefixRules, "/api/")

	assert.Len(t, router.regexRules, 1)
	assert.Equal(t, "rule3", router.regexRules[0].ID)
}
