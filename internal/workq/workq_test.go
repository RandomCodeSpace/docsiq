package workq

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool_SubmitRunsJob(t *testing.T) {
	t.Parallel()
	p := New(Config{Workers: 2, QueueDepth: 4})
	defer p.Close(context.Background())

	var ran atomic.Int32
	done := make(chan struct{})
	if err := p.Submit(func(ctx context.Context) {
		ran.Add(1)
		close(done)
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("job did not run within 1s")
	}
	if got := ran.Load(); got != 1 {
		t.Fatalf("want ran=1, got %d", got)
	}
}

func TestPool_SubmitReturnsErrQueueFull(t *testing.T) {
	t.Parallel()
	p := New(Config{Workers: 1, QueueDepth: 1})
	defer p.Close(context.Background())

	block := make(chan struct{})
	// Occupy the single worker.
	_ = p.Submit(func(ctx context.Context) { <-block })
	// Fill the single queue slot.
	if err := p.Submit(func(ctx context.Context) {}); err != nil {
		t.Fatalf("queue slot submit: %v", err)
	}
	// Third submit must fail fast.
	err := p.Submit(func(ctx context.Context) {})
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("want ErrQueueFull, got %v", err)
	}
	close(block)
}

func TestPool_CloseDrainsInflight(t *testing.T) {
	t.Parallel()
	p := New(Config{Workers: 2, QueueDepth: 4})
	var ran atomic.Int32
	for i := 0; i < 4; i++ {
		_ = p.Submit(func(ctx context.Context) {
			time.Sleep(20 * time.Millisecond)
			ran.Add(1)
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
	if got := ran.Load(); got != 4 {
		t.Fatalf("want ran=4 after drain, got %d", got)
	}
}

func TestPool_CloseCancelsOnContextDeadline(t *testing.T) {
	t.Parallel()
	p := New(Config{Workers: 1, QueueDepth: 1})
	start := make(chan struct{})
	_ = p.Submit(func(ctx context.Context) {
		close(start)
		<-ctx.Done() // honour cancellation
	})
	<-start
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := p.Close(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded, got %v", err)
	}
}
