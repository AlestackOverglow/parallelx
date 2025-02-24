package resilience

import (
	"errors"
	"time"
)

// ErrCircuitOpen возвращается, когда circuit breaker находится в открытом состоянии
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitState represents the state of the circuit breaker
type CircuitState int32

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

// DegradationLevel represents the current level of service degradation
type DegradationLevel int32

const (
	LevelNormal DegradationLevel = iota
	LevelReduced
	LevelMinimal
	LevelEmergency
)

// CircuitBreakerConfig defines circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold uint32
	ResetTimeout     time.Duration
	HalfOpenMaxCalls uint32
	FailureWindow    time.Duration
}

// DefaultCircuitBreakerConfig returns default circuit breaker configuration
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		ResetTimeout:     10 * time.Second,
		HalfOpenMaxCalls: 3,
		FailureWindow:    60 * time.Second,
	}
}
