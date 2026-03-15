package detectors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"prerender-shield/internal/config"
)

// ============ 测试检测器创建 ============

// TestGeoIPDetector_Name 测试检测器名称
func TestGeoIPDetector_Name(t *testing.T) {
	cfg := &config.GeoIPConfig{Enabled: false}
	detector := NewGeoIPDetector(cfg)
	assert.NotNil(t, detector)
	assert.Equal(t, "geoip", detector.Name())
}

// TestGeoIPDetector_NilConfig 测试空配置
func TestGeoIPDetector_NilConfig(t *testing.T) {
	detector := NewGeoIPDetector(nil)
	assert.NotNil(t, detector)
	assert.Equal(t, "geoip", detector.Name())
}

// ============ 测试地理位置访问控制禁用 ============

// TestGeoIPDetector_Detect_GeoIPDisabled 测试地理位置访问控制禁用
func TestGeoIPDetector_Detect_GeoIPDisabled(t *testing.T) {
	cfg := &config.GeoIPConfig{Enabled: false}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestGeoIPDetector_Detect_NilConfig 测试空配置
func TestGeoIPDetector_Detect_NilConfig(t *testing.T) {
	detector := NewGeoIPDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// ============ 测试 BlockList 阻止列表 ============

// TestGeoIPDetector_Detect_BlockedCountry 测试国家被阻止
func TestGeoIPDetector_Detect_BlockedCountry(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{"US", "RU", "CN"},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// 模拟模式：本地 IP 被视为 CN
	req.RemoteAddr = "127.0.0.1:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "geoip", threats[0].Type)
	assert.Equal(t, "country_block", threats[0].SubType)
	assert.Equal(t, "high", threats[0].Severity)
	assert.Contains(t, threats[0].Message, "blocked country")
}

// TestGeoIPDetector_Detect_BlockedCountry_NonLocalIP 测试非本地 IP 被阻止
func TestGeoIPDetector_Detect_BlockedCountry_NonLocalIP(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{"US"},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// 模拟模式：非本地 IP 被视为 US
	req.RemoteAddr = "8.8.8.8:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "geoip", threats[0].Type)
	assert.Equal(t, "country_block", threats[0].SubType)
}

// TestGeoIPDetector_Detect_NotInBlockList 测试不在阻止列表中
func TestGeoIPDetector_Detect_NotInBlockList(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{"RU", "CN"},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// 模拟模式：本地 IP 被视为 CN，但 CN 在阻止列表中
	// 需要使用非本地 IP 来测试不阻止的情况
	req.RemoteAddr = "192.168.1.100:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	// 非本地 IP 在模拟模式下被视为 US，不在阻止列表中
	assert.Empty(t, threats)
}

// TestGeoIPDetector_Detect_MultipleBlockedCountries 测试多个国家被阻止
func TestGeoIPDetector_Detect_MultipleBlockedCountries(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{"US", "RU", "CN", "KP", "IR"},
	}
	detector := NewGeoIPDetector(cfg)

	// 测试本地 IP（模拟为 CN）
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "geoip", threats[0].Type)
}

// ============ 测试 AllowList 允许列表 ============

// TestGeoIPDetector_Detect_AllowedCountry 测试国家在允许列表中
func TestGeoIPDetector_Detect_AllowedCountry(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		AllowList: []string{"CN", "US", "JP"},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// 模拟模式：本地 IP 被视为 CN
	req.RemoteAddr = "127.0.0.1:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestGeoIPDetector_Detect_NotInAllowList 测试国家不在允许列表中
func TestGeoIPDetector_Detect_NotInAllowList(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		AllowList: []string{"CN", "JP"},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// 模拟模式：非本地 IP 被视为 US
	req.RemoteAddr = "8.8.8.8:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "geoip", threats[0].Type)
	assert.Equal(t, "country_allow", threats[0].SubType)
	assert.Contains(t, threats[0].Message, "not in allow list")
}

// TestGeoIPDetector_Detect_EmptyAllowList 测试空允许列表
func TestGeoIPDetector_Detect_EmptyAllowList(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		AllowList: []string{},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "8.8.8.8:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	// 空允许列表不进行检查
	assert.Empty(t, threats)
}

// ============ 测试 BlockList 和 AllowList 组合 ============

// TestGeoIPDetector_Detect_BlockListTakesPrecedence 测试阻止列表优先级
func TestGeoIPDetector_Detect_BlockListTakesPrecedence(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{"CN"},
		AllowList: []string{"CN", "US"},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// 模拟模式：本地 IP 被视为 CN
	req.RemoteAddr = "127.0.0.1:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	// CN 在阻止列表中，应该被阻止
	assert.Equal(t, "country_block", threats[0].SubType)
}

// TestGeoIPDetector_Detect_BothListsNormal 测试两个列表都正常配置
func TestGeoIPDetector_Detect_BothListsNormal(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{"RU", "KP"},
		AllowList: []string{"CN", "US", "JP"},
	}
	detector := NewGeoIPDetector(cfg)

	// 测试允许的国家
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345" // CN

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// ============ 测试 IP 地址获取 ============

// TestGeoIPDetector_Detect_XForwardedFor 测试 X-Forwarded-For 头
func TestGeoIPDetector_Detect_XForwardedFor(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{"US"},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8, 10.0.0.1, 192.168.1.1")

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "geoip", threats[0].Type)
}

// TestGeoIPDetector_Detect_XRealIP 测试 X-Real-IP 头
func TestGeoIPDetector_Detect_XRealIP(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{"US"},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "geoip", threats[0].Type)
}

// TestGeoIPDetector_Detect_XForwardedFor_MultipleIPs 测试多个 X-Forwarded-For IP
func TestGeoIPDetector_Detect_XForwardedFor_MultipleIPs(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{"US"},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8,10.0.0.1,192.168.1.1")

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// TestGeoIPDetector_Detect_RemoteAddr 测试 RemoteAddr
func TestGeoIPDetector_Detect_RemoteAddr(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{"CN"},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// TestGeoIPDetector_Detect_RemoteAddr_WithPort 测试带端口的 RemoteAddr
func TestGeoIPDetector_Detect_RemoteAddr_WithPort(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		AllowList: []string{"CN"},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:8080"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats) // 127.0.0.1 在模拟模式下是 CN，在允许列表中
}

// TestGeoIPDetector_Detect_RemoteAddr_IPv6 测试 IPv6 地址
func TestGeoIPDetector_Detect_RemoteAddr_IPv6(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		AllowList: []string{"CN", "US"},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "[::1]:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	// IPv6 地址 [::1] 在模拟模式下被视为 US（非本地 IPv4）
	// US 在允许列表中，所以不应该有威胁
	assert.Empty(t, threats)
}

// TestGeoIPDetector_Detect_EmptyIP 测试空 IP
func TestGeoIPDetector_Detect_EmptyIP(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{"US"},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = ""

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// ============ 测试 getClientIP 辅助函数 ============

// TestGetClientIP_XForwardedFor 测试获取 X-Forwarded-For IP
func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100, 10.0.0.1")

	ip := getClientIP(req)
	assert.Equal(t, "192.168.1.100", ip)
}

// TestGetClientIP_XForwardedFor_SingleIP 测试单个 X-Forwarded-For IP
func TestGetClientIP_XForwardedFor_SingleIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100")

	ip := getClientIP(req)
	assert.Equal(t, "192.168.1.100", ip)
}

// TestGetClientIP_XRealIP 测试获取 X-Real-IP IP
func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.100")

	ip := getClientIP(req)
	assert.Equal(t, "192.168.1.100", ip)
}

// TestGetClientIP_RemoteAddr 测试获取 RemoteAddr IP
func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	ip := getClientIP(req)
	assert.Equal(t, "192.168.1.100", ip)
}

// TestGetClientIP_RemoteAddr_NoPort 测试不带端口的 RemoteAddr
func TestGetClientIP_RemoteAddr_NoPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100"

	ip := getClientIP(req)
	assert.Equal(t, "192.168.1.100", ip)
}

// TestGetClientIP_Priority 测试 IP 获取优先级
func TestGetClientIP_Priority(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "1.1.1.1")
	req.Header.Set("X-Real-IP", "2.2.2.2")
	req.RemoteAddr = "3.3.3.3:12345"

	ip := getClientIP(req)
	// X-Forwarded-For 优先级最高
	assert.Equal(t, "1.1.1.1", ip)
}

// TestGetClientIP_XForwardedFor_WithSpaces 测试带空格的 X-Forwarded-For
func TestGetClientIP_XForwardedFor_WithSpaces(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "  192.168.1.100  , 10.0.0.1")

	ip := getClientIP(req)
	assert.Equal(t, "192.168.1.100", ip)
}

// ============ 测试威胁详情 ============

// TestGeoIPDetector_ThreatDetails_Block 测试阻止的威胁详情
func TestGeoIPDetector_ThreatDetails_Block(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{"CN"},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)

	threat := threats[0]
	assert.Equal(t, "geoip", threat.Type)
	assert.Equal(t, "country_block", threat.SubType)
	assert.Equal(t, "high", threat.Severity)
	assert.NotEmpty(t, threat.Message)
	assert.NotEmpty(t, threat.SourceIP)
	assert.NotNil(t, threat.Details)
	assert.Contains(t, threat.Details, "country")
}

// TestGeoIPDetector_ThreatDetails_Allow 测试允许的威胁详情
func TestGeoIPDetector_ThreatDetails_Allow(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		AllowList: []string{"US"},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)

	threat := threats[0]
	assert.Equal(t, "geoip", threat.Type)
	assert.Equal(t, "country_allow", threat.SubType)
	assert.Equal(t, "high", threat.Severity)
	assert.Contains(t, threat.Message, "not in allow list")
}

// ============ 测试边界情况 ============

// TestGeoIPDetector_Detect_EmptyBlockList 测试空阻止列表
func TestGeoIPDetector_Detect_EmptyBlockList(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{},
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "8.8.8.8:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestGeoIPDetector_Detect_NilBlockList 测试 nil 阻止列表
func TestGeoIPDetector_Detect_NilBlockList(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: nil,
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "8.8.8.8:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestGeoIPDetector_Detect_NilAllowList 测试 nil 允许列表
func TestGeoIPDetector_Detect_NilAllowList(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		AllowList: nil,
	}
	detector := NewGeoIPDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "8.8.8.8:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// ============ 测试并发安全 ============

// TestGeoIPDetector_ConcurrentAccess 测试并发访问安全性
func TestGeoIPDetector_ConcurrentAccess(t *testing.T) {
	cfg := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{"US", "RU"},
		AllowList: []string{"CN", "JP"},
	}
	detector := NewGeoIPDetector(cfg)

	done := make(chan bool)
	errors := make(chan error, 20)

	for i := 0; i < 20; i++ {
		go func() {
			defer func() { done <- true }()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = "8.8.8.8:12345"
			_, err := detector.Detect(req)
			if err != nil {
				errors <- err
			}
		}()
	}

	for i := 0; i < 20; i++ {
		select {
		case <-done:
		case err := <-errors:
			t.Errorf("Concurrent access error: %v", err)
		}
	}
}
