package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// healthPingerStub is a test double for whatever interface the ready
// probe accepts for "ping this SQLite handle". See health.go.
type healthPingerStub struct {
	err  error
	hits atomic.Int32
}

func (p *healthPingerStub) Ping(ctx context.Context) error {
	p.hits.Add(1)
	return p.err
}

// llmPingerStub is a test double for the LLM reachability probe.
type llmPingerStub struct {
	err  error
	hits atomic.Int32
}

func (p *llmPingerStub) Ping(ctx context.Context) error {
	p.hits.Add(1)
	return p.err
}

func TestHealthz_Always200(t *testing.T) {
	t.Parallel()
	h := healthzHandler()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status=%q want ok", body["status"])
	}
}

func TestReadyz_AllChecksOKReturns200(t *testing.T) {
	t.Parallel()
	sq := &healthPingerStub{}
	llm := &llmPingerStub{}
	h := readyzHandler(sq, llm)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}
	var body readyzBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body.Status != "ready" {
		t.Errorf("status=%q want ready", body.Status)
	}
	if body.Checks["sqlite"].Status != "ok" {
		t.Errorf("sqlite=%+v", body.Checks["sqlite"])
	}
	if body.Checks["llm"].Status != "ok" {
		t.Errorf("llm=%+v", body.Checks["llm"])
	}
}

func TestReadyz_SQLiteDownReturns503(t *testing.T) {
	t.Parallel()
	sq := &healthPingerStub{err: errors.New("database is locked")}
	llm := &llmPingerStub{}
	h := readyzHandler(sq, llm)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503", rec.Code)
	}
	var body readyzBody
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Status != "not_ready" {
		t.Errorf("status=%q want not_ready", body.Status)
	}
	if body.Checks["sqlite"].Status != "error" {
		t.Errorf("sqlite status=%q want error", body.Checks["sqlite"].Status)
	}
	if body.Checks["sqlite"].Err == "" {
		t.Errorf("sqlite err empty; should carry 'database is locked'")
	}
}

func TestReadyz_NilLLMReportsSkippedAndStaysReady(t *testing.T) {
	t.Parallel()
	sq := &healthPingerStub{}
	h := readyzHandler(sq, nil) // nil llm == provider:none

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	var body readyzBody
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Checks["llm"].Status != "skipped" {
		t.Errorf("llm=%+v want skipped", body.Checks["llm"])
	}
}

func TestReadyz_CachesResultFor10s(t *testing.T) {
	t.Parallel()
	sq := &healthPingerStub{}
	llm := &llmPingerStub{}
	h := readyzHandler(sq, llm)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
	}

	// Each of the 5 requests must have hit the pingers at most once.
	if got := sq.hits.Load(); got > 1 {
		t.Errorf("sqlite pinger called %d times in a single TTL window; want <=1", got)
	}
	if got := llm.hits.Load(); got > 1 {
		t.Errorf("llm pinger called %d times in a single TTL window; want <=1", got)
	}
}

func TestReadyz_PingerContextIsBounded(t *testing.T) {
	t.Parallel()
	var seenDeadline atomic.Bool
	llm := &llmPingerStub{}

	// Wrap the sq probe so it reports whether the caller bounded the context.
	wrapped := healthPingerFunc(func(ctx context.Context) error {
		if _, ok := ctx.Deadline(); ok {
			seenDeadline.Store(true)
		}
		return nil
	})
	h := readyzHandler(wrapped, llm)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !seenDeadline.Load() {
		t.Errorf("readyzHandler must bound the pinger context with a deadline")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status=%d", rec.Code)
	}
}

// healthPingerFunc is an adapter so the test above can use an inline
// closure without hand-rolling another stub type.
type healthPingerFunc func(ctx context.Context) error

func (f healthPingerFunc) Ping(ctx context.Context) error { return f(ctx) }

// TestReadyz_ProbeCtxDecoupledFromRequestCtx: if the probing client
// cancels the request (disconnect or its own deadline), the probe must
// still complete successfully so the cached result is not poisoned
// with context.Canceled for the whole TTL window.
func TestReadyz_ProbeCtxDecoupledFromRequestCtx(t *testing.T) {
	t.Parallel()

	var seenCancel atomic.Bool
	// Pinger: check whether the ctx passed in has already been canceled
	// by the caller. If the probe ctx was derived from the request ctx,
	// it would inherit cancellation and this flag would flip.
	sq := healthPingerFunc(func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			seenCancel.Store(true)
			return err
		}
		return nil
	})
	llm := &llmPingerStub{}
	h := readyzHandler(sq, llm)

	// Request ctx that is already canceled.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seenCancel.Load() {
		t.Errorf("probe saw caller's canceled ctx — readiness cache would be poisoned by client disconnects")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status=%d; want 200 (probe succeeded despite canceled request ctx)", rec.Code)
	}
}

// Guardrail: test clock advance simulation ensures cached result refreshes.
func TestReadyz_RefreshesAfterTTL(t *testing.T) {
	t.Parallel()
	sq := &healthPingerStub{}
	llm := &llmPingerStub{}
	h := readyzHandlerForTest(sq, llm, 50*time.Millisecond)

	req := func() {
		r := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
	}

	req()
	req()
	time.Sleep(120 * time.Millisecond)
	req()

	if got := sq.hits.Load(); got != 2 {
		t.Errorf("sqlite pinger hits=%d want 2 (one per TTL window)", got)
	}
}
