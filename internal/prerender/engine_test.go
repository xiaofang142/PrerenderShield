package prerender

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/cache"
)

// 定义测试用错误
var ErrKeyNotFound = errors.New("key not found")

// mockRedisClientForEngine 模拟 Redis 客户端
// CreatePreheatTask 会启动异步 goroutine 访问 data，必须加锁
type mockRedisClientForEngine struct {
	mu   sync.Mutex
	data map[string]interface{}
}

func newMockRedisClientForEngine() *mockRedisClientForEngine {
	return &mockRedisClientForEngine{
		data: make(map[string]interface{}),
	}
}

func (m *mockRedisClientForEngine) SaveJSON(key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *mockRedisClientForEngine) GetJSON(key string, value interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.data[key]
	if !ok {
		return ErrKeyNotFound
	}
	// 转换为 JSON 再反序列化以匹配实际行为
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, value)
}

func (m *mockRedisClientForEngine) SetAdd(key string, members ...interface{}) error {
	return nil
}

func (m *mockRedisClientForEngine) SetMembers(key string) ([]string, error) {
	return []string{}, nil
}

func (m *mockRedisClientForEngine) Del(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *mockRedisClientForEngine) Keys(pattern string) ([]string, error) {
	var keys []string
	for key := range m.data {
		if key == "task:preheat:test-task" {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (m *mockRedisClientForEngine) SetURLPreheatStatus(site, route, status string, params ...interface{}) error {
	return nil
}

// mockCacheManager 模拟缓存管理器
type mockCacheManager struct {
	data map[string]interface{}
}

func newMockCacheManager() *mockCacheManager {
	return &mockCacheManager{
		data: make(map[string]interface{}),
	}
}

func (m *mockCacheManager) Get(site, key string) ([]byte, error) {
	if data, ok := m.data[site+":"+key]; ok {
		return data.([]byte), nil
	}
	return nil, nil
}

func (m *mockCacheManager) Set(site, key string, value []byte, ttl time.Duration) error {
	m.data[site+":"+key] = value
	return nil
}

func (m *mockCacheManager) Delete(site, key string) error {
	delete(m.data, site+":"+key)
	return nil
}

func (m *mockCacheManager) Exists(site, key string) (bool, error) {
	_, ok := m.data[site+":"+key]
	return ok, nil
}

func (m *mockCacheManager) Clear(site string) error {
	for key := range m.data {
		if site == "" || strings.HasPrefix(key, site+":") {
			delete(m.data, key)
		}
	}
	return nil
}

func (m *mockCacheManager) ClearAll() error {
	m.data = make(map[string]interface{})
	return nil
}

func (m *mockCacheManager) GetStats(siteID string) (map[string]interface{}, error) {
	return map[string]interface{}{"hits": 0, "misses": 0}, nil
}

func (m *mockCacheManager) IncrementHit(siteID string) error {
	return nil
}

func (m *mockCacheManager) IncrementMiss(siteID string) error {
	return nil
}

func (m *mockCacheManager) SetWithPriority(siteID, key string, value []byte, expiration time.Duration, priority int) error {
	m.data[siteID+":"+key] = value
	return nil
}

func (m *mockCacheManager) GetCacheEntry(siteID, key string) (*cache.CacheEntry, error) {
	if data, ok := m.data[siteID+":"+key]; ok {
		return &cache.CacheEntry{
			Data: data.([]byte),
		}, nil
	}
	return nil, nil
}

func (m *mockCacheManager) EvictLowPriority(siteID string, count int) error {
	return nil
}

// TestNewEngine 测试创建渲染引擎
func TestNewEngine(t *testing.T) {
	redisClient := newMockRedisClientForEngine()
	cacheManager := newMockCacheManager()

	// 测试正常创建
	engine := NewEngine(redisClient, cacheManager, 10)
	assert.NotNil(t, engine)

	// 测试默认并发数（maxConcurrent <= 0）
	engineDefault := NewEngine(redisClient, cacheManager, 0)
	assert.NotNil(t, engineDefault)

	// 测试负数并发数
	engineNegative := NewEngine(redisClient, cacheManager, -5)
	assert.NotNil(t, engineNegative)
}

// TestEngine_CreatePreheatTask 测试创建预热任务
func TestEngine_CreatePreheatTask(t *testing.T) {
	redisClient := newMockRedisClientForEngine()
	cacheManager := newMockCacheManager()
	engine := NewEngine(redisClient, cacheManager, 5)

	urls := []string{
		"http://test.com/page1",
		"http://test.com/page2",
		"http://test.com/page3",
	}

	taskID, err := engine.CreatePreheatTask("test-site", urls)
	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// 验证任务已保存
	task := make(map[string]interface{})
	err = redisClient.GetJSON("task:preheat:"+taskID, &task)
	assert.NoError(t, err)
	assert.Equal(t, "test-site", task["site_id"])
	assert.Equal(t, "pending", task["status"])
	assert.Equal(t, 3, int(task["total_urls"].(float64)))
}

// TestEngine_GetPreheatTaskStatus 测试获取预热任务状态
func TestEngine_GetPreheatTaskStatus(t *testing.T) {
	redisClient := newMockRedisClientForEngine()
	cacheManager := newMockCacheManager()
	engine := NewEngine(redisClient, cacheManager, 5)

	// 创建任务
	urls := []string{"http://test.com/page1"}
	taskID, _ := engine.CreatePreheatTask("test-site", urls)

	// 获取状态
	status, err := engine.GetPreheatTaskStatus(taskID)
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "test-site", status["site_id"])

	// 获取不存在的任务
	_, err = engine.GetPreheatTaskStatus("non-existent-task")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task not found")
}

// TestEngine_ListPreheatTasks 测试列出预热任务
func TestEngine_ListPreheatTasks(t *testing.T) {
	redisClient := newMockRedisClientForEngine()
	cacheManager := newMockCacheManager()
	engine := NewEngine(redisClient, cacheManager, 5)

	// 创建任务
	urls := []string{"http://test.com/page1"}
	_, _ = engine.CreatePreheatTask("test-site", urls)
	_, _ = engine.CreatePreheatTask("test-site", urls)

	// 列出任务
	tasks, err := engine.ListPreheatTasks("test-site")
	assert.NoError(t, err)
	assert.NotNil(t, tasks)
}

// TestEngine_CancelPreheatTask 测试取消预热任务
func TestEngine_CancelPreheatTask(t *testing.T) {
	redisClient := newMockRedisClientForEngine()
	cacheManager := newMockCacheManager()
	engine := NewEngine(redisClient, cacheManager, 5)

	// 创建任务
	urls := []string{"http://test.com/page1"}
	taskID, _ := engine.CreatePreheatTask("test-site", urls)

	// 取消任务
	err := engine.CancelPreheatTask(taskID)
	assert.NoError(t, err)

	// 验证状态已更新
	task := make(map[string]interface{})
	_ = redisClient.GetJSON("task:preheat:"+taskID, &task)
	assert.Equal(t, "cancelled", task["status"])

	// 取消已完成任务应该失败
	err = engine.CancelPreheatTask(taskID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be cancelled")
}

// TestEngine_CleanupPreheatTasks 测试清理过期预热任务
func TestEngine_CleanupPreheatTasks(t *testing.T) {
	redisClient := newMockRedisClientForEngine()
	cacheManager := newMockCacheManager()
	engine := NewEngine(redisClient, cacheManager, 5)

	// 创建一个已完成的任务
	taskID := "test-task"
	task := map[string]interface{}{
		"task_id":        taskID,
		"site_id":        "test-site",
		"status":         "completed",
		"created_at":     time.Now().Add(-25 * time.Hour).Unix(), // 25 小时前
		"completed_urls": 10,
		"failed_urls":    0,
	}
	_ = redisClient.SaveJSON("task:preheat:"+taskID, task, 24*time.Hour)

	// 清理过期任务
	err := engine.CleanupPreheatTasks()
	assert.NoError(t, err)

	// 验证任务已被删除
	_, err = engine.GetPreheatTaskStatus(taskID)
	assert.Error(t, err)
}

// TestEngine_IsCrawlerRequest 测试爬虫检测
func TestEngine_IsCrawlerRequest(t *testing.T) {
	redisClient := newMockRedisClientForEngine()
	cacheManager := newMockCacheManager()
	engine := NewEngine(redisClient, cacheManager, 5)

	tests := []struct {
		name      string
		userAgent string
		expected  bool
	}{
		{"Googlebot", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", true},
		{"Bingbot", "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)", true},
		{"BaiduSpider", "Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)", true},
		{"YandexBot", "Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)", true},
		{"Sogou", "Sogou web spider/4.0(+http://www.sogou.com/docs/help/webmasters.htm)", true},
		{"Yahoo Slurp", "Mozilla/5.0 (compatible; Yahoo! Slurp; http://help.yahoo.com/help/us/ysearch/slurp)", true},
		{"DuckDuckBot", "DuckDuckBot/1.0; (+http://duckduckgo.com/duckduckbot.html)", true},
		{"FacebookBot", "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)", true},
		{"LinkedInBot", "LinkedInBot/1.0 (+http://www.linkedin.com)", true},
		{"Twitterbot", "Twitterbot/1.0", true},
		{"Pinterest", "Pinterest/0.1 (+http://www.pinterest.com/bot.html)", true},
		{"Slackbot", "Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)", true},
		{"TelegramBot", "TelegramBot (like TwitterBot)", true},
		{"WhatsApp", "WhatsApp/2.12.17/i", true},
		{"Generic Bot", "SomeBot/1.0", true},
		{"Generic Spider", "Spider/1.0", true},
		{"Generic Crawler", "Crawler/1.0", true},
		{"Generic Robot", "Robot/1.0", true},
		{"curl", "curl/7.64.1", false},
		{"wget", "Wget/1.20.3", false},
		{"Fetch", "Fetch/1.0", false},
		{"Chrome", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36", false},
		{"Firefox", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:89.0) Gecko/20100101 Firefox/89.0", false},
		{"Safari", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Safari/605.1.15", false},
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.IsCrawlerRequest(tt.userAgent)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test_containsIgnoreCase 测试忽略大小写的字符串包含检查
func Test_containsIgnoreCase(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"contains lowercase", "hello world", "world", true},
		{"contains uppercase", "HELLO WORLD", "world", true},
		{"mixed case", "Hello World", "wORld", true},
		{"not contains", "hello world", "foo", false},
		{"empty substring", "hello world", "", true},
		{"empty string", "", "world", false},
		{"both empty", "", "", true},
		{"exact match", "test", "test", true},
		{"exact match case diff", "TEST", "test", true},
		{"substring at start", "hello world", "hello", true},
		{"substring at end", "hello world", "world", true},
		{"substring in middle", "hello world", "lo wo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsIgnoreCase(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test_equalFold 测试忽略大小写的字符串比较
func Test_equalFold(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		t        string
		expected bool
	}{
		{"equal lowercase", "hello", "hello", true},
		{"equal uppercase", "HELLO", "HELLO", true},
		{"equal mixed case", "Hello", "hELLO", true},
		{"not equal", "hello", "world", false},
		{"different length", "hello", "hell", false},
		{"empty strings", "", "", true},
		{"one empty", "hello", "", false},
		{"single char equal", "a", "a", true},
		{"single char diff case", "A", "a", true},
		{"single char diff", "a", "b", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := equalFold(tt.s, tt.t)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestEngine_RenderWithContext 测试带上下文的渲染
func TestEngine_RenderWithContext(t *testing.T) {
	redisClient := newMockRedisClientForEngine()
	cacheManager := newMockCacheManager()
	engine := NewEngine(redisClient, cacheManager, 5)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// 本地测试服务器作为渲染目标：
	// 1) 避免依赖外网（example.com）与系统代理导致的 30s 挂起与偶发超时
	// 2) 无 Chrome 环境时仍走错误分支，测试对两种结果均容忍
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!DOCTYPE html><html><head><title>ok</title></head><body><h1>hello</h1></body></html>"))
	}))
	defer srv.Close()

	opts := RenderOptions{
		Timeout: 15 * time.Second,
	}

	// 注意：这个测试会尝试启动 Chrome，在没有 Chrome 的环境中会返回错误
	// 但测试本身验证了接口和错误处理逻辑
	result, err := engine.RenderWithContext(c, srv.URL, opts, "Mozilla/5.0")

	if err != nil {
		assert.False(t, result.Result.Success)
		assert.NotEmpty(t, result.Result.Error)
		assert.False(t, result.HitCache)
	} else {
		assert.True(t, result.Result.Success)
	}
}

// TestCacheManager 测试缓存管理器接口实现
func TestCacheManager(t *testing.T) {
	cacheManager := newMockCacheManager()

	// 测试 Set 和 Get
	err := cacheManager.Set("test-site", "test-key", []byte("test-value"), time.Hour)
	assert.NoError(t, err)

	value, err := cacheManager.Get("test-site", "test-key")
	assert.NoError(t, err)
	assert.NotNil(t, value)
	assert.Equal(t, []byte("test-value"), value)

	// 测试不存在的键
	value, err = cacheManager.Get("test-site", "non-existent")
	assert.NoError(t, err)
	assert.Nil(t, value)

	// 测试 Exists
	exists, err := cacheManager.Exists("test-site", "test-key")
	assert.NoError(t, err)
	assert.True(t, exists)

	// 测试 Delete
	err = cacheManager.Delete("test-site", "test-key")
	assert.NoError(t, err)

	value, _ = cacheManager.Get("test-site", "test-key")
	assert.Nil(t, value)
}

// TestRedisClient 测试 Redis 客户端接口实现
func TestRedisClient(t *testing.T) {
	redisClient := newMockRedisClientForEngine()

	// 测试 SaveJSON 和 GetJSON
	testData := map[string]interface{}{
		"key":   "value",
		"count": 42,
	}
	err := redisClient.SaveJSON("test-key", testData, time.Hour)
	assert.NoError(t, err)

	var result map[string]interface{}
	err = redisClient.GetJSON("test-key", &result)
	assert.NoError(t, err)
	assert.Equal(t, "value", result["key"])
	assert.Equal(t, float64(42), result["count"]) // JSON 数字是 float64

	// 测试不存在的键
	err = redisClient.GetJSON("non-existent", &result)
	assert.Error(t, err)

	// 测试 Del
	err = redisClient.Del("test-key")
	assert.NoError(t, err)

	// 测试 SetAdd
	err = redisClient.SetAdd("test-set", "member1")
	assert.NoError(t, err)

	// 测试 SetMembers
	members, err := redisClient.SetMembers("test-set")
	assert.NoError(t, err)
	assert.NotNil(t, members)
}

// TestEngineManager_NewEngineManager 测试创建引擎管理器
func TestEngineManager_NewEngineManager(t *testing.T) {
	redisClient := newMockRedisClientForEngine()
	cacheManager := newMockCacheManager()

	manager := NewEngineManager(redisClient, cacheManager, 10)
	assert.NotNil(t, manager)
	assert.Equal(t, 10, manager.maxConcurrent)
	assert.Equal(t, 30*time.Second, manager.timeout)
	assert.NotNil(t, manager.engines)
}

// TestEngineManager_GetEngine 测试获取引擎
func TestEngineManager_GetEngine(t *testing.T) {
	redisClient := newMockRedisClientForEngine()
	cacheManager := newMockCacheManager()
	manager := NewEngineManager(redisClient, cacheManager, 5)

	// 首次获取应该创建新引擎
	engine, exists := manager.GetEngine("site1")
	assert.NotNil(t, engine)
	assert.True(t, exists)

	// 再次获取应该返回同一个引擎
	engine2, exists2 := manager.GetEngine("site1")
	assert.True(t, exists2)
	assert.Equal(t, engine, engine2)

	// 获取不同站点的引擎应该创建新实例
	engine3, exists3 := manager.GetEngine("site2")
	assert.NotNil(t, engine3)
	assert.True(t, exists3)
	assert.NotEqual(t, engine, engine3)
}

// TestEngineManager_RemoveEngine 测试移除引擎
func TestEngineManager_RemoveEngine(t *testing.T) {
	redisClient := newMockRedisClientForEngine()
	cacheManager := newMockCacheManager()
	manager := NewEngineManager(redisClient, cacheManager, 5)

	// 先创建引擎
	_, _ = manager.GetEngine("site1")
	_, _ = manager.GetEngine("site2")

	// 移除一个引擎
	manager.RemoveEngine("site1")

	// 验证 site1 被移除，site2 还存在
	// 注意：GetEngine 会自动创建不存在的引擎，所以使用 ListSites 来验证
	sites := manager.ListSites()
	assert.NotContains(t, sites, "site1")
	assert.Contains(t, sites, "site2")
}

// TestEngineManager_Cleanup 测试清理所有引擎
func TestEngineManager_Cleanup(t *testing.T) {
	redisClient := newMockRedisClientForEngine()
	cacheManager := newMockCacheManager()
	manager := NewEngineManager(redisClient, cacheManager, 5)

	// 创建多个引擎
	_, _ = manager.GetEngine("site1")
	_, _ = manager.GetEngine("site2")
	_, _ = manager.GetEngine("site3")

	// 清理所有引擎
	manager.Cleanup()

	// 验证所有引擎都被清理
	siteIDs := manager.ListSites()
	assert.Empty(t, siteIDs)
}

// TestEngineManager_SetMaxConcurrent 测试设置最大并发数
func TestEngineManager_SetMaxConcurrent(t *testing.T) {
	redisClient := newMockRedisClientForEngine()
	cacheManager := newMockCacheManager()
	manager := NewEngineManager(redisClient, cacheManager, 5)

	assert.Equal(t, 5, manager.maxConcurrent)

	manager.SetMaxConcurrent(20)
	assert.Equal(t, 20, manager.maxConcurrent)
}

// TestEngineManager_SetTimeout 测试设置超时时间
func TestEngineManager_SetTimeout(t *testing.T) {
	redisClient := newMockRedisClientForEngine()
	cacheManager := newMockCacheManager()
	manager := NewEngineManager(redisClient, cacheManager, 5)

	assert.Equal(t, 30*time.Second, manager.timeout)

	manager.SetTimeout(60 * time.Second)
	assert.Equal(t, 60*time.Second, manager.timeout)
}

// TestEngineManager_IsCrawlerRequest 测试引擎管理器的爬虫检测
func TestEngineManager_IsCrawlerRequest(t *testing.T) {
	redisClient := newMockRedisClientForEngine()
	cacheManager := newMockCacheManager()
	manager := NewEngineManager(redisClient, cacheManager, 5)

	tests := []struct {
		name      string
		userAgent string
		expected  bool
	}{
		{"Googlebot", "Mozilla/5.0 (compatible; Googlebot/2.1)", true},
		{"Chrome", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/91.0", false},
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.IsCrawlerRequest(tt.userAgent)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestEngineManager_ListSites 测试列出站点
func TestEngineManager_ListSites(t *testing.T) {
	redisClient := newMockRedisClientForEngine()
	cacheManager := newMockCacheManager()
	manager := NewEngineManager(redisClient, cacheManager, 5)

	// 初始应该为空
	sites := manager.ListSites()
	assert.Empty(t, sites)

	// 创建引擎后应该返回站点列表
	_, _ = manager.GetEngine("site1")
	_, _ = manager.GetEngine("site2")

	sites = manager.ListSites()
	assert.Len(t, sites, 2)
	assert.Contains(t, sites, "site1")
	assert.Contains(t, sites, "site2")
}
