package i18n

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTranslator(t *testing.T) {
	translator := NewTranslator()
	assert.NotNil(t, translator)
	assert.NotNil(t, translator.messages)
}

func TestTranslator_AddMessages(t *testing.T) {
	translator := NewTranslator()

	messages := map[string]string{
		"hello": "Hello",
		"world": "World",
	}

	translator.AddMessages("en", messages)

	translator.mutex.RLock()
	defer translator.mutex.RUnlock()

	assert.Contains(t, translator.messages["en"], "hello")
	assert.Contains(t, translator.messages["en"], "world")
}

func TestTranslator_AddMessages_MultipleLangs(t *testing.T) {
	translator := NewTranslator()

	translator.AddMessages("en", map[string]string{
		"hello": "Hello",
	})
	translator.AddMessages("zh", map[string]string{
		"hello": "你好",
	})
	translator.AddMessages("ja", map[string]string{
		"hello": "こんにちは",
	})

	translator.mutex.RLock()
	defer translator.mutex.RUnlock()

	assert.Len(t, translator.messages, 3)
	assert.Equal(t, "Hello", translator.messages["en"]["hello"])
	assert.Equal(t, "你好", translator.messages["zh"]["hello"])
	assert.Equal(t, "こんにちは", translator.messages["ja"]["hello"])
}

func TestTranslator_Translate_ExistingKey(t *testing.T) {
	translator := NewTranslator()
	translator.AddMessages("en", map[string]string{
		"greeting": "Hello, %s!",
	})

	result := translator.Translate("en", "greeting", "World")
	assert.Equal(t, "Hello, World!", result)
}

func TestTranslator_Translate_WithoutArgs(t *testing.T) {
	translator := NewTranslator()
	translator.AddMessages("en", map[string]string{
		"greeting": "Hello!",
	})

	result := translator.Translate("en", "greeting")
	assert.Equal(t, "Hello!", result)
}

func TestTranslator_Translate_FallbackToEnglish(t *testing.T) {
	translator := NewTranslator()
	translator.AddMessages("en", map[string]string{
		"greeting": "Hello!",
	})

	// 使用不存在的语言，应该回退到英文
	result := translator.Translate("fr", "greeting")
	assert.Equal(t, "Hello!", result)
}

func TestTranslator_Translate_FallbackToKey(t *testing.T) {
	translator := NewTranslator()
	translator.AddMessages("en", map[string]string{
		"other_key": "Other value",
	})

	// 使用不存在的键，应该返回键名
	result := translator.Translate("en", "nonexistent_key")
	assert.Equal(t, "nonexistent_key", result)
}

func TestTranslator_Translate_NonExistentLang(t *testing.T) {
	translator := NewTranslator()
	translator.AddMessages("en", map[string]string{
		"greeting": "Hello!",
	})

	// 使用不存在的语言和键
	result := translator.Translate("de", "nonexistent")
	assert.Equal(t, "nonexistent", result)
}

func TestTranslator_LoadDefaultMessages(t *testing.T) {
	translator := NewTranslator()
	translator.LoadDefaultMessages()

	// 测试英文消息
	assert.Equal(t, "Internal server error", translator.Translate("en", "error.internal_server_error"))
	assert.Equal(t, "Invalid request", translator.Translate("en", "error.invalid_request"))
	assert.Equal(t, "Resource not found", translator.Translate("en", "error.not_found"))
	assert.Equal(t, "Unauthorized access", translator.Translate("en", "error.unauthorized"))
	assert.Equal(t, "Forbidden access", translator.Translate("en", "error.forbidden"))
	assert.Equal(t, "Rate limit exceeded", translator.Translate("en", "error.rate_limit_exceeded"))
	assert.Equal(t, "Browser pool exhausted", translator.Translate("en", "error.browser_pool_exhausted"))
	assert.Equal(t, "Render timeout", translator.Translate("en", "error.render_timeout"))
	assert.Equal(t, "Cache error", translator.Translate("en", "error.cache_error"))
	assert.Equal(t, "Request blocked by firewall", translator.Translate("en", "error.firewall_blocked"))
	assert.Equal(t, "Operation completed successfully", translator.Translate("en", "success.operation_completed"))
	assert.Equal(t, "Cache hit", translator.Translate("en", "success.cache_hit"))
	assert.Equal(t, "Cache miss", translator.Translate("en", "success.cache_miss"))
	assert.Equal(t, "Render started", translator.Translate("en", "info.render_started"))
	assert.Equal(t, "Render completed", translator.Translate("en", "info.render_completed"))
	assert.Equal(t, "Browser launched", translator.Translate("en", "info.browser_launched"))
	assert.Equal(t, "Browser closed", translator.Translate("en", "info.browser_closed"))

	// 测试中文消息
	assert.Equal(t, "服务器内部错误", translator.Translate("zh", "error.internal_server_error"))
	assert.Equal(t, "无效的请求", translator.Translate("zh", "error.invalid_request"))
	assert.Equal(t, "资源未找到", translator.Translate("zh", "error.not_found"))
	assert.Equal(t, "未授权访问", translator.Translate("zh", "error.unauthorized"))
	assert.Equal(t, "禁止访问", translator.Translate("zh", "error.forbidden"))
	assert.Equal(t, "超出速率限制", translator.Translate("zh", "error.rate_limit_exceeded"))
	assert.Equal(t, "浏览器池耗尽", translator.Translate("zh", "error.browser_pool_exhausted"))
	assert.Equal(t, "渲染超时", translator.Translate("zh", "error.render_timeout"))
	assert.Equal(t, "缓存错误", translator.Translate("zh", "error.cache_error"))
	assert.Equal(t, "请求被防火墙阻止", translator.Translate("zh", "error.firewall_blocked"))
	assert.Equal(t, "操作执行成功", translator.Translate("zh", "success.operation_completed"))
	assert.Equal(t, "缓存命中", translator.Translate("zh", "success.cache_hit"))
	assert.Equal(t, "缓存未命中", translator.Translate("zh", "success.cache_miss"))
	assert.Equal(t, "渲染开始", translator.Translate("zh", "info.render_started"))
	assert.Equal(t, "渲染完成", translator.Translate("zh", "info.render_completed"))
	assert.Equal(t, "浏览器启动", translator.Translate("zh", "info.browser_launched"))
	assert.Equal(t, "浏览器关闭", translator.Translate("zh", "info.browser_closed"))
}

func TestGetTranslator(t *testing.T) {
	// 重置全局翻译器
	globalTranslator = nil
	once = sync.Once{}

	translator1 := GetTranslator()
	assert.NotNil(t, translator1)

	translator2 := GetTranslator()
	assert.Equal(t, translator1, translator2) // 应该是同一个实例

	// 检查是否加载了默认消息
	result := translator1.Translate("en", "error.internal_server_error")
	assert.Equal(t, "Internal server error", result)
}

func TestT(t *testing.T) {
	// 重置全局翻译器
	globalTranslator = nil
	once = sync.Once{}

	result := T("en", "error.internal_server_error")
	assert.Equal(t, "Internal server error", result)

	resultWithArgs := T("en", "success.operation_completed")
	assert.Equal(t, "Operation completed successfully", resultWithArgs)
}

func TestT_WithArgs(t *testing.T) {
	// 重置全局翻译器并添加自定义消息
	globalTranslator = nil
	once = sync.Once{}

	translator := GetTranslator()
	translator.AddMessages("en", map[string]string{
		"greeting": "Hello, %s!",
	})

	result := T("en", "greeting", "World")
	assert.Equal(t, "Hello, World!", result)
}

func TestTranslator_ConcurrentAccess(t *testing.T) {
	translator := NewTranslator()
	translator.LoadDefaultMessages()

	done := make(chan bool, 10)

	// 启动多个协程同时访问
	for i := 0; i < 10; i++ {
		go func() {
			translator.Translate("en", "error.internal_server_error")
			translator.Translate("zh", "error.internal_server_error")
			done <- true
		}()
	}

	// 等待所有协程完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestTranslator_PartialFallback(t *testing.T) {
	translator := NewTranslator()
	translator.AddMessages("en", map[string]string{
		"key1": "Value 1",
		"key2": "Value 2",
	})
	translator.AddMessages("zh", map[string]string{
		"key1": "值 1",
		// key2 不存在
	})

	// 中文中存在的键
	result1 := translator.Translate("zh", "key1")
	assert.Equal(t, "值 1", result1)

	// 中文中不存在的键，应该回退到英文
	result2 := translator.Translate("zh", "key2")
	assert.Equal(t, "Value 2", result2)
}
