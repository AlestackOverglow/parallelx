package patterns

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPipeline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Создаем источник данных
	source := make(chan int)
	go func() {
		defer close(source)
		for i := 1; i <= 5; i++ {
			source <- i
		}
	}()

	// Создаем этапы конвейера
	double := func(ctx context.Context, in <-chan int) <-chan int {
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

	addOne := func(ctx context.Context, in <-chan int) <-chan int {
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

	// Запускаем конвейер
	output := Pipeline(ctx, source, double, addOne)

	// Проверяем результаты
	expected := []int{3, 5, 7, 9, 11}
	for i, want := range expected {
		if got := <-output; got != want {
			t.Errorf("Pipeline output %d: got %d, want %d", i, got, want)
		}
	}
}

func TestFanOutFanIn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Создаем источник данных
	source := make(chan int)
	go func() {
		defer close(source)
		for i := 1; i <= 10; i++ {
			source <- i
		}
	}()

	// Распределяем работу между 3 горутинами
	outputs := FanOut(ctx, source, 3)

	// Объединяем результаты
	results := FanIn(ctx, outputs...)

	// Проверяем, что получили все числа
	received := make(map[int]bool)
	for i := 0; i < 10; i++ {
		val := <-results
		received[val] = true
	}

	// Проверяем, что все числа получены
	for i := 1; i <= 10; i++ {
		if !received[i] {
			t.Errorf("Missing value %d in results", i)
		}
	}
}

func TestPubSub(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ps := NewPubSub[string](5)

	// Создаем подписчиков
	sub1 := ps.Subscribe()
	sub2 := ps.Subscribe()

	// Публикуем сообщения
	messages := []string{"hello", "world", "test"}
	for _, msg := range messages {
		ps.Publish(ctx, msg)
	}

	// Проверяем, что оба подписчика получили все сообщения
	for _, want := range messages {
		if got := <-sub1; got != want {
			t.Errorf("Subscriber 1 got %q, want %q", got, want)
		}
		if got := <-sub2; got != want {
			t.Errorf("Subscriber 2 got %q, want %q", got, want)
		}
	}

	// Отписываем первого подписчика
	ps.Unsubscribe(sub1)

	// Публикуем еще одно сообщение
	ps.Publish(ctx, "final")

	// Проверяем, что только второй подписчик получил сообщение
	select {
	case msg := <-sub2:
		if msg != "final" {
			t.Errorf("Subscriber 2 got %q, want 'final'", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for message")
	}
}

func TestBarrier(t *testing.T) {
	const numGoroutines = 5
	var reached int32

	// Создаем барьер
	barrier := NewBarrier(numGoroutines, func() {
		atomic.AddInt32(&reached, 1)
	})

	// Запускаем горутины
	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(50 * time.Millisecond)
			barrier.Wait()
		}()
	}

	wg.Wait()

	// Проверяем, что барьер был достигнут один раз
	if atomic.LoadInt32(&reached) != 1 {
		t.Errorf("Barrier callback executed %d times, want 1", reached)
	}
}

func TestSemaphore(t *testing.T) {
	ctx := context.Background()
	sem := NewSemaphore(2)

	// Тестируем успешное получение разрешений
	if err := sem.Acquire(ctx); err != nil {
		t.Errorf("Failed to acquire first permit: %v", err)
	}
	if err := sem.Acquire(ctx); err != nil {
		t.Errorf("Failed to acquire second permit: %v", err)
	}

	// Тестируем TryAcquire
	if sem.TryAcquire() {
		t.Error("TryAcquire succeeded when semaphore was full")
	}

	// Освобождаем разрешение
	sem.Release()

	// Теперь должны смочь получить разрешение
	if !sem.TryAcquire() {
		t.Error("TryAcquire failed after Release")
	}

	// Тестируем отмену контекста
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Пытаемся получить разрешение, когда семафор полон
	err := sem.Acquire(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}
}
