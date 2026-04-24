package workq

import (
	"context"
	"errors"
	"sync"
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
	for range 4 {
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

func TestPool_SubmitRaceDuringClose(t *testing.T) {
	t.Parallel()
	for range 50 {
		p := New(Config{Workers: 4, QueueDepth: 8})
		var wg sync.WaitGroup
		for range 32 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = p.Submit(func(ctx context.Context) {})
			}()
		}
		_ = p.Close(context.Background())
		wg.Wait()
	}
}

func TestPool_StatsReportsDepthAndRejected(t *testing.T) {
	t.Parallel()
	// The underlying jobs channel has capacity = Workers + QueueDepth.
	// With 1 worker + 2 queue slots the buffer is 3 wide. The worker
	// pulls the first job off (blocks on <-block) freeing a slot, so
	// three more blocking submits fit before the channel is saturated.
	p := New(Config{Workers: 1, QueueDepth: 2})
	block := make(chan struct{})
	defer func() {
		select {
		case <-block:
		default:
			close(block)
		}
		_ = p.Close(context.Background())
	}()

	started := make(chan struct{})
	if err := p.Submit(func(ctx context.Context) { close(started); <-block }); err != nil {
		t.Fatalf("submit 1: %v", err)
	}
	<-started
	// Fill the channel buffer with blocking jobs (3 slots available
	// once the worker has pulled submit 1 off).
	for i := 2; i <= 4; i++ {
		if err := p.Submit(func(ctx context.Context) { <-block }); err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
	}

	// Next two submissions must be rejected; Rejected must grow by 2.
	if err := p.Submit(func(ctx context.Context) {}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("submit 5: want ErrQueueFull, got %v", err)
	}
	if err := p.Submit(func(ctx context.Context) {}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("submit 6: want ErrQueueFull, got %v", err)
	}

	stats := p.Stats()
	if stats.Depth != 3 {
		t.Errorf("Depth=%d want 3", stats.Depth)
	}
	if stats.Rejected != 2 {
		t.Errorf("Rejected=%d want 2", stats.Rejected)
	}
}
