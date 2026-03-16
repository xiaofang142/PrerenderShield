package prerender

import (
	"sync"
	"time"

	"prerender-shield/internal/logging"
)

// ConcurrencyManager 动态并发管理器
type ConcurrencyManager struct {
	mu             sync.RWMutex
	currentLimit   int
	minLimit       int
	maxLimit       int
	activeCount    int
	waitingCount   int
	successCount   int64
	failureCount   int64
	lastAdjustTime time.Time
	adjustInterval time.Duration
	avgRenderTime  float64
	renderTimes    []float64
}

// NewConcurrencyManager 创建动态并发管理器
func NewConcurrencyManager(minLimit, maxLimit, initialLimit int) *ConcurrencyManager {
	if minLimit < 1 {
		minLimit = 1
	}
	if maxLimit < minLimit {
		maxLimit = minLimit * 5
	}
	if initialLimit < minLimit || initialLimit > maxLimit {
		initialLimit = minLimit
	}

	return &ConcurrencyManager{
		currentLimit:   initialLimit,
		minLimit:       minLimit,
		maxLimit:       maxLimit,
		activeCount:    0,
		waitingCount:   0,
		lastAdjustTime: time.Now(),
		adjustInterval: 30 * time.Second,
		renderTimes:    make([]float64, 0, 100),
	}
}

// Acquire 获取并发许可
func (m *ConcurrencyManager) Acquire() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeCount < m.currentLimit {
		m.activeCount++
		return true
	}

	// 超过限制，需要等待
	m.waitingCount++
	return false
}

// Release 释放并发许可
func (m *ConcurrencyManager) Release() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeCount > 0 {
		m.activeCount--
	}
	if m.waitingCount > 0 {
		m.waitingCount--
	}
}

// RecordSuccess 记录成功渲染
func (m *ConcurrencyManager) RecordSuccess(duration float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.successCount++
	m.addRenderTime(duration)

	// 检查是否需要调整并发限制
	if time.Since(m.lastAdjustTime) >= m.adjustInterval {
		m.adjustLimitLocked()
		m.lastAdjustTime = time.Now()
	}
}

// RecordFailure 记录失败渲染
func (m *ConcurrencyManager) RecordFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.failureCount++

	// 失败时立即降低并发限制
	if m.currentLimit > m.minLimit {
		newLimit := m.currentLimit - 1
		if newLimit < m.minLimit {
			newLimit = m.minLimit
		}
		m.currentLimit = newLimit
		logging.DefaultLogger.Info("Concurrency limit reduced to %d due to failure", newLimit)
	}
}

// addRenderTime 添加渲染时间记录
func (m *ConcurrencyManager) addRenderTime(duration float64) {
	m.renderTimes = append(m.renderTimes, duration)

	// 保持最近的 100 条记录
	if len(m.renderTimes) > 100 {
		m.renderTimes = m.renderTimes[1:]
	}

	// 计算平均渲染时间
	total := 0.0
	for _, t := range m.renderTimes {
		total += t
	}
	if len(m.renderTimes) > 0 {
		m.avgRenderTime = total / float64(len(m.renderTimes))
	}
}

// adjustLimitLocked 调整并发限制（已持有锁）
func (m *ConcurrencyManager) adjustLimitLocked() {
	// 根据成功率调整
	totalCount := m.successCount + m.failureCount
	var successRate float64
	if totalCount > 0 {
		successRate = float64(m.successCount) / float64(totalCount) * 100
	} else {
		successRate = 100 // 默认 100% 成功率
	}

	// 根据平均渲染时间调整
	avgTime := m.avgRenderTime

	logging.DefaultLogger.Info("Adjusting concurrency: success_rate=%.2f%%, avg_render_time=%.2fs, current_limit=%d",
		successRate, avgTime, m.currentLimit)

	// 调整策略
	if successRate >= 95 && avgTime < 5.0 {
		// 成功率高且渲染时间短，增加并发
		if m.currentLimit < m.maxLimit {
			m.currentLimit++
			logging.DefaultLogger.Info("Concurrency limit increased to %d (high success rate, low latency)", m.currentLimit)
		}
	} else if successRate < 80 || avgTime > 15.0 {
		// 成功率低或渲染时间长，降低并发
		if m.currentLimit > m.minLimit {
			m.currentLimit--
			logging.DefaultLogger.Info("Concurrency limit decreased to %d (low success rate or high latency)", m.currentLimit)
		}
	}
}

// GetCurrentLimit 获取当前并发限制
func (m *ConcurrencyManager) GetCurrentLimit() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentLimit
}

// GetActiveCount 获取当前活动数
func (m *ConcurrencyManager) GetActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeCount
}

// GetWaitingCount 获取等待数
func (m *ConcurrencyManager) GetWaitingCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.waitingCount
}

// GetStats 获取并发统计
func (m *ConcurrencyManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalCount := m.successCount + m.failureCount
	var successRate float64
	if totalCount > 0 {
		successRate = float64(m.successCount) / float64(totalCount) * 100
	}

	return map[string]interface{}{
		"current_limit":   m.currentLimit,
		"min_limit":       m.minLimit,
		"max_limit":       m.maxLimit,
		"active_count":    m.activeCount,
		"waiting_count":   m.waitingCount,
		"success_count":   m.successCount,
		"failure_count":   m.failureCount,
		"avg_render_time": m.avgRenderTime,
		"success_rate":    successRate,
	}
}

// SetAdjustInterval 设置调整间隔
func (m *ConcurrencyManager) SetAdjustInterval(interval time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.adjustInterval = interval
}

// Reset 重置统计
func (m *ConcurrencyManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.successCount = 0
	m.failureCount = 0
	m.avgRenderTime = 0
	m.renderTimes = m.renderTimes[:0]
}
