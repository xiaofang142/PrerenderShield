package i18n

import (
	"fmt"
	"sync"
)

// Translator 国际化翻译器

type Translator struct {
	messages map[string]map[string]string
	mutex    sync.RWMutex
}

// NewTranslator 创建新的翻译器
func NewTranslator() *Translator {
	return &Translator{
		messages: make(map[string]map[string]string),
	}
}

// AddMessages 添加语言消息
func (t *Translator) AddMessages(lang string, messages map[string]string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if _, exists := t.messages[lang]; !exists {
		t.messages[lang] = make(map[string]string)
	}

	for key, value := range messages {
		t.messages[lang][key] = value
	}
}

// Translate 翻译消息
func (t *Translator) Translate(lang, key string, args ...interface{}) string {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	// 优先使用指定语言
	if langMap, exists := t.messages[lang]; exists {
		if msg, exists := langMap[key]; exists {
			if len(args) > 0 {
				return fmt.Sprintf(msg, args...)
			}
			return msg
		}
	}

	// 回退到英文
	if langMap, exists := t.messages["en"]; exists {
		if msg, exists := langMap[key]; exists {
			if len(args) > 0 {
				return fmt.Sprintf(msg, args...)
			}
			return msg
		}
	}

	// 回退到键名
	return key
}

// LoadDefaultMessages 加载默认消息
func (t *Translator) LoadDefaultMessages() {
	// 英文消息
	t.AddMessages("en", map[string]string{
		"error.internal_server_error":  "Internal server error",
		"error.invalid_request":        "Invalid request",
		"error.not_found":              "Resource not found",
		"error.unauthorized":           "Unauthorized access",
		"error.forbidden":              "Forbidden access",
		"error.rate_limit_exceeded":    "Rate limit exceeded",
		"error.browser_pool_exhausted": "Browser pool exhausted",
		"error.render_timeout":         "Render timeout",
		"error.cache_error":            "Cache error",
		"error.firewall_blocked":       "Request blocked by firewall",
		"success.operation_completed":  "Operation completed successfully",
		"success.cache_hit":            "Cache hit",
		"success.cache_miss":           "Cache miss",
		"info.render_started":          "Render started",
		"info.render_completed":        "Render completed",
		"info.browser_launched":        "Browser launched",
		"info.browser_closed":          "Browser closed",
	})

	// 中文消息
	t.AddMessages("zh", map[string]string{
		"error.internal_server_error":  "服务器内部错误",
		"error.invalid_request":        "无效的请求",
		"error.not_found":              "资源未找到",
		"error.unauthorized":           "未授权访问",
		"error.forbidden":              "禁止访问",
		"error.rate_limit_exceeded":    "超出速率限制",
		"error.browser_pool_exhausted": "浏览器池耗尽",
		"error.render_timeout":         "渲染超时",
		"error.cache_error":            "缓存错误",
		"error.firewall_blocked":       "请求被防火墙阻止",
		"success.operation_completed":  "操作执行成功",
		"success.cache_hit":            "缓存命中",
		"success.cache_miss":           "缓存未命中",
		"info.render_started":          "渲染开始",
		"info.render_completed":        "渲染完成",
		"info.browser_launched":        "浏览器启动",
		"info.browser_closed":          "浏览器关闭",
	})
}

// 全局翻译器实例
var (
	globalTranslator *Translator
	once             sync.Once
)

// GetTranslator 获取全局翻译器
func GetTranslator() *Translator {
	once.Do(func() {
		globalTranslator = NewTranslator()
		globalTranslator.LoadDefaultMessages()
	})
	return globalTranslator
}

// T 便捷翻译函数
func T(lang, key string, args ...interface{}) string {
	return GetTranslator().Translate(lang, key, args...)
}
