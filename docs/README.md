# ParallelX - High-Performance Concurrency Library for Go

## Table of Contents
1. [Installation](#installation)
2. [Core Components](#core-components)
   - [Worker Pool](#worker-pool)
   - [Concurrency Patterns](#concurrency-patterns)
   - [Resilience Mechanisms](#resilience-mechanisms)
3. [Usage Examples](#usage-examples)
4. [API Reference](#api-reference)

## Installation

```bash
go get github.com/AlestackOverglow/parallelx
```

## Core Components

### Worker Pool

Worker Pool provides an efficient mechanism for executing tasks concurrently.

#### Creating a Pool

```go
// Create a pool with 5 workers
pool := pool.New(5)
defer pool.Shutdown()

// Create a pool with context
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
pool := pool.WithContext(ctx, 5)
```

#### Task Management

```go
// Submit a simple task
pool.Submit(func() {
    // Your code here
})

// Submit a task with error handling
pool.SubmitWithError(func() error {
    // Your code here
    return nil
})

// Submit a task with priority
pool.SubmitWithPriority(func() {
    // Critical task
}, PriorityCritical)
```

#### Periodic Tasks

```go
// Periodic task
task, err := pool.SubmitPeriodic(func() error {
    // Executes every second
    return nil
}, time.Second)

// Stop periodic task
pool.StopPeriodic(task)
```

#### Pool Management

```go
// Scale the pool
pool.Scale(8)

// Pause and resume
pool.Pause()
pool.Resume()

// Get metrics
metrics := pool.GetMetrics()
fmt.Printf("Tasks completed: %d\n", metrics.TasksCompleted)
```

### Concurrency Patterns

#### Pipeline

```go
// Define stages
stage1 := func(ctx context.Context, in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for x := range in {
            select {
            case out <- x * 2:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

stage2 := func(ctx context.Context, in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for x := range in {
            select {
            case out <- x + 1:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

// Use pipeline
source := make(chan int)
result := patterns.Pipeline(ctx, source, stage1, stage2)
```

#### Fan-Out/Fan-In

```go
// Distribute work
outputs := patterns.FanOut(ctx, source, 3)

// Collect results
results := patterns.FanIn(ctx, outputs...)
```

#### Pub/Sub

```go
// Create pub/sub system
ps := patterns.NewPubSub[string](5)

// Subscribe
sub1 := ps.Subscribe()
sub2 := ps.Subscribe()

// Publish
ps.Publish(ctx, "message")

// Unsubscribe
ps.Unsubscribe(sub1)
```

#### Barrier

```go
// Create barrier
barrier := patterns.NewBarrier(5, func() {
    fmt.Println("All goroutines reached the barrier")
})

// Use barrier
barrier.Wait()
```

#### Semaphore

```go
// Create semaphore
sem := patterns.NewSemaphore(2)

// Acquire permit
if err := sem.Acquire(ctx); err != nil {
    return err
}
defer sem.Release()

// Try non-blocking acquire
if sem.TryAcquire() {
    defer sem.Release()
    // Use resource
}
```

### Resilience Mechanisms

#### Retry

```go
// Configuration
config := resilience.DefaultRetryConfig()
config.MaxAttempts = 3
config.InitialDelay = 100 * time.Millisecond
config.BackoffMultiplier = 2.0

// Usage
result, err := resilience.Retry(ctx, func() (string, error) {
    return callExternalService()
}, config)
```

#### Type-Safe Circuit Breaker

```go
// Create circuit breaker
config := resilience.DefaultCircuitBreakerConfig()
cb := resilience.NewCircuitBreaker[UserData](config)

// Execute operation
result, err := cb.Execute(ctx, func() (UserData, error) {
    return fetchUserData(userID)
})

// With fallback
result, err = cb.ExecuteWithFallback(ctx,
    func() (UserData, error) {
        return fetchUserData(userID)
    },
    func(err error) (UserData, error) {
        return fetchCachedUserData(userID)
    },
)

// With default value fallback
result, err = cb.ExecuteWithDefaultFallback(ctx,
    func() (UserData, error) {
        return fetchUserData(userID)
    },
    UserData{Status: "default"}, // Default value used when circuit is open or error occurs
)
```

#### Graceful Degradation

```go
// Configuration
config := resilience.DefaultDegradationConfig[UserData]()
if config.LoadMetric == nil {
    config.LoadMetric = getCurrentSystemLoad
}
config.Fallbacks = map[resilience.DegradationLevel]UserData{
    resilience.LevelReduced: {Status: "basic"},
    resilience.LevelMinimal: {Status: "minimal"},
}

// Create handler
dh := resilience.NewDegradationHandler(config)

// Execute with possible degradation
result, err := dh.Execute(ctx, func() (UserData, error) {
    return fetchFullUserData(userID)
})

// With explicit fallback
result, err = dh.ExecuteWithFallback(ctx,
    func() (UserData, error) {
        return fetchFullUserData(userID)
    },
    map[resilience.DegradationLevel]UserData{
        resilience.LevelReduced: {Status: "cached"},
        resilience.LevelMinimal: {Status: "minimal"},
    },
)

// With reduced functionality
result, err = dh.WithReducedFunctionality(ctx,
    func() (UserData, error) {
        return fetchFullUserData(userID)
    },
    func() (UserData, error) {
        return fetchBasicUserData(userID)
    },
)
```

## API Reference

### Pool

```go
type Pool struct { ... }

func New(workers int) *Pool
func WithContext(ctx context.Context, workers int) *Pool
func (p *Pool) Submit(task func()) bool
func (p *Pool) SubmitWithError(task func() error) bool
func (p *Pool) SubmitWithPriority(task func(), priority int) bool
func (p *Pool) SubmitPeriodic(task func() error, interval time.Duration) (*PeriodicTask, error)
func (p *Pool) SubmitPeriodicWithPriority(task func() error, interval time.Duration, priority int) (*PeriodicTask, error)
func (p *Pool) StopPeriodic(task *PeriodicTask)
func (p *Pool) SubmitStream(task StreamTask) chan Result
func (p *Pool) SubmitStreamWithPriority(task StreamTask, priority int) chan Result
func (p *Pool) SubmitBatchStream(task BatchStreamTask) chan Result
func (p *Pool) SubmitBatchStreamWithPriority(task BatchStreamTask, priority int) chan Result
func (p *Pool) Scale(workers int) error
func (p *Pool) Pause()
func (p *Pool) Resume()
func (p *Pool) Shutdown()
func (p *Pool) GetMetrics() metrics.Snapshot
```

### Patterns

```go
// Pipeline
type Stage[In, Out any] func(context.Context, <-chan In) <-chan Out
func Pipeline[T any](ctx context.Context, source <-chan T, stages ...Stage[T, T]) <-chan T

// Fan-Out/Fan-In
func FanOut[T any](ctx context.Context, source <-chan T, n int) []<-chan T
func FanIn[T any](ctx context.Context, channels ...<-chan T) <-chan T

// PubSub
type PubSub[T any] struct { ... }
func NewPubSub[T any](buffer int) *PubSub[T]
func (ps *PubSub[T]) Subscribe() <-chan T
func (ps *PubSub[T]) Unsubscribe(ch <-chan T)
func (ps *PubSub[T]) Publish(ctx context.Context, value T)

// Barrier
type Barrier struct { ... }
func NewBarrier(count int, callback func()) *Barrier
func (b *Barrier) Wait()

// Semaphore
type Semaphore struct { ... }
func NewSemaphore(count int) *Semaphore
func (s *Semaphore) Acquire(ctx context.Context) error
func (s *Semaphore) Release()
func (s *Semaphore) TryAcquire() bool
```

### Resilience

```go
// Retry
type RetryConfig struct {
    MaxAttempts       int
    InitialDelay      time.Duration
    MaxDelay          time.Duration
    BackoffMultiplier float64
    RetryableErrors   []error
}
func Retry[T any](ctx context.Context, fn func() (T, error), config RetryConfig) (T, error)

// Circuit Breaker
type CircuitBreaker[T any] struct { ... }
func NewCircuitBreaker[T any](config CircuitBreakerConfig) *CircuitBreaker[T]
func (cb *CircuitBreaker[T]) Execute(ctx context.Context, fn func() (T, error)) (T, error)
func (cb *CircuitBreaker[T]) ExecuteWithFallback(ctx context.Context, fn func() (T, error), fallback func(error) (T, error)) (T, error)

// Degradation
type DegradationHandler[T any] struct { ... }
func NewDegradationHandler[T any](config DegradationConfig[T]) *DegradationHandler[T]
func (dh *DegradationHandler[T]) Execute(ctx context.Context, fn func() (T, error)) (T, error)
func (dh *DegradationHandler[T]) ExecuteWithFallback(ctx context.Context, fn func() (T, error), fallbacks map[DegradationLevel]T) (T, error)
func (dh *DegradationHandler[T]) WithReducedFunctionality(ctx context.Context, normalFn func() (T, error), reducedFn func() (T, error)) (T, error)
```

## Best Practices

1. **Resource Management**
   - Always close pools using `defer pool.Shutdown()`
   - Use `context` for operation lifecycle management
   - Release semaphore resources using `defer sem.Release()`

2. **Error Handling**
   - Use `SubmitWithError` instead of `Submit` for tasks that can return errors
   - Configure error handlers for pools using `WithErrorHandler`
   - Use resilience mechanisms for handling transient failures

3. **Performance**
   - Choose optimal pool size based on task characteristics
   - Use priorities for critical tasks
   - Monitor pool metrics for optimization

4. **Scaling**
   - Scale pools dynamically based on load
   - Use Fan-Out/Fan-In for load distribution
   - Apply Graceful Degradation under high load

## Usage Examples

### Parallel Data Processing

```go
func ProcessData(data []Item) error {
    pool := pool.New(runtime.NumCPU())
    defer pool.Shutdown()

    var wg sync.WaitGroup
    for _, item := range data {
        wg.Add(1)
        item := item // Create local copy
        pool.SubmitWithError(func() error {
            defer wg.Done()
            return processItem(item)
        })
    }

    wg.Wait()
    return nil
}
```

### Periodic Task Execution

```go
func StartMonitoring(ctx context.Context) error {
    pool := pool.WithContext(ctx, 2)
    defer pool.Shutdown()

    // High-priority check every 30 seconds
    _, err := pool.SubmitPeriodicWithPriority(func() error {
        return checkCriticalServices()
    }, 30*time.Second, PriorityCritical)
    if err != nil {
        return err
    }

    // Regular check every 5 minutes
    _, err = pool.SubmitPeriodic(func() error {
        return checkNonCriticalServices()
    }, 5*time.Minute)
    if err != nil {
        return err
    }

    return nil
}
```

### Pipeline Processing

```go
func ProcessStream(ctx context.Context, input <-chan Data) <-chan Result {
    // Stage 1: Validation
    validate := func(ctx context.Context, in <-chan Data) <-chan Data {
        out := make(chan Data)
        go func() {
            defer close(out)
            for data := range in {
                if isValid(data) {
                    select {
                    case out <- data:
                    case <-ctx.Done():
                        return
                    }
                }
            }
        }()
        return out
    }

    // Stage 2: Transformation
    transform := func(ctx context.Context, in <-chan Data) <-chan Result {
        out := make(chan Result)
        go func() {
            defer close(out)
            for data := range in {
                select {
                case out <- transformData(data):
                case <-ctx.Done():
                    return
                }
            }
        }()
        return out
    }

    // Launch pipeline
    return patterns.Pipeline(ctx, input, validate, transform)
}
```

### Resilient Service

```go
type Service struct {
    cb  *resilience.CircuitBreaker[Response]
    dh  *resilience.DegradationHandler[Response]
    sem *patterns.Semaphore
}

func NewService() *Service {
    cbConfig := resilience.DefaultCircuitBreakerConfig()
    dhConfig := resilience.DefaultDegradationConfig[Response]()
    if dhConfig.LoadMetric == nil {
        dhConfig.LoadMetric = getCurrentSystemLoad
    }
    
    return &Service{
        cb:  resilience.NewCircuitBreaker[Response](cbConfig),
        dh:  resilience.NewDegradationHandler(dhConfig),
        sem: patterns.NewSemaphore(100), // Maximum 100 concurrent requests
    }
}

func (s *Service) HandleRequest(ctx context.Context, req Request) (Response, error) {
    // Acquire semaphore permit
    if err := s.sem.Acquire(ctx); err != nil {
        return Response{}, err
    }
    defer s.sem.Release()

    // Execute request with circuit breaker and degradation
    return s.cb.ExecuteWithFallback(ctx,
        func() (Response, error) {
            return s.dh.Execute(ctx, func() (Response, error) {
                return processRequest(req)
            })
        },
        func(err error) (Response, error) {
            return getCachedResponse(req)
        },
    )
} 