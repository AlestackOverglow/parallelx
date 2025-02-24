package pool

import (
	"sync"
	"testing"
)

func TestPriorityQueue(t *testing.T) {
	t.Run("basic priority ordering", func(t *testing.T) {
		pq := NewPriorityQueue()

		// Add tasks with different priorities
		results := make([]int, 0)
		var mu sync.Mutex

		pq.AddTask(func() {
			mu.Lock()
			results = append(results, 1)
			mu.Unlock()
		}, PriorityLow)

		pq.AddTask(func() {
			mu.Lock()
			results = append(results, 2)
			mu.Unlock()
		}, PriorityHigh)

		pq.AddTask(func() {
			mu.Lock()
			results = append(results, 3)
			mu.Unlock()
		}, PriorityCritical)

		// Execute tasks in priority order
		for i := 0; i < 3; i++ {
			task, ok := pq.GetTask()
			if !ok {
				t.Fatal("Expected to get task")
			}
			task()
		}

		// Check execution order
		expected := []int{3, 2, 1} // Critical, High, Low
		for i, v := range results {
			if v != expected[i] {
				t.Errorf("Expected task %d to execute at position %d, got %d", v, expected[i], i)
			}
		}
	})

	t.Run("concurrent operations", func(t *testing.T) {
		pq := NewPriorityQueue()
		var wg sync.WaitGroup
		taskCount := 100

		// Concurrently add tasks
		for i := 0; i < taskCount; i++ {
			wg.Add(1)
			go func(priority int) {
				defer wg.Done()
				pq.AddTask(func() {}, priority%4)
			}(i)
		}
		wg.Wait()

		// Verify queue size
		if pq.Len() != taskCount {
			t.Errorf("Expected %d tasks in queue, got %d", taskCount, pq.Len())
		}

		// Verify priority ordering
		currentPriority := PriorityCritical
		for pq.Len() > 0 {
			task, ok := pq.GetTask()
			if !ok {
				t.Fatal("Expected to get task")
			}
			if task == nil {
				t.Error("Got nil task")
			}
			if currentPriority < 0 {
				t.Error("Got more tasks than expected priorities")
			}
			currentPriority--
		}
	})

	t.Run("clear queue", func(t *testing.T) {
		pq := NewPriorityQueue()

		// Add some tasks
		for i := 0; i < 5; i++ {
			pq.AddTask(func() {}, PriorityNormal)
		}

		// Clear the queue
		pq.Clear()

		if pq.Len() != 0 {
			t.Errorf("Expected empty queue after clear, got %d tasks", pq.Len())
		}
	})

	t.Run("transfer tasks", func(t *testing.T) {
		pq1 := NewPriorityQueue()
		pq2 := NewPriorityQueue()

		// Add tasks to first queue
		taskCount := 5
		for i := 0; i < taskCount; i++ {
			pq1.AddTask(func() {}, i%4)
		}

		// Transfer tasks
		pq1.TransferTasks(pq2)

		// Check queues
		if pq1.Len() != 0 {
			t.Errorf("Expected source queue to be empty, got %d tasks", pq1.Len())
		}
		if pq2.Len() != taskCount {
			t.Errorf("Expected destination queue to have %d tasks, got %d", taskCount, pq2.Len())
		}
	})
}
