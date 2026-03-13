package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// TierType 缓存层级类型
type TierType int

const (
	TierL1 TierType = iota // L1: 内存缓存
	TierL2                 // L2: Redis 缓存
)

func (t TierType) String() string {
	switch t {
	case TierL1:
		return "L1"
	case TierL2:
		return "L2"
	default:
		return "unknown"
	}
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Key        string                 `json:"key"`
	Value      []byte                 `json:"value"`
	Metadata   map[string]interface{} `json:"metadata"`
	CreatedAt  time.Time              `json:"created_at"`
	ExpiresAt  time.Time              `json:"expires_at"`
	HitCount   int64                  `json:"hit_count"`
	LastHitAt  time.Time              `json:"last_hit_at"`
	Priority   int                    `json:"priority"` // 1-5, 5 为最高
	Tier       TierType               `json:"tier"`
}

// IsExpired 检查是否过期
func (e *CacheEntry) IsExpired() bool {
	if e.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.ExpiresAt)
}

// TTL 返回剩余生存时间
func (e *CacheEntry) TTL() time.Duration {
	if e.ExpiresAt.IsZero() {
		return time.Duration(0)
	}
	return time.Until(e.ExpiresAt)
}

// Config 多级缓存配置
type Config struct {
	// L1 内存缓存配置
	L1Enabled bool
	L1MaxSize int           // 最大条目数
	L1DefaultTTL time.Duration

	// L2 Redis 缓存配置
	L2Enabled bool
	L2Prefix  string
	L2DefaultTTL time.Duration

	// 写策略
	WriteThrough bool // true=直写模式，false=回写模式

	// 读策略
	ReadThrough bool // 是否自动从 L2 回填 L1

	// 监控
	EnableMetrics bool
	MetricsSampleInterval time.Duration

	// 日志
	LogLevel int
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		L1Enabled:        true,
		L1MaxSize:        1000,
		L1DefaultTTL:     5 * time.Minute,

		L2Enabled:        true,
		L2Prefix:         "prerender:cache:",
		L2DefaultTTL:     1 * time.Hour,

		WriteThrough:     true,
		ReadThrough:      true,

		EnableMetrics:    true,
		MetricsSampleInterval: 10 * time.Second,
	}
}

// Metrics 缓存指标
type Metrics struct {
	mu sync.RWMutex

	// L1 指标
	L1Hits       int64
	L1Misses     int64
	L1Evictions  int64
	L1Writes     int64

	// L2 指标
	L2Hits       int64
	L2Misses     int64
	L2Writes     int64
	L2Errors     int64

	// 总体指标
	TotalReads   int64
	TotalWrites  int64
	HitRate      float64

	// 延迟统计
	AvgReadLatency  time.Duration
	AvgWriteLatency time.Duration

	// 容量统计
	L1Size int
	L2Size int64
}

// RecordL1Hit 记录 L1 命中
func (m *Metrics) RecordL1Hit(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.L1Hits++
	m.TotalReads++
	m.updateReadLatency(latency)
	m.updateHitRate()
}

// RecordL1Miss 记录 L1 未命中
func (m *Metrics) RecordL1Miss(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.L1Misses++
	m.TotalReads++
	m.updateReadLatency(latency)
	m.updateHitRate()
}

// RecordL2Hit 记录 L2 命中
func (m *Metrics) RecordL2Hit(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.L2Hits++
	m.TotalReads++
	m.updateReadLatency(latency)
	m.updateHitRate()
}

// RecordL2Miss 记录 L2 未命中
func (m *Metrics) RecordL2Miss(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.L2Misses++
	m.TotalReads++
	m.updateReadLatency(latency)
	m.updateHitRate()
}

// RecordL1Write 记录 L1 写入
func (m *Metrics) RecordL1Write(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.L1Writes++
	m.TotalWrites++
	m.updateWriteLatency(latency)
}

// RecordL2Write 记录 L2 写入
func (m *Metrics) RecordL2Write(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.L2Writes++
	m.TotalWrites++
	m.updateWriteLatency(latency)
}

// RecordL2Error 记录 L2 错误
func (m *Metrics) RecordL2Error() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.L2Errors++
}

// RecordEviction 记录驱逐
func (m *Metrics) RecordEviction() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.L1Evictions++
}

// SetSizes 设置容量
func (m *Metrics) SetSizes(l1Size int, l2Size int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.L1Size = l1Size
	m.L2Size = l2Size
}

func (m *Metrics) updateReadLatency(latency time.Duration) {
	count := m.TotalReads
	m.AvgReadLatency = (m.AvgReadLatency*time.Duration(count-1) + latency) / time.Duration(count)
}

func (m *Metrics) updateWriteLatency(latency time.Duration) {
	count := m.TotalWrites
	m.AvgWriteLatency = (m.AvgWriteLatency*time.Duration(count-1) + latency) / time.Duration(count)
}

func (m *Metrics) updateHitRate() {
	total := m.L1Hits + m.L2Hits + m.L1Misses + m.L2Misses
	if total > 0 {
		m.HitRate = float64(m.L1Hits+m.L2Hits) / float64(total) * 100
	}
}

// GetSnapshot 获取指标快照
func (m *Metrics) GetSnapshot() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"l1_hits":         m.L1Hits,
		"l1_misses":       m.L1Misses,
		"l1_evictions":    m.L1Evictions,
		"l1_writes":       m.L1Writes,
		"l2_hits":         m.L2Hits,
		"l2_misses":       m.L2Misses,
		"l2_writes":       m.L2Writes,
		"l2_errors":       m.L2Errors,
		"total_reads":     m.TotalReads,
		"total_writes":    m.TotalWrites,
		"hit_rate":        m.HitRate,
		"avg_read_latency": m.AvgReadLatency.String(),
		"avg_write_latency": m.AvgWriteLatency.String(),
		"l1_size":         m.L1Size,
		"l2_size":         m.L2Size,
	}
}

// TieredCache 多级缓存实现
type TieredCache struct {
	config     Config
	logger     *zap.Logger
	metrics    *Metrics
	ctx        context.Context
	cancel     context.CancelFunc

	// L1: 内存缓存
	l1mu      sync.RWMutex
	l1items   map[string]*CacheEntry
	l1queue   []string // 用于 LRU  eviction

	// L2: Redis 缓存
	redisClient RedisClientForTieredCache

	// 回写队列（写回模式）
	writeBackChan chan *writeBackTask
	wg            sync.WaitGroup
}

type writeBackTask struct {
	key       string
	entry     *CacheEntry
	timestamp time.Time
}

// RedisClientForTieredCache 是 redis.Client 的接口，用于测试
type RedisClientForTieredCache interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Keys(ctx context.Context, pattern string) *redis.StringSliceCmd
}

// Ensure redis.Client implements the interface
var _ RedisClientForTieredCache = (*redis.Client)(nil)

// NewTieredCache 创建多级缓存
func NewTieredCache(config Config, redisClient RedisClientForTieredCache, logger *zap.Logger) *TieredCache {
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	tc := &TieredCache{
		config:        config,
		logger:        logger,
		metrics:       &Metrics{},
		ctx:           ctx,
		cancel:        cancel,
		l1items:       make(map[string]*CacheEntry),
		l1queue:       make([]string, 0, config.L1MaxSize),
		redisClient:   redisClient,
		writeBackChan: make(chan *writeBackTask, 1000),
	}

	// 启动回写协程（如果配置为写回模式）
	if !config.WriteThrough && config.L2Enabled {
		tc.wg.Add(1)
		go tc.writeBackWorker()
	}

	// 启动指标收集协程
	if config.EnableMetrics {
		tc.wg.Add(1)
		go tc.metricsWorker()
	}

	return tc
}

// writeBackWorker 回写工作协程
func (tc *TieredCache) writeBackWorker() {
	defer tc.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	batch := make([]*writeBackTask, 0, 100)
	flushInterval := 5 * time.Second

	flushTimer := time.NewTimer(flushInterval)
	defer flushTimer.Stop()

	for {
		select {
		case <-tc.ctx.Done():
			// 清空回写队列
			for len(tc.writeBackChan) > 0 {
				task := <-tc.writeBackChan
				tc.writeToL2(task.key, task.entry)
			}
			return

		case task := <-tc.writeBackChan:
			batch = append(batch, task)
			if len(batch) >= 100 {
				tc.flushBatch(batch)
				batch = batch[:0]
			}

		case <-flushTimer.C:
			if len(batch) > 0 {
				tc.flushBatch(batch)
				batch = batch[:0]
			}
			flushTimer.Reset(flushInterval)

		case <-ticker.C:
			// 定期清理过期条目
			tc.cleanupExpired()
		}
	}
}

func (tc *TieredCache) flushBatch(batch []*writeBackTask) {
	for _, task := range batch {
		tc.writeToL2(task.key, task.entry)
	}
}

// metricsWorker 指标收集协程
func (tc *TieredCache) metricsWorker() {
	defer tc.wg.Done()

	ticker := time.NewTicker(tc.config.MetricsSampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tc.ctx.Done():
			return
		case <-ticker.C:
			tc.updateMetricsSize()
		}
	}
}

func (tc *TieredCache) updateMetricsSize() {
	tc.l1mu.RLock()
	l1Size := len(tc.l1items)
	tc.l1mu.RUnlock()

	var l2Size int64
	if tc.config.L2Enabled && tc.redisClient != nil {
		pattern := fmt.Sprintf("%s*", tc.config.L2Prefix)
		keys, err := tc.redisClient.Keys(tc.ctx, pattern).Result()
		if err == nil {
			l2Size = int64(len(keys))
		}
	}

	tc.metrics.SetSizes(l1Size, l2Size)
}

// cleanupExpired 清理过期条目
func (tc *TieredCache) cleanupExpired() {
	tc.l1mu.Lock()
	defer tc.l1mu.Unlock()

	now := time.Now()
	for key, entry := range tc.l1items {
		if entry.IsExpired() {
			delete(tc.l1items, key)
			tc.metrics.RecordEviction()
		}
		_ = now
	}
}

// Get 获取缓存
func (tc *TieredCache) Get(key string) ([]byte, error) {
	startTime := time.Now()

	// 尝试 L1
	if tc.config.L1Enabled {
		if data, hit := tc.getL1(key); hit {
			latency := time.Since(startTime)
			tc.metrics.RecordL1Hit(latency)
			tc.logger.Debug("L1 cache hit", zap.String("key", key), zap.Duration("latency", latency))
			return data, nil
		}
	}

	// L1 未命中，尝试 L2
	if tc.config.L2Enabled && tc.redisClient != nil {
		data, hit, err := tc.getL2(key)
		latency := time.Since(startTime)

		if err != nil {
			tc.metrics.RecordL2Error()
			tc.logger.Warn("L2 cache error", zap.String("key", key), zap.Error(err))
			return nil, err
		}

		if hit {
			tc.metrics.RecordL2Hit(latency)
			tc.logger.Debug("L2 cache hit", zap.String("key", key), zap.Duration("latency", latency))

			// 回填 L1
			if tc.config.ReadThrough && tc.config.L1Enabled {
				tc.setL1(key, data, nil)
			}

			return data, nil
		}

		tc.metrics.RecordL2Miss(latency)
	}

	// 未命中
	if tc.config.L1Enabled {
		tc.metrics.RecordL1Miss(time.Since(startTime))
	}

	return nil, nil
}

// getL1 从 L1 获取
func (tc *TieredCache) getL1(key string) ([]byte, bool) {
	tc.l1mu.RLock()
	defer tc.l1mu.RUnlock()

	entry, exists := tc.l1items[key]
	if !exists {
		return nil, false
	}

	if entry.IsExpired() {
		return nil, false
	}

	// 更新命中统计
	entry.HitCount++
	entry.LastHitAt = time.Now()

	return entry.Value, true
}

// getL2 从 L2 获取
func (tc *TieredCache) getL2(key string) ([]byte, bool, error) {
	fullKey := tc.config.L2Prefix + key

	data, err := tc.redisClient.Get(tc.ctx, fullKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}

	return data, true, nil
}

// Set 设置缓存
func (tc *TieredCache) Set(key string, value []byte, ttl time.Duration) error {
	return tc.SetWithMetadata(key, value, ttl, nil)
}

// SetWithMetadata 设置缓存（带元数据）
func (tc *TieredCache) SetWithMetadata(key string, value []byte, ttl time.Duration, metadata map[string]interface{}) error {
	now := time.Now()

	entry := &CacheEntry{
		Key:       key,
		Value:     value,
		Metadata:  metadata,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
		HitCount:  0,
		Priority:  3,
	}

	// 写入 L1
	if tc.config.L1Enabled {
		tc.setL1(key, value, metadata)
	}

	// 写入 L2
	if tc.config.L2Enabled && tc.config.WriteThrough {
		if err := tc.writeToL2(key, entry); err != nil {
			return err
		}
	} else if tc.config.L2Enabled {
		// 写回模式，加入队列
		tc.writeBackChan <- &writeBackTask{
			key:       key,
			entry:     entry,
			timestamp: now,
		}
	}

	return nil
}

// setL1 写入 L1
func (tc *TieredCache) setL1(key string, value []byte, metadata map[string]interface{}) {
	tc.l1mu.Lock()
	defer tc.l1mu.Unlock()

	// 检查是否需要驱逐
	if len(tc.l1items) >= tc.config.L1MaxSize {
		tc.evictL1()
	}

	entry := &CacheEntry{
		Key:       key,
		Value:     value,
		Metadata:  metadata,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(tc.config.L1DefaultTTL),
		HitCount:  0,
		Priority:  3,
		Tier:      TierL1,
	}

	tc.l1items[key] = entry
	tc.l1queue = append(tc.l1queue, key)

	tc.metrics.RecordL1Write(0)
}

// evictL1 LRU 驱逐
func (tc *TieredCache) evictL1() {
	if len(tc.l1queue) == 0 {
		return
	}

	// 找到最久未使用的条目
	oldestKey := tc.l1queue[0]
	oldestTime := time.Now()

	for key, entry := range tc.l1items {
		if entry.LastHitAt.Before(oldestTime) || entry.LastHitAt.IsZero() {
			oldestKey = key
			oldestTime = entry.LastHitAt
		}
	}

	// 驱逐
	if _, exists := tc.l1items[oldestKey]; exists {
		delete(tc.l1items, oldestKey)
		tc.metrics.RecordEviction()

		// 从队列移除
		for i, k := range tc.l1queue {
			if k == oldestKey {
				tc.l1queue = append(tc.l1queue[:i], tc.l1queue[i+1:]...)
				break
			}
		}
	}
}

// writeToL2 写入 L2
func (tc *TieredCache) writeToL2(key string, entry *CacheEntry) error {
	startTime := time.Now()

	fullKey := tc.config.L2Prefix + key
	ttl := entry.TTL()
	if ttl <= 0 {
		ttl = tc.config.L2DefaultTTL
	}

	// 序列化
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}

	err = tc.redisClient.Set(tc.ctx, fullKey, data, ttl).Err()
	if err != nil {
		tc.metrics.RecordL2Error()
		return fmt.Errorf("failed to write to L2: %w", err)
	}

	tc.metrics.RecordL2Write(time.Since(startTime))
	return nil
}

// Delete 删除缓存
func (tc *TieredCache) Delete(key string) error {
	// 删除 L1
	if tc.config.L1Enabled {
		tc.l1mu.Lock()
		delete(tc.l1items, key)
		tc.l1mu.Unlock()
	}

	// 删除 L2
	if tc.config.L2Enabled && tc.redisClient != nil {
		fullKey := tc.config.L2Prefix + key
		if err := tc.redisClient.Del(tc.ctx, fullKey).Err(); err != nil {
			return err
		}
	}

	return nil
}

// DeletePattern 按模式删除
func (tc *TieredCache) DeletePattern(pattern string) error {
	if !tc.config.L2Enabled || tc.redisClient == nil {
		return nil
	}

	fullPattern := tc.config.L2Prefix + pattern
	keys, err := tc.redisClient.Keys(tc.ctx, fullPattern).Result()
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		return tc.redisClient.Del(tc.ctx, keys...).Err()
	}

	return nil
}

// GetWithMetadata 获取缓存及元数据
func (tc *TieredCache) GetWithMetadata(key string) (*CacheEntry, error) {
	data, err := tc.Get(key)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, nil
	}

	// 尝试获取元数据
	tc.l1mu.RLock()
	entry, exists := tc.l1items[key]
	tc.l1mu.RUnlock()

	if exists && !entry.IsExpired() {
		return entry, nil
	}

	// 从 L2 获取完整条目
	if tc.config.L2Enabled && tc.redisClient != nil {
		fullKey := tc.config.L2Prefix + key
		raw, err := tc.redisClient.Get(tc.ctx, fullKey).Bytes()
		if err == nil {
			var cachedEntry *CacheEntry
			if err := json.Unmarshal(raw, &cachedEntry); err == nil {
				return cachedEntry, nil
			}
		}
	}

	// 返回基本条目
	return &CacheEntry{
		Key:       key,
		Value:     data,
		CreatedAt: time.Now(),
		Tier:      TierL2,
	}, nil
}

// GetMetrics 获取指标
func (tc *TieredCache) GetMetrics() map[string]interface{} {
	return tc.metrics.GetSnapshot()
}

// GetStats 获取详细统计
func (tc *TieredCache) GetStats() map[string]interface{} {
	tc.l1mu.RLock()
	l1Count := len(tc.l1items)
	tc.l1mu.RUnlock()

	var l2Count int64
	if tc.config.L2Enabled && tc.redisClient != nil {
		pattern := fmt.Sprintf("%s*", tc.config.L2Prefix)
		keys, err := tc.redisClient.Keys(tc.ctx, pattern).Result()
		if err == nil {
			l2Count = int64(len(keys))
		}
	}

	metrics := tc.metrics.GetSnapshot()
	metrics["l1_count"] = l1Count
	metrics["l2_count"] = l2Count
	metrics["l1_max_size"] = tc.config.L1MaxSize
	metrics["write_queue_size"] = len(tc.writeBackChan)

	return metrics
}

// Flush 刷新所有缓存到 L2
func (tc *TieredCache) Flush() error {
	if !tc.config.L2Enabled || tc.redisClient == nil {
		return nil
	}

	tc.l1mu.RLock()
	keys := make([]string, 0, len(tc.l1items))
	for key := range tc.l1items {
		keys = append(keys, key)
	}
	tc.l1mu.RUnlock()

	for _, key := range keys {
		tc.l1mu.RLock()
		entry, exists := tc.l1items[key]
		tc.l1mu.RUnlock()

		if exists {
			if err := tc.writeToL2(key, entry); err != nil {
				tc.logger.Warn("failed to flush entry", zap.String("key", key), zap.Error(err))
			}
		}
	}

	return nil
}

// Close 关闭缓存
func (tc *TieredCache) Close() error {
	tc.cancel()

	// 等待协程退出
	tc.wg.Wait()

	// 刷新剩余数据
	return tc.Flush()
}

// Clear 清空所有缓存
func (tc *TieredCache) Clear() error {
	tc.l1mu.Lock()
	tc.l1items = make(map[string]*CacheEntry)
	tc.l1queue = make([]string, 0, tc.config.L1MaxSize)
	tc.l1mu.Unlock()

	if tc.config.L2Enabled && tc.redisClient != nil {
		pattern := fmt.Sprintf("%s*", tc.config.L2Prefix)
		keys, err := tc.redisClient.Keys(tc.ctx, pattern).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			return tc.redisClient.Del(tc.ctx, keys...).Err()
		}
	}

	return nil
}
