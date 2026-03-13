package detectors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBlacklistDetector_Name 测试检测器名称
func TestBlacklistDetector_Name(t *testing.T) {
	detector := NewBlacklistDetector(nil, "site1", []string{}, []string{})
	assert.Equal(t, "blacklist", detector.Name())
}

// TestBlacklistDetector_Detect_StaticBlacklist 测试静态黑名单匹配
func TestBlacklistDetector_Detect_StaticBlacklist(t *testing.T) {
	detector := NewBlacklistDetector(nil, "site1", []string{"192.168.1.100"}, []string{})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "blacklist", threats[0].Type)
	assert.Equal(t, "critical", threats[0].Severity)
	assert.Contains(t, threats[0].Message, "matches static blacklist")
}

// TestBlacklistDetector_Detect_StaticWhitelist 测试静态白名单放行
func TestBlacklistDetector_Detect_StaticWhitelist(t *testing.T) {
	detector := NewBlacklistDetector(nil, "site1", []string{"192.168.1.100"}, []string{"192.168.1.50"})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.50:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestBlacklistDetector_Detect_NotInList 测试不在黑白名单的情况
func TestBlacklistDetector_Detect_NotInList(t *testing.T) {
	detector := NewBlacklistDetector(nil, "site1", []string{"192.168.1.100"}, []string{"192.168.1.50"})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.200:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestBlacklistDetector_Detect_MultipleBlacklist 测试多个黑名单 IP
func TestBlacklistDetector_Detect_MultipleBlacklist(t *testing.T) {
	blacklist := []string{"192.168.1.100", "10.0.0.50", "172.16.0.1"}
	detector := NewBlacklistDetector(nil, "site1", blacklist, []string{})

	for _, blockedIP := range blacklist {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = blockedIP + ":12345"

		threats, err := detector.Detect(req)
		assert.NoError(t, err)
		assert.NotEmpty(t, threats)
		assert.Equal(t, "blacklist", threats[0].Type)
	}
}

// TestBlacklistDetector_Detect_WhitelistTakesPrecedence 测试白名单优先级高于黑名单
func TestBlacklistDetector_Detect_WhitelistTakesPrecedence(t *testing.T) {
	// IP 同时在黑白名单中，应该被放行
	detector := NewBlacklistDetector(nil, "site1", []string{"192.168.1.100"}, []string{"192.168.1.100"})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestBlacklistDetector_Detect_NilRedis 测试 Redis 为 nil 时的行为
func TestBlacklistDetector_Detect_NilRedis(t *testing.T) {
	// Redis 为 nil 时，只检查静态黑白名单
	detector := NewBlacklistDetector(nil, "site1", []string{}, []string{})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestBlacklistDetector_EmptyLists 测试空黑白名单
func TestBlacklistDetector_EmptyLists(t *testing.T) {
	detector := NewBlacklistDetector(nil, "site1", []string{}, []string{})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestBlacklistDetector_NilLists 测试 nil 黑白名单
func TestBlacklistDetector_NilLists(t *testing.T) {
	detector := NewBlacklistDetector(nil, "site1", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}
