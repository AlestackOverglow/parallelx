package resilience

import (
	"context"
	"errors"
	"time"
)

// RetryConfig defines retry behavior configuration
type RetryConfig struct {
	MaxAttempts       int
	InitialDelay      time.Duration
	MaxDelay          time.Duration
	BackoffMultiplier float64
	RetryableErrors   []error
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          2 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// IsRetryable checks if the error should trigger a retry
func (c *RetryConfig) IsRetryable(err error) bool {
	if len(c.RetryableErrors) == 0 {
		return true
	}
	for _, retryableErr := range c.RetryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}
	return false
}

// Retry executes the function with retry logic
func Retry[T any](ctx context.Context, fn func() (T, error), config RetryConfig) (T, error) {
	var result T
	var err error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		result, err = fn()
		if err == nil {
			return result, nil
		}

		if !config.IsRetryable(err) {
			return result, err
		}

		if attempt == config.MaxAttempts {
			return result, err
		}

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(delay):
			delay = time.Duration(float64(delay) * config.BackoffMultiplier)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
		}
	}

	return result, err
}

// RetryWithFallback executes the function with retry logic and fallback
func RetryWithFallback[T any](
	ctx context.Context,
	fn func() (T, error),
	fallback func(error) (T, error),
	config RetryConfig,
) (T, error) {
	result, err := Retry(ctx, fn, config)
	if err != nil {
		return fallback(err)
	}
	return result, nil
}
