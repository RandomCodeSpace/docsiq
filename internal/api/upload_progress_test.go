package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestUploadProgress_JobIDFiltering is a regression test for P1-1.
// Two concurrent jobs with different IDs must have independent SSE
// streams. Completing job B must not terminate job A's stream, and
// job A must only see events whose key matches its ID.
func TestUploadProgress_JobIDFiltering(t *testing.T) {
	h := &handlers{}

	// Seed two jobs in progress.
	h.setProgress("job-A", "indexing: a.md")
	h.setProgress("job-B", "indexing: b.md")

	// Verify progressForJob filters correctly.
	if m, _ := h.progressForJob("job-A"); m != "indexing: a.md" {
		t.Fatalf("progressForJob(job-A) = %q; want %q", m, "indexing: a.md")
	}
	if m, _ := h.progressForJob("job-B"); m != "indexing: b.md" {
		t.Fatalf("progressForJob(job-B) = %q; want %q", m, "indexing: b.md")
	}
	if _, ok := h.progressForJob("ghost"); ok {
		t.Fatalf("progressForJob(ghost) returned ok=true; want false")
	}

	// Launch handler for job A in a goroutine. We expect it to stream
	// events until we complete job A (NOT when we complete job B).
	startStream := func(jobID string) (*httptest.ResponseRecorder, context.CancelFunc, *sync.WaitGroup) {
		req := httptest.NewRequest(http.MethodGet,
			"/api/upload/progress?job_id="+jobID, nil)
		ctx, cancel := context.WithCancel(req.Context())
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.uploadProgress(rec, req)
		}()
		return rec, cancel, &wg
	}

	recA, cancelA, wgA := startStream("job-A")
	defer cancelA()

	// Complete job B; recA must not terminate.
	time.Sleep(700 * time.Millisecond) // one tick at 500 ms
	h.setProgress("job-B", "done")

	// Wait another tick; A should still be streaming (the goroutine
	// should not have finished yet).
	time.Sleep(700 * time.Millisecond)
	select {
	case <-streamDone(wgA):
		t.Fatalf("stream A terminated when job B completed; SSE filtering broken. body=%q",
			recA.Body.String())
	default:
		// expected: still streaming
	}

	// Now complete job A — stream should terminate.
	h.setProgress("job-A", "done")
	select {
	case <-streamDone(wgA):
		// ok
	case <-time.After(2 * time.Second):
		cancelA()
		t.Fatalf("stream A did not terminate after job A done")
	}

	// Verify job A was pruned from the map.
	if _, ok := h.progressForJob("job-A"); ok {
		t.Fatalf("job A not cleared from progress map after done")
	}

	// Job A's body should contain its own events but never "indexing: b.md".
	body := recA.Body.String()
	if !strings.Contains(body, "indexing: a.md") {
		t.Errorf("stream A missed its own event; body=%q", body)
	}
	if strings.Contains(body, "indexing: b.md") {
		t.Errorf("stream A leaked job B's event; body=%q", body)
	}
}

// streamDone returns a chan that closes when the handler goroutine
// returns. Small helper to use with select+timeout.
func streamDone(wg *sync.WaitGroup) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		wg.Wait()
		close(ch)
	}()
	return ch
}
