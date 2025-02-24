package metrics

import (
	"sync/atomic"
	"time"
)

// PoolMetrics represents metrics for a worker pool
type PoolMetrics struct {
	tasksSubmitted     atomic.Uint64
	tasksCompleted     atomic.Uint64
	tasksErrored       atomic.Uint64
	totalExecutionTime atomic.Int64
	activeWorkers      atomic.Int32
	queuedTasks        atomic.Int32
}

// NewPoolMetrics creates a new PoolMetrics instance
func NewPoolMetrics() *PoolMetrics {
	return &PoolMetrics{}
}

// TaskSubmitted increments the submitted tasks counter
func (m *PoolMetrics) TaskSubmitted() {
	m.tasksSubmitted.Add(1)
	m.queuedTasks.Add(1)
}

// TaskStarted updates metrics when a task starts
func (m *PoolMetrics) TaskStarted() {
	m.activeWorkers.Add(1)
	m.queuedTasks.Add(-1)
}

// TaskCompleted updates metrics when a task completes successfully
func (m *PoolMetrics) TaskCompleted(duration time.Duration) {
	m.tasksCompleted.Add(1)
	m.activeWorkers.Add(-1)
	m.totalExecutionTime.Add(int64(duration))
}

// TaskErrored updates metrics when a task errors
func (m *PoolMetrics) TaskErrored(duration time.Duration) {
	m.tasksErrored.Add(1)
	m.activeWorkers.Add(-1)
	m.totalExecutionTime.Add(int64(duration))
}

// Snapshot returns the current metrics
type Snapshot struct {
	TasksSubmitted     uint64
	TasksCompleted     uint64
	TasksErrored       uint64
	TotalExecutionTime time.Duration
	ActiveWorkers      int32
	QueuedTasks        int32
	AverageTaskTime    time.Duration
}

// GetSnapshot returns the current state of metrics
func (m *PoolMetrics) GetSnapshot() Snapshot {
	totalTasks := m.tasksCompleted.Load() + m.tasksErrored.Load()
	var avgTime time.Duration
	if totalTasks > 0 {
		avgTime = time.Duration(m.totalExecutionTime.Load() / int64(totalTasks))
	}

	return Snapshot{
		TasksSubmitted:     m.tasksSubmitted.Load(),
		TasksCompleted:     m.tasksCompleted.Load(),
		TasksErrored:       m.tasksErrored.Load(),
		TotalExecutionTime: time.Duration(m.totalExecutionTime.Load()),
		ActiveWorkers:      m.activeWorkers.Load(),
		QueuedTasks:        m.queuedTasks.Load(),
		AverageTaskTime:    avgTime,
	}
}
