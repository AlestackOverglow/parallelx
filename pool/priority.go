package pool

import (
	"container/heap"
	"sync"
)

// Priority levels for tasks
const (
	PriorityLow      = 0
	PriorityNormal   = 1
	PriorityHigh     = 2
	PriorityCritical = 3
)

// PriorityTask represents a task with priority
type PriorityTask struct {
	task     func()
	priority int
	index    int // index in the heap
}

// PriorityQueue implements heap.Interface and holds PriorityTasks
type PriorityQueue struct {
	tasks []*PriorityTask
	mu    sync.Mutex
}

func NewPriorityQueue() *PriorityQueue {
	pq := &PriorityQueue{
		tasks: make([]*PriorityTask, 0),
	}
	heap.Init(pq)
	return pq
}

// Len returns the number of tasks in the queue
func (pq *PriorityQueue) Len() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return len(pq.tasks)
}

// Less compares tasks by priority (higher priority comes first)
func (pq *PriorityQueue) Less(i, j int) bool {
	return pq.tasks[i].priority > pq.tasks[j].priority
}

// Swap swaps tasks at positions i and j
func (pq *PriorityQueue) Swap(i, j int) {
	pq.tasks[i], pq.tasks[j] = pq.tasks[j], pq.tasks[i]
	pq.tasks[i].index = i
	pq.tasks[j].index = j
}

// Push adds a task to the queue
func (pq *PriorityQueue) Push(x interface{}) {
	task := x.(*PriorityTask)
	task.index = len(pq.tasks)
	pq.tasks = append(pq.tasks, task)
}

// Pop removes and returns the highest priority task
func (pq *PriorityQueue) Pop() interface{} {
	old := pq.tasks
	n := len(old)
	task := old[n-1]
	old[n-1] = nil
	task.index = -1
	pq.tasks = old[0 : n-1]
	return task
}

// AddTask adds a new task with priority
func (pq *PriorityQueue) AddTask(task func(), priority int) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pt := &PriorityTask{
		task:     task,
		priority: priority,
	}
	heap.Push(pq, pt)
}

// GetTask returns the highest priority task
func (pq *PriorityQueue) GetTask() (func(), bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	if len(pq.tasks) == 0 {
		return nil, false
	}
	task := heap.Pop(pq).(*PriorityTask)
	return task.task, true
}

// Clear removes all tasks from the queue
func (pq *PriorityQueue) Clear() {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.tasks = make([]*PriorityTask, 0)
}

// TransferTasks transfers all tasks to another queue
func (pq *PriorityQueue) TransferTasks(newQueue *PriorityQueue) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for _, task := range pq.tasks {
		newQueue.AddTask(task.task, task.priority)
	}
	pq.tasks = make([]*PriorityTask, 0)
}
