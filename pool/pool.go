package pool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/AlestackOverglow/parallelx/metrics"
)

// ErrorHandler is a function that handles errors from tasks
type ErrorHandler func(error)

// Pool represents a worker pool that can execute tasks concurrently
type Pool struct {
	workers      int
	tasks        *PriorityQueue
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	isRunning    bool
	isPaused     bool
	mu           sync.Mutex
	errorHandler ErrorHandler
	metrics      *metrics.PoolMetrics
	pauseChan    chan struct{}
	resumeChan   chan struct{}
	workerChan   chan struct{}
	taskChan     chan struct{} // Channel to signal new task availability
}

// New creates a new worker pool with the specified number of workers
func New(workers int) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		workers:    workers,
		tasks:      NewPriorityQueue(),
		ctx:        ctx,
		cancel:     cancel,
		isRunning:  true,
		metrics:    metrics.NewPoolMetrics(),
		pauseChan:  make(chan struct{}),
		resumeChan: make(chan struct{}),
		workerChan: make(chan struct{}),
		taskChan:   make(chan struct{}, 1),
	}

	p.start()
	return p
}

// start initializes and starts the worker goroutines
func (p *Pool) start() {
	for i := 0; i < p.workers; i++ {
		p.startWorker()
	}
}

// startWorker starts a single worker goroutine
func (p *Pool) startWorker() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			select {
			case <-p.taskChan:
				// Check for pause state
				select {
				case <-p.pauseChan:
					<-p.resumeChan
				default:
				}

				// Get next task from priority queue
				if task, ok := p.tasks.GetTask(); ok {
					p.metrics.TaskStarted()
					start := time.Now()
					func() {
						defer func() {
							if r := recover(); r != nil {
								duration := time.Since(start)
								p.metrics.TaskErrored(duration)
								if p.errorHandler != nil {
									if err, ok := r.(error); ok {
										p.errorHandler(err)
									}
								}
							}
						}()
						task()
						p.metrics.TaskCompleted(time.Since(start))
					}()
				}
			case <-p.workerChan:
				return
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

// Scale changes the number of workers in the pool
func (p *Pool) Scale(workers int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		return fmt.Errorf("cannot scale: pool is not running")
	}

	if workers <= 0 {
		return fmt.Errorf("worker count must be positive")
	}

	// Calculate the difference in workers
	diff := workers - p.workers

	if diff > 0 {
		// Add new workers
		for i := 0; i < diff; i++ {
			p.startWorker()
		}
	} else if diff < 0 {
		// Remove excess workers
		for i := 0; i < -diff; i++ {
			p.workerChan <- struct{}{}
		}
	}

	// Create new queue and transfer tasks
	newTasks := NewPriorityQueue()
	p.tasks.TransferTasks(newTasks)
	p.tasks = newTasks
	p.workers = workers

	return nil
}

// GetWorkerCount returns the current number of workers
func (p *Pool) GetWorkerCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.workers
}

// Submit adds a task to the pool with normal priority
func (p *Pool) Submit(task func()) bool {
	return p.SubmitWithPriority(task, PriorityNormal)
}

// SubmitWithPriority adds a task to the pool with specified priority
func (p *Pool) SubmitWithPriority(task func(), priority int) bool {
	p.mu.Lock()
	if !p.isRunning {
		p.mu.Unlock()
		return false
	}
	p.mu.Unlock()

	p.metrics.TaskSubmitted()
	p.tasks.AddTask(task, priority)

	// Signal task availability
	select {
	case p.taskChan <- struct{}{}:
	default:
	}

	return true
}

// SubmitWithError adds a task that can return an error to the pool with normal priority
func (p *Pool) SubmitWithError(task func() error) bool {
	return p.SubmitWithErrorAndPriority(task, PriorityNormal)
}

// SubmitWithErrorAndPriority adds a task that can return an error with specified priority
func (p *Pool) SubmitWithErrorAndPriority(task func() error, priority int) bool {
	return p.SubmitWithPriority(func() {
		if err := task(); err != nil && p.errorHandler != nil {
			p.errorHandler(err)
		}
	}, priority)
}

// Pause temporarily stops processing new tasks
// Already running tasks will complete
func (p *Pool) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isPaused && p.isRunning {
		p.isPaused = true
		close(p.pauseChan)
		p.pauseChan = make(chan struct{})
	}
}

// Resume continues processing tasks after a pause
func (p *Pool) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isPaused && p.isRunning {
		p.isPaused = false
		close(p.resumeChan)
		p.resumeChan = make(chan struct{})
	}
}

// IsPaused returns whether the pool is currently paused
func (p *Pool) IsPaused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.isPaused
}

// GetMetrics returns the current metrics snapshot
func (p *Pool) GetMetrics() metrics.Snapshot {
	return p.metrics.GetSnapshot()
}

// Shutdown gracefully shuts down the worker pool
func (p *Pool) Shutdown() {
	p.mu.Lock()
	if !p.isRunning {
		p.mu.Unlock()
		return
	}
	p.isRunning = false
	p.mu.Unlock()

	p.cancel()
	p.tasks.Clear()
	p.wg.Wait()
}

// WithContext returns a new Pool that will be cancelled when the provided context is cancelled
func WithContext(ctx context.Context, workers int) *Pool {
	ctx, cancel := context.WithCancel(ctx)
	p := &Pool{
		workers:    workers,
		tasks:      NewPriorityQueue(),
		ctx:        ctx,
		cancel:     cancel,
		isRunning:  true,
		metrics:    metrics.NewPoolMetrics(),
		pauseChan:  make(chan struct{}),
		resumeChan: make(chan struct{}),
		workerChan: make(chan struct{}),
		taskChan:   make(chan struct{}, 1),
	}

	p.start()
	return p
}

// PeriodicTask represents a task that runs periodically
type PeriodicTask struct {
	task     func() error
	interval time.Duration
	priority int
	stopChan chan struct{}
}

// SubmitPeriodic submits a task to be executed periodically with normal priority
func (p *Pool) SubmitPeriodic(task func() error, interval time.Duration) (*PeriodicTask, error) {
	return p.SubmitPeriodicWithPriority(task, interval, PriorityNormal)
}

// SubmitPeriodicWithPriority submits a task to be executed periodically with specified priority
func (p *Pool) SubmitPeriodicWithPriority(task func() error, interval time.Duration, priority int) (*PeriodicTask, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("interval must be positive")
	}

	p.mu.Lock()
	if !p.isRunning {
		p.mu.Unlock()
		return nil, fmt.Errorf("pool is not running")
	}
	p.mu.Unlock()

	pt := &PeriodicTask{
		task:     task,
		interval: interval,
		priority: priority,
		stopChan: make(chan struct{}),
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-pt.stopChan:
				return
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				p.SubmitWithErrorAndPriority(task, priority)
			}
		}
	}()

	return pt, nil
}

// StopPeriodic stops a periodic task
func (p *Pool) StopPeriodic(pt *PeriodicTask) {
	if pt != nil {
		close(pt.stopChan)
	}
}

// WithErrorHandler sets an error handler for the pool
func (p *Pool) WithErrorHandler(handler ErrorHandler) *Pool {
	p.errorHandler = handler
	return p
}

// Result represents the result of task execution
type Result struct {
	Value interface{}
	Err   error
}

// StreamTask represents a task that returns a result
type StreamTask func() (interface{}, error)

// SubmitStream adds a task that will return a result in a channel
func (p *Pool) SubmitStream(task StreamTask) chan Result {
	resultChan := make(chan Result, 1)

	p.Submit(func() {
		value, err := task()
		resultChan <- Result{Value: value, Err: err}
		close(resultChan)
	})

	return resultChan
}

// SubmitStreamWithPriority adds a task with priority, the result of which will be sent to the channel
func (p *Pool) SubmitStreamWithPriority(task StreamTask, priority int) chan Result {
	resultChan := make(chan Result, 1)

	p.SubmitWithPriority(func() {
		value, err := task()
		resultChan <- Result{Value: value, Err: err}
		close(resultChan)
	}, priority)

	return resultChan
}

// BatchStreamTask represents a batch task that sends results to a channel
type BatchStreamTask func(chan<- Result)

// SubmitBatchStream adds a task that can send multiple results
func (p *Pool) SubmitBatchStream(task BatchStreamTask) chan Result {
	resultChan := make(chan Result)

	p.Submit(func() {
		defer close(resultChan)
		task(resultChan)
	})

	return resultChan
}

// SubmitBatchStreamWithPriority adds a task with priority, which can send multiple results
func (p *Pool) SubmitBatchStreamWithPriority(task BatchStreamTask, priority int) chan Result {
	resultChan := make(chan Result)

	p.SubmitWithPriority(func() {
		defer close(resultChan)
		task(resultChan)
	}, priority)

	return resultChan
}
