# ParallelX

High-performance concurrency library for Go that provides robust abstractions for parallel task execution, worker pools, and resilience patterns.

## Features

- **Worker Pool** with priority queues and dynamic scaling
- **Concurrency Patterns**:
  - Pipeline
  - Fan-Out/Fan-In
  - Pub/Sub
  - Barrier
  - Semaphore
- **Resilience Mechanisms**:
  - Type-safe Circuit Breaker
  - Graceful Degradation
  - Retry with backoff
- **Metrics and Monitoring**
- **Generic Type Support**
- **Context-Aware Operations**

## Installation

```bash
go get github.com/AlestackOverglow/parallelx
```

## Documentation

For detailed documentation and examples, see [docs/README.md](docs/README.md)

## Quick Start

```go
// Create a worker pool
pool := pool.New(runtime.NumCPU())
defer pool.Shutdown()

// Submit tasks with priorities
pool.SubmitWithPriority(func() {
    // Critical task
}, PriorityCritical)

// Use circuit breaker
cb := resilience.NewCircuitBreaker[Response](config)
result, err := cb.Execute(ctx, func() (Response, error) {
    return callService()
})
```

## TODO
- Add a task queue size limit to Worker Pool
- Implement rate limiting
- Add timeouts for operations
- Write benchmarks
- Improve handling of slow subscribers in PubSub
- Add a crash recovery mechanism
- Expand documentation with godoc-examples
- Add performance metrics
- Implement a deferred task mechanism
- Add support for context-sensitive timeouts
  
## License

MIT License - see [LICENSE](LICENSE) for details 
