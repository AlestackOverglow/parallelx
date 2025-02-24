package resilience

import (
	"context"
	"sync/atomic"
)

// DegradationConfig defines configuration for graceful degradation
type DegradationConfig[T any] struct {
	Levels     []DegradationLevel
	Thresholds []float64
	LoadMetric func() float64
	Fallbacks  map[DegradationLevel]T
}

// DegradationHandler manages graceful degradation with type safety
type DegradationHandler[T any] struct {
	config       DegradationConfig[T]
	currentLevel atomic.Int32
}

// NewDegradationHandler creates a new degradation handler
func NewDegradationHandler[T any](config DegradationConfig[T]) *DegradationHandler[T] {
	dh := &DegradationHandler[T]{
		config: config,
	}
	dh.currentLevel.Store(int32(LevelNormal))
	return dh
}

// Execute runs the function with graceful degradation
func (dh *DegradationHandler[T]) Execute(ctx context.Context, fn func() (T, error)) (T, error) {
	var zero T
	level := dh.updateLevel()

	if level != LevelNormal {
		if fallback, ok := dh.config.Fallbacks[level]; ok {
			return fallback, nil
		}
	}

	result, err := fn()
	if err != nil {
		if fallback, ok := dh.config.Fallbacks[dh.GetLevel()]; ok {
			return fallback, nil
		}
		return zero, err
	}

	return result, nil
}

// ExecuteWithFallback runs the function with explicit fallback values for each level
func (dh *DegradationHandler[T]) ExecuteWithFallback(
	ctx context.Context,
	fn func() (T, error),
	fallbacks map[DegradationLevel]T,
) (T, error) {
	var zero T
	level := dh.updateLevel()

	if level != LevelNormal {
		if fallback, ok := fallbacks[level]; ok {
			return fallback, nil
		}
	}

	result, err := fn()
	if err != nil {
		if fallback, ok := fallbacks[dh.GetLevel()]; ok {
			return fallback, nil
		}
		return zero, err
	}

	return result, nil
}

// WithReducedFunctionality executes function with reduced functionality
func (dh *DegradationHandler[T]) WithReducedFunctionality(
	ctx context.Context,
	normalFn func() (T, error),
	reducedFn func() (T, error),
) (T, error) {
	level := dh.updateLevel()
	if level > LevelNormal {
		return reducedFn()
	}
	return normalFn()
}

// updateLevel updates the degradation level based on current load
func (dh *DegradationHandler[T]) updateLevel() DegradationLevel {
	if dh.config.LoadMetric == nil {
		return LevelNormal
	}

	load := dh.config.LoadMetric()
	var newLevel DegradationLevel = LevelNormal

	// Iterate through thresholds in reverse order to find the highest applicable level
	for i := len(dh.config.Thresholds) - 1; i >= 0; i-- {
		if load >= dh.config.Thresholds[i] {
			newLevel = dh.config.Levels[i+1] // i+1 because first level is Normal
			break
		}
	}

	dh.currentLevel.Store(int32(newLevel))
	return newLevel
}

// GetLevel returns the current degradation level
func (dh *DegradationHandler[T]) GetLevel() DegradationLevel {
	return DegradationLevel(dh.currentLevel.Load())
}

// DefaultDegradationConfig returns default degradation configuration
func DefaultDegradationConfig[T any]() DegradationConfig[T] {
	return DegradationConfig[T]{
		Levels: []DegradationLevel{
			LevelNormal,
			LevelReduced,
			LevelMinimal,
			LevelEmergency,
		},
		Thresholds: []float64{0.7, 0.85, 0.95},
		LoadMetric: func() float64 { return 0.0 },
		Fallbacks:  make(map[DegradationLevel]T),
	}
}
