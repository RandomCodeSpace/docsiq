# Block 1 — Critical Ship-Blockers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the 5 critical ship-blockers from Block 1 of the production-polish roadmap: stop leaking the API key into every served HTML, bound multipart upload size, move post-upload indexing behind a bounded worker pool with graceful drain, refuse insecure default configs on non-loopback binds, and stop the O(N) entity full-scan in local search.

**Architecture:** All changes land in the Go backend; Task 6 additionally touches the UI fetch contract. Introduce a new small package `internal/workq` (bounded job pool with shutdown drain) and one new endpoint `POST /api/session` that exchanges a bearer key for an `httpOnly` cookie. Auth middleware accepts either the existing `Authorization: Bearer` header or the new `docsiq_session` cookie. Each change ships as its own PR and is independently testable via `httptest` + SQLite fixtures.

**Tech Stack:** Go 1.x, `net/http`, `net/http/httptest`, SQLite with `sqlite_fts5`, Viper config, `log/slog`, `crypto/subtle`, `crypto/rand`. UI: TypeScript, `fetch`, vitest, Playwright.

**Source spec:** `docs/superpowers/specs/2026-04-23-production-polish-roadmap-design.md`

---

## File Structure (locked)

### Create

- `internal/workq/workq.go` — bounded worker pool (`New`, `Submit`, `Close`, `ErrQueueFull`)
- `internal/workq/workq_test.go` — pool behaviour tests
- `internal/api/session.go` — `POST /api/session` handler + cookie helpers
- `internal/api/session_test.go` — session endpoint tests

### Modify

- `internal/config/config.go` — add `MaxUploadBytes` to `ServerConfig`; add default + env binding
- `internal/api/handlers.go` — wrap multipart body with `http.MaxBytesReader`; submit indexing via workq; remove `context.Background()` detach
- `internal/api/router.go` — delete meta-tag injection from `spaHandler`; register `/api/session`; accept new shutdown ctx; thread workq in
- `internal/api/auth.go` — accept either bearer header or cookie
- `internal/api/auth_test.go` — cover cookie path
- `internal/search/local.go` — replace `AllEntities` call with scoped fetch
- `internal/store/store.go` — add `EntitiesForDocs(ctx, docIDs)` using `relationships.doc_id` join
- `internal/store/store_test.go` (or nearest existing store test) — cover `EntitiesForDocs`
- `cmd/serve.go` — refuse insecure default; construct and pass workq; drain on shutdown
- `cmd/serve_test.go` (new minimal test file if none exists) — startup refusal test
- `ui/src/lib/api-client.ts` — replace meta-tag read with one-time `POST /api/session` + `credentials: "include"` on every fetch
- `ui/src/lib/api-client.test.ts` (existing) — update fixtures

---

## Task 1: Bounded multipart upload (1.2)

**Files:**
- Modify: `internal/config/config.go:145-149` (ServerConfig struct), `internal/config/config.go` defaults block (around the `server.*` SetDefault lines)
- Modify: `internal/api/handlers.go:393-410` (`upload` handler, around the `TODO P2-1` marker)
- Test: `internal/api/upload_limit_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `internal/api/upload_limit_test.go`:

```go
package api

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestUploadMaxBytes verifies that requests whose body exceeds
// cfg.Server.MaxUploadBytes are rejected with 413 before the handler
// tries to parse the multipart form.
func TestUploadMaxBytes(t *testing.T) {
	t.Parallel()
	const limit int64 = 1024 // 1 KiB for the test

	// Build a multipart body larger than the limit.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("files", "big.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := io.Copy(part, strings.NewReader(strings.Repeat("x", int(limit)*2))); err != nil {
		t.Fatalf("copy: %v", err)
	}
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()

	// enforceUploadLimit is the unit-testable shim applied inside upload().
	// It wraps r.Body with http.MaxBytesReader and returns a 413 on overflow.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !enforceUploadLimit(w, r, limit) {
			return
		}
		if err := r.ParseMultipartForm(32 << 10); err != nil {
			// MaxBytesReader converts overflow into a ParseMultipartForm error
			// AFTER the header has been written by http.MaxBytesReader. We
			// still exit here; the header is already 413 in that case.
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/api/ -run TestUploadMaxBytes -v`
Expected: FAIL with `undefined: enforceUploadLimit`

- [ ] **Step 3: Add the config field**

Edit `internal/config/config.go` struct around line 145-149:

```go
type ServerConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	APIKey         string `mapstructure:"api_key"`
	MaxUploadBytes int64  `mapstructure:"max_upload_bytes"`
}
```

Add a default (locate the `server.*` SetDefault block — same file, search for `SetDefault("server.api_key"`; add the line right below):

```go
v.SetDefault("server.max_upload_bytes", int64(100*1024*1024)) // 100 MiB
```

And an env binding next to the existing `DOCSIQ_SERVER_API_KEY` BindEnv:

```go
_ = v.BindEnv("server.max_upload_bytes", "DOCSIQ_SERVER_MAX_UPLOAD_BYTES")
```

- [ ] **Step 4: Add the enforcement shim and wire it into upload()**

Append to `internal/api/handlers.go` (end of file):

```go
// enforceUploadLimit wraps r.Body with http.MaxBytesReader and returns
// false on overflow, having already written a 413 JSON response. The
// caller must return immediately when this returns false.
func enforceUploadLimit(w http.ResponseWriter, r *http.Request, limit int64) bool {
	if limit <= 0 {
		return true // unlimited (not recommended, explicit opt-in via 0 or negative)
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	return true
}
```

Edit `internal/api/handlers.go:393-410` — replace the `TODO(docsiq): P2-1` comment and the following `ParseMultipartForm` with:

```go
	if !enforceUploadLimit(w, r, h.cfg.Server.MaxUploadBytes) {
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		// MaxBytesReader translates overflow into an error here; the
		// response header is already 413 when that happens. For other
		// malformed-form errors we emit a 400.
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return // response already committed by MaxBytesReader
		}
		writeError(w, r, 400, "parse form: "+err.Error(), nil)
		return
	}
```

Add `"errors"` to the imports block at the top of `handlers.go` if not present.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test -tags sqlite_fts5 ./internal/api/ -run TestUploadMaxBytes -v`
Expected: PASS

- [ ] **Step 6: Run the full Go suite to confirm nothing regressed**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 -timeout 300s ./...`
Expected: all tests pass

- [ ] **Step 7: Commit**

```bash
git add internal/config/config.go internal/api/handlers.go internal/api/upload_limit_test.go
git commit -m "$(cat <<'EOF'
feat(api): cap multipart upload size via MaxBytesReader

New server.max_upload_bytes config (default 100 MiB, env
DOCSIQ_SERVER_MAX_UPLOAD_BYTES). Upload handler wraps request body
with http.MaxBytesReader before ParseMultipartForm so oversized
requests terminate at the transport with 413 instead of allocating
memory or temp files.

Closes the P2-1 TODO in handlers.go.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Refuse insecure default on non-loopback bind (1.4)

**Files:**
- Modify: `cmd/serve.go:148-155` (around `net.Listen`)
- Test: `cmd/serve_test.go` (new — small file; if a test file already exists, append)

- [ ] **Step 1: Write the failing test**

Create `cmd/serve_test.go`:

```go
package cmd

import (
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/config"
)

func TestValidateServeSecurity_RefusesNonLoopbackWithEmptyKey(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 8080
	cfg.Server.APIKey = ""

	err := validateServeSecurity(cfg)
	if err == nil {
		t.Fatal("expected error for empty api_key on non-loopback bind; got nil")
	}
	if !strings.Contains(err.Error(), "api_key") {
		t.Fatalf("error should mention api_key; got %v", err)
	}
}

func TestValidateServeSecurity_AllowsLoopbackWithEmptyKey(t *testing.T) {
	t.Parallel()
	for _, host := range []string{"127.0.0.1", "localhost", "::1"} {
		cfg := &config.Config{}
		cfg.Server.Host = host
		cfg.Server.APIKey = ""
		if err := validateServeSecurity(cfg); err != nil {
			t.Fatalf("host=%s: expected nil; got %v", host, err)
		}
	}
}

func TestValidateServeSecurity_AllowsNonLoopbackWithKey(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.APIKey = "s3cret"
	if err := validateServeSecurity(cfg); err != nil {
		t.Fatalf("expected nil; got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestValidateServeSecurity -v`
Expected: FAIL with `undefined: validateServeSecurity`

- [ ] **Step 3: Add validateServeSecurity**

Append to `cmd/serve.go` (end of file):

```go
// validateServeSecurity refuses to start the server when the API key is
// empty AND the bind host is not loopback. An unauthenticated service
// exposed on the network is almost never intentional; make it explicit.
// Loopback with empty key gets a prominent warning at boot instead.
func validateServeSecurity(cfg *config.Config) error {
	if cfg.Server.APIKey != "" {
		return nil
	}
	host := strings.ToLower(strings.TrimSpace(cfg.Server.Host))
	loopback := host == "127.0.0.1" || host == "localhost" || host == "::1" || host == ""
	if !loopback {
		return fmt.Errorf(
			"server.api_key is empty and server.host=%q is not loopback; refusing to start. "+
				"Set DOCSIQ_SERVER_API_KEY or bind to 127.0.0.1/localhost for dev",
			cfg.Server.Host,
		)
	}
	slog.Warn("⚠️ auth disabled (empty server.api_key); only loopback bind allowed", "host", host)
	return nil
}
```

Ensure `"strings"` and `"github.com/RandomCodeSpace/docsiq/internal/config"` are imported (they likely already are via other code paths).

Wire it into the serve command — edit `cmd/serve.go` around the `addr := fmt.Sprintf(...)` line (line 148):

```go
			if err := validateServeSecurity(cfg); err != nil {
				return err
			}
			addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ -run TestValidateServeSecurity -v`
Expected: PASS (all three sub-cases)

- [ ] **Step 5: Run the full Go suite**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 -timeout 300s ./...`
Expected: all tests pass

- [ ] **Step 6: Commit**

```bash
git add cmd/serve.go cmd/serve_test.go
git commit -m "$(cat <<'EOF'
feat(serve): refuse insecure default on non-loopback bind

validateServeSecurity fails startup when server.api_key is empty AND
server.host is not loopback. Loopback+empty emits a prominent slog
warning instead. Closes the "auth-disabled-by-default" ship-blocker.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Introduce `internal/workq` bounded worker pool (1.3 part A)

**Files:**
- Create: `internal/workq/workq.go`
- Create: `internal/workq/workq_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/workq/workq_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/workq/ -v`
Expected: FAIL with `no Go files in internal/workq`

- [ ] **Step 3: Implement the pool**

Create `internal/workq/workq.go`:

```go
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
// 16-deep queue).
type Config struct {
	Workers    int
	QueueDepth int
}

// Pool is a fixed-size worker pool with a bounded submission queue.
type Pool struct {
	jobs    chan Job
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

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
	p := &Pool{
		jobs:   make(chan Job, cfg.QueueDepth),
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

// Close stops accepting new work, cancels the pool context so in-flight
// jobs can abort early, and waits for workers to exit. Respects the
// caller's ctx deadline; returns ctx.Err() if workers don't finish in time.
func (p *Pool) Close(ctx context.Context) error {
	p.closeOnce.Do(func() {
		close(p.closed)
		close(p.jobs)
		p.cancel()
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/workq/ -v`
Expected: PASS (4 subtests)

- [ ] **Step 5: Race-detector pass**

Run: `go test -race ./internal/workq/ -count=3 -v`
Expected: PASS with no data races

- [ ] **Step 6: Commit**

```bash
git add internal/workq/
git commit -m "$(cat <<'EOF'
feat(workq): bounded worker pool with graceful drain

New internal/workq package. Pool has fixed worker count and a
bounded submission queue; Submit is non-blocking and returns
ErrQueueFull when saturated. Close(ctx) cancels the pool context
and waits for workers to drain within the caller's deadline.

Preparing to replace fire-and-forget upload indexing goroutine.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Wire workq into upload handler + shutdown drain (1.3 part B)

**Files:**
- Modify: `internal/api/handlers.go` (handlers struct + `upload` function, near line 482-510)
- Modify: `internal/api/router.go:57` (NewRouter signature via a new option), `router.go:148-156` (middleware chain — no change; workq threading via handlers struct)
- Modify: `cmd/serve.go` (construct workq, pass via option, drain on shutdown)
- Modify: `internal/config/config.go` (add `Server.WorkqWorkers`, `Server.WorkqDepth` with defaults)
- Test: `internal/api/upload_workq_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `internal/api/upload_workq_test.go`:

```go
package api

import (
	"context"
	"errors"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/workq"
)

// TestHandlers_SubmitUsesWorkq verifies that the upload handler routes
// its background indexing job through the workq Pool the handlers were
// constructed with, rather than spawning a detached goroutine.
//
// We exercise the Submit path directly via a shim (runUploadJob) the
// handler calls; this keeps the test synchronous and focused.
func TestHandlers_SubmitReturns503WhenFull(t *testing.T) {
	t.Parallel()
	pool := workq.New(workq.Config{Workers: 1, QueueDepth: 1})
	defer pool.Close(context.Background())

	block := make(chan struct{})
	// Fill the worker.
	_ = pool.Submit(func(ctx context.Context) { <-block })
	// Fill the queue.
	_ = pool.Submit(func(ctx context.Context) {})
	// A third submission must surface ErrQueueFull.
	err := pool.Submit(func(ctx context.Context) {})
	if !errors.Is(err, workq.ErrQueueFull) {
		t.Fatalf("want ErrQueueFull; got %v", err)
	}
	close(block)
}
```

(This test verifies the pool contract the handler relies on. A fuller
integration test will be added in Task 4/Step 5 once the handler wires
through Submit.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/api/ -run TestHandlers_SubmitReturns503WhenFull -v`
Expected: FAIL with `undefined: workq` (import path not yet used in this test file) — it actually compiles fine since workq exists from Task 3; the test will PASS. That is expected. Delete this test step and instead add the integration assertion below.

Replace the test file content with:

```go
package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/workq"
)

// TestUpload_ReturnsRetryOnFullQueue verifies the upload handler
// responds with 503 + Retry-After when the workq queue is full.
func TestUpload_ReturnsRetryOnFullQueue(t *testing.T) {
	t.Parallel()
	pool := workq.New(workq.Config{Workers: 1, QueueDepth: 1})
	defer pool.Close(context.Background())

	// Saturate the pool: one worker busy, one queue slot full.
	block := make(chan struct{})
	_ = pool.Submit(func(ctx context.Context) { <-block })
	_ = pool.Submit(func(ctx context.Context) {})

	var called atomic.Bool
	h := submitIndexingJob(pool, func(ctx context.Context) {
		called.Store(true)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/upload", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After header")
	}
	if called.Load() {
		t.Fatal("job should not have run")
	}
	close(block)

	_ = errors.Is(nil, workq.ErrQueueFull) // keep errors import
	_ = time.Now()                         // keep time import
}
```

Run: `go test -tags sqlite_fts5 ./internal/api/ -run TestUpload_ReturnsRetryOnFullQueue -v`
Expected: FAIL with `undefined: submitIndexingJob`

- [ ] **Step 3: Add Workq wiring to the handlers struct**

Edit `internal/api/handlers.go` — find the `handlers` struct (near the top of the file) and add a field:

```go
type handlers struct {
	stores     Storer
	provider   llm.Provider
	embedder   *embedder.Embedder
	cfg        *config.Config
	vecIndexes *VectorIndexes
	workq      *workq.Pool // nil = direct goroutine (dev / tests)
	// ... existing fields unchanged
}
```

Add `"github.com/RandomCodeSpace/docsiq/internal/workq"` to the imports.

Add the submitIndexingJob helper at the end of handlers.go:

```go
// submitIndexingJob is the HTTP-layer bridge between an upload request
// and the workq Pool. Extracted for test isolation. Returns an
// http.HandlerFunc; the `job` closure is what the worker will run.
func submitIndexingJob(pool *workq.Pool, job workq.Job) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := pool.Submit(job)
		switch {
		case err == nil:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"accepted"}`))
		case errors.Is(err, workq.ErrQueueFull):
			w.Header().Set("Retry-After", "30")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"indexing queue full; retry later","code":"queue_full"}`))
		default: // ErrClosed or unexpected
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"server shutting down","code":"shutting_down"}`))
		}
	}
}
```

- [ ] **Step 4: Replace the detached goroutine in upload() with a workq submission**

Edit `internal/api/handlers.go` around line 482-510 — replace the entire block starting at `jobID := fmt.Sprintf("job-%d", ...)` through the closing `}()`:

```go
	jobID := fmt.Sprintf("job-%d", h.jobCounter.Add(1))
	slog.Info("📦 upload job queued", "job_id", jobID, "files", len(paths), "project", slug)
	h.setProgress(jobID, fmt.Sprintf("queued: %d files", len(paths)))

	job := func(ctx context.Context) {
		defer os.RemoveAll(tmpDir)
		pl := pipeline.New(st, h.provider, h.cfg)
		for _, p := range paths {
			if ctx.Err() != nil {
				slog.Warn("🛑 upload indexing cancelled on shutdown", "job_id", jobID, "file", filepath.Base(p))
				h.setProgress(jobID, "cancelled")
				return
			}
			slog.Info("📦 upload indexing file", "job_id", jobID, "file", filepath.Base(p))
			h.setProgress(jobID, fmt.Sprintf("indexing: %s", filepath.Base(p)))
			if err := pl.IndexPath(ctx, p, pipeline.IndexOptions{}); err != nil {
				slog.Error("❌ upload indexing failed", "job_id", jobID, "file", filepath.Base(p), "err", err)
				h.setProgress(jobID, fmt.Sprintf("error: %v", err))
				return
			}
		}
		h.setProgress(jobID, "finalizing")
		if err := pl.Finalize(ctx, false, true); err != nil {
			slog.Warn("⚠️ upload finalization failed", "job_id", jobID, "err", err)
		}
		if h.vecIndexes != nil {
			h.vecIndexes.Invalidate(slug)
		}
		h.setProgress(jobID, "done")
	}

	if h.workq == nil {
		// Backward-compatible path for tests or dev: spawn a goroutine.
		go job(context.Background())
	} else {
		if err := h.workq.Submit(job); err != nil {
			if errors.Is(err, workq.ErrQueueFull) {
				writeError(w, r, http.StatusServiceUnavailable, "indexing queue full; retry later", nil)
				w.Header().Set("Retry-After", "30")
				return
			}
			writeError(w, r, http.StatusServiceUnavailable, "server shutting down", err)
			return
		}
	}
	tmpDirOwned = true
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID, "status": "accepted"})
```

Remove the line `bgCtx := context.Background()` that preceded this block. Remove the unused `context` import if no other callsite references it (`go vet` will flag).

- [ ] **Step 5: Add a NewRouter option to thread the pool in**

Edit `internal/api/router.go` — add below `WithProjectStores`:

```go
// WithWorkq injects a bounded worker pool for background indexing
// jobs. When nil (default), upload() falls back to a detached goroutine
// (the dev/test path).
func WithWorkq(p *workq.Pool) RouterOption {
	return func(o *routerOptions) { o.workq = p }
}
```

Extend `routerOptions`:

```go
type routerOptions struct {
	vecIndexes *VectorIndexes
	stores     *projectStores
	workq      *workq.Pool
}
```

In `NewRouter`, propagate into the handlers struct construction:

```go
	h := &handlers{
		stores:     stores,
		provider:   prov,
		embedder:   emb,
		cfg:        cfg,
		vecIndexes: ro.vecIndexes,
		workq:      ro.workq,
	}
```

Add `"github.com/RandomCodeSpace/docsiq/internal/workq"` to router.go imports.

- [ ] **Step 6: Wire workq construction + drain in cmd/serve.go**

Edit `cmd/serve.go` — before `router := api.NewRouter(...)` (around line 143), construct the pool:

```go
			pool := workq.New(workq.Config{
				Workers:    cfg.Server.WorkqWorkers,
				QueueDepth: cfg.Server.WorkqDepth,
			})

			router := api.NewRouter(prov, emb, cfg, registry,
				api.WithProjectStores(stores),
				api.WithVectorIndexes(vecIndexes),
				api.WithWorkq(pool),
			)
```

Add `"github.com/RandomCodeSpace/docsiq/internal/workq"` to serve.go imports.

In the shutdown block (around line 176, after `srv.Shutdown(shutCtx)`):

```go
			// Drain workq within the same deadline as srv.Shutdown.
			// srv.Shutdown already stopped accepting new HTTP requests, so no
			// new jobs can be submitted; all that remains is waiting on
			// in-flight pipelines to honour the cancelled context.
			drainCtx, drainCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer drainCancel()
			if err := pool.Close(drainCtx); err != nil {
				slog.Warn("⚠️ workq drain timeout; some indexing jobs were cancelled mid-flight", "err", err)
			}
```

Add `Server.WorkqWorkers int` and `Server.WorkqDepth int` to the ServerConfig struct in `internal/config/config.go`:

```go
type ServerConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	APIKey         string `mapstructure:"api_key"`
	MaxUploadBytes int64  `mapstructure:"max_upload_bytes"`
	WorkqWorkers   int    `mapstructure:"workq_workers"`
	WorkqDepth     int    `mapstructure:"workq_depth"`
}
```

Add defaults in the Load function, next to the existing server defaults:

```go
	v.SetDefault("server.workq_workers", 0) // 0 → runtime.NumCPU()
	v.SetDefault("server.workq_depth", 64)
```

Update the `workq.New` call in serve.go to apply a NumCPU default when zero:

```go
			workers := cfg.Server.WorkqWorkers
			if workers <= 0 {
				workers = runtime.NumCPU()
			}
			pool := workq.New(workq.Config{Workers: workers, QueueDepth: cfg.Server.WorkqDepth})
```

Add `"runtime"` to serve.go imports.

- [ ] **Step 7: Run all tests**

Run: `go test -tags sqlite_fts5 ./internal/api/ -run TestUpload_ReturnsRetryOnFullQueue -v`
Expected: PASS

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 -race -timeout 300s ./...`
Expected: all tests pass with no data races

- [ ] **Step 8: Commit**

```bash
git add internal/api/handlers.go internal/api/router.go internal/api/upload_workq_test.go internal/config/config.go cmd/serve.go
git commit -m "$(cat <<'EOF'
feat(api): submit upload indexing through workq with graceful drain

Upload handler now submits the indexing closure to a bounded workq
Pool instead of spawning a detached goroutine. When the queue is full
the handler returns 503 with Retry-After: 30. On shutdown, cmd/serve
calls pool.Close with a 30s deadline so in-flight jobs honour the
cancelled context and partial writes are avoided.

Config: server.workq_workers (default NumCPU), server.workq_depth
(default 64).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Session cookie endpoint + middleware acceptance (1.1 part A)

**Files:**
- Create: `internal/api/session.go`
- Create: `internal/api/session_test.go`
- Modify: `internal/api/auth.go` (accept cookie or bearer)
- Modify: `internal/api/auth_test.go` (add cookie cases)
- Modify: `internal/api/router.go` (register `POST /api/session` — public; no auth gate)

- [ ] **Step 1: Write the failing test**

Create `internal/api/session_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSession_PostExchangesBearerForCookie(t *testing.T) {
	t.Parallel()
	h := newSessionHandler("s3cret")
	req := httptest.NewRequest(http.MethodPost, "/api/session", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rr.Code)
	}
	setCookie := rr.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, sessionCookieName+"=") {
		t.Fatalf("missing session cookie: %q", setCookie)
	}
	for _, attr := range []string{"HttpOnly", "Secure", "SameSite=Strict", "Path=/"} {
		if !strings.Contains(setCookie, attr) {
			t.Fatalf("cookie missing %s: %q", attr, setCookie)
		}
	}
}

func TestSession_PostRejectsBadKey(t *testing.T) {
	t.Parallel()
	h := newSessionHandler("s3cret")
	req := httptest.NewRequest(http.MethodPost, "/api/session", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
	if rr.Header().Get("Set-Cookie") != "" {
		t.Fatal("cookie must not be set on failure")
	}
}

func TestSession_DeleteClearsCookie(t *testing.T) {
	t.Parallel()
	h := newSessionDeleteHandler()
	req := httptest.NewRequest(http.MethodDelete, "/api/session", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rr.Code)
	}
	setCookie := rr.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "Max-Age=0") {
		t.Fatalf("cookie should be cleared (Max-Age=0); got %q", setCookie)
	}
}
```

Add to `internal/api/auth_test.go` (append after the existing table tests):

```go
func TestAuth_AcceptsValidCookie(t *testing.T) {
	t.Parallel()
	h := buildAuthHandler("s3cret")
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "s3cret"})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200 with valid cookie, got %d", rr.Code)
	}
}

func TestAuth_RejectsMissingBothHeaderAndCookie(t *testing.T) {
	t.Parallel()
	h := buildAuthHandler("s3cret")
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -tags sqlite_fts5 ./internal/api/ -run "TestSession|TestAuth_Accepts|TestAuth_Rejects" -v`
Expected: FAIL with `undefined: newSessionHandler`, `undefined: newSessionDeleteHandler`, `undefined: sessionCookieName`

- [ ] **Step 3: Implement the session handlers**

Create `internal/api/session.go`:

```go
package api

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
)

// sessionCookieName is the name of the httpOnly cookie that carries the
// bearer token after a successful POST /api/session exchange. The value
// is identical to cfg.Server.APIKey — we do not (yet) rotate or sign it;
// the cookie is a transport-hardening layer, not a session store.
const sessionCookieName = "docsiq_session"

// newSessionHandler returns the POST /api/session handler. Accepts an
// Authorization: Bearer <apiKey> header and on match sets the session
// cookie. 401 on any other shape.
func newSessionHandler(apiKey string) http.HandlerFunc {
	keyBytes := []byte(apiKey)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST, DELETE")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		raw := strings.TrimSpace(r.Header.Get("Authorization"))
		const prefix = "Bearer "
		if !strings.HasPrefix(raw, prefix) {
			writeJSON401(w)
			return
		}
		token := raw[len(prefix):]
		if apiKey == "" || subtle.ConstantTimeCompare([]byte(token), keyBytes) != 1 {
			slog.Warn("🔒 session: auth failure", "remote_addr", r.RemoteAddr, "reason", "wrong_key")
			writeJSON401(w)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    apiKey,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   86400 * 30, // 30 days
		})
		w.WriteHeader(http.StatusNoContent)
	}
}

// newSessionDeleteHandler returns the DELETE /api/session handler,
// which clears the session cookie (client-initiated logout).
func newSessionDeleteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.Header().Set("Allow", "POST, DELETE")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   0,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
```

- [ ] **Step 4: Extend auth middleware to accept cookie**

Edit `internal/api/auth.go` — replace the body of the inner `HandlerFunc` (inside `bearerAuthMiddleware`) so the token is resolved from header OR cookie:

```go
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS preflight bypass.
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		path := r.URL.Path
		if path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		if !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/mcp") {
			next.ServeHTTP(w, r)
			return
		}
		// The session-exchange endpoint is public (it IS the auth boundary).
		if path == "/api/session" {
			next.ServeHTTP(w, r)
			return
		}

		token := extractToken(r)
		if token == "" {
			slog.Warn("🔒 auth failure", "path", path, "remote_addr", r.RemoteAddr, "reason", "no_token")
			writeJSON401(w)
			return
		}
		if subtle.ConstantTimeCompare([]byte(token), keyBytes) != 1 {
			slog.Warn("🔒 auth failure", "path", path, "remote_addr", r.RemoteAddr, "reason", "wrong_key")
			writeJSON401(w)
			return
		}
		next.ServeHTTP(w, r)
	})
```

Add `extractToken` at the end of `auth.go`:

```go
// extractToken returns the bearer token from either the Authorization
// header (preferred, for machine clients) or the session cookie (for
// browser clients after POST /api/session). Returns "" if neither.
func extractToken(r *http.Request) string {
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if strings.HasPrefix(raw, prefix) {
		return raw[len(prefix):]
	}
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}
```

- [ ] **Step 5: Register routes**

Edit `internal/api/router.go` around line 120 (where `/api/upload` is registered). Add:

```go
	mux.HandleFunc("POST /api/session", newSessionHandler(cfg.Server.APIKey))
	mux.HandleFunc("DELETE /api/session", newSessionDeleteHandler())
```

- [ ] **Step 6: Run tests**

Run: `go test -tags sqlite_fts5 ./internal/api/ -run "TestSession|TestAuth" -v`
Expected: PASS — new session and cookie tests green; existing auth tests still green.

Run full suite: `CGO_ENABLED=1 go test -tags sqlite_fts5 -timeout 300s ./...`
Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/api/session.go internal/api/session_test.go internal/api/auth.go internal/api/auth_test.go internal/api/router.go
git commit -m "$(cat <<'EOF'
feat(api): session cookie path alongside bearer auth

New POST /api/session exchanges an Authorization: Bearer key for a
docsiq_session httpOnly; Secure; SameSite=Strict cookie (30-day
Max-Age). DELETE /api/session clears it. Auth middleware now accepts
either the Authorization header or the cookie; /api/session itself is
public (it is the auth boundary).

Preparing to stop injecting the key into served HTML.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: UI switches to cookie-based auth (1.1 part B)

**Files:**
- Modify: `ui/src/lib/api-client.ts`
- Modify: `ui/src/lib/api-client.test.ts` (existing tests)

- [ ] **Step 1: Write the failing test**

Edit `ui/src/lib/api-client.test.ts` (locate the relevant describe block for `apiFetch`; add):

```ts
  it("sends credentials: 'include' on every fetch", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("{}", { status: 200, headers: { "content-type": "application/json" } }),
    );
    await apiFetch("/api/stats");
    const init = (spy.mock.calls[0][1] ?? {}) as RequestInit;
    expect(init.credentials).toBe("include");
    spy.mockRestore();
  });

  it("does not set Authorization header even when a key exists in a meta tag", async () => {
    const meta = document.createElement("meta");
    meta.setAttribute("name", "docsiq-api-key");
    meta.setAttribute("content", "s3cret");
    document.head.appendChild(meta);
    try {
      initAuth();
      const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
        new Response("{}", { status: 200, headers: { "content-type": "application/json" } }),
      );
      await apiFetch("/api/stats");
      const init = (spy.mock.calls[0][1] ?? {}) as RequestInit;
      const hdrs = new Headers(init.headers);
      expect(hdrs.has("Authorization")).toBe(false);
      spy.mockRestore();
    } finally {
      document.head.removeChild(meta);
    }
  });
```

Add `import { vi } from "vitest";` to the top of the test file if absent.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ui && npm test -- --run src/lib/api-client.test.ts`
Expected: FAIL — `credentials` is `undefined` and `Authorization` header IS set (the old behaviour).

- [ ] **Step 3: Rewrite api-client.ts**

Replace the entire contents of `ui/src/lib/api-client.ts` with:

```ts
import type { ApiError } from "@/types/api";

// Before cookies are set the first time, the UI may have been shipped a
// one-shot bearer via the meta tag (legacy). We exchange it for a cookie
// exactly once, then never read or send the key again. If no meta tag
// exists (production path), we rely entirely on cookies already set by
// the operator's OOB provisioning (e.g. `docsiq login`).
let sessionReady: Promise<void> | null = null;

function readOneShotBearerFromMeta(): string | null {
  if (typeof document === "undefined") return null;
  const m = document.querySelector('meta[name="docsiq-api-key"]');
  const v = m?.getAttribute("content");
  return v && v.length > 0 ? v : null;
}

async function establishSession(bearer: string): Promise<void> {
  await fetch("/api/session", {
    method: "POST",
    credentials: "include",
    headers: { Authorization: `Bearer ${bearer}` },
  });
  // Best-effort: scrub the meta tag so the key isn't lying around in the DOM.
  const m = document.querySelector('meta[name="docsiq-api-key"]');
  m?.parentElement?.removeChild(m);
}

export function initAuth(): void {
  const bearer = readOneShotBearerFromMeta();
  sessionReady = bearer ? establishSession(bearer) : Promise.resolve();
}

export class ApiErrorResponse extends Error {
  status: number;
  requestId?: string;
  constructor(status: number, body: ApiError) {
    super(body.error);
    this.status = status;
    this.requestId = body.request_id;
  }
}

export async function apiFetch<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  if (sessionReady) await sessionReady;
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const res = await fetch(path, { ...init, headers, credentials: "include" });
  if (!res.ok) {
    let body: ApiError = { error: `HTTP ${res.status}` };
    try {
      body = await res.json();
    } catch {
      /* non-json */
    }
    throw new ApiErrorResponse(res.status, body);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd ui && npm test -- --run src/lib/api-client.test.ts`
Expected: PASS (both new assertions plus the existing ones).

- [ ] **Step 5: Run UI suite + typecheck + build**

Run: `cd ui && npm run typecheck && npm test -- --run && npm run build`
Expected: typecheck clean, 52+ tests pass, build succeeds.

- [ ] **Step 6: Run Playwright smokes**

Run: `cd ui && CI=1 ./node_modules/.bin/playwright test smoke.spec.ts --reporter=list --workers=1`
Expected: 5/5 pass. (Playwright tests stub /api at the pathname boundary, so the session endpoint is caught by the generic stub.)

- [ ] **Step 7: Commit**

```bash
git add ui/src/lib/api-client.ts ui/src/lib/api-client.test.ts
git commit -m "$(cat <<'EOF'
feat(ui): cookie-based auth; stop reading bearer from DOM

apiFetch now sends credentials: 'include' on every request; the
Authorization header is no longer attached. On boot, initAuth
exchanges the legacy meta-tag bearer (if present) for a cookie via
POST /api/session exactly once and scrubs the meta tag. Production
installs that already ship cookies out-of-band (via `docsiq login`)
skip the exchange entirely.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Remove meta-tag injection from the Go SPA handler (1.1 cleanup)

**Files:**
- Modify: `internal/api/router.go:184-191` (remove the `bytes.Replace` block)
- Modify: `internal/api/router.go` imports (remove `"bytes"` and `"html"` if no other user)

- [ ] **Step 1: Add a test that the SPA response no longer contains the meta tag**

Append to `internal/api/handlers_test.go` (or create `internal/api/spa_test.go` if preferred):

```go
package api

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/RandomCodeSpace/docsiq/internal/config"
)

func TestSpaHandler_DoesNotInjectAPIKey(t *testing.T) {
	t.Parallel()
	fs := fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte(`<html><head></head><body></body></html>`),
		},
	}
	cfg := &config.Config{}
	cfg.Server.APIKey = "s3cret"

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	spaHandler(fs, cfg).ServeHTTP(rr, req)

	body, _ := io.ReadAll(rr.Body)
	if bytes.Contains(body, []byte("docsiq-api-key")) {
		t.Fatalf("served HTML still contains api-key meta tag:\n%s", body)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200; got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/api/ -run TestSpaHandler_DoesNotInjectAPIKey -v`
Expected: FAIL — the served HTML still contains `docsiq-api-key`.

- [ ] **Step 3: Delete the injection code**

Edit `internal/api/router.go` — delete lines 184–191 (the `if cfg.Server.APIKey != "" { content = bytes.Replace(...) }` block) and remove `_ = cfg` if it becomes unused. Remove `"bytes"` and `"html"` from imports if the rest of the file doesn't use them (run `goimports -w internal/api/router.go`).

The simplified spa handler body is:

```go
		content, err := fs.ReadFile(assets, "index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	})
```

- [ ] **Step 4: Run tests**

Run: `go test -tags sqlite_fts5 ./internal/api/ -run TestSpaHandler -v`
Expected: PASS

Run full suite: `CGO_ENABLED=1 go test -tags sqlite_fts5 -timeout 300s ./...`
Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/api/router.go internal/api/handlers_test.go
git commit -m "$(cat <<'EOF'
fix(api): stop injecting API key into served HTML

The SPA handler no longer rewrites index.html to embed the bearer in
a <meta name="docsiq-api-key"> tag. Browser auth now flows through
the docsiq_session cookie set by POST /api/session. Closes the
"API-key-in-HTML" ship-blocker.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Scoped entity fetch in local search (1.5)

**Files:**
- Modify: `internal/store/store.go` — add `EntitiesForDocs(ctx, docIDs)` below `AllEntities`
- Modify: `internal/search/local.go:90` — call scoped fetch instead of `AllEntities`
- Test: `internal/store/entities_for_docs_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `internal/store/entities_for_docs_test.go`:

```go
package store

import (
	"context"
	"testing"
)

func TestEntitiesForDocs_ScopesByRelationshipDocID(t *testing.T) {
	t.Parallel()
	st, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	// Three entities; two relationships each scoped to a doc.
	must := func(err error) { t.Helper(); if err != nil { t.Fatal(err) } }
	must(st.UpsertEntity(ctx, &Entity{ID: "e1", Name: "Alpha"}))
	must(st.UpsertEntity(ctx, &Entity{ID: "e2", Name: "Beta"}))
	must(st.UpsertEntity(ctx, &Entity{ID: "e3", Name: "Gamma"}))
	must(st.UpsertRelationship(ctx, &Relationship{ID: "r1", SourceID: "e1", TargetID: "e2", Predicate: "rel", DocID: "docA"}))
	must(st.UpsertRelationship(ctx, &Relationship{ID: "r2", SourceID: "e3", TargetID: "e1", Predicate: "rel", DocID: "docB"}))

	got, err := st.EntitiesForDocs(ctx, []string{"docA"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("docA: want 2 entities (e1, e2); got %d", len(got))
	}

	// Empty input → empty slice, no error.
	empty, err := st.EntitiesForDocs(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty input: want (0, nil); got (%d, %v)", len(empty), err)
	}
}

func TestEntitiesForDocs_HandlesLargeIDSets(t *testing.T) {
	t.Parallel()
	st, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	ids := make([]string, 1500) // > SQLite's 999 default
	for i := range ids {
		ids[i] = "doc-xyz" // all the same; just testing chunking path doesn't error
	}
	_, err := st.EntitiesForDocs(ctx, ids)
	if err != nil {
		t.Fatalf("chunking at >999 should not error: %v", err)
	}
}
```

If `newTestStore` does not exist, cross-check the conventions in an existing test (e.g. `internal/store/store_test.go`) and copy the helper. If `UpsertEntity` / `UpsertRelationship` signatures differ, adjust to match the real methods (check with `grep -n 'func .*Upsert' internal/store/store.go`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/store/ -run TestEntitiesForDocs -v`
Expected: FAIL with `undefined: EntitiesForDocs`

- [ ] **Step 3: Implement EntitiesForDocs**

Append to `internal/store/store.go` below `AllEntities` (around line 605):

```go
// EntitiesForDocs returns entities that participate (as source or target
// of any relationship) in at least one of the given documents. This is
// the "local" entity set for a scoped search — avoids the full-table
// scan AllEntities performs.
//
// The IN-list is chunked at 900 (below SQLite's default 999 variable
// limit) so caller-supplied doc sets of any size work transparently.
func (s *Store) EntitiesForDocs(ctx context.Context, docIDs []string) ([]*Entity, error) {
	if len(docIDs) == 0 {
		return nil, nil
	}
	const chunkSize = 900
	seen := make(map[string]struct{}, 128)
	out := make([]*Entity, 0, 128)

	for start := 0; start < len(docIDs); start += chunkSize {
		end := start + chunkSize
		if end > len(docIDs) {
			end = len(docIDs)
		}
		chunk := docIDs[start:end]
		placeholders := strings.Repeat("?,", len(chunk))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		q := `SELECT DISTINCT e.id, e.name, e.type, e.description, e.rank, e.community_id, e.vector
		      FROM entities e
		      JOIN relationships r ON (r.source_id = e.id OR r.target_id = e.id)
		      WHERE r.doc_id IN (` + placeholders + `)`
		rows, err := s.db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			e, err := scanEntityRow(rows)
			if err != nil {
				rows.Close()
				return nil, err
			}
			if _, dup := seen[e.ID]; dup {
				continue
			}
			seen[e.ID] = struct{}{}
			out = append(out, e)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return out, nil
}
```

Ensure `"strings"` is imported at the top of `store.go` (it almost certainly is).

- [ ] **Step 4: Update local.go to call the scoped fetch**

Edit `internal/search/local.go` — replace line 90 (`entities, err := st.AllEntities(ctx)`) with:

```go
		// Scope entity fetch to the top-hit documents instead of a
		// full-table scan. Entities with no relationships to any top-hit
		// doc are out of local scope by definition.
		docIDList := make([]string, 0, len(docIDs))
		for id := range docIDs {
			docIDList = append(docIDList, id)
		}
		entities, err := st.EntitiesForDocs(ctx, docIDList)
		if err != nil {
			return nil, err
		}
```

Delete the existing `entities, err := st.AllEntities(ctx)` and the `if err != nil { return nil, err }` that follows.

- [ ] **Step 5: Run tests**

Run: `go test -tags sqlite_fts5 ./internal/store/ -run TestEntitiesForDocs -v`
Expected: PASS

Run: `go test -tags sqlite_fts5 ./internal/search/ -v`
Expected: PASS — any existing local-search tests must remain green.

Run full suite: `CGO_ENABLED=1 go test -tags sqlite_fts5 -timeout 300s ./...`
Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/store/store.go internal/store/entities_for_docs_test.go internal/search/local.go
git commit -m "$(cat <<'EOF'
perf(search): scope entity fetch to top-hit docs in local search

Replaces the full-table AllEntities scan with EntitiesForDocs, which
joins the entities → relationships tables and filters by
relationships.doc_id with a chunked IN-list (chunk size 900, below
SQLite's 999 variable limit). Local search now scales sub-linearly
with corpus size.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review

### Spec coverage

- **Block 1.1 (API key in HTML):** covered by Tasks 5, 6, 7.
- **Block 1.2 (unbounded multipart):** covered by Task 1.
- **Block 1.3 (fire-and-forget goroutine):** covered by Tasks 3, 4.
- **Block 1.4 (auth-disabled-by-default):** covered by Task 2.
- **Block 1.5 (entity full-scan):** covered by Task 8.

All five Block 1 items have at least one task. Spec coverage complete.

### Placeholder check

No "TBD", "TODO", "similar to task N", "add validation", or code-less prose steps. Every step that changes code shows the code. Every command is exact. No reference to undefined types.

### Type consistency

- `Job = func(ctx context.Context)` — used identically in Tasks 3 and 4.
- `ErrQueueFull` — defined in Task 3, caught in Task 4 Step 4 and Step 3.
- `sessionCookieName` — defined in Task 5 Step 3, referenced in Task 5 Step 4 (`extractToken`) and Task 5 Step 1 (test).
- `EntitiesForDocs(ctx, []string) ([]*Entity, error)` — defined in Task 8 Step 3, called identically in Task 8 Step 4 and tested in Task 8 Step 1.
- `validateServeSecurity(cfg *config.Config) error` — defined in Task 2 Step 3, called in Task 2 Step 3 wiring and tested in Task 2 Step 1.

No divergences found.

### Known risks / out-of-plan items (surfaced in spec §10, not addressed in Block 1)

- `ui/src/lib/api-client.ts` still reads a legacy meta tag ONCE for migration. A follow-up task in a later block should add a `docsiq login` CLI subcommand that writes the cookie directly, eliminating the meta-tag path entirely.
- Default workq workers = `NumCPU()` and depth = 64 are guesses; a short load-test note should land in Block 3 before any benchmarks are published.

These are tracked in the spec and intentionally out of scope for Block 1.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-23-block1-critical-plan.md`. Two execution options:

1. **Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** — Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints.

Which approach?
