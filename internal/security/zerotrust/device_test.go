package zerotrust

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultDeviceFingerprintConfig(t *testing.T) {
	config := DefaultDeviceFingerprintConfig()

	assert.NotNil(t, config)
	assert.Equal(t, true, config.EnableCanvas)
	assert.Equal(t, true, config.EnableWebGL)
	assert.Equal(t, 24*time.Hour, config.SessionTimeout)
	assert.Equal(t, 0.7, config.EmulatorThreshold)
}

func TestNewDeviceFingerprintEngine(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultDeviceFingerprintConfig()

	engine := NewDeviceFingerprintEngine(config, logger)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.cache)
	assert.NotNil(t, engine.trustDB)
}

func TestDeviceFingerprintEngine_GenerateFingerprint(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewDeviceFingerprintEngine(DefaultDeviceFingerprintConfig(), logger)

	headers := map[string]string{
		"user-agent": "Mozilla/5.0",
		"accept":     "text/html",
	}

	fingerprint := engine.GenerateFingerprint(
		"192.168.1.1",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/91.0",
		"1920x1080",
		"Asia/Shanghai",
		"zh-CN",
		"webgl-hash-123",
		"audio-hash-456",
		"canvas-hash-789",
		"fonts-hash-abc",
		[]string{"plugin1", "plugin2"},
		headers,
		"ja3-hash-xyz",
	)

	assert.NotNil(t, fingerprint)
	assert.NotEmpty(t, fingerprint.ID)
	assert.NotEmpty(t, fingerprint.FingerprintHash)
	assert.Equal(t, "192.168.1.1", fingerprint.IP)
	assert.False(t, fingerprint.IsEmulator)
	assert.False(t, fingerprint.IsVM)
}

func TestDeviceFingerprintEngine_DetectEmulator(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewDeviceFingerprintEngine(DefaultDeviceFingerprintConfig(), logger)

	tests := []struct {
		name         string
		userAgent    string
		plugins      []string
		expectEmulator bool
	}{
		{
			name:         "Normal browser",
			userAgent:    "Mozilla/5.0 Chrome/91.0",
			plugins:      []string{"Flash", "PDF"},
			expectEmulator: false,
		},
		{
			name:         "BlueStacks",
			userAgent:    "Mozilla/5.0 (BlueStacks)",
			plugins:      []string{},
			expectEmulator: true,
		},
		{
			name:         "Genymotion",
			userAgent:    "Mozilla/5.0 (Genymotion)",
			plugins:      []string{},
			expectEmulator: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.detectEmulator(tt.userAgent, tt.plugins)
			assert.Equal(t, tt.expectEmulator, result)
		})
	}
}

func TestDeviceFingerprintEngine_DetectVM(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewDeviceFingerprintEngine(DefaultDeviceFingerprintConfig(), logger)

	tests := []struct {
		name      string
		userAgent string
		expectVM  bool
	}{
		{
			name:      "Normal browser",
			userAgent: "Mozilla/5.0 Chrome/91.0",
			expectVM:  false,
		},
		{
			name:      "VirtualBox",
			userAgent: "Mozilla/5.0 (VirtualBox)",
			expectVM:  true,
		},
		{
			name:      "VMware",
			userAgent: "Mozilla/5.0 (VMware)",
			expectVM:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.detectVM(tt.userAgent, []string{})
			assert.Equal(t, tt.expectVM, result)
		})
	}
}

func TestDeviceFingerprintEngine_CalculateTrustScore(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewDeviceFingerprintEngine(DefaultDeviceFingerprintConfig(), logger)

	fingerprint := &DeviceFingerprint{
		ID:         "test-device",
		IsEmulator: false,
		IsVM:       false,
	}

	behavior := &DeviceBehavior{
		BrowserConsistency: true,
		TimezoneConsistent: true,
		ActiveHours:        []int{9, 10, 11, 14, 15, 16},
	}

	score := engine.CalculateTrustScore("test-device", fingerprint, behavior)

	assert.Greater(t, score, 50.0)
	assert.LessOrEqual(t, score, 100.0)
}

func TestDeviceFingerprintEngine_UpdateTrustOnEvent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewDeviceFingerprintEngine(DefaultDeviceFingerprintConfig(), logger)

	deviceID := "trust-test-device"

	// 初始信任
	engine.UpdateTrustOnEvent(deviceID, "normal_activity", true)

	trust := engine.GetDeviceTrust(deviceID)
	assert.NotNil(t, trust)
	assert.Greater(t, trust.TrustScore, 0.0)

	// 成功登录
	engine.UpdateTrustOnEvent(deviceID, "login_success", true)
	trust = engine.GetDeviceTrust(deviceID)
	assert.Greater(t, trust.SuccessfulAuths, int64(0))

	// 失败登录
	engine.UpdateTrustOnEvent(deviceID, "login_success", false)
	trust = engine.GetDeviceTrust(deviceID)
	assert.Greater(t, trust.FailedAuths, int64(0))
}

func TestDeviceFingerprintEngine_DetermineTrustLevel(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewDeviceFingerprintEngine(DefaultDeviceFingerprintConfig(), logger)

	tests := []struct {
		score    float64
		expected string
	}{
		{85, "trusted"},
		{70, "verified"},
		{50, "unknown"},
		{30, "suspicious"},
		{10, "blocked"},
	}

	for _, tt := range tests {
		level := engine.determineTrustLevel(tt.score)
		assert.Equal(t, tt.expected, level, "score=%f", tt.score)
	}
}

func TestDeviceCache(t *testing.T) {
	cache := NewDeviceCache(100, 1*time.Second)

	fp := &DeviceFingerprint{
		ID:         "test-fp",
		FingerprintHash: "hash-123",
		IP:         "192.168.1.1",
	}

	cache.Set("test-key", fp)

	// 立即获取应该存在
	cached := cache.Get("test-key")
	assert.NotNil(t, cached)
	assert.Equal(t, "test-fp", cached.ID)

	// 等待过期
	time.Sleep(1500 * time.Millisecond)

	cached = cache.Get("test-key")
	assert.Nil(t, cached)
}

func TestDeviceTrustDB(t *testing.T) {
	db := NewDeviceTrustDB()

	trust := &DeviceTrust{
		DeviceID:   "test-device",
		TrustScore: 75.0,
		TrustLevel: "verified",
		IsKnown:    true,
	}

	db.Set("test-device", trust)

	// 获取
	retrieved := db.Get("test-device")
	assert.NotNil(t, retrieved)
	assert.Equal(t, "test-device", retrieved.DeviceID)
	assert.Equal(t, 75.0, retrieved.TrustScore)

	// 删除
	db.Delete("test-device")
	retrieved = db.Get("test-device")
	assert.Nil(t, retrieved)
}

func TestDeviceFingerprintEngine_GetStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewDeviceFingerprintEngine(DefaultDeviceFingerprintConfig(), logger)

	// 生成一些指纹
	for i := 0; i < 5; i++ {
		engine.GenerateFingerprint(
			"192.168.1."+string(rune('1'+i)),
			"Mozilla/5.0",
			"1920x1080",
			"Asia/Shanghai",
			"zh-CN",
			"webgl-hash",
			"audio-hash",
			"canvas-hash",
			"fonts-hash",
			[]string{"plugin"},
			map[string]string{},
			"ja3-hash",
		)
	}

	stats := engine.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, int64(5), stats.TotalDevices)
}

func TestDeviceFingerprint(t *testing.T) {
	now := time.Now()
	fp := &DeviceFingerprint{
		ID:              "dev-test123",
		FingerprintHash: "hash-abc-xyz",
		IP:              "192.168.1.100",
		UserAgent:       "Mozilla/5.0 Chrome/91.0",
		ScreenRes:       "1920x1080",
		Timezone:        "Asia/Shanghai",
		Language:        "zh-CN",
		WebGLHash:       "webgl-123",
		AudioHash:       "audio-456",
		CanvasHash:      "canvas-789",
		FontsHash:       "fonts-abc",
		Plugins:         []string{"Flash", "PDF"},
		Confidence:      0.95,
		IsEmulator:      false,
		IsVM:            false,
		RiskScore:       10.0,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	assert.NotNil(t, fp)
	assert.Equal(t, "dev-test123", fp.ID)
	assert.Equal(t, 0.95, fp.Confidence)
	assert.Equal(t, 10.0, fp.RiskScore)
}

func TestDeviceTrust(t *testing.T) {
	now := time.Now()
	trust := &DeviceTrust{
		DeviceID:        "dev-trust-test",
		TrustScore:      85.0,
		TrustLevel:      "trusted",
		FirstSeen:       now,
		LastSeen:        now,
		TotalVisits:     100,
		FailedAuths:     2,
		SuccessfulAuths: 98,
		LastIP:          "192.168.1.1",
		LastUserAgent:   "Mozilla/5.0",
		IsKnown:         true,
		IsBlocked:       false,
		IsVerified:      true,
		Tags:            []string{"premium", "verified"},
	}

	assert.NotNil(t, trust)
	assert.Equal(t, "trusted", trust.TrustLevel)
	assert.Equal(t, 85.0, trust.TrustScore)
	assert.True(t, trust.IsKnown)
}

func TestDeviceBehavior(t *testing.T) {
	now := time.Now()
	behavior := &DeviceBehavior{
		DeviceID:           "dev-behavior-test",
		AvgSessionDuration: 300.5,
		AvgPagesPerSession: 5.2,
		ActiveHours:        []int{9, 10, 14, 15, 16},
		ActiveDays:         []int{1, 2, 3, 4, 5},
		TypicalIPs:         []string{"192.168.1.1", "192.168.1.2"},
		TypicalLocations:   []string{"Beijing", "Shanghai"},
		BrowserConsistency: true,
		TimezoneConsistent: true,
		LastUpdated:        now,
	}

	assert.NotNil(t, behavior)
	assert.Len(t, behavior.ActiveHours, 5)
	assert.True(t, behavior.BrowserConsistency)
}

func TestDeviceFingerprintEngine_CalculateRiskScore(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewDeviceFingerprintEngine(DefaultDeviceFingerprintConfig(), logger)

	tests := []struct {
		name       string
		isEmulator bool
		isVM       bool
		plugins    []string
		userAgent  string
		expectMin  float64
	}{
		{
			name:       "Normal device",
			isEmulator: false,
			isVM:       false,
			plugins:    []string{"Flash", "PDF"},
			userAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/91.0",
			expectMin:  0,
		},
		{
			name:       "Emulator",
			isEmulator: true,
			isVM:       false,
			plugins:    []string{},
			userAgent:  "Mozilla/5.0 (BlueStacks)",
			expectMin:  30,
		},
		{
			name:       "VM",
			isEmulator: false,
			isVM:       true,
			plugins:    []string{},
			userAgent:  "Mozilla/5.0 (VirtualBox)",
			expectMin:  20,
		},
		{
			name:       "Short UA",
			isEmulator: false,
			isVM:       false,
			plugins:    []string{},
			userAgent:  "short",
			expectMin:  25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := engine.calculateRiskScore(tt.isEmulator, tt.isVM, tt.plugins, tt.userAgent)
			assert.GreaterOrEqual(t, score, tt.expectMin)
		})
	}
}
