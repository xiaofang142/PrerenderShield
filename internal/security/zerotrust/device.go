package zerotrust

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"prerender-shield/internal/logging"
)

// DeviceFingerprint 设备指纹
type DeviceFingerprint struct {
	ID              string            `json:"id"`
	FingerprintHash string            `json:"fingerprint_hash"`
	IP              string            `json:"ip"`
	UserAgent       string            `json:"user_agent"`
	ScreenRes       string            `json:"screen_res,omitempty"`
	Timezone        string            `json:"timezone,omitempty"`
	Language        string            `json:"language,omitempty"`
	Platform        string            `json:"platform,omitempty"`
	WebGLHash       string            `json:"webgl_hash,omitempty"`
	AudioHash       string            `json:"audio_hash,omitempty"`
	CanvasHash      string            `json:"canvas_hash,omitempty"`
	FontsHash       string            `json:"fonts_hash,omitempty"`
	Plugins         []string          `json:"plugins,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	TLSFingerprint  string            `json:"tls_fingerprint,omitempty"`
	Confidence      float64           `json:"confidence"`
	IsEmulator      bool              `json:"is_emulator"`
	IsVM            bool              `json:"is_vm"`
	RiskScore       float64           `json:"risk_score"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// DeviceTrust 设备信任状态
type DeviceTrust struct {
	DeviceID        string    `json:"device_id"`
	TrustScore      float64   `json:"trust_score"` // 0-100
	TrustLevel      string    `json:"trust_level"` // trusted, verified, unknown, suspicious, blocked
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	TotalVisits     int64     `json:"total_visits"`
	FailedAuths     int64     `json:"failed_auths"`
	SuccessfulAuths int64     `json:"successful_auths"`
	LastIP          string    `json:"last_ip"`
	LastUserAgent   string    `json:"last_user_agent"`
	LastLocation    *Location `json:"last_location,omitempty"`
	IsKnown         bool      `json:"is_known"`
	IsBlocked       bool      `json:"is_blocked"`
	IsVerified      bool      `json:"is_verified"`
	Tags            []string  `json:"tags,omitempty"`
	Notes           string    `json:"notes,omitempty"`
}

// Location 位置信息
type Location struct {
	Country  string  `json:"country"`
	Region   string  `json:"region"`
	City     string  `json:"city"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Timezone string  `json:"timezone"`
	Accuracy float64 `json:"accuracy,omitempty"`
}

// DeviceBehavior 设备行为特征
type DeviceBehavior struct {
	DeviceID           string    `json:"device_id"`
	AvgSessionDuration float64   `json:"avg_session_duration"` // 平均会话时长（秒）
	AvgPagesPerSession float64   `json:"avg_pages_per_session"`
	ActiveHours        []int     `json:"active_hours"` // 活跃小时 [0-23]
	ActiveDays         []int     `json:"active_days"`  // 活跃天数 [0-6]
	TypicalIPs         []string  `json:"typical_ips"`
	TypicalLocations   []string  `json:"typical_locations"`
	BrowserConsistency bool      `json:"browser_consistency"`
	TimezoneConsistent bool      `json:"timezone_consistent"`
	LastUpdated        time.Time `json:"last_updated"`
}

// DeviceFingerprintConfig 设备指纹配置
type DeviceFingerprintConfig struct {
	EnableCanvas         bool `json:"enable_canvas"`
	EnableWebGL          bool `json:"enable_webgl"`
	EnableAudio          bool `json:"enable_audio"`
	EnableFonts          bool `json:"enable_fonts"`
	EnablePlugins        bool `json:"enable_plugins"`
	EnableScreen         bool `json:"enable_screen"`
	EnableTimezone       bool `json:"enable_timezone"`
	EnableLanguage       bool `json:"enable_language"`
	EnableHeaders        bool `json:"enable_headers"`
	EnableTLSFingerprint bool `json:"enable_tls_fingerprint"`

	// 信任配置
	TrustDecayRate float64       `json:"trust_decay_rate"`
	TrustBoostRate float64       `json:"trust_boost_rate"`
	MinTrustScore  float64       `json:"min_trust_score"`
	MaxTrustScore  float64       `json:"max_trust_score"`
	SessionTimeout time.Duration `json:"session_timeout"`

	// 阈值
	EmulatorThreshold float64 `json:"emulator_threshold"`
	VMThreshold       float64 `json:"vm_threshold"`

	// 缓存配置
	CacheSize int           `json:"cache_size"`
	CacheTTL  time.Duration `json:"cache_ttl"`
}

// DefaultDeviceFingerprintConfig 返回默认配置
func DefaultDeviceFingerprintConfig() *DeviceFingerprintConfig {
	return &DeviceFingerprintConfig{
		EnableCanvas:         true,
		EnableWebGL:          true,
		EnableAudio:          true,
		EnableFonts:          true,
		EnablePlugins:        true,
		EnableScreen:         true,
		EnableTimezone:       true,
		EnableLanguage:       true,
		EnableHeaders:        true,
		EnableTLSFingerprint: true,

		TrustDecayRate: 0.01,
		TrustBoostRate: 0.05,
		MinTrustScore:  0,
		MaxTrustScore:  100,
		SessionTimeout: 24 * time.Hour,

		EmulatorThreshold: 0.7,
		VMThreshold:       0.7,

		CacheSize: 10000,
		CacheTTL:  24 * time.Hour,
	}
}

// DeviceFingerprintEngine 设备指纹引擎
type DeviceFingerprintEngine struct {
	config  *DeviceFingerprintConfig
	logger  *zap.Logger
	cache   *DeviceCache
	trustDB *DeviceTrustDB
	mu      sync.RWMutex
	stats   *DeviceStats
}

// DeviceCache 设备缓存
type DeviceCache struct {
	data map[string]*DeviceCacheEntry
	mu   sync.RWMutex
	ttl  time.Duration
	size int
}

// DeviceCacheEntry 缓存条目
type DeviceCacheEntry struct {
	Fingerprint *DeviceFingerprint
	ExpiresAt   time.Time
}

// DeviceTrustDB 设备信任数据库（内存存储）
type DeviceTrustDB struct {
	devices map[string]*DeviceTrust
	mu      sync.RWMutex
}

// DeviceStats 设备统计
type DeviceStats struct {
	TotalDevices      int64 `json:"total_devices"`
	KnownDevices      int64 `json:"known_devices"`
	UnknownDevices    int64 `json:"unknown_devices"`
	TrustedDevices    int64 `json:"trusted_devices"`
	BlockedDevices    int64 `json:"blocked_devices"`
	EmulatorsDetected int64 `json:"emulators_detected"`
	VMsDetected       int64 `json:"vms_detected"`
	CacheHits         int64 `json:"cache_hits"`
	CacheMisses       int64 `json:"cache_misses"`
}

// NewDeviceFingerprintEngine 创建设备指纹引擎
func NewDeviceFingerprintEngine(config *DeviceFingerprintConfig, logger *zap.Logger) *DeviceFingerprintEngine {
	if config == nil {
		config = DefaultDeviceFingerprintConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	engine := &DeviceFingerprintEngine{
		config:  config,
		logger:  logger,
		cache:   NewDeviceCache(config.CacheSize, config.CacheTTL),
		trustDB: NewDeviceTrustDB(),
		stats:   &DeviceStats{},
	}

	return engine
}

// NewDeviceCache 创建设备缓存
func NewDeviceCache(size int, ttl time.Duration) *DeviceCache {
	cache := &DeviceCache{
		data: make(map[string]*DeviceCacheEntry, size),
		ttl:  ttl,
		size: size,
	}

	// 启动清理协程
	go cache.cleanupWorker()

	return cache
}

// NewDeviceTrustDB 创建设备信任数据库
func NewDeviceTrustDB() *DeviceTrustDB {
	return &DeviceTrustDB{
		devices: make(map[string]*DeviceTrust),
	}
}

// GenerateFingerprint 生成设备指纹
func (e *DeviceFingerprintEngine) GenerateFingerprint(
	ip string,
	userAgent string,
	screenRes string,
	timezone string,
	language string,
	webglHash string,
	audioHash string,
	canvasHash string,
	fontsHash string,
	plugins []string,
	headers map[string]string,
	tlsFingerprint string,
) *DeviceFingerprint {
	e.stats.TotalDevices++

	// 生成指纹哈希
	fingerprintData := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s",
		ip, userAgent, screenRes, timezone, language,
		webglHash, audioHash, canvasHash, fontsHash,
	)
	fingerprintHash := e.hashString(fingerprintData)

	// 检测设备类型
	isEmulator := e.detectEmulator(userAgent, plugins)
	isVM := e.detectVM(userAgent, plugins)

	// 计算风险分数
	riskScore := e.calculateRiskScore(isEmulator, isVM, plugins, userAgent)

	// 生成设备 ID
	deviceID := e.generateDeviceID(fingerprintHash)

	fingerprint := &DeviceFingerprint{
		ID:              deviceID,
		FingerprintHash: fingerprintHash,
		IP:              ip,
		UserAgent:       userAgent,
		ScreenRes:       screenRes,
		Timezone:        timezone,
		Language:        language,
		WebGLHash:       webglHash,
		AudioHash:       audioHash,
		CanvasHash:      canvasHash,
		FontsHash:       fontsHash,
		Plugins:         plugins,
		Headers:         headers,
		TLSFingerprint:  tlsFingerprint,
		Confidence:      0.8,
		IsEmulator:      isEmulator,
		IsVM:            isVM,
		RiskScore:       riskScore,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// 更新统计
	if isEmulator {
		e.stats.EmulatorsDetected++
	}
	if isVM {
		e.stats.VMsDetected++
	}

	// 缓存指纹
	e.cache.Set(fingerprintHash, fingerprint)

	e.logger.Debug("生成设备指纹",
		zap.String("device_id", deviceID),
		zap.Bool("is_emulator", isEmulator),
		zap.Bool("is_vm", isVM),
		zap.Float64("risk_score", riskScore),
	)

	return fingerprint
}

// GetDeviceTrust 获取设备信任状态
func (e *DeviceFingerprintEngine) GetDeviceTrust(deviceID string) *DeviceTrust {
	return e.trustDB.Get(deviceID)
}

// UpdateDeviceTrust 更新设备信任状态
func (e *DeviceFingerprintEngine) UpdateDeviceTrust(deviceID string, trust *DeviceTrust) {
	e.trustDB.Set(deviceID, trust)
}

// CalculateTrustScore 计算信任分数
func (e *DeviceFingerprintEngine) CalculateTrustScore(
	deviceID string,
	fingerprint *DeviceFingerprint,
	behavior *DeviceBehavior,
) float64 {
	score := 50.0 // 基础分数

	// 1. 基于设备特征的信任
	if !fingerprint.IsEmulator && !fingerprint.IsVM {
		score += 10
	}

	// 2. 基于历史行为
	if behavior != nil {
		// 浏览器一致性
		if behavior.BrowserConsistency {
			score += 5
		}
		// 时区一致性
		if behavior.TimezoneConsistent {
			score += 5
		}
		// 活跃模式正常
		if len(behavior.ActiveHours) > 0 {
			score += 5
		}
	}

	// 3. 基于设备历史
	existingTrust := e.trustDB.Get(deviceID)
	if existingTrust != nil {
		// 已知设备加分
		if existingTrust.IsKnown {
			score += 15
		}
		// 验证设备加分
		if existingTrust.IsVerified {
			score += 20
		}
		// 成功认证历史
		if existingTrust.SuccessfulAuths > 0 {
			score += float64(existingTrust.SuccessfulAuths) * 2
		}
		// 失败认证减分
		if existingTrust.FailedAuths > 0 {
			score -= float64(existingTrust.FailedAuths) * 5
		}
	}

	// 限制分数范围
	if score < e.config.MinTrustScore {
		score = e.config.MinTrustScore
	}
	if score > e.config.MaxTrustScore {
		score = e.config.MaxTrustScore
	}

	return score
}

// UpdateTrustOnEvent 根据事件更新信任
func (e *DeviceFingerprintEngine) UpdateTrustOnEvent(deviceID string, event string, success bool) {
	trust := e.trustDB.Get(deviceID)
	if trust == nil {
		trust = &DeviceTrust{
			DeviceID:   deviceID,
			TrustScore: 50,
			FirstSeen:  time.Now(),
			LastSeen:   time.Now(),
		}
	}

	switch event {
	case "login_success":
		if success {
			trust.SuccessfulAuths++
			trust.TrustScore += e.config.TrustBoostRate * 10
		} else {
			trust.FailedAuths++
			trust.TrustScore -= 10
		}
	case "challenge_passed":
		if success {
			trust.TrustScore += e.config.TrustBoostRate * 5
		}
	case "challenge_failed":
		trust.TrustScore -= 5
	case "normal_activity":
		// 正常活动轻微提升信任
		if success {
			trust.TrustScore += e.config.TrustBoostRate
		}
	}

	// 限制分数范围
	if trust.TrustScore < e.config.MinTrustScore {
		trust.TrustScore = e.config.MinTrustScore
	}
	if trust.TrustScore > e.config.MaxTrustScore {
		trust.TrustScore = e.config.MaxTrustScore
	}

	// 更新信任等级
	trust.TrustLevel = e.determineTrustLevel(trust.TrustScore)
	trust.LastSeen = time.Now()

	e.trustDB.Set(deviceID, trust)
}

// determineTrustLevel 确定信任等级
func (e *DeviceFingerprintEngine) determineTrustLevel(score float64) string {
	if score >= 80 {
		return "trusted"
	}
	if score >= 60 {
		return "verified"
	}
	if score >= 40 {
		return "unknown"
	}
	if score >= 20 {
		return "suspicious"
	}
	return "blocked"
}

// detectEmulator 检测模拟器
func (e *DeviceFingerprintEngine) detectEmulator(userAgent string, plugins []string) bool {
	// 检测模拟器特征
	emulatorPatterns := []string{
		"android-sdk",
		"genymotion",
		"bluestacks",
		"nox",
		"memu",
		"ldplayer",
	}

	ua := userAgent
	for _, pattern := range emulatorPatterns {
		if containsIgnoreCase(ua, pattern) {
			return true
		}
	}

	// 检测异常插件
	for _, plugin := range plugins {
		if containsIgnoreCase(plugin, "emulator") {
			return true
		}
	}

	return false
}

// detectVM 检测虚拟机
func (e *DeviceFingerprintEngine) detectVM(userAgent string, plugins []string) bool {
	// 检测虚拟机特征
	vmPatterns := []string{
		"virtualbox",
		"vmware",
		"parallels",
		"qemu",
		"kvm",
		"hyperv",
	}

	ua := userAgent
	for _, pattern := range vmPatterns {
		if containsIgnoreCase(ua, pattern) {
			return true
		}
	}

	return false
}

// calculateRiskScore 计算风险分数
func (e *DeviceFingerprintEngine) calculateRiskScore(
	isEmulator bool,
	isVM bool,
	plugins []string,
	userAgent string,
) float64 {
	score := 0.0

	if isEmulator {
		score += 30
	}
	if isVM {
		score += 20
	}

	// 缺少插件
	if len(plugins) == 0 {
		score += 10
	}

	// 异常 User-Agent
	if len(userAgent) < 20 {
		score += 15
	}

	return score
}

// Get 获取缓存
func (c *DeviceCache) Get(key string) *DeviceFingerprint {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.data[key]
	if !exists {
		return nil
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil
	}

	return entry.Fingerprint
}

// Set 设置缓存
func (c *DeviceCache) Set(key string, fp *DeviceFingerprint) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果缓存已满，删除最旧的
	if len(c.data) >= c.size {
		c.evictOldest()
	}

	c.data[key] = &DeviceCacheEntry{
		Fingerprint: fp,
		ExpiresAt:   time.Now().Add(c.ttl),
	}
}

// evictOldest 删除最旧的条目
func (c *DeviceCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.data {
		if oldestKey == "" || entry.ExpiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.ExpiresAt
		}
	}

	if oldestKey != "" {
		delete(c.data, oldestKey)
	}
}

// cleanupWorker 清理工
func (c *DeviceCache) cleanupWorker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup 清理过期条目
func (c *DeviceCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	deleted := 0

	for key, entry := range c.data {
		if now.After(entry.ExpiresAt) {
			delete(c.data, key)
			deleted++
		}
	}

	if deleted > 0 {
		logging.DefaultLogger.Info("清理过期设备缓存：%d 条\n", deleted)
	}
}

// Get 获取设备信任
func (db *DeviceTrustDB) Get(deviceID string) *DeviceTrust {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.devices[deviceID]
}

// Set 设置设备信任
func (db *DeviceTrustDB) Set(deviceID string, trust *DeviceTrust) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.devices[deviceID] = trust
}

// Delete 删除设备信任
func (db *DeviceTrustDB) Delete(deviceID string) {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.devices, deviceID)
}

// List 列出所有设备
func (db *DeviceTrustDB) List() []*DeviceTrust {
	db.mu.RLock()
	defer db.mu.RUnlock()

	result := make([]*DeviceTrust, 0, len(db.devices))
	for _, trust := range db.devices {
		result = append(result, trust)
	}
	return result
}

// hashString 生成哈希
func (e *DeviceFingerprintEngine) hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// generateDeviceID 生成设备 ID
func (e *DeviceFingerprintEngine) generateDeviceID(fingerprintHash string) string {
	return "dev_" + fingerprintHash[:16]
}

// GetStats 获取统计
func (e *DeviceFingerprintEngine) GetStats() *DeviceStats {
	// 更新设备统计
	e.stats.KnownDevices = int64(len(e.trustDB.devices))
	e.stats.UnknownDevices = e.stats.TotalDevices - e.stats.KnownDevices

	// 计算信任设备数
	for _, trust := range e.trustDB.devices {
		if trust.TrustLevel == "trusted" || trust.TrustLevel == "verified" {
			e.stats.TrustedDevices++
		}
		if trust.IsBlocked {
			e.stats.BlockedDevices++
		}
	}

	return e.stats
}

// containsIgnoreCase 忽略大小写检查包含
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					indexIgnoreCase(s, substr) >= 0)))
}

func indexIgnoreCase(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalIgnoreCase(s[i:i+len(substr)], substr) {
			return i
		}
	}
	return -1
}

func equalIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
