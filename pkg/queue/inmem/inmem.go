// Package inmem provides a channel-backed implementation of queue.Queue
// for use in monolith deployments.
package inmem

import (
	"context"
	"errors"
	"sync"

	"github.com/observer-io/observer/pkg/queue"
)

type Q struct {
	ch     chan queue.Job
	closed bool
	mu     sync.Mutex
}

func New(buffer int) *Q {
	return &Q{ch: make(chan queue.Job, buffer)}
}

func (q *Q) Enqueue(ctx context.Context, j queue.Job) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return errors.New("queue closed")
	}
	q.mu.Unlock()
	select {
	case q.ch <- j:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *Q) Consume(ctx context.Context, h queue.Handler) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case j, ok := <-q.ch:
			if !ok {
				return nil
			}
			if err := h(ctx, j); err != nil {
				// v1: log via caller; no retry in inmem path
				_ = err
			}
		}
	}
}

func (q *Q) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil
	}
	q.closed = true
	close(q.ch)
	return nil
}
