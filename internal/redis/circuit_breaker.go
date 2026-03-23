package redis

import (
	"errors"
	"sync"
	"time"
)

// CircuitBreakerState 熔断器状态
type CircuitBreakerState int

const (
	// StateClosed 正常状态，允许请求通过
	StateClosed CircuitBreakerState = iota
	// StateOpen 熔断状态，拒绝所有请求
	StateOpen
	// StateHalfOpen 半开状态，允许有限请求通过以测试恢复
	StateHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	// FailureThreshold 失败阈值，达到此值后触发熔断
	FailureThreshold int
	// SuccessThreshold 成功阈值，半开状态下达到此值后恢复
	SuccessThreshold int
	// Timeout 熔断超时时间，超时后从打开状态转为半开状态
	Timeout time.Duration
}

// DefaultCircuitBreakerConfig 默认熔断器配置
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
	}
}

// CircuitBreaker 熔断器结构体
type CircuitBreaker struct {
	mu sync.RWMutex

	// 当前状态
	state CircuitBreakerState

	// 失败计数
	failures int

	// 半开状态下的成功计数
	halfOpenSuccesses int

	// 上次失败时间
	lastFailureTime time.Time

	// 配置
	config CircuitBreakerConfig
}

// ErrCircuitBreakerOpen 熔断器打开错误
var ErrCircuitBreakerOpen = errors.New("circuit breaker is open")

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		state:  StateClosed,
		config: config,
	}
}

// Allow 检查是否允许请求通过
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// 检查是否已超时，超时则转为半开状态
		if time.Since(cb.lastFailureTime) >= cb.config.Timeout {
			cb.state = StateHalfOpen
			cb.halfOpenSuccesses = 0
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess 记录成功
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		// 重置失败计数
		cb.failures = 0
	case StateHalfOpen:
		cb.halfOpenSuccesses++
		// 成功次数达到阈值，转为关闭状态
		if cb.halfOpenSuccesses >= cb.config.SuccessThreshold {
			cb.state = StateClosed
			cb.failures = 0
			cb.halfOpenSuccesses = 0
		}
	}
}

// RecordFailure 记录失败
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		// 失败次数达到阈值，转为打开状态
		if cb.failures >= cb.config.FailureThreshold {
			cb.state = StateOpen
		}
	case StateHalfOpen:
		// 半开状态下任何失败都会重新打开
		cb.state = StateOpen
		cb.halfOpenSuccesses = 0
	}
}

// State 获取当前状态
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.halfOpenSuccesses = 0
	cb.lastFailureTime = time.Time{}
}

// GetFailures 获取当前失败计数（用于测试）
func (cb *CircuitBreaker) GetFailures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// GetHalfOpenSuccesses 获取半开状态下的成功计数（用于测试）
func (cb *CircuitBreaker) GetHalfOpenSuccesses() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.halfOpenSuccesses
}
