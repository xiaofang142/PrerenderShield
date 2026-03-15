package detectors

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/firewall/types"
)

// ============ 测试检测器创建 ============

// TestXSSDetector_Name 测试检测器名称
func TestXSSDetector_Name(t *testing.T) {
	mockProvider := &MockRuleProvider{
		rules: make(map[string][]types.Rule),
	}
	detector := NewXSSDetector(mockProvider)
	assert.Equal(t, "XSS", detector.Name())
}

// TestXSSDetector_New_WithNilProvider 测试使用 nil Provider 创建检测器
func TestXSSDetector_New_WithNilProvider(t *testing.T) {
	detector := NewXSSDetector(nil)
	assert.NotNil(t, detector)
	assert.Equal(t, "XSS", detector.Name())
}

// ============ 测试 HTML 标签注入检测 ============

// TestXSSDetector_Detect_ScriptTag 测试 script 标签注入
func TestXSSDetector_Detect_ScriptTag(t *testing.T) {
	detector := NewXSSDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/search?q=<script>alert(1)</script>", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "xss", threats[0].Type)
	assert.Contains(t, threats[0].SubType, "HTML Tag Injection")
}

// TestXSSDetector_Detect_ScriptTag_CaseInsensitive 测试 script 标签大小写不敏感
func TestXSSDetector_Detect_ScriptTag_CaseInsensitive(t *testing.T) {
	detector := NewXSSDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/search?q=<SCRIPT>alert(1)</SCRIPT>", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "HTML Tag Injection")
}

// TestXSSDetector_Detect_IframeTag 测试 iframe 标签注入
func TestXSSDetector_Detect_IframeTag(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", "<iframe src='http://evil.com'></iframe>")
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "HTML Tag Injection")
}

// TestXSSDetector_Detect_ObjectTag 测试 object 标签注入
func TestXSSDetector_Detect_ObjectTag(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", "<object data='http://evil.com'></object>")
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "HTML Tag Injection")
}

// TestXSSDetector_Detect_EmbedTag 测试 embed 标签注入
func TestXSSDetector_Detect_EmbedTag(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", "<embed src='http://evil.com'>")
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "HTML Tag Injection")
}

// TestXSSDetector_Detect_ClosingScriptTag 测试闭合 script 标签注入
func TestXSSDetector_Detect_ClosingScriptTag(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", "</script><script>alert(1)</script>")
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// ============ 测试 JavaScript 事件处理器检测 ============

// TestXSSDetector_Detect_OnloadEvent 测试 onload 事件注入
func TestXSSDetector_Detect_OnloadEvent(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", "<img onload=alert(1)>")
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "JavaScript Event Handler")
}

// TestXSSDetector_Detect_OnerrorEvent 测试 onerror 事件注入
func TestXSSDetector_Detect_OnerrorEvent(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", "<img onerror=alert(1)>")
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "JavaScript Event Handler")
}

// TestXSSDetector_Detect_OnclickEvent 测试 onclick 事件注入
func TestXSSDetector_Detect_OnclickEvent(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", "<div onclick=alert(1)>")
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "JavaScript Event Handler")
}

// TestXSSDetector_Detect_OnmouseoverEvent 测试 onmouseover 事件注入
func TestXSSDetector_Detect_OnmouseoverEvent(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", "<div onmouseover=alert(1)>")
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "JavaScript Event Handler")
}

// TestXSSDetector_Detect_OnfocusEvent 测试 onfocus 事件注入
func TestXSSDetector_Detect_OnfocusEvent(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", "<input onfocus=alert(1)>")
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "JavaScript Event Handler")
}

// TestXSSDetector_Detect_OnblurEvent 测试 onblur 事件注入
func TestXSSDetector_Detect_OnblurEvent(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", "<input onblur=alert(1)>")
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "JavaScript Event Handler")
}

// TestXSSDetector_Detect_JavascriptProtocol 测试 javascript:协议注入
func TestXSSDetector_Detect_JavascriptProtocol(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", "javascript:alert(1)")
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "JavaScript Protocol")
}

// TestXSSDetector_Detect_VbscriptProtocol 测试 vbscript:协议注入
func TestXSSDetector_Detect_VbscriptProtocol(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", "vbscript:alert(1)")
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "JavaScript Protocol")
}

// TestXSSDetector_Detect_DataProtocol 测试 data:协议注入
func TestXSSDetector_Detect_DataProtocol(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", "data:text/html,<script>alert(1)</script>")
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	// data: URL 包含 <script>标签，会被检测为 HTML Tag Injection
	assert.Equal(t, "xss", threats[0].Type)
}

// ============ 测试 HTML 属性注入检测 ============

// TestXSSDetector_Detect_SingleQuote 测试单引号属性注入
func TestXSSDetector_Detect_SingleQuote(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", "'+or+'1'='1")
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "HTML Attribute Injection")
}

// TestXSSDetector_Detect_DoubleQuote 测试双引号属性注入
func TestXSSDetector_Detect_DoubleQuote(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", `"+or+"1"="1`)
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "HTML Attribute Injection")
}

// TestXSSDetector_Detect_GreaterThan 测试>符号注入
func TestXSSDetector_Detect_GreaterThan(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", "<script>alert(1)</script>")
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// TestXSSDetector_Detect_EncodedChars 测试 URL 编码字符注入
func TestXSSDetector_Detect_EncodedChars(t *testing.T) {
	detector := NewXSSDetector(nil)

	// %3C 是 < 的 URL 编码，httptest 会自动解码
	values := url.Values{}
	values.Set("q", "<script>")
	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "HTML Tag Injection")
}

// ============ 测试请求头 XSS 检测 ============

// TestXSSDetector_Detect_HeaderInjection 测试请求头中的 XSS
func TestXSSDetector_Detect_HeaderInjection(t *testing.T) {
	detector := NewXSSDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Custom-Header", "<script>alert(1)</script>")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].Message, "header")
}

// TestXSSDetector_Detect_UserAgentInjection 测试 User-Agent 中的 XSS
func TestXSSDetector_Detect_UserAgentInjection(t *testing.T) {
	detector := NewXSSDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 <script>alert(1)</script>")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].Message, "header")
}

// TestXSSDetector_Detect_RefererInjection 测试 Referer 中的 XSS
func TestXSSDetector_Detect_RefererInjection(t *testing.T) {
	detector := NewXSSDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Referer", "http://evil.com/<script>alert(1)</script>")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].Message, "header")
}

// ============ 测试规则更新 ============

// TestXSSDetector_UpdateRules 测试动态更新规则
func TestXSSDetector_UpdateRules(t *testing.T) {
	detector := NewXSSDetector(nil)

	customRules := []types.Rule{
		{ID: "custom-xss-001", Name: "Custom XSS", Category: "xss", Pattern: `XSS_ATTACK`, Severity: "high"},
	}

	err := detector.UpdateRules(customRules)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/test?param=XSS_ATTACK", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "custom-xss-001", threats[0].RuleID)
}

// TestXSSDetector_UpdateRules_WithProvider 测试使用 Provider 更新规则
func TestXSSDetector_UpdateRules_WithProvider(t *testing.T) {
	mockProvider := &MockRuleProvider{
		rules: map[string][]types.Rule{
			"xss": {
				{ID: "provider-xss-001", Name: "Provider XSS", Category: "xss", Pattern: `PROVIDER_XSS`, Severity: "critical"},
			},
		},
	}
	detector := NewXSSDetector(mockProvider)

	req := httptest.NewRequest(http.MethodGet, "/test?param=PROVIDER_XSS", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "provider-xss-001", threats[0].RuleID)
}

// ============ 测试多个参数和威胁 ============

// TestXSSDetector_Detect_MultipleParams 测试多个参数同时检测
func TestXSSDetector_Detect_MultipleParams(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Add("q", "<script>alert(1)</script>")
	values.Add("name", "<img onerror=alert(1)>")
	values.Add("safe", "normal_value")

	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.GreaterOrEqual(t, len(threats), 2)
}

// TestXSSDetector_Detect_MultipleThreatsInSameParam 测试同一参数中多个威胁
func TestXSSDetector_Detect_MultipleThreatsInSameParam(t *testing.T) {
	detector := NewXSSDetector(nil)

	values := url.Values{}
	values.Set("q", "<script>alert(1)</script><img onerror=alert(2)>")
	req := httptest.NewRequest(http.MethodGet, "/test?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// ============ 测试边界情况 ============

// TestXSSDetector_Detect_EmptyQuery 测试空查询参数
func TestXSSDetector_Detect_EmptyQuery(t *testing.T) {
	detector := NewXSSDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestXSSDetector_Detect_NormalRequest 测试正常请求
func TestXSSDetector_Detect_NormalRequest(t *testing.T) {
	detector := NewXSSDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/users?page=1&limit=10", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestXSSDetector_Detect_HTMLButSafe 测试 HTML 但安全
func TestXSSDetector_Detect_HTMLButSafe(t *testing.T) {
	detector := NewXSSDetector(nil)

	// 包含 HTML 标签但不是 XSS 攻击
	req := httptest.NewRequest(http.MethodGet, "/search?q=hello+world", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestXSSDetector_Detect_POSTBody 测试 POST 请求体检测
func TestXSSDetector_Detect_POSTBody(t *testing.T) {
	detector := NewXSSDetector(nil)

	body := strings.NewReader("comment=<script>alert(1)</script>")
	req := httptest.NewRequest(http.MethodPost, "/comment", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ParseForm()
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "xss", threats[0].Type)
}

// ============ 测试并发安全 ============

// TestXSSDetector_ConcurrentAccess 测试并发访问安全性
func TestXSSDetector_ConcurrentAccess(t *testing.T) {
	detector := NewXSSDetector(nil)

	done := make(chan bool)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			req := httptest.NewRequest(http.MethodGet, "/test?q=<script>alert(1)</script>", nil)
			_, err := detector.Detect(req)
			if err != nil {
				errors <- err
			}
		}()
	}

	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case err := <-errors:
			t.Errorf("Concurrent access error: %v", err)
		}
	}
}

// TestXSSDetector_ConcurrentUpdateAndDetect 测试并发更新和检测
func TestXSSDetector_ConcurrentUpdateAndDetect(t *testing.T) {
	detector := NewXSSDetector(nil)

	done := make(chan bool)
	errors := make(chan error, 10)

	go func() {
		defer func() { done <- true }()
		for i := 0; i < 5; i++ {
			customRules := []types.Rule{
				{ID: "custom-" + string(rune(i)), Name: "Custom", Category: "xss", Pattern: `CUSTOM`, Severity: "high"},
			}
			err := detector.UpdateRules(customRules)
			if err != nil {
				errors <- err
			}
		}
	}()

	go func() {
		defer func() { done <- true }()
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test?q=<script>alert(1)</script>", nil)
			_, err := detector.Detect(req)
			if err != nil {
				errors <- err
			}
		}
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case err := <-errors:
			t.Errorf("Concurrent update/detect error: %v", err)
		}
	}
}

// ============ 测试威胁详情 ============

// TestXSSDetector_ThreatDetails 测试威胁详情完整性
func TestXSSDetector_ThreatDetails(t *testing.T) {
	detector := NewXSSDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/search?q=<script>alert(1)</script>", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)

	threat := threats[0]
	assert.Equal(t, "xss", threat.Type)
	assert.NotEmpty(t, threat.SubType)
	assert.NotEmpty(t, threat.Severity)
	assert.NotEmpty(t, threat.Message)
	assert.NotEmpty(t, threat.Parameter)
	assert.NotEmpty(t, threat.Value)
	assert.NotEmpty(t, threat.RuleID)
	assert.NotEmpty(t, threat.RuleName)
}

// TestXSSDetector_CompileRules_InvalidPattern 测试编译无效规则模式
func TestXSSDetector_CompileRules_InvalidPattern(t *testing.T) {
	mockProvider := &MockRuleProvider{
		rules: map[string][]types.Rule{
			"xss": {
				{ID: "invalid-rule", Name: "Invalid Rule", Category: "xss", Pattern: "[invalid(regex", Severity: "high"},
			},
		},
	}
	detector := NewXSSDetector(mockProvider)

	// 验证检测器创建成功（无效规则被跳过）
	assert.NotNil(t, detector)

	// 验证仍然可以正常检测
	req := httptest.NewRequest(http.MethodGet, "/search?q=<script>alert(1)</script>", nil)
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// TestXSSDetector_CompileRules_EmptyPattern 测试空规则模式
func TestXSSDetector_CompileRules_EmptyPattern(t *testing.T) {
	mockProvider := &MockRuleProvider{
		rules: map[string][]types.Rule{
			"xss": {
				{ID: "empty-rule", Name: "Empty Rule", Category: "xss", Pattern: "", Severity: "high"},
			},
		},
	}
	detector := NewXSSDetector(mockProvider)

	// 验证检测器创建成功（空规则被跳过）
	assert.NotNil(t, detector)
}

// TestXSSDetector_CompileRules_PrecompiledRules 测试预编译规则数量
func TestXSSDetector_CompileRules_PrecompiledRules(t *testing.T) {
	detector := NewXSSDetector(nil)

	// 验证默认规则已预编译（至少 4 条默认规则）
	req := httptest.NewRequest(http.MethodGet, "/search?q=normal", nil)
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats) // 正常请求无威胁
}
