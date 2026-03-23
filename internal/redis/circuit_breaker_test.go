package redis

import (
	"sync"
	"testing"
	"time"
)

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}

	cb := NewCircuitBreaker(config)

	// 初始状态应该是 Closed
	if cb.State() != StateClosed {
		t.Errorf("expected initial state to be closed, got %v", cb.State())
	}

	// 允许请求通过
	if !cb.Allow() {
		t.Error("expected Allow() to return true in closed state")
	}

	// 记录失败直到触发熔断
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	// 状态应该变为 Open
	if cb.State() != StateOpen {
		t.Errorf("expected state to be open after failures, got %v", cb.State())
	}

	// Open 状态下不允许请求
	if cb.Allow() {
		t.Error("expected Allow() to return false in open state")
	}

	// 等待超时
	time.Sleep(150 * time.Millisecond)

	// 超时后应该允许请求（转为 HalfOpen）
	if !cb.Allow() {
		t.Error("expected Allow() to return true after timeout")
	}

	// 记录成功直到恢复
	cb.RecordSuccess()
	cb.RecordSuccess()

	// 状态应该恢复为 Closed
	if cb.State() != StateClosed {
		t.Errorf("expected state to be closed after successes, got %v", cb.State())
	}

	// 失败计数应该重置
	if cb.GetFailures() != 0 {
		t.Errorf("expected failures to be reset, got %d", cb.GetFailures())
	}
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}

	cb := NewCircuitBreaker(config)

	// 触发熔断
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Errorf("expected state to be open, got %v", cb.State())
	}

	// 等待超时
	time.Sleep(150 * time.Millisecond)

	// 进入半开状态
	if !cb.Allow() {
		t.Error("expected Allow() to return true after timeout")
	}

	// 记录一次成功
	cb.RecordSuccess()
	if cb.GetHalfOpenSuccesses() != 1 {
		t.Errorf("expected halfOpenSuccesses to be 1, got %d", cb.GetHalfOpenSuccesses())
	}

	// 记录一次失败，应该重新打开
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Errorf("expected state to be open after failure in half-open, got %v", cb.State())
	}

	if cb.GetHalfOpenSuccesses() != 0 {
		t.Errorf("expected halfOpenSuccesses to be reset, got %d", cb.GetHalfOpenSuccesses())
	}
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 100,
		SuccessThreshold: 10,
		Timeout:          1 * time.Second,
	}

	cb := NewCircuitBreaker(config)

	var wg sync.WaitGroup
	numGoroutines := 10
	opsPerGoroutine := 50

	// 并发记录失败
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				cb.Allow()
				cb.RecordFailure()
			}
		}()
	}

	wg.Wait()

	// 状态应该已经是 Open
	if cb.State() != StateOpen {
		t.Errorf("expected state to be open after concurrent failures, got %v", cb.State())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
	}

	cb := NewCircuitBreaker(config)

	// 触发熔断
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Errorf("expected state to be open, got %v", cb.State())
	}

	// 重置熔断器
	cb.Reset()

	// 状态应该恢复为 Closed
	if cb.State() != StateClosed {
		t.Errorf("expected state to be closed after reset, got %v", cb.State())
	}

	if cb.GetFailures() != 0 {
		t.Errorf("expected failures to be 0 after reset, got %d", cb.GetFailures())
	}
}

func TestCircuitBreaker_AllowInClosedState(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker(config)

	// 在 Closed 状态下，应该一直允许请求
	for i := 0; i < 100; i++ {
		if !cb.Allow() {
			t.Errorf("expected Allow() to return true in closed state, iteration %d", i)
		}
		cb.RecordSuccess()
	}

	if cb.State() != StateClosed {
		t.Errorf("expected state to remain closed, got %v", cb.State())
	}
}

func TestCircuitBreaker_TimeoutNotElapsed(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          500 * time.Millisecond,
	}

	cb := NewCircuitBreaker(config)

	// 触发熔断
	cb.RecordFailure()
	cb.RecordFailure()

	// 立即检查，不允许请求
	if cb.Allow() {
		t.Error("expected Allow() to return false before timeout")
	}

	// 等待较短时间（未超时）
	time.Sleep(200 * time.Millisecond)

	// 仍然不允许请求
	if cb.Allow() {
		t.Error("expected Allow() to return false before timeout elapsed")
	}
}
