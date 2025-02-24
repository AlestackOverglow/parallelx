package metrics

import (
	"testing"
	"time"
)

func TestPoolMetrics(t *testing.T) {
	metrics := NewPoolMetrics()

	// Test task submission
	metrics.TaskSubmitted()
	snapshot := metrics.GetSnapshot()
	if snapshot.TasksSubmitted != 1 {
		t.Errorf("Expected 1 task submitted, got %d", snapshot.TasksSubmitted)
	}
	if snapshot.QueuedTasks != 1 {
		t.Errorf("Expected 1 queued task, got %d", snapshot.QueuedTasks)
	}

	// Test task start
	metrics.TaskStarted()
	snapshot = metrics.GetSnapshot()
	if snapshot.ActiveWorkers != 1 {
		t.Errorf("Expected 1 active worker, got %d", snapshot.ActiveWorkers)
	}
	if snapshot.QueuedTasks != 0 {
		t.Errorf("Expected 0 queued tasks, got %d", snapshot.QueuedTasks)
	}

	// Test task completion
	duration := 100 * time.Millisecond
	metrics.TaskCompleted(duration)
	snapshot = metrics.GetSnapshot()
	if snapshot.TasksCompleted != 1 {
		t.Errorf("Expected 1 task completed, got %d", snapshot.TasksCompleted)
	}
	if snapshot.ActiveWorkers != 0 {
		t.Errorf("Expected 0 active workers, got %d", snapshot.ActiveWorkers)
	}
	if snapshot.TotalExecutionTime != duration {
		t.Errorf("Expected total execution time %v, got %v", duration, snapshot.TotalExecutionTime)
	}

	// Test task error
	metrics.TaskSubmitted()
	metrics.TaskStarted()
	metrics.TaskErrored(duration)
	snapshot = metrics.GetSnapshot()
	if snapshot.TasksErrored != 1 {
		t.Errorf("Expected 1 task errored, got %d", snapshot.TasksErrored)
	}

	// Test average time calculation
	if snapshot.AverageTaskTime != duration {
		t.Errorf("Expected average time %v, got %v", duration, snapshot.AverageTaskTime)
	}
}

func TestConcurrentMetrics(t *testing.T) {
	metrics := NewPoolMetrics()
	taskCount := 1000
	done := make(chan struct{})

	// Simulate concurrent task submissions and completions
	go func() {
		for i := 0; i < taskCount; i++ {
			metrics.TaskSubmitted()
			metrics.TaskStarted()
			if i%2 == 0 {
				metrics.TaskCompleted(100 * time.Millisecond)
			} else {
				metrics.TaskErrored(100 * time.Millisecond)
			}
		}
		close(done)
	}()

	<-done
	snapshot := metrics.GetSnapshot()

	totalTasks := snapshot.TasksCompleted + snapshot.TasksErrored
	if totalTasks != uint64(taskCount) {
		t.Errorf("Expected %d total tasks, got %d", taskCount, totalTasks)
	}

	if snapshot.TasksCompleted != snapshot.TasksErrored {
		t.Error("Expected equal number of completed and errored tasks")
	}

	if snapshot.ActiveWorkers != 0 {
		t.Errorf("Expected 0 active workers after completion, got %d", snapshot.ActiveWorkers)
	}
}
