// Package workq is a minimal bounded worker pool for fire-and-forget
// background work (e.g. post-upload indexing). Jobs carry a context
// derived from the pool's root context; Close() cancels that context
// and waits for workers to drain, honouring the caller's deadline.
package workq

import (
	"context"
	"errors"
	"sync"
)

// ErrQueueFull is returned by Submit when the job queue is saturated.
// Callers should surface this as 503 Service Unavailable with Retry-After.
var ErrQueueFull = errors.New("workq: queue full")

// ErrClosed is returned by Submit after Close has been called.
var ErrClosed = errors.New("workq: closed")

// Job is a unit of work. It receives the pool's context so it can
// abort on shutdown.
type Job func(ctx context.Context)

// Config sizes the pool. Zero values use safe defaults (1 worker,
// 16-deep queue). Total in-flight + queued capacity is Workers + QueueDepth.
type Config struct {
	Workers    int
	QueueDepth int
}

// Pool is a fixed-size worker pool with a bounded submission queue.
type Pool struct {
	jobs   chan Job
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	closeOnce sync.Once
	closed    chan struct{}
}

// New constructs and starts a Pool.
func New(cfg Config) *Pool {
	if cfg.Workers < 1 {
		cfg.Workers = 1
	}
	if cfg.QueueDepth < 1 {
		cfg.QueueDepth = 16
	}
	ctx, cancel := context.WithCancel(context.Background())
	// Total buffered capacity = Workers + QueueDepth, so Submit succeeds
	// whenever at least one worker is idle OR there is a free queue slot.
	p := &Pool{
		jobs:   make(chan Job, cfg.Workers+cfg.QueueDepth),
		ctx:    ctx,
		cancel: cancel,
		closed: make(chan struct{}),
	}
	for i := 0; i < cfg.Workers; i++ {
		p.wg.Add(1)
		go p.run()
	}
	return p
}

// Submit enqueues job. Non-blocking: returns ErrQueueFull immediately
// if no queue slot is available, ErrClosed if the pool is shutting down.
func (p *Pool) Submit(job Job) error {
	select {
	case <-p.closed:
		return ErrClosed
	default:
	}
	select {
	case p.jobs <- job:
		return nil
	default:
		return ErrQueueFull
	}
}

// Close stops accepting new work and waits for workers to drain. If
// the caller's ctx fires before drain completes, the pool context is
// cancelled so in-flight jobs honouring cancellation can abort, and
// ctx.Err() is returned.
func (p *Pool) Close(ctx context.Context) error {
	p.closeOnce.Do(func() {
		close(p.closed)
		close(p.jobs)
	})
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		p.cancel()
		return ctx.Err()
	}
}

func (p *Pool) run() {
	defer p.wg.Done()
	for job := range p.jobs {
		// Trap panics per-job so one bad job cannot kill a worker.
		func() {
			defer func() {
				_ = recover()
			}()
			job(p.ctx)
		}()
	}
}
