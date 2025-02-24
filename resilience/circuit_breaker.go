package resilience

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitBreaker is a generic circuit breaker implementation
type CircuitBreaker[T any] struct {
	state           atomic.Int32
	failures        atomic.Uint32
	lastFailureTime atomic.Int64
	halfOpenCalls   atomic.Uint32
	config          CircuitBreakerConfig
	mu              sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker[T any](config CircuitBreakerConfig) *CircuitBreaker[T] {
	cb := &CircuitBreaker[T]{
		config: config,
	}
	cb.state.Store(int32(StateClosed))
	return cb
}

// Execute runs the provided function with circuit breaker protection
func (cb *CircuitBreaker[T]) Execute(ctx context.Context, fn func() (T, error)) (T, error) {
	var zero T
	if !cb.allowRequest() {
		return zero, ErrCircuitOpen
	}

	result, err := fn()
	cb.recordResult(err)
	if err != nil {
		return zero, err
	}
	return result, nil
}

// ExecuteWithFallback runs the function with circuit breaker and fallback
func (cb *CircuitBreaker[T]) ExecuteWithFallback(
	ctx context.Context,
	fn func() (T, error),
	fallback func(error) (T, error),
) (T, error) {
	result, err := cb.Execute(ctx, fn)
	if err != nil {
		return fallback(err)
	}
	return result, nil
}

// ExecuteWithDefaultFallback runs the function with a default value fallback
func (cb *CircuitBreaker[T]) ExecuteWithDefaultFallback(
	ctx context.Context,
	fn func() (T, error),
	defaultValue T,
) (T, error) {
	result, err := cb.Execute(ctx, fn)
	if err != nil {
		return defaultValue, nil
	}
	return result, nil
}

func (cb *CircuitBreaker[T]) allowRequest() bool {
	state := CircuitState(cb.state.Load())

	switch state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(time.Unix(0, cb.lastFailureTime.Load())) > cb.config.ResetTimeout {
			if cb.state.CompareAndSwap(int32(StateOpen), int32(StateHalfOpen)) {
				cb.halfOpenCalls.Store(0)
			}
			return true
		}
		return false
	case StateHalfOpen:
		return cb.halfOpenCalls.Load() < cb.config.HalfOpenMaxCalls
	default:
		return false
	}
}

func (cb *CircuitBreaker[T]) recordResult(err error) {
	state := CircuitState(cb.state.Load())

	if err != nil {
		switch state {
		case StateClosed:
			cb.failures.Add(1)
			cb.lastFailureTime.Store(time.Now().UnixNano())
			if cb.failures.Load() >= cb.config.FailureThreshold {
				cb.state.Store(int32(StateOpen))
			}
		case StateHalfOpen:
			cb.state.Store(int32(StateOpen))
			cb.lastFailureTime.Store(time.Now().UnixNano())
		}
	} else {
		switch state {
		case StateClosed:
			cb.failures.Store(0)
		case StateHalfOpen:
			calls := cb.halfOpenCalls.Add(1)
			if calls >= cb.config.HalfOpenMaxCalls {
				cb.state.Store(int32(StateClosed))
				cb.failures.Store(0)
			}
		}
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker[T]) GetState() CircuitState {
	return CircuitState(cb.state.Load())
}

// Reset forces the circuit breaker back to closed state
func (cb *CircuitBreaker[T]) Reset() {
	cb.state.Store(int32(StateClosed))
	cb.failures.Store(0)
	cb.halfOpenCalls.Store(0)
}
