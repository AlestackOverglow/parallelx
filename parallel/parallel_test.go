package parallel

import (
	"context"
	"testing"
	"time"
)

func TestExecute(t *testing.T) {
	t.Run("basic execution", func(t *testing.T) {
		tasks := []Task{
			func() interface{} { return 1 },
			func() interface{} { return 2 },
			func() interface{} { return 3 },
		}

		results, err := Execute(tasks...)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if len(results) != 3 {
			t.Errorf("Expected 3 results, got %d", len(results))
		}

		for i, result := range results {
			if result != i+1 {
				t.Errorf("Expected result %d to be %d, got %v", i, i+1, result)
			}
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		tasks := []Task{
			func() interface{} {
				time.Sleep(100 * time.Millisecond)
				return 1
			},
			func() interface{} {
				time.Sleep(100 * time.Millisecond)
				return 2
			},
		}

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		results, err := ExecuteContext(ctx, tasks...)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		for _, result := range results {
			if err, ok := result.(error); ok {
				if err != context.Canceled {
					t.Errorf("Expected context.Canceled error, got %v", err)
				}
			}
		}
	})
}

func TestExecuteWithLimit(t *testing.T) {
	t.Run("respects concurrency limit", func(t *testing.T) {
		const limit = 2
		running := make(chan struct{}, 10)
		completed := 0

		tasks := make([]Task, 5)
		for i := range tasks {
			tasks[i] = func() interface{} {
				running <- struct{}{}
				time.Sleep(50 * time.Millisecond)
				<-running
				completed++
				return nil
			}
		}

		results, err := ExecuteWithLimit(limit, tasks...)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if len(results) != 5 {
			t.Errorf("Expected 5 results, got %d", len(results))
		}

		if completed != 5 {
			t.Errorf("Expected 5 tasks to complete, got %d", completed)
		}

		if len(running) > limit {
			t.Errorf("More than %d tasks were running concurrently", limit)
		}
	})
}

func TestMap(t *testing.T) {
	t.Run("parallel mapping", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5}
		expected := []int{2, 4, 6, 8, 10}

		results := Map(input, func(x int) int {
			return x * 2
		})

		if len(results) != len(expected) {
			t.Errorf("Expected %d results, got %d", len(expected), len(results))
		}

		for i, result := range results {
			if result != expected[i] {
				t.Errorf("Expected result %d to be %d, got %d", i, expected[i], result)
			}
		}
	})
}
