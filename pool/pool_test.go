package pool

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool(t *testing.T) {
	t.Run("basic functionality", func(t *testing.T) {
		pool := New(3)
		defer pool.Shutdown()

		var counter int32
		var wg sync.WaitGroup

		for i := 0; i < 10; i++ {
			wg.Add(1)
			if !pool.Submit(func() {
				defer wg.Done()
				atomic.AddInt32(&counter, 1)
			}) {
				t.Error("Failed to submit task")
			}
		}

		wg.Wait()
		if counter != 10 {
			t.Errorf("Expected counter to be 10, got %d", counter)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		pool := WithContext(ctx, 3)
		defer pool.Shutdown()

		var counter int32
		var wg sync.WaitGroup

		// Submit some tasks
		for i := 0; i < 5; i++ {
			wg.Add(1)
			pool.Submit(func() {
				defer wg.Done()
				time.Sleep(100 * time.Millisecond)
				atomic.AddInt32(&counter, 1)
			})
		}

		// Cancel context before all tasks complete
		cancel()
		wg.Wait()

		if counter >= 5 {
			t.Error("Expected some tasks to be cancelled")
		}
	})

	t.Run("shutdown behavior", func(t *testing.T) {
		pool := New(3)

		var counter int32
		var wg sync.WaitGroup

		// Submit tasks
		for i := 0; i < 5; i++ {
			wg.Add(1)
			pool.Submit(func() {
				defer wg.Done()
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt32(&counter, 1)
			})
		}

		// Shutdown while tasks are running
		pool.Shutdown()
		wg.Wait()

		// Try to submit after shutdown
		if pool.Submit(func() {}) {
			t.Error("Should not be able to submit tasks after shutdown")
		}

		if counter != 5 {
			t.Errorf("Expected 5 tasks to complete, got %d", counter)
		}
	})
}

func TestPoolMetrics(t *testing.T) {
	pool := New(3)
	defer pool.Shutdown()

	// Submit some tasks
	taskCount := 5
	var wg sync.WaitGroup
	for i := 0; i < taskCount; i++ {
		wg.Add(1)
		pool.Submit(func() {
			time.Sleep(50 * time.Millisecond)
			wg.Done()
		})
	}

	// Wait for tasks to complete
	wg.Wait()

	// Check metrics
	metrics := pool.GetMetrics()
	if metrics.TasksSubmitted != uint64(taskCount) {
		t.Errorf("Expected %d tasks submitted, got %d", taskCount, metrics.TasksSubmitted)
	}
	if metrics.TasksCompleted != uint64(taskCount) {
		t.Errorf("Expected %d tasks completed, got %d", taskCount, metrics.TasksCompleted)
	}
	if metrics.TasksErrored != 0 {
		t.Errorf("Expected 0 tasks errored, got %d", metrics.TasksErrored)
	}
	if metrics.ActiveWorkers != 0 {
		t.Errorf("Expected 0 active workers, got %d", metrics.ActiveWorkers)
	}
}

func TestPoolMetricsWithErrors(t *testing.T) {
	pool := New(3)
	defer pool.Shutdown()

	errorCount := 0
	pool.WithErrorHandler(func(err error) {
		errorCount++
	})

	// Submit tasks that will error
	taskCount := 5
	var wg sync.WaitGroup
	for i := 0; i < taskCount; i++ {
		wg.Add(1)
		pool.SubmitWithError(func() error {
			defer wg.Done()
			time.Sleep(50 * time.Millisecond)
			return fmt.Errorf("test error")
		})
	}

	// Wait for tasks to complete
	wg.Wait()

	// Check metrics
	metrics := pool.GetMetrics()
	if metrics.TasksSubmitted != uint64(taskCount) {
		t.Errorf("Expected %d tasks submitted, got %d", taskCount, metrics.TasksSubmitted)
	}
	if metrics.TasksErrored != uint64(taskCount) {
		t.Errorf("Expected %d tasks errored, got %d", taskCount, metrics.TasksErrored)
	}
	if errorCount != taskCount {
		t.Errorf("Expected %d error handler calls, got %d", taskCount, errorCount)
	}
	if metrics.AverageTaskTime == 0 {
		t.Error("Expected non-zero average task time")
	}
}

func TestPoolPauseResume(t *testing.T) {
	pool := New(3)
	defer pool.Shutdown()

	// Тест базовой функциональности паузы
	t.Run("basic pause/resume", func(t *testing.T) {
		var counter int32
		var wg sync.WaitGroup

		// Запускаем задачи
		for i := 0; i < 5; i++ {
			wg.Add(1)
			pool.Submit(func() {
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt32(&counter, 1)
				wg.Done()
			})
		}

		// Приостанавливаем пул
		pool.Pause()
		if !pool.IsPaused() {
			t.Error("Pool should be paused")
		}

		// Добавляем еще задач во время паузы
		for i := 0; i < 5; i++ {
			wg.Add(1)
			pool.Submit(func() {
				atomic.AddInt32(&counter, 1)
				wg.Done()
			})
		}

		// Проверяем, что счетчик не увеличился сразу
		time.Sleep(100 * time.Millisecond)
		counterAfterPause := atomic.LoadInt32(&counter)

		// Возобновляем работу
		pool.Resume()
		if pool.IsPaused() {
			t.Error("Pool should not be paused")
		}

		// Ждем завершения всех задач
		wg.Wait()

		// Проверяем, что все задачи выполнились
		if counter != 10 {
			t.Errorf("Expected 10 tasks to complete, got %d", counter)
		}

		// Проверяем, что во время паузы выполнились не все задачи
		if counterAfterPause >= 10 {
			t.Error("Too many tasks completed while pool was paused")
		}
	})

	// Тест множественных пауз/возобновлений
	t.Run("multiple pause/resume", func(t *testing.T) {
		var counter int32
		var wg sync.WaitGroup

		for i := 0; i < 10; i++ {
			wg.Add(1)
			pool.Submit(func() {
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt32(&counter, 1)
				wg.Done()
			})

			if i%2 == 0 {
				pool.Pause()
				time.Sleep(10 * time.Millisecond)
				pool.Resume()
			}
		}

		wg.Wait()

		if counter != 10 {
			t.Errorf("Expected 10 tasks to complete, got %d", counter)
		}
	})

	// Тест паузы во время выполнения длинных задач
	t.Run("pause with long-running tasks", func(t *testing.T) {
		var counter int32
		var wg sync.WaitGroup

		// Запускаем длинную задачу
		wg.Add(1)
		pool.Submit(func() {
			time.Sleep(200 * time.Millisecond)
			atomic.AddInt32(&counter, 1)
			wg.Done()
		})

		// Приостанавливаем пул
		time.Sleep(50 * time.Millisecond)
		pool.Pause()

		// Добавляем короткую задачу
		wg.Add(1)
		pool.Submit(func() {
			atomic.AddInt32(&counter, 1)
			wg.Done()
		})

		// Проверяем состояние после небольшой задержки
		time.Sleep(100 * time.Millisecond)
		counterMid := atomic.LoadInt32(&counter)

		// Возобновляем работу
		pool.Resume()
		wg.Wait()

		// Проверяем, что длинная задача завершилась во время паузы
		if counterMid != 1 {
			t.Error("Long-running task should complete during pause")
		}

		// Проверяем, что все задачи завершились после возобновления
		if counter != 2 {
			t.Errorf("Expected 2 tasks to complete, got %d", counter)
		}
	})
}

func TestPoolScaling(t *testing.T) {
	pool := New(3)
	defer pool.Shutdown()

	t.Run("scale up", func(t *testing.T) {
		// Initial worker count
		if count := pool.GetWorkerCount(); count != 3 {
			t.Errorf("Expected 3 workers, got %d", count)
		}

		// Scale up to 5 workers
		if err := pool.Scale(5); err != nil {
			t.Errorf("Failed to scale up: %v", err)
		}

		if count := pool.GetWorkerCount(); count != 5 {
			t.Errorf("Expected 5 workers after scale up, got %d", count)
		}

		// Test functionality with increased workers
		var counter int32
		var wg sync.WaitGroup

		for i := 0; i < 10; i++ {
			wg.Add(1)
			pool.Submit(func() {
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt32(&counter, 1)
				wg.Done()
			})
		}

		wg.Wait()
		if counter != 10 {
			t.Errorf("Expected 10 tasks to complete, got %d", counter)
		}
	})

	t.Run("scale down", func(t *testing.T) {
		// Scale down to 2 workers
		if err := pool.Scale(2); err != nil {
			t.Errorf("Failed to scale down: %v", err)
		}

		if count := pool.GetWorkerCount(); count != 2 {
			t.Errorf("Expected 2 workers after scale down, got %d", count)
		}

		// Test functionality with decreased workers
		var counter int32
		var wg sync.WaitGroup

		for i := 0; i < 5; i++ {
			wg.Add(1)
			pool.Submit(func() {
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt32(&counter, 1)
				wg.Done()
			})
		}

		wg.Wait()
		if counter != 5 {
			t.Errorf("Expected 5 tasks to complete, got %d", counter)
		}
	})

	t.Run("invalid scale parameters", func(t *testing.T) {
		// Try to scale to 0 workers
		if err := pool.Scale(0); err == nil {
			t.Error("Expected error when scaling to 0 workers")
		}

		// Try to scale to negative workers
		if err := pool.Scale(-1); err == nil {
			t.Error("Expected error when scaling to negative workers")
		}
	})

	t.Run("scale during task execution", func(t *testing.T) {
		var counter int32
		var wg sync.WaitGroup

		// Submit long-running tasks
		for i := 0; i < 5; i++ {
			wg.Add(1)
			pool.Submit(func() {
				time.Sleep(100 * time.Millisecond)
				atomic.AddInt32(&counter, 1)
				wg.Done()
			})
		}

		// Scale while tasks are running
		if err := pool.Scale(4); err != nil {
			t.Errorf("Failed to scale during execution: %v", err)
		}

		// Submit more tasks
		for i := 0; i < 5; i++ {
			wg.Add(1)
			pool.Submit(func() {
				atomic.AddInt32(&counter, 1)
				wg.Done()
			})
		}

		wg.Wait()
		if counter != 10 {
			t.Errorf("Expected 10 tasks to complete after scaling, got %d", counter)
		}
	})
}

func TestPoolPriority(t *testing.T) {
	pool := New(3)
	defer pool.Shutdown()

	t.Run("priority execution order", func(t *testing.T) {
		var results []int
		var mu sync.Mutex
		var wg sync.WaitGroup

		// Submit tasks with different priorities
		wg.Add(3)

		// Low priority task
		pool.SubmitWithPriority(func() {
			defer wg.Done()
			time.Sleep(50 * time.Millisecond)
			mu.Lock()
			results = append(results, 1)
			mu.Unlock()
		}, PriorityLow)

		// High priority task
		pool.SubmitWithPriority(func() {
			defer wg.Done()
			time.Sleep(50 * time.Millisecond)
			mu.Lock()
			results = append(results, 2)
			mu.Unlock()
		}, PriorityHigh)

		// Critical priority task
		pool.SubmitWithPriority(func() {
			defer wg.Done()
			time.Sleep(50 * time.Millisecond)
			mu.Lock()
			results = append(results, 3)
			mu.Unlock()
		}, PriorityCritical)

		wg.Wait()

		// Check execution order
		expected := []int{3, 2, 1} // Critical, High, Low
		if len(results) != len(expected) {
			t.Errorf("Expected %d results, got %d", len(expected), len(results))
		}
		for i, v := range results {
			if v != expected[i] {
				t.Errorf("Expected task %d at position %d, got %d", expected[i], i, v)
			}
		}
	})

	t.Run("mixed priority tasks", func(t *testing.T) {
		var completedTasks int32
		var criticalTasks int32
		var wg sync.WaitGroup

		taskCount := 20
		wg.Add(taskCount)

		// Submit mix of normal and critical tasks
		for i := 0; i < taskCount; i++ {
			priority := PriorityNormal
			if i%4 == 0 {
				priority = PriorityCritical
			}

			pool.SubmitWithPriority(func() {
				defer wg.Done()
				if priority == PriorityCritical {
					atomic.AddInt32(&criticalTasks, 1)
				}
				time.Sleep(10 * time.Millisecond)
				atomic.AddInt32(&completedTasks, 1)
			}, priority)
		}

		wg.Wait()

		if completedTasks != int32(taskCount) {
			t.Errorf("Expected %d tasks to complete, got %d", taskCount, completedTasks)
		}

		expectedCritical := taskCount / 4
		if criticalTasks != int32(expectedCritical) {
			t.Errorf("Expected %d critical tasks, got %d", expectedCritical, criticalTasks)
		}
	})

	t.Run("error handling with priority", func(t *testing.T) {
		var errorCount int32
		pool.WithErrorHandler(func(err error) {
			atomic.AddInt32(&errorCount, 1)
		})

		var wg sync.WaitGroup
		wg.Add(2)

		// Submit error task with high priority
		pool.SubmitWithErrorAndPriority(func() error {
			defer wg.Done()
			return fmt.Errorf("high priority error")
		}, PriorityHigh)

		// Submit error task with low priority
		pool.SubmitWithErrorAndPriority(func() error {
			defer wg.Done()
			return fmt.Errorf("low priority error")
		}, PriorityLow)

		wg.Wait()

		if errorCount != 2 {
			t.Errorf("Expected 2 errors, got %d", errorCount)
		}
	})
}

func TestPeriodicTasks(t *testing.T) {
	pool := New(3)
	defer pool.Shutdown()

	t.Run("basic periodic execution", func(t *testing.T) {
		var counter int32
		interval := 100 * time.Millisecond

		// Создаем периодическую задачу
		pt, err := pool.SubmitPeriodic(func() error {
			atomic.AddInt32(&counter, 1)
			return nil
		}, interval)

		if err != nil {
			t.Fatalf("Failed to submit periodic task: %v", err)
		}

		// Ждем несколько выполнений
		time.Sleep(350 * time.Millisecond)
		pool.StopPeriodic(pt)

		// Проверяем, что задача выполнилась примерно 3-4 раза
		count := atomic.LoadInt32(&counter)
		if count < 3 || count > 4 {
			t.Errorf("Expected 3-4 executions, got %d", count)
		}
	})

	t.Run("periodic task with priority", func(t *testing.T) {
		var normalCount, criticalCount int32
		interval := 50 * time.Millisecond

		// Создаем периодическую задачу с нормальным приоритетом
		ptNormal, err := pool.SubmitPeriodic(func() error {
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt32(&normalCount, 1)
			return nil
		}, interval)
		if err != nil {
			t.Fatalf("Failed to submit normal priority task: %v", err)
		}

		// Создаем периодическую задачу с критическим приоритетом
		ptCritical, err := pool.SubmitPeriodicWithPriority(func() error {
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt32(&criticalCount, 1)
			return nil
		}, interval, PriorityCritical)
		if err != nil {
			t.Fatalf("Failed to submit critical priority task: %v", err)
		}

		// Ждем несколько выполнений
		time.Sleep(200 * time.Millisecond)

		pool.StopPeriodic(ptNormal)
		pool.StopPeriodic(ptCritical)

		// Проверяем, что критические задачи выполнялись чаще
		nCount := atomic.LoadInt32(&normalCount)
		cCount := atomic.LoadInt32(&criticalCount)
		if cCount <= nCount {
			t.Errorf("Expected critical tasks (%d) to execute more than normal tasks (%d)", cCount, nCount)
		}
	})

	t.Run("error handling in periodic tasks", func(t *testing.T) {
		var errorCount int32
		interval := 50 * time.Millisecond

		pool.WithErrorHandler(func(err error) {
			if err.Error() == "test error" {
				atomic.AddInt32(&errorCount, 1)
			}
		})

		// Создаем периодическую задачу с ошибкой
		pt, err := pool.SubmitPeriodic(func() error {
			return fmt.Errorf("test error")
		}, interval)
		if err != nil {
			t.Fatalf("Failed to submit periodic task: %v", err)
		}

		// Ждем несколько выполнений
		time.Sleep(200 * time.Millisecond)
		pool.StopPeriodic(pt)

		// Проверяем, что ошибки были обработаны
		count := atomic.LoadInt32(&errorCount)
		if count < 3 {
			t.Errorf("Expected at least 3 errors, got %d", count)
		}
	})

	t.Run("invalid interval", func(t *testing.T) {
		_, err := pool.SubmitPeriodic(func() error { return nil }, 0)
		if err == nil {
			t.Error("Expected error for zero interval")
		}

		_, err = pool.SubmitPeriodic(func() error { return nil }, -1*time.Second)
		if err == nil {
			t.Error("Expected error for negative interval")
		}
	})

	t.Run("shutdown behavior", func(t *testing.T) {
		var counter int32
		interval := 50 * time.Millisecond

		pt, err := pool.SubmitPeriodic(func() error {
			atomic.AddInt32(&counter, 1)
			return nil
		}, interval)
		if err != nil {
			t.Fatalf("Failed to submit periodic task: %v", err)
		}

		// Ждем несколько выполнений
		time.Sleep(200 * time.Millisecond)

		// Завершаем работу пула
		pool.Shutdown()

		// Пробуем остановить задачу после завершения работы пула
		pool.StopPeriodic(pt)

		// Ждем еще немного
		time.Sleep(200 * time.Millisecond)

		// Запоминаем значение счетчика
		countAfterShutdown := atomic.LoadInt32(&counter)

		// Проверяем, что счетчик не увеличился после завершения работы
		if atomic.LoadInt32(&counter) != countAfterShutdown {
			t.Error("Tasks continued to execute after shutdown")
		}

		// Проверяем, что нельзя добавить новую периодическую задачу
		_, err = pool.SubmitPeriodic(func() error { return nil }, interval)
		if err == nil {
			t.Error("Should not be able to submit tasks after shutdown")
		}
	})
}

func TestStreamResults(t *testing.T) {
	pool := New(3)
	defer pool.Shutdown()

	t.Run("single result stream", func(t *testing.T) {
		// Создаем задачу, возвращающую результат
		task := func() (interface{}, error) {
			time.Sleep(50 * time.Millisecond)
			return "test result", nil
		}

		// Отправляем задачу и получаем канал результатов
		resultChan := pool.SubmitStream(task)

		// Получаем результат
		result := <-resultChan
		if result.Err != nil {
			t.Errorf("Unexpected error: %v", result.Err)
		}
		if result.Value != "test result" {
			t.Errorf("Expected 'test result', got %v", result.Value)
		}

		// Проверяем, что канал закрыт
		if _, ok := <-resultChan; ok {
			t.Error("Expected channel to be closed")
		}
	})

	t.Run("stream with error", func(t *testing.T) {
		// Создаем задачу, возвращающую ошибку
		task := func() (interface{}, error) {
			return nil, fmt.Errorf("test error")
		}

		// Отправляем задачу и получаем результат
		resultChan := pool.SubmitStream(task)
		result := <-resultChan

		if result.Err == nil {
			t.Error("Expected error, got nil")
		}
		if result.Err.Error() != "test error" {
			t.Errorf("Expected 'test error', got %v", result.Err)
		}
	})

	t.Run("batch stream results", func(t *testing.T) {
		expected := []int{1, 2, 3, 4, 5}

		// Создаем пакетную задачу
		task := func(results chan<- Result) {
			for _, val := range expected {
				results <- Result{Value: val, Err: nil}
				time.Sleep(10 * time.Millisecond)
			}
		}

		// Отправляем задачу и получаем канал результатов
		resultChan := pool.SubmitBatchStream(task)

		// Собираем результаты
		var received []int
		for result := range resultChan {
			if result.Err != nil {
				t.Errorf("Unexpected error: %v", result.Err)
			}
			received = append(received, result.Value.(int))
		}

		// Проверяем результаты
		if len(received) != len(expected) {
			t.Errorf("Expected %d results, got %d", len(expected), len(received))
		}
		for i, val := range expected {
			if received[i] != val {
				t.Errorf("Expected %d at position %d, got %d", val, i, received[i])
			}
		}
	})

	t.Run("priority stream results", func(t *testing.T) {
		var results []int
		var mu sync.Mutex

		// Создаем задачи с разными приоритетами
		lowTask := func() (interface{}, error) {
			time.Sleep(50 * time.Millisecond)
			return 1, nil
		}
		highTask := func() (interface{}, error) {
			time.Sleep(50 * time.Millisecond)
			return 2, nil
		}
		criticalTask := func() (interface{}, error) {
			time.Sleep(50 * time.Millisecond)
			return 3, nil
		}

		// Отправляем задачи
		chans := []chan Result{
			pool.SubmitStreamWithPriority(lowTask, PriorityLow),
			pool.SubmitStreamWithPriority(highTask, PriorityHigh),
			pool.SubmitStreamWithPriority(criticalTask, PriorityCritical),
		}

		// Собираем результаты
		var wg sync.WaitGroup
		for _, ch := range chans {
			wg.Add(1)
			go func(c chan Result) {
				defer wg.Done()
				result := <-c
				if result.Err != nil {
					t.Errorf("Unexpected error: %v", result.Err)
					return
				}
				mu.Lock()
				results = append(results, result.Value.(int))
				mu.Unlock()
			}(ch)
		}

		wg.Wait()

		// Проверяем порядок выполнения
		expected := []int{3, 2, 1} // Critical, High, Low
		if len(results) != len(expected) {
			t.Errorf("Expected %d results, got %d", len(expected), len(results))
		}
		for i, val := range results {
			if val != expected[i] {
				t.Errorf("Expected %d at position %d, got %d", expected[i], i, val)
			}
		}
	})

	t.Run("batch stream with context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		pool := WithContext(ctx, 3)
		defer pool.Shutdown()

		// Создаем пакетную задачу с бесконечным циклом
		task := func(results chan<- Result) {
			count := 0
			for {
				select {
				case <-ctx.Done():
					return
				default:
					results <- Result{Value: count, Err: nil}
					count++
					time.Sleep(10 * time.Millisecond)
				}
			}
		}

		// Отправляем задачу
		resultChan := pool.SubmitBatchStream(task)

		// Читаем несколько результатов
		receivedCount := 0
		timeout := time.After(100 * time.Millisecond)

	readLoop:
		for {
			select {
			case result, ok := <-resultChan:
				if !ok {
					break readLoop
				}
				if result.Err != nil {
					t.Errorf("Unexpected error: %v", result.Err)
				}
				receivedCount++
			case <-timeout:
				cancel()
				break readLoop
			}
		}

		if receivedCount == 0 {
			t.Error("Expected to receive some results before cancellation")
		}
	})
}
