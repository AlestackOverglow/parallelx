package parallel

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Task represents a function that returns an interface{} and can be executed in parallel
type Task func() interface{}

// Execute runs the given tasks in parallel and returns their results
// The results are returned in the same order as the input tasks
func Execute(tasks ...Task) ([]interface{}, error) {
	return ExecuteContext(context.Background(), tasks...)
}

// ExecuteContext runs the given tasks in parallel with context support
func ExecuteContext(ctx context.Context, tasks ...Task) ([]interface{}, error) {
	results := make([]interface{}, len(tasks))
	var wg sync.WaitGroup

	for i, task := range tasks {
		wg.Add(1)
		go func(index int, t Task) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				results[index] = ctx.Err()
			default:
				results[index] = t()
			}
		}(i, task)
	}

	wg.Wait()
	return results, nil
}

// ExecuteWithLimit runs tasks in parallel but limits the number of concurrent executions
func ExecuteWithLimit(limit int, tasks ...Task) ([]interface{}, error) {
	return ExecuteWithLimitContext(context.Background(), limit, tasks...)
}

// ExecuteWithLimitContext runs tasks in parallel with a concurrency limit and context support
func ExecuteWithLimitContext(ctx context.Context, limit int, tasks ...Task) ([]interface{}, error) {
	if limit <= 0 {
		limit = len(tasks)
	}

	results := make([]interface{}, len(tasks))
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup

	for i, task := range tasks {
		wg.Add(1)
		go func(index int, t Task) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				select {
				case <-ctx.Done():
					results[index] = ctx.Err()
				default:
					results[index] = t()
				}
			case <-ctx.Done():
				results[index] = ctx.Err()
			}
		}(i, task)
	}

	wg.Wait()
	return results, nil
}

// Map executes the mapper function on each item in parallel
func Map[T, R any](items []T, mapper func(T) R) []R {
	results := make([]R, len(items))
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		go func(index int, value T) {
			defer wg.Done()
			results[index] = mapper(value)
		}(i, item)
	}

	wg.Wait()
	return results
}

// ExecuteWithTimeout runs tasks in parallel with a timeout for each task
func ExecuteWithTimeout(timeout time.Duration, tasks ...Task) ([]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return ExecuteContext(ctx, tasks...)
}

// ExecuteWithTimeoutPerTask runs tasks in parallel with individual timeouts for each task
func ExecuteWithTimeoutPerTask(timeout time.Duration, tasks ...Task) ([]interface{}, error) {
	results := make([]interface{}, len(tasks))
	var wg sync.WaitGroup

	for i, task := range tasks {
		wg.Add(1)
		go func(index int, t Task) {
			defer wg.Done()

			done := make(chan interface{}, 1)
			go func() {
				done <- t()
			}()

			select {
			case result := <-done:
				results[index] = result
			case <-time.After(timeout):
				results[index] = fmt.Errorf("task timeout after %v", timeout)
			}
		}(i, task)
	}

	wg.Wait()
	return results, nil
}
