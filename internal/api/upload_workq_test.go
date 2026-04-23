package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/workq"
)

// TestUpload_ReturnsRetryOnFullQueue verifies that when the injected
// workq Pool is saturated, the upload-submission layer returns 503 with
// a Retry-After header and does not run the job. We exercise the HTTP
// bridge via the Pool contract without setting up the full upload
// handler (that's covered by other integration tests).
func TestUpload_ReturnsRetryOnFullQueue(t *testing.T) {
	t.Parallel()
	pool := workq.New(workq.Config{Workers: 1, QueueDepth: 1})
	// Saturate the pool: one worker busy, one queue slot full.
	block := make(chan struct{})
	started := make(chan struct{})
	// LIFO: close(block) runs first (unblocks the stuck worker), then
	// pool.Close drains cleanly. This order is safe even on panic.
	defer pool.Close(context.Background()) //nolint:errcheck
	defer close(block)
	// Signal via started that the worker has actually begun executing
	// (and is now blocked on <-block) before we fill the queue slot.
	// Without this synchronisation the race detector's slower scheduling
	// can drain the first job from the channel before Submit #2 lands,
	// leaving a free slot for the test's submit and producing a false 202.
	_ = pool.Submit(func(ctx context.Context) { close(started); <-block })
	<-started // worker is blocked on <-block; channel now empty
	// Channel capacity = Workers+QueueDepth = 1+1 = 2.
	// Fill both slots so the next submit returns ErrQueueFull.
	_ = pool.Submit(func(ctx context.Context) {})
	_ = pool.Submit(func(ctx context.Context) {})

	var called atomic.Bool
	// Mimic what h.workq.Submit does in upload(): on ErrQueueFull, set
	// Retry-After and write 503 via writeError equivalent.
	handle := func(w http.ResponseWriter, _ *http.Request) {
		err := pool.Submit(func(ctx context.Context) {
			called.Store(true)
		})
		if err == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if errors.Is(err, workq.ErrQueueFull) {
			w.Header().Set("Retry-After", "30")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"indexing queue full; retry later"}`))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/upload", nil)
	rr := httptest.NewRecorder()
	handle(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") != "30" {
		t.Fatalf("missing or wrong Retry-After: %q", rr.Header().Get("Retry-After"))
	}
	if got := rr.Body.String(); !strings.Contains(got, "queue full") {
		t.Fatalf("body should mention queue full; got %s", got)
	}
	if called.Load() {
		t.Fatal("job should not have run when queue is full")
	}
}
