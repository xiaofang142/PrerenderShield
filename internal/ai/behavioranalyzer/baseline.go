package behavioranalyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// UserBehaviorBaseline 用户行为基线服务
type UserBehaviorBaseline struct {
	config       *BaselineConfig
	userProfiles map[string]*UserProfile
	ipProfiles   map[string]*IPProfile
	mu           sync.RWMutex
	logger       *zap.Logger
	stats        *BaselineStats
}

// BaselineConfig 基线配置
type BaselineConfig struct {
	// 学习期配置
	LearningPeriod    time.Duration // 学习期时长 (默认 7 天)
	MinSamplesForBase int           // 建立基线所需最小样本数

	// 偏离检测配置
	DeviationThreshold float64 // 偏离阈值 (标准差倍数)
	AnomalyThreshold   int     // 异常判定阈值 (偏离维度数)

	// 时间窗口配置
	HourlyWindow    bool // 启用小时级基线
	DayOfWeekWindow bool // 启用星期级基线

	// 地理位置配置
	EnableGeoBaseline bool
	MaxGeoDistance    float64 // 最大地理距离 (km)

	// 设备指纹配置
	EnableDeviceBaseline bool

	// 存储配置
	MaxUserProfiles int
	MaxHistoryDays  int
	CacheTTL        time.Duration

	// 衰减配置
	DecayFactor float64 // 日衰减因子
}

// UserProfile 用户行为画像
type UserProfile struct {
	UserID       string                 `json:"user_id"`
	FirstSeen    time.Time              `json:"first_seen"`
	LastSeen     time.Time              `json:"last_seen"`
	TotalEvents  int64                  `json:"total_events"`

	// 时间基线
	HourlyBaseline   map[int]*TimeStats   `json:"hourly_baseline"`   // 0-23 点
	DayOfWeekBaseline map[int]*TimeStats  `json:"dow_baseline"`      // 0-6 (周日 - 周六)

	// 行为基线
	RequestBaseline   *BehaviorStats `json:"request_baseline"`
	ErrorBaseline     *BehaviorStats `json:"error_baseline"`
	LatencyBaseline   *LatencyStats  `json:"latency_baseline"`

	// 地理位置基线
	GeoBaseline       *GeoStats      `json:"geo_baseline"`

	// 设备基线
	DeviceBaseline    *DeviceStats   `json:"device_baseline"`

	//  URI 基线
	URIBaseline       map[string]int64 `json:"uri_baseline"` // 常用 URI 及访问次数

	// 状态
	IsBaselineReady bool `json:"is_baseline_ready"` // 基线是否已建立

	mu sync.RWMutex
}

// TimeStats 时间统计
type TimeStats struct {
	Count      int64   `json:"count"`
	Mean       float64 `json:"mean"`
	StdDev     float64 `json:"std_dev"`
	Min        int64   `json:"min"`
	Max        int64   `json:"max"`
	LastUpdate time.Time `json:"last_update"`
}

// BehaviorStats 行为统计
type BehaviorStats struct {
	Count           int64       `json:"count"`
	RequestsPerMin  float64     `json:"requests_per_min"`
	StdDev          float64     `json:"std_dev"`
	UniqueURIs      int64       `json:"unique_uris"`
	UniqueMethods   int64       `json:"unique_methods"`
	CommonMethods   map[string]int64 `json:"common_methods"`
	CommonStatuses  map[string]int64 `json:"common_statuses"`
	LastUpdate      time.Time   `json:"last_update"`
}

// LatencyStats 延迟统计
type LatencyStats struct {
	Count      int64   `json:"count"`
	Mean       float64 `json:"mean"`
	StdDev     float64 `json:"std_dev"`
	P50        float64 `json:"p50"`
	P90        float64 `json:"p90"`
	P95        float64 `json:"p95"`
	P99        float64 `json:"p99"`
	Min        float64 `json:"min"`
	Max        float64 `json:"max"`
	LastUpdate time.Time `json:"last_update"`
	values     []float64 // 用于计算分位数
}

// GeoStats 地理位置统计
type GeoStats struct {
	Count         int64              `json:"count"`
	Countries     map[string]int64   `json:"countries"`
	Cities        map[string]int64   `json:"cities"`
	Latitudes     []float64          `json:"latitudes"`
	Longitudes    []float64          `json:"longitudes"`
	MeanLat       float64            `json:"mean_lat"`
	MeanLon        float64           `json:"mean_lon"`
	StdDevDistance float64           `json:"std_dev_distance"`
	LastUpdate    time.Time          `json:"last_update"`
}

// DeviceStats 设备统计
type DeviceStats struct {
	Count         int64              `json:"count"`
	UserAgents    map[string]int64   `json:"user_agents"`
	Browsers      map[string]int64   `json:"browsers"`
	OS            map[string]int64   `json:"os"`
	Devices       map[string]int64   `json:"devices"`
	Resolutions   map[string]int64   `json:"resolutions"`
	CommonHash    string             `json:"common_hash"` // 常见设备指纹 hash
	LastUpdate    time.Time          `json:"last_update"`
}

// IPProfile IP 画像
type IPProfile struct {
	IP            string                 `json:"ip"`
	FirstSeen     time.Time              `json:"first_seen"`
	LastSeen      time.Time              `json:"last_seen"`
	TotalEvents   int64                  `json:"total_events"`
	UserIDs       map[string]int64       `json:"user_ids"` // 关联的用户 ID
	RequestBaseline *BehaviorStats       `json:"request_baseline"`
	GeoBaseline     *GeoStats            `json:"geo_baseline"`
	mu            sync.RWMutex
}

// BaselineStats 基线服务统计
type BaselineStats struct {
	TotalUsers      int64 `json:"total_users"`
	TotalIPs        int64 `json:"total_ips"`
	ReadyUsers      int64 `json:"ready_users"`
	ReadyIPs        int64 `json:"ready_ips"`
	LearningUsers   int64 `json:"learning_users"`
	LearningIPs     int64 `json:"learning_ips"`
}

// BaselineEvent 基线事件
type BaselineEvent struct {
	UserID     string                 `json:"user_id,omitempty"`
	IP         string                 `json:"ip"`
	Timestamp  time.Time              `json:"timestamp"`
	URI        string                 `json:"uri"`
	Method     string                 `json:"method"`
	StatusCode int                    `json:"status_code"`
	Latency    float64                `json:"latency,omitempty"`
	Country    string                 `json:"country,omitempty"`
	City       string                 `json:"city,omitempty"`
	Latitude   float64                `json:"latitude,omitempty"`
	Longitude  float64                `json:"longitude,omitempty"`
	UserAgent  string                 `json:"user_agent,omitempty"`
	DeviceID   string                 `json:"device_id,omitempty"`
	Extra      map[string]interface{} `json:"extra,omitempty"`
}

// DeviationResult 偏离检测结果
type DeviationResult struct {
	UserID        string             `json:"user_id"`
	IP            string             `json:"ip"`
	IsAnomaly     bool               `json:"is_anomaly"`
	DeviationScore float64           `json:"deviation_score"` // 偏离分数 0-100
	DeviationDims  []DeviationDimension `json:"deviation_dims"` // 偏离维度
	Confidence    float64            `json:"confidence"`
	Timestamp     time.Time          `json:"timestamp"`
}

// DeviationDimension 偏离维度
type DeviationDimension struct {
	Dimension  string  `json:"dimension"`
	Expected   string  `json:"expected"`
	Actual     string  `json:"actual"`
	Deviation  float64 `json:"deviation"` // 偏离程度 (标准差)
	Severity   string  `json:"severity"`  // low/medium/high/critical
}

// DefaultBaselineConfig 返回默认配置
func DefaultBaselineConfig() *BaselineConfig {
	return &BaselineConfig{
		LearningPeriod:       7 * 24 * time.Hour, // 7 天学习期
		MinSamplesForBase:    100,                // 最少 100 个样本
		DeviationThreshold:   2.0,                // 2 倍标准差
		AnomalyThreshold:     2,                  // 2 个维度偏离即为异常
		HourlyWindow:         true,
		DayOfWeekWindow:      true,
		EnableGeoBaseline:    true,
		MaxGeoDistance:       100,                // 100km
		EnableDeviceBaseline: true,
		MaxUserProfiles:      100000,
		MaxHistoryDays:       90,
		CacheTTL:             1 * time.Hour,
		DecayFactor:          0.02,               // 每日衰减 2%
	}
}

// NewUserBehaviorBaseline 创建用户行为基线服务
func NewUserBehaviorBaseline(config *BaselineConfig, logger *zap.Logger) *UserBehaviorBaseline {
	if config == nil {
		config = DefaultBaselineConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	baseline := &UserBehaviorBaseline{
		config:       config,
		userProfiles: make(map[string]*UserProfile),
		ipProfiles:   make(map[string]*IPProfile),
		logger:       logger,
		stats:        &BaselineStats{},
	}

	// 启动定期衰减协程
	go baseline.decayWorker()

	return baseline
}

// RecordEvent 记录事件并更新基线
func (b *UserBehaviorBaseline) RecordEvent(ctx context.Context, event BaselineEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 更新用户画像
	if event.UserID != "" {
		profile := b.getOrCreateUserProfile(event.UserID)
		b.updateUserProfile(profile, event)
	}

	// 更新 IP 画像
	ipProfile := b.getOrCreateIPProfile(event.IP)
	b.updateIPProfile(ipProfile, event)

	b.logger.Debug("记录行为事件",
		zap.String("user_id", event.UserID),
		zap.String("ip", event.IP),
		zap.String("uri", event.URI))
}

// CheckDeviation 检查行为偏离
func (b *UserBehaviorBaseline) CheckDeviation(ctx context.Context, event BaselineEvent) *DeviationResult {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := &DeviationResult{
		UserID:        event.UserID,
		IP:            event.IP,
		IsAnomaly:     false,
		DeviationScore: 0,
		DeviationDims:  make([]DeviationDimension, 0),
		Timestamp:     time.Now(),
	}

	// 检查用户基线偏离
	if event.UserID != "" {
		profile, exists := b.userProfiles[event.UserID]
		if exists && profile.IsBaselineReady {
			dims := b.checkUserDeviation(profile, event)
			result.DeviationDims = append(result.DeviationDims, dims...)
		}
	}

	// 检查 IP 基线偏离
	ipProfile, exists := b.ipProfiles[event.IP]
	if exists && ipProfile.TotalEvents >= int64(b.config.MinSamplesForBase) {
		dims := b.checkIPDeviation(ipProfile, event)
		result.DeviationDims = append(result.DeviationDims, dims...)
	}

	// 计算偏离分数
	if len(result.DeviationDims) > 0 {
		maxDeviation := 0.0
		for _, dim := range result.DeviationDims {
			if dim.Deviation > maxDeviation {
				maxDeviation = dim.Deviation
			}
		}
		result.DeviationScore = math.Min(100, maxDeviation*20) // 转换为 0-100
		result.IsAnomaly = len(result.DeviationDims) >= b.config.AnomalyThreshold
		result.Confidence = math.Min(100, float64(len(result.DeviationDims))*25)
	}

	return result
}

// GetProfile 获取用户画像
func (b *UserBehaviorBaseline) GetProfile(userID string) *UserProfile {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.userProfiles[userID]
}

// GetIPProfile 获取 IP 画像
func (b *UserBehaviorBaseline) GetIPProfile(ip string) *IPProfile {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.ipProfiles[ip]
}

// GetStats 获取统计信息
func (b *UserBehaviorBaseline) GetStats() *BaselineStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	readyUsers := int64(0)
	readyIPs := int64(0)

	for _, p := range b.userProfiles {
		if p.IsBaselineReady {
			readyUsers++
		}
	}
	for _, p := range b.ipProfiles {
		if p.TotalEvents >= int64(b.config.MinSamplesForBase) {
			readyIPs++
		}
	}

	return &BaselineStats{
		TotalUsers:    int64(len(b.userProfiles)),
		TotalIPs:      int64(len(b.ipProfiles)),
		ReadyUsers:    readyUsers,
		ReadyIPs:      readyIPs,
		LearningUsers: int64(len(b.userProfiles)) - readyUsers,
		LearningIPs:   int64(len(b.ipProfiles)) - readyIPs,
	}
}

// Clear 清空基线数据
func (b *UserBehaviorBaseline) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.userProfiles = make(map[string]*UserProfile)
	b.ipProfiles = make(map[string]*IPProfile)
	b.stats = &BaselineStats{}
}

func (b *UserBehaviorBaseline) getOrCreateUserProfile(userID string) *UserProfile {
	profile, exists := b.userProfiles[userID]
	if !exists {
		profile = &UserProfile{
			UserID:            userID,
			FirstSeen:         time.Now(),
			HourlyBaseline:    make(map[int]*TimeStats),
			DayOfWeekBaseline: make(map[int]*TimeStats),
			URIBaseline:       make(map[string]int64),
		}
		b.userProfiles[userID] = profile

		// 限制用户数量
		if len(b.userProfiles) > b.config.MaxUserProfiles {
			b.evictOldestUserProfile()
		}
	}
	return profile
}

func (b *UserBehaviorBaseline) getOrCreateIPProfile(ip string) *IPProfile {
	profile, exists := b.ipProfiles[ip]
	if !exists {
		profile = &IPProfile{
			IP:        ip,
			FirstSeen: time.Now(),
			UserIDs:   make(map[string]int64),
		}
		b.ipProfiles[ip] = profile
	}
	return profile
}

func (b *UserBehaviorBaseline) updateUserProfile(profile *UserProfile, event BaselineEvent) {
	profile.mu.Lock()
	defer profile.mu.Unlock()

	profile.TotalEvents++
	profile.LastSeen = event.Timestamp

	// 更新小时基线
	hour := event.Timestamp.Hour()
	b.updateTimeStats(profile.HourlyBaseline, hour, 1)

	// 更新星期基线
	dow := int(event.Timestamp.Weekday())
	b.updateTimeStats(profile.DayOfWeekBaseline, dow, 1)

	// 更新 URI 基线
	profile.URIBaseline[event.URI]++

	// 更新请求基线
	if profile.RequestBaseline == nil {
		profile.RequestBaseline = &BehaviorStats{
			CommonMethods:  make(map[string]int64),
			CommonStatuses: make(map[string]int64),
		}
	}
	b.updateBehaviorStats(profile.RequestBaseline, event)

	// 更新延迟基线
	if event.Latency > 0 {
		if profile.LatencyBaseline == nil {
			profile.LatencyBaseline = &LatencyStats{
				values: make([]float64, 0, 1000),
			}
		}
		b.updateLatencyStats(profile.LatencyBaseline, event.Latency)
	}

	// 更新地理基线
	if b.config.EnableGeoBaseline && event.Latitude != 0 && event.Longitude != 0 {
		if profile.GeoBaseline == nil {
			profile.GeoBaseline = &GeoStats{
				Countries:  make(map[string]int64),
				Cities:     make(map[string]int64),
				Latitudes:  make([]float64, 0, 100),
				Longitudes: make([]float64, 0, 100),
			}
		}
		b.updateGeoStats(profile.GeoBaseline, event)
	}

	// 更新设备基线
	if b.config.EnableDeviceBaseline && event.UserAgent != "" {
		if profile.DeviceBaseline == nil {
			profile.DeviceBaseline = &DeviceStats{
				UserAgents:  make(map[string]int64),
				Browsers:    make(map[string]int64),
				OS:          make(map[string]int64),
				Devices:     make(map[string]int64),
				Resolutions: make(map[string]int64),
			}
		}
		b.updateDeviceStats(profile.DeviceBaseline, event)
	}

	// 检查基线是否就绪
	if profile.TotalEvents >= int64(b.config.MinSamplesForBase) {
		profile.IsBaselineReady = true
	}
}

func (b *UserBehaviorBaseline) updateIPProfile(profile *IPProfile, event BaselineEvent) {
	profile.mu.Lock()
	defer profile.mu.Unlock()

	profile.TotalEvents++
	profile.LastSeen = event.Timestamp

	// 关联用户 ID
	if event.UserID != "" {
		profile.UserIDs[event.UserID]++
	}

	// 更新请求基线
	if profile.RequestBaseline == nil {
		profile.RequestBaseline = &BehaviorStats{
			CommonMethods:  make(map[string]int64),
			CommonStatuses: make(map[string]int64),
		}
	}
	b.updateBehaviorStats(profile.RequestBaseline, event)

	// 更新地理基线
	if b.config.EnableGeoBaseline && event.Country != "" {
		if profile.GeoBaseline == nil {
			profile.GeoBaseline = &GeoStats{
				Countries: make(map[string]int64),
				Cities:    make(map[string]int64),
			}
		}
		if event.Country != "" {
			profile.GeoBaseline.Countries[event.Country]++
		}
		if event.City != "" {
			profile.GeoBaseline.Cities[event.City]++
		}
	}
}

func (b *UserBehaviorBaseline) updateTimeStats(stats map[int]*TimeStats, key int, value int64) {
	if _, exists := stats[key]; !exists {
		stats[key] = &TimeStats{}
	}
	s := stats[key]
	s.Count++
	s.LastUpdate = time.Now()

	// 在线更新均值和方差
	delta := float64(value) - s.Mean
	s.Mean += delta / float64(s.Count)
	if s.Count > 1 {
		s.StdDev = math.Sqrt(s.StdDev*s.StdDev + delta*delta/float64(s.Count))
	}
}

func (b *UserBehaviorBaseline) updateBehaviorStats(stats *BehaviorStats, event BaselineEvent) {
	stats.Count++
	stats.LastUpdate = time.Now()

	stats.CommonMethods[event.Method]++
	stats.CommonStatuses[fmt.Sprintf("%d", event.StatusCode)]++

	// 计算每分钟请求数 (简化)
	duration := event.Timestamp.Sub(stats.LastUpdate).Minutes()
	if duration > 0 {
		stats.RequestsPerMin = float64(stats.Count) / duration
	}
}

func (b *UserBehaviorBaseline) updateLatencyStats(stats *LatencyStats, latency float64) {
	stats.Count++
	stats.LastUpdate = time.Now()

	// 更新最值
	if latency < stats.Min || stats.Count == 1 {
		stats.Min = latency
	}
	if latency > stats.Max {
		stats.Max = latency
	}

	// 在线更新均值
	delta := latency - stats.Mean
	stats.Mean += delta / float64(stats.Count)

	// 保存部分值用于分位数计算
	if len(stats.values) < 1000 {
		stats.values = append(stats.values, latency)
	}

	// 简化计算标准差
	if stats.Count > 1 {
		stats.StdDev = math.Sqrt(stats.StdDev*stats.StdDev + delta*delta/float64(stats.Count))
	}

	// 每 100 次计算一次分位数
	if stats.Count%100 == 0 {
		b.calculatePercentiles(stats)
	}
}

func (b *UserBehaviorBaseline) calculatePercentiles(stats *LatencyStats) {
	if len(stats.values) == 0 {
		return
	}

	sorted := make([]float64, len(stats.values))
	copy(sorted, stats.values)

	// 简单排序
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	stats.P50 = percentile(sorted, 0.50)
	stats.P90 = percentile(sorted, 0.90)
	stats.P95 = percentile(sorted, 0.95)
	stats.P99 = percentile(sorted, 0.99)
}

func (b *UserBehaviorBaseline) updateGeoStats(stats *GeoStats, event BaselineEvent) {
	stats.Count++
	stats.LastUpdate = time.Now()

	if event.Country != "" {
		stats.Countries[event.Country]++
	}
	if event.City != "" {
		stats.Cities[event.City]++
	}

	// 保存坐标用于距离计算
	if len(stats.Latitudes) < 100 {
		stats.Latitudes = append(stats.Latitudes, event.Latitude)
		stats.Longitudes = append(stats.Longitudes, event.Longitude)
	}

	// 更新平均坐标
	deltaLat := event.Latitude - stats.MeanLat
	stats.MeanLat += deltaLat / float64(stats.Count)
	deltaLon := event.Longitude - stats.MeanLon
	stats.MeanLon += deltaLon / float64(stats.Count)
}

func (b *UserBehaviorBaseline) updateDeviceStats(stats *DeviceStats, event BaselineEvent) {
	stats.Count++
	stats.LastUpdate = time.Now()

	stats.UserAgents[event.UserAgent]++

	// 解析 User-Agent (简化版)
	browser, os := parseUserAgent(event.UserAgent)
	stats.Browsers[browser]++
	stats.OS[os]++

	if event.DeviceID != "" {
		stats.Devices[event.DeviceID]++
	}
}

func (b *UserBehaviorBaseline) checkUserDeviation(profile *UserProfile, event BaselineEvent) []DeviationDimension {
	dims := make([]DeviationDimension, 0)

	// 检查小时偏离
	if hourStats, exists := profile.HourlyBaseline[event.Timestamp.Hour()]; exists {
		if hourStats.Count > 0 {
			currentCount := float64(1)
			deviation := calculateZScore(currentCount, hourStats.Mean, hourStats.StdDev)
			if math.Abs(deviation) > b.config.DeviationThreshold {
				dims = append(dims, DeviationDimension{
					Dimension: "hour_of_day",
					Expected:  fmt.Sprintf("%.1f requests/hour", hourStats.Mean),
					Actual:    fmt.Sprintf("%.0f requests", currentCount),
					Deviation: deviation,
					Severity:  getSeverity(deviation),
				})
			}
		}
	}

	// 检查星期偏离
	if dowStats, exists := profile.DayOfWeekBaseline[int(event.Timestamp.Weekday())]; exists {
		if dowStats.Count > 0 {
			currentCount := float64(1)
			deviation := calculateZScore(currentCount, dowStats.Mean, dowStats.StdDev)
			if math.Abs(deviation) > b.config.DeviationThreshold {
				dims = append(dims, DeviationDimension{
					Dimension: "day_of_week",
					Expected:  fmt.Sprintf("%.1f requests/day", dowStats.Mean),
					Actual:    fmt.Sprintf("%.0f requests", currentCount),
					Deviation: deviation,
					Severity:  getSeverity(deviation),
				})
			}
		}
	}

	// 检查延迟偏离
	if profile.LatencyBaseline != nil && event.Latency > 0 {
		deviation := calculateZScore(event.Latency, profile.LatencyBaseline.Mean, profile.LatencyBaseline.StdDev)
		if math.Abs(deviation) > b.config.DeviationThreshold {
			dims = append(dims, DeviationDimension{
				Dimension: "latency",
				Expected:  fmt.Sprintf("%.0fms (σ=%.0f)", profile.LatencyBaseline.Mean, profile.LatencyBaseline.StdDev),
				Actual:    fmt.Sprintf("%.0fms", event.Latency),
				Deviation: deviation,
				Severity:  getSeverity(deviation),
			})
		}
	}

	// 检查地理偏离
	if b.config.EnableGeoBaseline && profile.GeoBaseline != nil && event.Latitude != 0 {
		distance := haversineDistance(
			profile.GeoBaseline.MeanLat, profile.GeoBaseline.MeanLon,
			event.Latitude, event.Longitude,
		)
		if distance > b.config.MaxGeoDistance {
			dims = append(dims, DeviationDimension{
				Dimension: "location",
				Expected:  fmt.Sprintf("%.2f, %.2f (%.0fkm)", profile.GeoBaseline.MeanLat, profile.GeoBaseline.MeanLon, 0.0),
				Actual:    fmt.Sprintf("%.2f, %.2f (%.0fkm from mean)", event.Latitude, event.Longitude, distance),
				Deviation: distance / b.config.MaxGeoDistance,
				Severity:  getSeverity(distance / b.config.MaxGeoDistance),
			})
		}
	}

	return dims
}

func (b *UserBehaviorBaseline) checkIPDeviation(profile *IPProfile, event BaselineEvent) []DeviationDimension {
	dims := make([]DeviationDimension, 0)

	// 检查多用户偏离 (一个 IP 多个用户使用)
	if len(profile.UserIDs) > 10 {
		dims = append(dims, DeviationDimension{
			Dimension: "shared_ip",
			Expected:  "single user",
			Actual:    fmt.Sprintf("%d users", len(profile.UserIDs)),
			Deviation: float64(len(profile.UserIDs)) / 10,
			Severity:  "medium",
		})
	}

	return dims
}

func (b *UserBehaviorBaseline) evictOldestUserProfile() {
	var oldest string
	var oldestTime time.Time

	for userID, profile := range b.userProfiles {
		if oldestTime.IsZero() || profile.LastSeen.Before(oldestTime) {
			oldest = userID
			oldestTime = profile.LastSeen
		}
	}

	if oldest != "" {
		delete(b.userProfiles, oldest)
	}
}

func (b *UserBehaviorBaseline) decayWorker() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		b.mu.Lock()
		b.applyDecay()
		b.mu.Unlock()
	}
}

func (b *UserBehaviorBaseline) applyDecay() {
	// 对基线数据应用时间衰减
	for _, profile := range b.userProfiles {
		profile.mu.Lock()
		if profile.RequestBaseline != nil {
			profile.RequestBaseline.RequestsPerMin *= (1 - b.config.DecayFactor)
		}
		profile.mu.Unlock()
	}
}

// 辅助函数
func calculateZScore(value, mean, stdDev float64) float64 {
	if stdDev == 0 {
		return 0
	}
	return (value - mean) / stdDev
}

func getSeverity(deviation float64) string {
	if deviation > 4 {
		return "critical"
	} else if deviation > 3 {
		return "high"
	} else if deviation > 2 {
		return "medium"
	}
	return "low"
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)) * p)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// haversineDistance 计算两点间地理距离 (km)
func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371 // 地球半径 km

	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// parseUserAgent 简化解析 User-Agent
func parseUserAgent(ua string) (browser, os string) {
	uaLower := strings.ToLower(ua[:min(len(ua), 200)])

	// 浏览器
	if strings.Contains(uaLower, "chrome") {
		browser = "Chrome"
	} else if strings.Contains(uaLower, "firefox") {
		browser = "Firefox"
	} else if strings.Contains(uaLower, "safari") {
		browser = "Safari"
	} else if strings.Contains(uaLower, "edge") {
		browser = "Edge"
	} else {
		browser = "Other"
	}

	// 操作系统
	if strings.Contains(uaLower, "windows") {
		os = "Windows"
	} else if strings.Contains(uaLower, "mac os") {
		os = "macOS"
	} else if strings.Contains(uaLower, "linux") {
		os = "Linux"
	} else if contains(uaLower, "android") {
		os = "Android"
	} else if contains(uaLower, "ios") {
		os = "iOS"
	} else {
		os = "Other"
	}

	return
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MarshalJSON 序列化 UserProfile
func (p *UserProfile) MarshalJSON() ([]byte, error) {
	type Alias UserProfile
	return json.Marshal(&struct {
		*Alias
		IsBaselineReady bool `json:"is_baseline_ready"`
	}{
		Alias:     (*Alias)(p),
		IsBaselineReady: p.IsBaselineReady,
	})
}
