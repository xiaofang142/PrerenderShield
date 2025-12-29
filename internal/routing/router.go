package routing

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Matcher 路由匹配器接口
type Matcher interface {
	Match(req *http.Request, rule *RouteRule) bool
}

// Cache 路由缓存接口
type Cache interface {
	Get(key string) interface{}
	Set(key string, value interface{}, ttl int) error
}

// MemoryCache 内存缓存实现
type MemoryCache struct {
	cache map[string]cacheItem
	mutex sync.RWMutex
}

type cacheItem struct {
	value      interface{}
	expiration time.Time
}

// NewMemoryCache 创建新的内存缓存
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		cache: make(map[string]cacheItem),
	}
}

// Get 从缓存获取值
func (mc *MemoryCache) Get(key string) interface{} {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	
	item, exists := mc.cache[key]
	if !exists {
		return nil
	}
	
	if time.Now().After(item.expiration) {
		// 缓存过期，删除
		mc.mutex.RUnlock()
		mc.mutex.Lock()
		delete(mc.cache, key)
		mc.mutex.Unlock()
		mc.mutex.RLock()
		return nil
	}
	
	return item.value
}

// Set 设置缓存值
func (mc *MemoryCache) Set(key string, value interface{}, ttl int) error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	
	expiration := time.Now().Add(time.Duration(ttl) * time.Second)
	mc.cache[key] = cacheItem{
		value:      value,
		expiration: expiration,
	}
	
	return nil
}

// RegexMatcher 正则表达式匹配器
type RegexMatcher struct {}

// Match 使用正则表达式匹配路由规则
func (rm *RegexMatcher) Match(req *http.Request, rule *RouteRule) bool {
	// 1. 检查域名匹配
	if rule.Domain != "" {
		// 获取请求的主机名（不包含端口）
		host := req.Host
		if idx := strings.LastIndex(host, ":"); idx != -1 {
			host = host[:idx]
		}
		
		// 域名匹配逻辑
		if rule.Domain == "*" {
			// 通配符匹配，匹配所有域名
		} else if strings.HasPrefix(rule.Domain, "*") {
			// 后缀匹配，如 *.example.com
			suffix := strings.TrimPrefix(rule.Domain, "*")
			if !strings.HasSuffix(host, suffix) {
				return false
			}
		} else if strings.HasSuffix(rule.Domain, "*") {
			// 前缀匹配，如 example.*
			prefix := strings.TrimSuffix(rule.Domain, "*")
			if !strings.HasPrefix(host, prefix) {
				return false
			}
		} else {
			// 精确匹配
			if host != rule.Domain {
				return false
			}
		}
	}
	
	// 2. 路径匹配
	pattern := rule.Pattern
	
	// 如果是精确匹配
	if strings.HasPrefix(pattern, "=") {
		exactPath := strings.TrimPrefix(pattern, "=")
		return req.URL.Path == exactPath
	}
	
	// 如果是前缀匹配
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(req.URL.Path, prefix)
	}
	
	// 否则使用正则表达式匹配
	matched, err := regexp.MatchString(pattern, req.URL.Path)
	if err != nil {
		return false
	}
	
	return matched
}

// Router 智能流量路由管理器
type Router struct {
	rules     []*RouteRule
	mutex     sync.RWMutex
	matcher   Matcher
	cache     Cache
	handlers  map[string]HandlerFunc
}

// RouteRule 路由规则
type RouteRule struct {
	ID       string
	Domain   string // 支持按域名匹配
	Pattern  string // 路径匹配模式
	Action   string
	Priority int
	Params   map[string]string
}

// HandlerFunc 路由处理函数
type HandlerFunc func(http.ResponseWriter, *http.Request, *RouteRule)

// Config 路由配置
type Config struct {
	Rules    []*RouteRule
	Cache    Cache
	Handlers map[string]HandlerFunc
}

// NewRouter 创建新的路由管理器
func NewRouter(config Config) *Router {
	router := &Router{
		rules:    config.Rules,
		cache:    config.Cache,
		handlers: config.Handlers,
		matcher:  &RegexMatcher{},
	}
	
	// 按优先级排序规则
	router.sortRules()
	
	return router
}

// sortRules 按优先级排序规则
func (r *Router) sortRules() {
	sort.Slice(r.rules, func(i, j int) bool {
		return r.rules[i].Priority > r.rules[j].Priority
	})
}

// ServeHTTP 路由处理HTTP请求
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// 查找匹配的路由规则
	rule := r.MatchRoute(req)
	if rule != nil {
		// 执行对应的处理函数
		r.executeHandler(w, req, rule)
		return
	}
	
	// 没有匹配的规则，返回404
	http.NotFound(w, req)
}

// MatchRoute 匹配路由规则
func (r *Router) MatchRoute(req *http.Request) *RouteRule {
	// 先检查缓存
	cacheKey := fmt.Sprintf("route:%s", req.URL.Path)
	if cachedRule, ok := r.cache.Get(cacheKey).(*RouteRule); ok {
		return cachedRule
	}
	
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	// 遍历规则，找到第一个匹配的
	for _, rule := range r.rules {
		if r.matcher.Match(req, rule) {
			// 缓存匹配结果
			r.cache.Set(cacheKey, rule, 3600)
			return rule
		}
	}
	
	return nil
}

// executeHandler 执行路由处理函数
func (r *Router) executeHandler(w http.ResponseWriter, req *http.Request, rule *RouteRule) {
	handler, exists := r.handlers[rule.Action]
	if !exists {
		// 默认处理函数
		http.Error(w, fmt.Sprintf("No handler for action: %s", rule.Action), http.StatusInternalServerError)
		return
	}
	
	handler(w, req, rule)
}

// UpdateRules 更新路由规则
func (r *Router) UpdateRules(rules []*RouteRule) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	r.rules = rules
	// 重新排序规则
	r.sortRules()
	// 清空路由缓存
	r.clearRouteCache()
	
	return nil
}

// AddRule 添加路由规则
func (r *Router) AddRule(rule *RouteRule) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	// 生成唯一ID
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	
	r.rules = append(r.rules, rule)
	// 重新排序规则
	r.sortRules()
	// 清空路由缓存
	r.clearRouteCache()
	
	return nil
}

// DeleteRule 删除路由规则
func (r *Router) DeleteRule(ruleID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	for i, rule := range r.rules {
		if rule.ID == ruleID {
			r.rules = append(r.rules[:i], r.rules[i+1:]...)
			// 清空路由缓存
			r.clearRouteCache()
			return nil
		}
	}
	
	return fmt.Errorf("rule not found: %s", ruleID)
}

// GetRules 获取所有路由规则
func (r *Router) GetRules() []*RouteRule {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	// 返回规则副本
	rules := make([]*RouteRule, len(r.rules))
	copy(rules, r.rules)
	
	return rules
}

// clearRouteCache 清空路由缓存
func (r *Router) clearRouteCache() {
	// 这里可以实现更智能的缓存清理
	// 暂时简单地忽略，因为内存缓存会自动过期
}

// AddHandler 添加路由处理函数
func (r *Router) AddHandler(action string, handler HandlerFunc) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	if r.handlers == nil {
		r.handlers = make(map[string]HandlerFunc)
	}
	
	r.handlers[action] = handler
}

// RemoveHandler 移除路由处理函数
func (r *Router) RemoveHandler(action string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	delete(r.handlers, action)
}
