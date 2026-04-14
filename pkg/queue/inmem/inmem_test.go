package inmem

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/queue"
)

func TestEnqueueAndConsume(t *testing.T) {
	q := New(16)
	defer q.Close()

	var (
		mu   sync.Mutex
		seen []uuid.UUID
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = q.Consume(ctx, func(_ context.Context, j queue.Job) error {
			mu.Lock()
			seen = append(seen, j.ID)
			mu.Unlock()
			return nil
		})
	}()

	want := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for _, id := range want {
		if err := q.Enqueue(context.Background(), queue.Job{ID: id}); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := len(seen)
		mu.Unlock()
		if got == len(want) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != len(want) {
		t.Fatalf("got %d jobs, want %d", len(seen), len(want))
	}
}

func TestEnqueueOnClosedReturnsError(t *testing.T) {
	q := New(1)
	_ = q.Close()
	if err := q.Enqueue(context.Background(), queue.Job{ID: uuid.New()}); err == nil {
		t.Fatal("expected error enqueueing on closed queue")
	}
}
