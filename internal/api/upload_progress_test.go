package api

import (
	"context"
	"encoding/json"
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

	// Seed two jobs in progress via the structured event log (the new
	// canonical store) AND the legacy plain-string map (kept for
	// progressForJob fall-back).
	h.setProgress("job-A", "indexing: a.md")
	h.setProgress("job-B", "indexing: b.md")
	h.appendEvent("job-A", uploadEvent{JobID: "job-A", Phase: "indexing", File: "a.md", Message: "indexing: a.md"})
	h.appendEvent("job-B", uploadEvent{JobID: "job-B", Phase: "indexing", File: "b.md", Message: "indexing: b.md"})

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
	time.Sleep(200 * time.Millisecond)
	h.finishEvent("job-B", uploadEvent{JobID: "job-B", Phase: "done", Message: "done"})

	// Wait long enough for any spurious wake; A should still be streaming.
	time.Sleep(200 * time.Millisecond)
	select {
	case <-streamDone(wgA):
		t.Fatalf("stream A terminated when job B completed; SSE filtering broken. body=%q",
			recA.Body.String())
	default:
		// expected: still streaming
	}

	// Now complete job A — stream should terminate.
	h.finishEvent("job-A", uploadEvent{JobID: "job-A", Phase: "done", Message: "done"})
	select {
	case <-streamDone(wgA):
		// ok
	case <-time.After(2 * time.Second):
		cancelA()
		t.Fatalf("stream A did not terminate after job A done")
	}

	// Verify job A was pruned from the structured event map.
	h.uploadMu.Lock()
	_, present := h.jobEvents["job-A"]
	h.uploadMu.Unlock()
	if present {
		t.Fatalf("job A not cleared from event map after done")
	}

	// Job A's body should contain its own events but never job B's.
	body := recA.Body.String()
	if !strings.Contains(body, "a.md") {
		t.Errorf("stream A missed its own event; body=%q", body)
	}
	if strings.Contains(body, "b.md") {
		t.Errorf("stream A leaked job B's event; body=%q", body)
	}

	// Each data: line must be a valid JSON event with job_id == "job-A".
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var evt uploadEvent
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &evt); err != nil {
			t.Errorf("malformed JSON event %q: %v", line, err)
			continue
		}
		if evt.JobID != "job-A" {
			t.Errorf("event leaked from job %q on stream A: %+v", evt.JobID, evt)
		}
	}
}

// TestUploadProgress_StructuredEventFormat asserts the SSE wire format
// the UI hook depends on: every emission is a `data: {json}\n\n` frame
// whose JSON parses into uploadEvent and surfaces phase/file/chunk
// counters from the pipeline ProgressEvent.
func TestUploadProgress_StructuredEventFormat(t *testing.T) {
	h := &handlers{}

	// Pre-seed a small phase sequence mirroring what indexFile emits.
	h.appendEvent("job-X", uploadEvent{JobID: "job-X", Phase: "queued", Message: "queued: 1 files"})
	h.appendEvent("job-X", uploadEvent{JobID: "job-X", Phase: "chunk", File: "doc.md", ChunksTotal: 5, Message: "split into 5 chunks"})
	h.appendEvent("job-X", uploadEvent{JobID: "job-X", Phase: "embed", File: "doc.md", ChunksDone: 5, ChunksTotal: 5, Message: "embedded 5/5 chunks"})
	h.appendEvent("job-X", uploadEvent{JobID: "job-X", Phase: "extract_entities", File: "doc.md", Message: "extracting entities and relationships"})
	h.appendEvent("job-X", uploadEvent{JobID: "job-X", Phase: "extract_claims", File: "doc.md", Message: "claims extracted"})
	h.appendEvent("job-X", uploadEvent{JobID: "job-X", Phase: "structure", File: "doc.md", Message: "structure summary complete"})
	h.finishEvent("job-X", uploadEvent{JobID: "job-X", Phase: "done", Message: "indexed 1 files"})

	req := httptest.NewRequest(http.MethodGet, "/api/upload/progress?job_id=job-X", nil)
	rec := httptest.NewRecorder()
	h.uploadProgress(rec, req)

	body := rec.Body.String()
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q; want text/event-stream", ct)
	}

	var phases []string
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var evt uploadEvent
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &evt); err != nil {
			t.Fatalf("malformed JSON event %q: %v", line, err)
		}
		phases = append(phases, evt.Phase)
		if evt.JobID != "job-X" {
			t.Errorf("event %q has wrong job_id %q", evt.Phase, evt.JobID)
		}
		if evt.Phase == "embed" {
			if evt.ChunksDone != 5 || evt.ChunksTotal != 5 {
				t.Errorf("embed phase chunk counts = %d/%d; want 5/5", evt.ChunksDone, evt.ChunksTotal)
			}
		}
	}

	// At least 6 distinct phase events plus the terminal one.
	wantPhases := []string{"queued", "chunk", "embed", "extract_entities", "extract_claims", "structure", "done"}
	for _, want := range wantPhases {
		found := false
		for _, p := range phases {
			if p == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing phase %q in stream; saw %v", want, phases)
		}
	}

	// Terminal event must set done:true.
	lastEvent := uploadEvent{}
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var evt uploadEvent
		_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &evt)
		lastEvent = evt
	}
	if !lastEvent.Done {
		t.Errorf("terminal event missing done:true: %+v", lastEvent)
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
