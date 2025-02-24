package resilience

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetry(t *testing.T) {
	t.Run("successful retry", func(t *testing.T) {
		attempts := 0
		fn := func() (string, error) {
			attempts++
			if attempts < 3 {
				return "", errors.New("temporary error")
			}
			return "success", nil
		}

		config := DefaultRetryConfig()
		result, err := Retry(context.Background(), fn, config)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "success" {
			t.Errorf("Expected 'success', got %q", result)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("max attempts exceeded", func(t *testing.T) {
		attempts := 0
		fn := func() (string, error) {
			attempts++
			return "", errors.New("persistent error")
		}

		config := DefaultRetryConfig()
		_, err := Retry(context.Background(), fn, config)

		if err == nil {
			t.Error("Expected error, got nil")
		}
		if attempts != config.MaxAttempts {
			t.Errorf("Expected %d attempts, got %d", config.MaxAttempts, attempts)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0
		fn := func() (string, error) {
			attempts++
			if attempts == 2 {
				cancel()
			}
			return "", errors.New("error")
		}

		config := DefaultRetryConfig()
		_, err := Retry(ctx, fn, config)

		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})
}

type TestResult struct {
	Value string
	Count int
}

func TestCircuitBreaker(t *testing.T) {
	t.Run("normal operation", func(t *testing.T) {
		cb := NewCircuitBreaker[TestResult](DefaultCircuitBreakerConfig())

		for i := 0; i < 10; i++ {
			_, err := cb.Execute(context.Background(), func() (TestResult, error) {
				return TestResult{Value: "ok", Count: i}, nil
			})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		}

		if cb.GetState() != StateClosed {
			t.Errorf("Expected circuit to be closed, got %v", cb.GetState())
		}
	})

	t.Run("circuit opens after failures", func(t *testing.T) {
		config := DefaultCircuitBreakerConfig()
		config.FailureThreshold = 3
		cb := NewCircuitBreaker[TestResult](config)

		// Generate failures
		for i := 0; i < int(config.FailureThreshold); i++ {
			_, _ = cb.Execute(context.Background(), func() (TestResult, error) {
				return TestResult{}, errors.New("error")
			})
		}

		// Circuit should be open
		_, err := cb.Execute(context.Background(), func() (TestResult, error) {
			return TestResult{Value: "test"}, nil
		})

		if !errors.Is(err, ErrCircuitOpen) {
			t.Errorf("Expected ErrCircuitOpen, got %v", err)
		}
		if cb.GetState() != StateOpen {
			t.Errorf("Expected circuit to be open, got %v", cb.GetState())
		}
	})

	t.Run("half-open state", func(t *testing.T) {
		config := DefaultCircuitBreakerConfig()
		config.FailureThreshold = 2
		config.ResetTimeout = 100 * time.Millisecond
		cb := NewCircuitBreaker[TestResult](config)

		// Open the circuit
		for i := 0; i < int(config.FailureThreshold); i++ {
			_, _ = cb.Execute(context.Background(), func() (TestResult, error) {
				return TestResult{}, errors.New("error")
			})
		}

		// Wait for reset timeout
		time.Sleep(config.ResetTimeout + 10*time.Millisecond)

		// First call should be allowed (half-open state)
		_, err := cb.Execute(context.Background(), func() (TestResult, error) {
			return TestResult{Value: "ok"}, nil
		})

		if err != nil {
			t.Errorf("Expected no error in half-open state, got %v", err)
		}
	})
}

func TestDegradation(t *testing.T) {
	t.Run("normal operation", func(t *testing.T) {
		load := &atomic.Value{}
		load.Store(float64(0))
		config := DefaultDegradationConfig[TestResult]()
		config.LoadMetric = func() float64 {
			return load.Load().(float64)
		}

		dh := NewDegradationHandler(config)

		result, err := dh.Execute(context.Background(), func() (TestResult, error) {
			return TestResult{Value: "normal"}, nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result.Value != "normal" {
			t.Errorf("Expected 'normal', got %v", result)
		}
		if dh.GetLevel() != LevelNormal {
			t.Errorf("Expected normal level, got %v", dh.GetLevel())
		}
	})

	t.Run("degradation levels", func(t *testing.T) {
		load := &atomic.Value{}
		load.Store(float64(0))
		config := DefaultDegradationConfig[TestResult]()
		config.LoadMetric = func() float64 {
			return load.Load().(float64)
		}
		config.Fallbacks = map[DegradationLevel]TestResult{
			LevelReduced: {Value: "reduced"},
			LevelMinimal: {Value: "minimal"},
		}

		dh := NewDegradationHandler(config)

		// Test different load levels
		testCases := []struct {
			load          float64
			expectedLevel DegradationLevel
			expectedValue string
		}{
			{0.5, LevelNormal, "normal"},
			{0.75, LevelReduced, "reduced"},
			{0.9, LevelMinimal, "minimal"},
		}

		for _, tc := range testCases {
			load.Store(tc.load)
			result, _ := dh.Execute(context.Background(), func() (TestResult, error) {
				return TestResult{Value: "normal"}, nil
			})

			if dh.GetLevel() != tc.expectedLevel {
				t.Errorf("Expected level %v for load %.2f, got %v",
					tc.expectedLevel, tc.load, dh.GetLevel())
			}
			if result.Value != tc.expectedValue {
				t.Errorf("Expected value %v for load %.2f, got %v",
					tc.expectedValue, tc.load, result.Value)
			}
		}
	})

	t.Run("reduced functionality", func(t *testing.T) {
		load := &atomic.Value{}
		load.Store(float64(0))
		config := DefaultDegradationConfig[TestResult]()
		config.LoadMetric = func() float64 {
			return load.Load().(float64)
		}

		dh := NewDegradationHandler(config)

		normalFn := func() (TestResult, error) {
			return TestResult{Value: "full"}, nil
		}

		reducedFn := func() (TestResult, error) {
			return TestResult{Value: "reduced"}, nil
		}

		// Test normal operation
		load.Store(float64(0.5))
		result, _ := dh.WithReducedFunctionality(context.Background(), normalFn, reducedFn)
		if result.Value != "full" {
			t.Errorf("Expected full functionality, got %v", result.Value)
		}

		// Test reduced operation
		load.Store(float64(0.8))
		result, _ = dh.WithReducedFunctionality(context.Background(), normalFn, reducedFn)
		if result.Value != "reduced" {
			t.Errorf("Expected reduced functionality, got %v", result.Value)
		}
	})
}
