package patterns

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

// Stage представляет этап конвейера
type Stage[In, Out any] func(context.Context, <-chan In) <-chan Out

// Pipeline объединяет несколько этапов в конвейер
func Pipeline[T any](ctx context.Context, source <-chan T, stages ...Stage[T, T]) <-chan T {
	current := source
	for _, stage := range stages {
		current = stage(ctx, current)
	}
	return current
}

// FanOut распределяет данные между несколькими горутинами
func FanOut[T any](ctx context.Context, source <-chan T, n int) []<-chan T {
	outputs := make([]<-chan T, n)
	for i := 0; i < n; i++ {
		ch := make(chan T)
		outputs[i] = ch
		go func(out chan<- T) {
			defer close(out)
			for {
				select {
				case <-ctx.Done():
					return
				case val, ok := <-source:
					if !ok {
						return
					}
					select {
					case out <- val:
					case <-ctx.Done():
						return
					}
				}
			}
		}(ch)
	}
	return outputs
}

// FanIn объединяет несколько каналов в один
func FanIn[T any](ctx context.Context, channels ...<-chan T) <-chan T {
	var wg sync.WaitGroup
	multiplexed := make(chan T)

	multiplex := func(c <-chan T) {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case val, ok := <-c:
				if !ok {
					return
				}
				select {
				case multiplexed <- val:
				case <-ctx.Done():
					return
				}
			}
		}
	}

	wg.Add(len(channels))
	for _, c := range channels {
		go multiplex(c)
	}

	go func() {
		wg.Wait()
		close(multiplexed)
	}()

	return multiplexed
}

// PubSub представляет систему публикации/подписки
type PubSub[T any] struct {
	mu          sync.RWMutex
	subscribers map[chan T]struct{}
	buffer      int
}

// NewPubSub создает новый экземпляр PubSub
func NewPubSub[T any](buffer int) *PubSub[T] {
	return &PubSub[T]{
		subscribers: make(map[chan T]struct{}),
		buffer:      buffer,
	}
}

// Subscribe создает новую подписку
func (ps *PubSub[T]) Subscribe() <-chan T {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ch := make(chan T, ps.buffer)
	ps.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe отменяет подписку
func (ps *PubSub[T]) Unsubscribe(ch <-chan T) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for sub := range ps.subscribers {
		if sub == ch {
			delete(ps.subscribers, sub)
			close(sub)
			return
		}
	}
}

// Publish публикует значение всем подписчикам
func (ps *PubSub[T]) Publish(ctx context.Context, value T) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	for sub := range ps.subscribers {
		select {
		case <-ctx.Done():
			return
		case sub <- value:
		default:
			// Пропускаем медленных подписчиков
		}
	}
}

// Barrier представляет барьер синхронизации
type Barrier struct {
	count    int
	current  int
	mutex    sync.Mutex
	cond     *sync.Cond
	callback func()
}

// NewBarrier создает новый барьер
func NewBarrier(count int, callback func()) *Barrier {
	b := &Barrier{
		count:    count,
		callback: callback,
	}
	b.cond = sync.NewCond(&b.mutex)
	return b
}

// Wait блокирует горутину до достижения барьера
func (b *Barrier) Wait() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.current++
	if b.current == b.count {
		if b.callback != nil {
			b.callback()
		}
		b.current = 0
		b.cond.Broadcast()
	} else {
		b.cond.Wait()
	}
}

// Semaphore представляет семафор
type Semaphore struct {
	permits chan struct{}
}

// NewSemaphore создает новый семафор
func NewSemaphore(count int) *Semaphore {
	return &Semaphore{
		permits: make(chan struct{}, count),
	}
}

// Acquire получает разрешение
func (s *Semaphore) Acquire(ctx context.Context) error {
	select {
	case s.permits <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release освобождает разрешение
func (s *Semaphore) Release() {
	select {
	case <-s.permits:
	default:
		// Игнорируем лишние освобождения
	}
}

// TryAcquire пытается получить разрешение без блокировки
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.permits <- struct{}{}:
		return true
	default:
		return false
	}
}

// ParallelExecute executes functions in parallel and collects errors
func ParallelExecute(ctx context.Context, fns ...func() error) error {
	g, ctx := errgroup.WithContext(ctx)
	for _, fn := range fns {
		fn := fn // capture loop variable
		g.Go(func() error {
			return fn()
		})
	}
	return g.Wait()
}
