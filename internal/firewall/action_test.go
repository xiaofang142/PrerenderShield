package firewall

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"prerender-shield/internal/firewall/types"
)

// TestDefaultActionHandler_Handle_Allowed 测试处理允许的请求
func TestDefaultActionHandler_Handle_Allowed(t *testing.T) {
	handler := NewDefaultActionHandler(ActionConfig{
		BlockMessage: "Blocked",
	}, "/tmp", "test-site")

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	w := httptest.NewRecorder()

	result := &CheckResult{
		Allow: true,
	}

	allowed := handler.Handle(w, req, result)
	assert.True(t, allowed)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestDefaultActionHandler_Handle_Blocked 测试处理被阻止的请求
func TestDefaultActionHandler_Handle_Blocked(t *testing.T) {
	handler := NewDefaultActionHandler(ActionConfig{
		BlockMessage: "Request blocked by WAF",
	}, "/tmp", "test-site")

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	w := httptest.NewRecorder()

	result := &CheckResult{
		Allow:   false,
		Threats: []types.Threat{{Type: "injection", Message: "SQL injection detected"}},
	}

	allowed := handler.Handle(w, req, result)
	assert.False(t, allowed)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Request blocked by WAF")
}

// TestDefaultActionHandler_Handle_Blocked_WithCustomPage 测试使用自定义拦截页面
func TestDefaultActionHandler_Handle_Blocked_WithCustomPage(t *testing.T) {
	// 创建临时目录和自定义页面
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "test-site")
	os.MkdirAll(siteDir, 0755)

	customPage := filepath.Join(siteDir, "waf_block.html")
	err := os.WriteFile(customPage, []byte("<html><body>Custom block page</body></html>"), 0644)
	assert.NoError(t, err)

	handler := NewDefaultActionHandler(ActionConfig{
		BlockMessage: "Blocked",
	}, tmpDir, "test-site")

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	w := httptest.NewRecorder()

	result := &CheckResult{
		Allow: false,
	}

	allowed := handler.Handle(w, req, result)
	assert.False(t, allowed)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Custom block page")
}

// TestDefaultActionHandler_Handle_Blocked_EmptyMessage 测试空阻止消息
func TestDefaultActionHandler_Handle_Blocked_EmptyMessage(t *testing.T) {
	handler := NewDefaultActionHandler(ActionConfig{
		BlockMessage: "",
	}, "/tmp", "test-site")

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	w := httptest.NewRecorder()

	result := &CheckResult{
		Allow: false,
	}

	allowed := handler.Handle(w, req, result)
	assert.False(t, allowed)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Access Denied by WAF")
}

// TestNewDefaultActionHandler 测试创建默认动作处理器
func TestNewDefaultActionHandler(t *testing.T) {
	config := ActionConfig{
		DefaultAction: "block",
		BlockMessage:  "Test message",
	}

	handler := NewDefaultActionHandler(config, "/static", "my-site")
	assert.NotNil(t, handler)
	assert.Equal(t, "block", handler.config.DefaultAction)
	assert.Equal(t, "Test message", handler.config.BlockMessage)
	assert.Equal(t, "/static", handler.staticDir)
	assert.Equal(t, "my-site", handler.siteName)
}
