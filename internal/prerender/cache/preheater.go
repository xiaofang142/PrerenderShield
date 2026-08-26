package cache

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Preheater 缓存预热器
type Preheater struct {
	config      *PreheaterConfig
	cache       *TieredCache
	logger      *zap.Logger
	heatmap     map[string]*HeatEntry
	mu          sync.RWMutex
	stopChan    chan struct{}
	wg          sync.WaitGroup
	preheatChan chan *PreheatTask
}

// PreheaterConfig 预热器配置
type PreheaterConfig struct {
	Enabled         bool          // 是否启用预热
	MinHits         int           // 最小命中次数触发预热
	PriorityBoost   int           // 预热优先级提升
	PreheatInterval time.Duration // 预热检查间隔
	BatchSize       int           // 批量预热大小
}

// HeatEntry 热度条目
type HeatEntry struct {
	Key      string
	HitCount int
	LastHit  time.Time
	IsHot    bool
	Priority int
}

// PreheatTask 预热任务
type PreheatTask struct {
	Key      string
	Priority int
	Source   string // 触发源：heatmap, manual, scheduled
}

// DefaultPreheaterConfig 返回默认预热器配置
func DefaultPreheaterConfig() *PreheaterConfig {
	return &PreheaterConfig{
		Enabled:         true,
		MinHits:         5,
		PriorityBoost:   2,
		PreheatInterval: 30 * time.Second,
		BatchSize:       10,
	}
}

// NewPreheater 创建预热器
func NewPreheater(config *PreheaterConfig, cache *TieredCache, logger *zap.Logger) *Preheater {
	if config == nil {
		config = DefaultPreheaterConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	p := &Preheater{
		config:      config,
		cache:       cache,
		logger:      logger,
		heatmap:     make(map[string]*HeatEntry),
		preheatChan: make(chan *PreheatTask, 100),
		stopChan:    make(chan struct{}),
	}

	if config.Enabled {
		p.wg.Add(2)
		go p.heatmapWorker()
		go p.preheatWorker()
	}

	return p
}

// RecordHit 记录访问命中
func (p *Preheater) RecordHit(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	entry, exists := p.heatmap[key]
	if !exists {
		entry = &HeatEntry{
			Key:      key,
			HitCount: 0,
		}
		p.heatmap[key] = entry
	}

	entry.HitCount++
	entry.LastHit = time.Now()

	// 检查是否变为热点
	if entry.HitCount >= p.config.MinHits && !entry.IsHot {
		entry.IsHot = true
		entry.Priority = p.config.PriorityBoost

		// 加入预热队列
		p.preheatChan <- &PreheatTask{
			Key:      key,
			Priority: entry.Priority,
			Source:   "heatmap",
		}

		p.logger.Info("检测到热点", zap.String("key", key), zap.Int("hits", entry.HitCount))
	}
}

// Preheat 手动预热
func (p *Preheater) Preheat(keys []string, priority int) {
	for _, key := range keys {
		p.preheatChan <- &PreheatTask{
			Key:      key,
			Priority: priority,
			Source:   "manual",
		}
	}
}

// heatmapWorker 热度图工作协程
func (p *Preheater) heatmapWorker() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.PreheatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanupHeatmap()
		case <-p.stopChan:
			return
		}
	}
}

// cleanupHeatmap 清理热度图
func (p *Preheater) cleanupHeatmap() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for key, entry := range p.heatmap {
		// 清理 5 分钟未访问的条目
		if now.Sub(entry.LastHit) > 5*time.Minute {
			delete(p.heatmap, key)
		}
	}
}

// preheatWorker 预热工作协程
func (p *Preheater) preheatWorker() {
	defer p.wg.Done()

	batch := make([]*PreheatTask, 0, p.config.BatchSize)
	flushTimer := time.NewTicker(time.Second)
	defer flushTimer.Stop()

	for {
		select {
		case task := <-p.preheatChan:
			batch = append(batch, task)
			if len(batch) >= p.config.BatchSize {
				p.flushBatch(batch)
				batch = batch[:0]
			}

		case <-flushTimer.C:
			if len(batch) > 0 {
				p.flushBatch(batch)
				batch = batch[:0]
			}

		case <-p.stopChan:
			return
		}
	}
}

// flushBatch 批量处理预热任务
func (p *Preheater) flushBatch(batch []*PreheatTask) {
	for _, task := range batch {
		p.logger.Debug("预热任务",
			zap.String("key", task.Key),
			zap.Int("priority", task.Priority),
			zap.String("source", task.Source),
		)

		// 实际预热：从缓存层获取数据以确保 L1 缓存命中
		if _, err := p.cache.Get(task.Key); err != nil {
			p.logger.Debug("预热缓存未命中",
				zap.String("key", task.Key),
				zap.Error(err),
			)
		} else {
			p.logger.Debug("预热缓存命中",
				zap.String("key", task.Key),
			)
		}
	}
}

// GetHotKeys 获取热点 Key 列表
func (p *Preheater) GetHotKeys() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	hotKeys := make([]string, 0)
	for key, entry := range p.heatmap {
		if entry.IsHot {
			hotKeys = append(hotKeys, key)
		}
	}

	return hotKeys
}

// GetStats 获取预热器统计
func (p *Preheater) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	hotCount := 0
	totalHits := 0
	for _, entry := range p.heatmap {
		if entry.IsHot {
			hotCount++
		}
		totalHits += entry.HitCount
	}

	return map[string]interface{}{
		"heatmap_size": len(p.heatmap),
		"hot_keys":     hotCount,
		"total_hits":   totalHits,
		"queue_size":   len(p.preheatChan),
	}
}

// Close 关闭预热器
func (p *Preheater) Close() {
	close(p.stopChan)
	p.wg.Wait()
}

// Invalidator 缓存失效管理器
type Invalidator struct {
	config     *InvalidatorConfig
	cache      *TieredCache
	logger     *zap.Logger
	rules      []InvalidationRule
	versionMap map[string]int // key 版本号
	mu         sync.RWMutex
}

// InvalidatorConfig 失效器配置
type InvalidatorConfig struct {
	EnableVersioning bool          // 启用版本控制
	DefaultTTL       time.Duration // 默认 TTL
}

// InvalidationRule 失效规则
type InvalidationRule struct {
	Pattern      string // 匹配模式（支持通配符）
	Condition    func(key string, value []byte) bool
	OnInvalidate func(key string)
}

// DefaultInvalidatorConfig 返回默认失效器配置
func DefaultInvalidatorConfig() *InvalidatorConfig {
	return &InvalidatorConfig{
		EnableVersioning: true,
		DefaultTTL:       time.Hour,
	}
}

// NewInvalidator 创建失效管理器
func NewInvalidator(config *InvalidatorConfig, cache *TieredCache, logger *zap.Logger) *Invalidator {
	if config == nil {
		config = DefaultInvalidatorConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Invalidator{
		config:     config,
		cache:      cache,
		logger:     logger,
		rules:      make([]InvalidationRule, 0),
		versionMap: make(map[string]int),
	}
}

// AddRule 添加失效规则
func (i *Invalidator) AddRule(rule InvalidationRule) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.rules = append(i.rules, rule)
}

// Invalidate 使缓存失效
func (i *Invalidator) Invalidate(key string) error {
	i.mu.RLock()
	rules := i.rules
	i.mu.RUnlock()

	// 检查是否匹配规则
	for _, rule := range rules {
		if matchPattern(rule.Pattern, key) {
			i.logger.Info("缓存失效",
				zap.String("key", key),
				zap.String("pattern", rule.Pattern),
			)

			if rule.OnInvalidate != nil {
				rule.OnInvalidate(key)
			}
		}
	}

	// 删除缓存
	return i.cache.Delete(key)
}

// InvalidatePattern 按模式使缓存失效
func (i *Invalidator) InvalidatePattern(pattern string) error {
	return i.cache.DeletePattern(pattern)
}

// GetVersion 获取版本
func (i *Invalidator) GetVersion(key string) int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.versionMap[key]
}

// IncrementVersion 增加版本
func (i *Invalidator) IncrementVersion(key string) int {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.versionMap[key]++
	return i.versionMap[key]
}

// SetWithVersion 带版本设置缓存
func (i *Invalidator) SetWithVersion(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	version := i.GetVersion(key)

	metadata := map[string]interface{}{
		"version": version,
	}

	return i.cache.SetWithMetadata(key, value, ttl, metadata)
}

// matchPattern 简单通配符匹配
func matchPattern(pattern, key string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == key {
		return true
	}
	// 简单前缀匹配
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(key) >= len(prefix) && key[:len(prefix)] == prefix
	}
	return false
}

// GetStats 获取失效管理器统计
func (i *Invalidator) GetStats() map[string]interface{} {
	i.mu.RLock()
	defer i.mu.RUnlock()

	return map[string]interface{}{
		"rules_count":    len(i.rules),
		"versions_count": len(i.versionMap),
	}
}
