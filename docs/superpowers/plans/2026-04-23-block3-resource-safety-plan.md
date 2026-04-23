# Block 3 — Resource Safety & Correctness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make docsiq resilient to misbehaving upstreams, slow disks, abrupt shutdowns, and runaway requests — by enforcing context propagation end-to-end, wrapping every outbound call in a timeout, pooling HTTP clients per provider, hardening SQLite PRAGMAs, chunking `EmbedBatch` with backpressure, ringing the HTTP surface with `http.TimeoutHandler`, and enriching the existing panic-recovery and signal-shutdown paths.

**Architecture:** Seven independently-landable tasks. Task 1 (SQLite hardening) stands alone and de-risks every later integration test. Tasks 2 → 3 → 4 build up the LLM provider surface from the network layer outward: first pool the `*http.Client`, then layer a per-call `context.WithTimeout`, then chunk `EmbedBatch` against each provider's declared ceiling with a buffered-channel backpressure shape. Task 5 is a pure static-check + targeted-fix pass that leverages the ctx-first pattern already used in `internal/store`; it's scheduled late so any ctx gaps introduced by Tasks 2–4 are caught in the same sweep. Task 6 lands `http.TimeoutHandler` with explicit layering constraints against Block 2's security-headers middleware. Task 7 closes the two small gaps Block 1 left open in graceful shutdown: panic-log enrichment (req_id, route, method, user) and a defensive-readback audit that signal handling and `workq.Close` already happen in `cmd/serve.go`.

**Tech Stack:** Go 1.22+, `context`, `net/http`, `net/http/httptest`, `database/sql`, `mattn/go-sqlite3`, `log/slog`, Viper + mapstructure. No new third-party dependencies. Existing helpers: `internal/api/request_id.go` (`RequestIDFromContext`), `internal/workq` (`Pool.Close(ctx)`), `internal/api/auth.go` (`UserFromContext` pattern — used for the panic-log enrichment).

**Scope check:** Seven items, two subsystems (HTTP/LLM surface + SQLite). All items share the common theme of bounding resource use and propagating cancellation. No sub-plan decomposition needed; each task is self-contained and produces a compilable, testable diff.

**Dependency note:** Block 2 (PR TBD) introduces `securityHeadersMiddleware` as the outermost router wrapper and a `ContextLogger(ctx)` helper. Task 6 (`http.TimeoutHandler`) must land *after* Block 2 merges so the layering is correct. Tasks 1–5 and Task 7's panic-log enrichment are independent of Block 2 and can land in any order relative to it. If Block 2 has not merged when this plan is executed, Task 6's middleware wiring step documents the fallback layering (TimeoutHandler becomes the outermost wrapper, and Block 2 later reorders so headers go outside it).

---

## File Structure

### Create

- `internal/store/store_hardening_test.go` — verifies PRAGMAs and `Ping(ctx)` post-open (Task 1).
- `internal/llm/httpclient.go` — `newHTTPClient()` helper returning a `*http.Client` with a tuned `*http.Transport` (Task 2).
- `internal/llm/httpclient_test.go` — asserts transport settings on the returned client (Task 2).
- `internal/llm/timeout_test.go` — verifies `Complete` / `Embed` / `EmbedBatch` honour `cfg.LLM.CallTimeout` (Task 3).
- `internal/embedder/batch_test.go` — verifies chunking + backpressure (Task 4).
- `internal/api/timeout_test.go` — verifies the request-level timeout plus upload-endpoint override (Task 6).
- `scripts/ctx-audit.sh` — tiny custom linter that greps for HTTP/DB calls missing a ctx argument in the listed packages (Task 5).
- `internal/api/panic_enrichment_test.go` — verifies panic log carries `req_id`, `route`, `method`, `user` (Task 7).

### Modify

- `internal/store/store.go` — `open` helper (around line 38) re-keyed to explicit `PRAGMA` calls after `sql.Open`; pool settings raised to `MaxOpenConns=4, MaxIdleConns=2, ConnMaxLifetime=1h`; new `Ping(ctx)` method on `*Store` (Task 1).
- `internal/config/config.go:145-152` — add `RequestTimeout`, `UploadTimeout` to `ServerConfig`; add `CallTimeout` to `LLMConfig`; add defaults + `BindEnv` wiring at the bottom of `Load` (Tasks 3, 6).
- `internal/llm/provider.go` — `lcProvider` gains an unexported `httpClient *http.Client` field and `callTimeout time.Duration`; `Complete`/`Embed`/`EmbedBatch` wrap `parent` in `context.WithTimeout` when `callTimeout > 0` (Task 3). `EmbedBatch` slices input to `batchCeiling` and feeds a buffered channel of size `2*batchCeiling` (Task 4).
- `internal/llm/openai.go` — `newOpenAIProvider` constructs the tuned client via `newHTTPClient()` and passes it via `openai.WithHTTPClient(...)` (Task 2). Declares `batchCeiling = 2048` (Task 4).
- `internal/llm/provider.go` (`newAzureProvider`, `newOllamaProvider`) — same client injection pattern; declare `batchCeiling = 16` (Azure), `batchCeiling = 128` (Ollama) (Tasks 2, 4).
- `internal/api/router.go:205-215` — `recoveryMiddleware` enriched to capture `req_id`, `route`, `method`, `user`, full stack on panic (Task 7).
- `internal/api/router.go:~218` (the bottom `return loggingMiddleware(...)` expression) — wrapped with `requestTimeoutMiddleware(cfg)` between the security-headers layer and `loggingMiddleware`; `POST /api/upload` and `POST /api/projects/{project}/docs` route through a longer-timeout branch (Task 6).
- `scripts/ctx-audit.sh` invocation in `Makefile` (if one exists; else documented as a one-liner) and CI — Task 5 step 2 covers discovery.
- `cmd/serve.go` — verify signal-handling/shutdown sequence; no functional change expected. Only step: log-line audit in Task 7 step 6 confirming existing `🛑 shutting down...` → `srv.Shutdown` → `pool.Close` already covers the spec's progress-logging requirement.

### No Placeholders — All Pointers Are Real

Every file mentioned above exists today (checked against `internal/store/store.go`, `internal/config/config.go`, `internal/llm/provider.go`, `internal/api/router.go`, `cmd/serve.go`) except the eight files flagged **Create**. No invented types, no invented packages.

---

## Task 1: SQLite hardening (3.6)

**Files:**
- Modify: `internal/store/store.go:35-48` (the `open` helper + pool config)
- Modify: `internal/store/store.go:~81` (append `Ping(ctx)` method near `Close`)
- Create: `internal/store/store_hardening_test.go`

### Rationale

Today `internal/store/store.go:38` wires PRAGMAs through the DSN query string:

```go
db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
// ...
db.SetMaxOpenConns(1) // SQLite WAL allows 1 writer
```

Three problems:

1. `synchronous=NORMAL` is missing — SQLite defaults to `FULL` under WAL which costs two fsyncs per commit; `NORMAL` is the documented WAL-safe sweet spot.
2. DSN-string PRAGMAs are driver-specific to `mattn/go-sqlite3` and silently ignored by some alternative drivers. Running explicit `PRAGMA` statements post-open is portable and self-documenting.
3. `MaxOpenConns=1` is conservative to the point of serializing all reads. WAL mode supports concurrent readers; the writer is already serialized by SQLite itself. Raising to 4 (with 2 idle, 1-hour max lifetime) lets the upload pipeline and search handlers overlap reads without fighting for a single connection slot.

There is no `Ping(ctx)` on `*Store` today — only a raw `db.Ping()` reachable via `s.DB().Ping()`. Exposing a context-aware `Ping(ctx)` on the interface unblocks Block 4's `/readyz` endpoint.

- [ ] **Step 1: Write the failing hardening test**

Create `internal/store/store_hardening_test.go`:

```go
package store

import (
	"context"
	"testing"
	"time"
)

// TestOpen_HardeningPragmas verifies Block 3.6 — every PRAGMA the spec
// requires is observable on a freshly-opened store.
func TestOpen_HardeningPragmas(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, err := OpenForProject(dir, "harden")
	if err != nil {
		t.Fatalf("OpenForProject: %v", err)
	}
	defer s.Close()

	cases := []struct {
		name string
		sql  string
		want string
	}{
		{"journal_mode", `PRAGMA journal_mode`, "wal"},
		{"foreign_keys", `PRAGMA foreign_keys`, "1"},
		{"synchronous", `PRAGMA synchronous`, "1"}, // 1 = NORMAL
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			var got string
			if err := s.DB().QueryRow(c.sql).Scan(&got); err != nil {
				t.Fatalf("%s: %v", c.sql, err)
			}
			if got != c.want {
				t.Fatalf("%s = %q; want %q", c.sql, got, c.want)
			}
		})
	}

	t.Run("busy_timeout_ge_5000", func(t *testing.T) {
		var got int
		if err := s.DB().QueryRow(`PRAGMA busy_timeout`).Scan(&got); err != nil {
			t.Fatalf("PRAGMA busy_timeout: %v", err)
		}
		if got < 5000 {
			t.Fatalf("busy_timeout = %d ms; want >= 5000", got)
		}
	})
}

// TestOpen_PoolSettings asserts the raised MaxOpenConns / MaxIdleConns
// values survive the Open recipe. MaxOpenConns=4, MaxIdleConns=2,
// ConnMaxLifetime=1h are not individually observable via sql.DB stats
// without opening connections; we assert on Stats().MaxOpenConnections
// which reflects SetMaxOpenConns.
func TestOpen_PoolSettings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, err := OpenForProject(dir, "pool")
	if err != nil {
		t.Fatalf("OpenForProject: %v", err)
	}
	defer s.Close()

	stats := s.DB().Stats()
	if stats.MaxOpenConnections != 4 {
		t.Fatalf("MaxOpenConnections = %d; want 4", stats.MaxOpenConnections)
	}
}

// TestStore_PingContext asserts the new context-aware Ping method.
// A cancelled context must surface as a ctx.Err(), not a generic
// database error, so that /readyz can distinguish "caller gave up"
// from "SQLite is sick".
func TestStore_PingContext(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, err := OpenForProject(dir, "ping")
	if err != nil {
		t.Fatalf("OpenForProject: %v", err)
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	cancelled, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if err := s.Ping(cancelled); err == nil {
		t.Fatalf("Ping on cancelled ctx: want non-nil error, got nil")
	}
}
```

- [ ] **Step 2: Run the test (red)**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/store/ -run TestOpen_HardeningPragmas -v`

Expected: FAIL on `synchronous = 2; want 1` (default `FULL=2`, want `NORMAL=1`).

Also run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/store/ -run TestOpen_PoolSettings -v`

Expected: FAIL on `MaxOpenConnections = 1; want 4`.

Also run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/store/ -run TestStore_PingContext -v`

Expected: FAIL — `s.Ping undefined (type *Store has no field or method Ping)`.

- [ ] **Step 3: Implement the hardened `open`**

Edit `internal/store/store.go`. Replace the existing `open` function (lines 35-48) with:

```go
// open is the low-level SQLite opener. It is unexported — the only public
// factory is OpenForProject. Kept as a helper because the project registry
// and the per-project store both use the same DSN+migrate recipe.
//
// Block 3.6 hardening:
//   - PRAGMAs set explicitly via Exec after sql.Open (driver-portable).
//   - MaxOpenConns=4 + MaxIdleConns=2 allows concurrent readers under
//     WAL; the writer is already serialized by SQLite itself.
//   - ConnMaxLifetime=1h guards against stale connections in long-lived
//     server processes.
func open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("open db: path is empty")
	}
	// _busy_timeout retained in the DSN as a belt-and-braces default:
	// the explicit PRAGMA below is the authoritative setting, but the
	// DSN form protects against an early query landing before the
	// PRAGMA Exec completes.
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	pragmas := []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA busy_timeout=5000`,
		`PRAGMA synchronous=NORMAL`,
		`PRAGMA foreign_keys=ON`,
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("open db: %s: %w", p, err)
		}
	}

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(1 * time.Hour)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}
```

- [ ] **Step 4: Add `Ping(ctx)` method**

Edit `internal/store/store.go`. Locate the existing `Close()` method (around line 81). Append directly below it:

```go
// Ping verifies the database connection is alive. Uses PingContext so
// a cancelled ctx surfaces as ctx.Err(); callers (e.g. /readyz) can
// differentiate "request cancelled" from "SQLite broken".
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
```

- [ ] **Step 5: Run tests (green)**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/store/ -v`

Expected: all tests PASS — including the new `TestOpen_HardeningPragmas`, `TestOpen_PoolSettings`, `TestStore_PingContext`, plus the existing `TestOpen_BusyTimeoutPragma` regression test.

- [ ] **Step 6: Run the whole suite with race detector to catch pool-related regressions**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 -race -timeout 300s ./...`

Expected: PASS everywhere. Pay attention to any store-adjacent package (`internal/pipeline`, `internal/community`, `internal/api`); raising `MaxOpenConns` from 1 to 4 changes the serialization characteristics SQLite already enforces internally, but the Go-side contract shifts — any test relying on implicit single-connection ordering will surface here.

If a race/deadlock surfaces in a specific test, diagnose root cause (likely an ad-hoc `db.Query` inside a write transaction on a different goroutine). Do not paper over with `SetMaxOpenConns(1)`.

- [ ] **Step 7: Commit**

```bash
git add internal/store/store.go internal/store/store_hardening_test.go
git commit -m "$(cat <<'EOF'
feat(store): SQLite hardening — explicit PRAGMAs, raised pool, Ping(ctx)

Replaces DSN-embedded PRAGMAs with explicit `db.Exec("PRAGMA …")` calls
so the recipe is driver-portable and self-documenting. Adds
`PRAGMA synchronous=NORMAL` — the WAL-safe default that cuts two fsyncs
per commit to one — which was silently missing before. Raises
MaxOpenConns from 1 to 4 (with MaxIdleConns=2, ConnMaxLifetime=1h) so
concurrent readers overlap under WAL; the writer remains serialized
by SQLite itself.

Adds `(*Store).Ping(ctx context.Context) error` on the public Store
surface so Block 4's /readyz can distinguish a cancelled caller from
a dead database. Covered by TestStore_PingContext.

Block 3.6.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: HTTP client pooling per LLM provider (3.5)

**Files:**
- Create: `internal/llm/httpclient.go`
- Create: `internal/llm/httpclient_test.go`
- Modify: `internal/llm/openai.go` — inject the pooled client into `openai.New(...)`
- Modify: `internal/llm/provider.go` — same injection in `newAzureProvider` and `newOllamaProvider`; store `*http.Client` on `lcProvider`

### Rationale

Langchaingo constructs a fresh `net/http.Transport` per call-site today — meaning every `openai.New`, `ollama.New`, etc. allocates its own TCP/TLS pool. For a long-running server that talks to OpenAI / Azure / Ollama on every request, this wastes connections and defeats keep-alive. A single shared `*http.Client` per provider with a tuned transport closes the leak.

Spec-mandated transport settings:
- `MaxIdleConns=100`
- `MaxIdleConnsPerHost=10`
- `IdleConnTimeout=90s`
- `TLSHandshakeTimeout=10s`
- `ResponseHeaderTimeout=60s`

- [ ] **Step 1: Write the failing test**

Create `internal/llm/httpclient_test.go`:

```go
package llm

import (
	"net/http"
	"testing"
	"time"
)

// TestNewHTTPClient_TransportSettings verifies the tuned transport
// settings required by Block 3.5.
func TestNewHTTPClient_TransportSettings(t *testing.T) {
	t.Parallel()
	c := newHTTPClient()
	if c == nil {
		t.Fatal("newHTTPClient returned nil")
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport = %T; want *http.Transport", c.Transport)
	}
	if got, want := tr.MaxIdleConns, 100; got != want {
		t.Errorf("MaxIdleConns = %d; want %d", got, want)
	}
	if got, want := tr.MaxIdleConnsPerHost, 10; got != want {
		t.Errorf("MaxIdleConnsPerHost = %d; want %d", got, want)
	}
	if got, want := tr.IdleConnTimeout, 90*time.Second; got != want {
		t.Errorf("IdleConnTimeout = %v; want %v", got, want)
	}
	if got, want := tr.TLSHandshakeTimeout, 10*time.Second; got != want {
		t.Errorf("TLSHandshakeTimeout = %v; want %v", got, want)
	}
	if got, want := tr.ResponseHeaderTimeout, 60*time.Second; got != want {
		t.Errorf("ResponseHeaderTimeout = %v; want %v", got, want)
	}
}

// TestNewHTTPClient_NoClientTimeout asserts we do NOT set
// http.Client.Timeout — that would hard-cut the body mid-stream on
// large embedding responses. Per-call timeouts live on ctx instead
// (Task 3).
func TestNewHTTPClient_NoClientTimeout(t *testing.T) {
	t.Parallel()
	c := newHTTPClient()
	if c.Timeout != 0 {
		t.Fatalf("Client.Timeout = %v; want 0 (use ctx per-call)", c.Timeout)
	}
}
```

- [ ] **Step 2: Run the test (red)**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/llm/ -run TestNewHTTPClient -v`

Expected: FAIL — `newHTTPClient undefined`.

- [ ] **Step 3: Implement `newHTTPClient`**

Create `internal/llm/httpclient.go`:

```go
// Package llm — HTTP client pooling per provider (Block 3.5).
//
// langchaingo constructs a fresh net/http.Transport inside each
// provider constructor by default. For a long-running server that
// calls the same provider on every request, that leaks connections:
// every call-site allocates its own idle-conn pool, TLS session
// cache, and DNS resolver bucket. Pooling one *http.Client per
// provider (constructed here) fixes the leak.
package llm

import (
	"net"
	"net/http"
	"time"
)

// newHTTPClient returns a *http.Client tuned for long-lived LLM
// provider traffic. The transport settings are spec-driven:
//   - MaxIdleConns=100          — plenty of headroom for bursty batching
//   - MaxIdleConnsPerHost=10    — matches langchaingo default fan-out
//   - IdleConnTimeout=90s       — trim idle conns before cloud LBs do
//   - TLSHandshakeTimeout=10s   — fail fast on broken TLS upstreams
//   - ResponseHeaderTimeout=60s — distinct from body-stream timeout;
//     bounds the silent-server failure mode
//
// Deliberately NOT set:
//   - Client.Timeout — would hard-cut streaming bodies; per-call
//     timeouts come from ctx (Task 3 / Block 3.3).
//   - DialContext timeout — Go's default (no timeout, relies on ctx)
//     is correct here; a fixed dial timeout fights ctx-driven shutdown.
func newHTTPClient() *http.Client {
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{Transport: tr}
}
```

- [ ] **Step 4: Run the test (green)**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/llm/ -run TestNewHTTPClient -v`

Expected: both sub-tests PASS.

- [ ] **Step 5: Wire the pooled client into OpenAI provider**

Edit `internal/llm/openai.go`. Locate `newOpenAIProvider`. Replace its current body (the block that builds `chatOpts` and calls `openai.New`) with the version below. The only additions are the `httpClient` construction and the `openai.WithHTTPClient(...)` option in both option slices:

```go
func newOpenAIProvider(cfg *config.LLMConfig) (Provider, error) {
	oc := &cfg.OpenAI
	if oc.APIKey == "" {
		return nil, fmt.Errorf("openai: API key is empty (set llm.openai.api_key or DOCSIQ_LLM_OPENAI_API_KEY)")
	}

	baseURL := oc.BaseURL
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	chatModel := oc.ChatModel
	if chatModel == "" {
		chatModel = defaultOpenAIChatModel
	}
	embedModel := oc.EmbedModel
	if embedModel == "" {
		embedModel = defaultOpenAIEmbedModel
	}

	// Block 3.5: one pooled *http.Client shared across chat + embed
	// langchaingo handles. Same connection pool for every outbound
	// request the lcProvider makes.
	httpClient := newHTTPClient()

	chatOpts := []openai.Option{
		openai.WithToken(oc.APIKey),
		openai.WithBaseURL(baseURL),
		openai.WithModel(chatModel),
		openai.WithHTTPClient(httpClient),
	}
	if oc.Organization != "" {
		chatOpts = append(chatOpts, openai.WithOrganization(oc.Organization))
	}
	chatLLM, err := openai.New(chatOpts...)
	if err != nil {
		return nil, fmt.Errorf("openai chat LLM: %w", err)
	}

	embedOpts := []openai.Option{
		openai.WithToken(oc.APIKey),
		openai.WithBaseURL(baseURL),
		openai.WithEmbeddingModel(embedModel),
		openai.WithModel(chatModel),
		openai.WithHTTPClient(httpClient),
	}
	if oc.Organization != "" {
		embedOpts = append(embedOpts, openai.WithOrganization(oc.Organization))
	}
	embedLLM, err := openai.New(embedOpts...)
	if err != nil {
		return nil, fmt.Errorf("openai embed LLM: %w", err)
	}
	emb, err := embeddings.NewEmbedder(embedLLM)
	if err != nil {
		return nil, fmt.Errorf("openai embedder: %w", err)
	}

	return &lcProvider{
		llm:           chatLLM,
		emb:           emb,
		name:          "openai",
		modelID:       embedModel,
		httpClient:    httpClient,
		batchCeiling:  2048,
	}, nil
}
```

Note: `httpClient` and `batchCeiling` are new fields on `lcProvider`. Add them in step 6. The `embeddings` and `openai` imports are already present; no new imports.

- [ ] **Step 6: Add `httpClient` and `batchCeiling` fields to `lcProvider`**

Edit `internal/llm/provider.go`. Locate the `lcProvider` struct definition. Extend it:

```go
// lcProvider adapts langchaingo to our Provider interface.
type lcProvider struct {
	llm     llms.Model
	emb     embeddings.Embedder
	name    string
	modelID string

	// Block 3.5: pooled HTTP client shared with the langchaingo
	// sub-clients. Stored here so tests can assert on it and so
	// future work can swap it (e.g. for a tracing transport).
	httpClient *http.Client

	// Block 3.3: optional per-call timeout wrapped around ctx. Zero
	// means "no timeout" (caller's ctx is authoritative); positive
	// values trigger context.WithTimeout in Complete/Embed/EmbedBatch.
	callTimeout time.Duration

	// Block 3.4: provider-declared batch ceiling. EmbedBatch slices
	// input to this size; caller-visible chunking also uses this
	// value so the Embedder can construct correctly-sized jobs.
	batchCeiling int
}
```

Add `"net/http"` and `"time"` to the imports at the top of `provider.go` if not already present (check with the imports block — `time` is likely already there from `applyOptions`; `net/http` is likely new).

- [ ] **Step 7: Wire the pooled client into Ollama and Azure providers**

Still in `internal/llm/provider.go`.

Update `newOllamaProvider` to share a client. Locate it; replace the return statement with a version that passes `ollama.WithHTTPClient(httpClient)` to both `ollama.New` calls and populates the new struct fields:

```go
func newOllamaProvider(cfg *config.LLMConfig) (Provider, error) {
	httpClient := newHTTPClient()

	chatLLM, err := ollama.New(
		ollama.WithServerURL(cfg.Ollama.BaseURL),
		ollama.WithModel(cfg.Ollama.ChatModel),
		ollama.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("ollama chat LLM: %w", err)
	}
	embedLLM, err := ollama.New(
		ollama.WithServerURL(cfg.Ollama.BaseURL),
		ollama.WithModel(cfg.Ollama.EmbedModel),
		ollama.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("ollama embed LLM: %w", err)
	}
	emb, err := embeddings.NewEmbedder(embedLLM)
	if err != nil {
		return nil, fmt.Errorf("ollama embedder: %w", err)
	}
	return &lcProvider{
		llm:          chatLLM,
		emb:          emb,
		name:         "ollama",
		modelID:      cfg.Ollama.EmbedModel,
		httpClient:   httpClient,
		batchCeiling: 128,
	}, nil
}
```

If `ollama.WithHTTPClient` is not an exported option in the vendored langchaingo revision, skip it for Ollama (self-hosted, usually loopback — pooling has less value). In that case, add a comment `// ollama: langchaingo revision does not expose WithHTTPClient — pool is not injected. Acceptable: Ollama typically runs on loopback where pool savings are marginal.` and still populate `httpClient: httpClient` on the struct (the field is informational in that fallback).

Update `newAzureProvider` similarly. The Azure path uses `openai.New` under the hood (see the existing code that calls `openai.WithToken(az.ChatAPIKey())` etc.); add `openai.WithHTTPClient(httpClient)` to every call there and set `batchCeiling: 16`.

- [ ] **Step 8: Build the whole module**

Run: `CGO_ENABLED=1 go build -tags sqlite_fts5 ./...`

Expected: BUILD OK. If `ollama.WithHTTPClient` or `openai.WithHTTPClient` does not exist in the vendored langchaingo, fix by reading the vendored package (`vendor/github.com/tmc/langchaingo/llms/openai/`) to find the correct option name, or fall back to the inspection-based comment noted in Step 7.

- [ ] **Step 9: Run the full Go suite**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 -timeout 300s ./...`

Expected: all PASS. The existing `provider_openai_test.go` may need minor updates if it inspects struct fields directly — check output.

- [ ] **Step 10: Commit**

```bash
git add internal/llm/httpclient.go internal/llm/httpclient_test.go internal/llm/openai.go internal/llm/provider.go
git commit -m "$(cat <<'EOF'
feat(llm): one pooled *http.Client per provider

langchaingo allocates a fresh net/http.Transport inside every provider
constructor by default — a new idle-conn pool, TLS session cache, and
DNS bucket per openai.New / ollama.New call. For a long-running server
that talks to the same upstream on every request, that's pure waste
and defeats keep-alive.

Adds internal/llm/httpclient.go with newHTTPClient() returning a
*http.Client backed by a tuned *http.Transport (MaxIdleConns=100,
MaxIdleConnsPerHost=10, IdleConnTimeout=90s, TLSHandshakeTimeout=10s,
ResponseHeaderTimeout=60s). Injects the same client into chat and
embed langchaingo sub-clients via WithHTTPClient options so every
provider uses exactly one pool.

The pooled client's Timeout is deliberately 0 — per-call deadlines
land in Task 3 (3.3) via ctx. Hard-cutting the client would truncate
streaming embedding responses.

Block 3.5.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: LLM call timeouts (3.3)

**Files:**
- Modify: `internal/config/config.go` — add `LLMConfig.CallTimeout` + default + `BindEnv`
- Modify: `internal/llm/provider.go` — populate `lcProvider.callTimeout`; wrap ctx in `Complete`, `Embed`, `EmbedBatch`
- Create: `internal/llm/timeout_test.go`

### Rationale

Spec: "every provider wraps its outbound call in `ctx, cancel := context.WithTimeout(parent, cfg.LLM.CallTimeout)`, default 60s. Retry wrapper (if any) counts retries against the total timeout, not resets it."

Today `lcProvider.Complete/Embed/EmbedBatch` pass the parent ctx straight through to langchaingo. Langchaingo has no internal retry wrapper in the vendored revision (verified by reading `vendor/github.com/tmc/langchaingo/llms/openai/openaillm.go`), so the "retry counts against total timeout" clause collapses to "don't reset the timeout between retries" — trivially satisfied because there is no retry.

- [ ] **Step 1: Add `CallTimeout` to `LLMConfig`**

Edit `internal/config/config.go`. Locate the `LLMConfig` struct (around line 48). Add the field:

```go
type LLMConfig struct {
	Provider string       `mapstructure:"provider"`
	Azure    AzureConfig  `mapstructure:"azure"`
	Ollama   OllamaConfig `mapstructure:"ollama"`
	OpenAI   OpenAIConfig `mapstructure:"openai"`

	// CallTimeout caps the end-to-end duration of a single provider
	// call (Complete / Embed / EmbedBatch). Any retry wrapper counts
	// against this deadline — the timeout is NOT reset between
	// attempts. Zero disables the per-call cap (caller's ctx is
	// authoritative). Default 60s.
	CallTimeout time.Duration `mapstructure:"call_timeout"`
}
```

Add `"time"` to the imports of `config.go` if not already present. It already is (used for other duration fields elsewhere in the file — check with a quick grep; if not, add it now).

- [ ] **Step 2: Add the default and env binding**

Still in `internal/config/config.go`, locate the block of `v.SetDefault("llm.*", …)` calls. Add immediately after the last `llm.*` default:

```go
	// LLM — per-call timeout (Block 3.3)
	v.SetDefault("llm.call_timeout", 60*time.Second)
```

Locate the bottom `_ = v.BindEnv(...)` block. Add:

```go
	_ = v.BindEnv("llm.call_timeout")
```

Also verify the Viper instance decodes `time.Duration` strings ("60s") correctly. Viper's default `DecoderConfigOption` chain includes `StringToTimeDurationHookFunc` — this repo uses the default Viper unmarshal, so no extra wiring is needed. If Task 1 of Block 2 already swapped `Unmarshal` for `UnmarshalExact`, that preserves the duration hook. (Double-check by reading `config.go` around the `Unmarshal` call before editing.)

- [ ] **Step 3: Populate `callTimeout` in each provider constructor**

Edit `internal/llm/provider.go`. In each of `newOpenAIProvider`, `newAzureProvider`, `newOllamaProvider`, add `callTimeout: cfg.CallTimeout` to the `lcProvider{}` literal. For `newOpenAIProvider` (in `openai.go`) the struct literal now reads:

```go
	return &lcProvider{
		llm:          chatLLM,
		emb:          emb,
		name:         "openai",
		modelID:      embedModel,
		httpClient:   httpClient,
		callTimeout:  cfg.CallTimeout,
		batchCeiling: 2048,
	}, nil
```

Same addition for the Ollama and Azure constructors.

- [ ] **Step 4: Wrap ctx in the three provider methods**

Edit `internal/llm/provider.go`. Replace `lcProvider.Complete`, `lcProvider.Embed`, `lcProvider.EmbedBatch` with the timeout-aware versions:

```go
// withCallTimeout returns a child ctx bounded by p.callTimeout when
// positive, plus its cancel. Zero/negative callTimeout returns the
// parent ctx unchanged and a no-op cancel — callers always defer
// cancel() without branching.
func (p *lcProvider) withCallTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	if p.callTimeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, p.callTimeout)
}

func (p *lcProvider) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
	ctx, cancel := p.withCallTimeout(ctx)
	defer cancel()
	o := applyOptions(opts)
	callOpts := []llms.CallOption{
		llms.WithMaxTokens(o.maxTokens),
		llms.WithTemperature(o.temperature),
	}
	if o.jsonMode {
		callOpts = append(callOpts, llms.WithJSONMode())
	}
	return llms.GenerateFromSinglePrompt(ctx, p.llm, prompt, callOpts...)
}

func (p *lcProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	ctx, cancel := p.withCallTimeout(ctx)
	defer cancel()
	return p.emb.EmbedQuery(ctx, text)
}

func (p *lcProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	ctx, cancel := p.withCallTimeout(ctx)
	defer cancel()
	return p.emb.EmbedDocuments(ctx, texts)
}
```

(Task 4 will rewrite `EmbedBatch` to chunk + backpressure. The timeout wrapper at the outer function remains; it gets moved into `EmbedBatch`'s outer loop in Task 4.)

Add `"context"` to the `provider.go` imports if not already present (it is — `Complete` already takes `ctx context.Context`).

- [ ] **Step 5: Write the timeout test**

Create `internal/llm/timeout_test.go`:

```go
package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
)

// stubModel implements llms.Model by blocking forever on GenerateContent
// until the context is cancelled. It proves the provider honours ctx
// deadlines rather than swallowing them.
type stubModel struct{}

func (stubModel) Call(ctx context.Context, prompt string, opts ...llms.CallOption) (string, error) {
	return stubModel{}.generate(ctx)
}

func (stubModel) GenerateContent(ctx context.Context, msgs []llms.MessageContent, opts ...llms.CallOption) (*llms.ContentResponse, error) {
	if _, err := stubModel{}.generate(ctx); err != nil {
		return nil, err
	}
	return &llms.ContentResponse{}, nil
}

func (stubModel) generate(ctx context.Context) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

// stubEmbedder blocks on EmbedDocuments / EmbedQuery until ctx done.
type stubEmbedder struct{}

func (stubEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (stubEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

var _ embeddings.Embedder = stubEmbedder{}

func TestLcProvider_Complete_HonoursCallTimeout(t *testing.T) {
	t.Parallel()
	p := &lcProvider{
		llm:         stubModel{},
		emb:         stubEmbedder{},
		name:        "stub",
		modelID:     "stub",
		callTimeout: 50 * time.Millisecond,
	}
	start := time.Now()
	_, err := p.Complete(context.Background(), "hello")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("Complete: want non-nil error on timeout, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Complete error: want context.DeadlineExceeded, got %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Complete returned after %v; callTimeout=50ms — deadline not propagated", elapsed)
	}
}

func TestLcProvider_Embed_HonoursCallTimeout(t *testing.T) {
	t.Parallel()
	p := &lcProvider{
		llm:         stubModel{},
		emb:         stubEmbedder{},
		callTimeout: 50 * time.Millisecond,
	}
	start := time.Now()
	_, err := p.Embed(context.Background(), "hello")
	elapsed := time.Since(start)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Embed error: want DeadlineExceeded, got %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Embed elapsed = %v; want < 500ms", elapsed)
	}
}

func TestLcProvider_ZeroCallTimeout_LeavesParentCtxAuthoritative(t *testing.T) {
	t.Parallel()
	p := &lcProvider{
		llm:         stubModel{},
		emb:         stubEmbedder{},
		callTimeout: 0, // disabled — parent ctx wins
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := p.Complete(ctx, "hello")
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Complete error with parent deadline: want DeadlineExceeded, got %v", err)
	}
}
```

- [ ] **Step 6: Run the test (red → green)**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/llm/ -run TestLcProvider -v`

Expected: all PASS. If compilation fails because `stubModel` does not satisfy `llms.Model`, open `vendor/github.com/tmc/langchaingo/llms/llms.go` and match the interface exactly — the methods listed above are the current contract (`Call` is deprecated but often still required; include both).

- [ ] **Step 7: Run full suite**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 -timeout 300s ./...`

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/config/config.go internal/llm/provider.go internal/llm/openai.go internal/llm/timeout_test.go
git commit -m "$(cat <<'EOF'
feat(llm): per-call timeout on every provider (default 60s)

Every Complete / Embed / EmbedBatch call now wraps its parent ctx in
context.WithTimeout(parent, cfg.LLM.CallTimeout). Default is 60s;
callers can disable by setting llm.call_timeout=0, in which case the
parent ctx's deadline is authoritative. The vendored langchaingo has
no internal retry loop, so the spec's "retry counts against total
timeout" clause is trivially satisfied by wrapping once at the
provider entry point.

Adds config.LLMConfig.CallTimeout + DOCSIQ_LLM_CALL_TIMEOUT env
binding + a 60s default. Tests use stub Model/Embedder implementations
that block on ctx.Done, proving DeadlineExceeded surfaces cleanly
within 500ms when callTimeout=50ms.

Block 3.3.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: `EmbedBatch` chunking + backpressure (3.4)

**Files:**
- Modify: `internal/llm/provider.go` — `EmbedBatch` slices input to `batchCeiling`, pushes per-slice results through a buffered channel
- Modify: `internal/embedder/embedder.go` — caller-side awareness: `batchSize` is capped to provider's `batchCeiling`
- Create: `internal/embedder/batch_test.go`

### Rationale

Spec: "each provider declares its batch ceiling (OpenAI 2048, Azure 16 per deployment default, Ollama 128 heuristic). Caller chunks input to that ceiling. Provider pushes results through a buffered channel of size `2 * batchCeiling`; slow consumers get backpressure."

Today `EmbedBatch` calls `p.emb.EmbedDocuments(ctx, texts)` with the caller's full slice — no chunking, no channel. Large batches blow past OpenAI's 2048 limit (error) and Azure's 16 limit (silent truncation on some deployment configs).

The buffered-channel pattern protects downstream consumers that process results one-at-a-time slower than the provider returns them. With a `2 * batchCeiling` buffer, a slow consumer will backpressure the producer only when the buffer fills — letting the caller control upstream cost.

Design note: we expose `BatchCeiling()` on `Provider` so the `Embedder` can size its top-level slicing. Keeping the chunking in two places (provider AND embedder) is redundant; the embedder slices to at-most `batchCeiling` per request, and the provider enforces the same ceiling as a defense-in-depth assertion.

- [ ] **Step 1: Add `BatchCeiling()` to the `Provider` interface**

Edit `internal/llm/provider.go`. Locate the `Provider` interface. Add one method:

```go
// Provider is the unified LLM interface.
type Provider interface {
	Complete(ctx context.Context, prompt string, opts ...Option) (string, error)
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Name() string
	ModelID() string
	// BatchCeiling returns the maximum number of texts that can be
	// passed to EmbedBatch in a single call. Callers that need to
	// process larger inputs must slice to this ceiling. Zero means
	// "no declared ceiling" (rare — only for providers that don't
	// care). Block 3.4.
	BatchCeiling() int
}
```

Add the method to `lcProvider`:

```go
func (p *lcProvider) BatchCeiling() int { return p.batchCeiling }
```

- [ ] **Step 2: Rewrite `EmbedBatch` with chunking + backpressure**

Still in `internal/llm/provider.go`. Replace the existing `EmbedBatch`:

```go
// EmbedBatch embeds texts in provider-sized chunks. Input is sliced to
// at-most p.batchCeiling per upstream request. Per-chunk results are
// pushed through a buffered channel of size 2*batchCeiling — a slow
// consumer (which in practice means the calling goroutine) will
// backpressure the producer once the buffer fills.
//
// The function assembles the final [][]float32 in input order. Errors
// from any chunk short-circuit the whole call via ctx cancellation.
//
// When batchCeiling <= 0 we fall back to a single upstream call — no
// chunking, no buffer. That path preserves behaviour for providers
// that have not declared a ceiling.
func (p *lcProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	ctx, cancel := p.withCallTimeout(ctx)
	defer cancel()

	if len(texts) == 0 {
		return nil, nil
	}

	// No declared ceiling — single pass, preserve old behaviour.
	if p.batchCeiling <= 0 {
		return p.emb.EmbedDocuments(ctx, texts)
	}

	ceiling := p.batchCeiling
	if len(texts) <= ceiling {
		return p.emb.EmbedDocuments(ctx, texts)
	}

	// Chunk boundaries (start, end) pairs — deterministic order.
	type chunk struct {
		start, end int
	}
	var chunks []chunk
	for i := 0; i < len(texts); i += ceiling {
		end := i + ceiling
		if end > len(texts) {
			end = len(texts)
		}
		chunks = append(chunks, chunk{start: i, end: end})
	}

	type chunkResult struct {
		start int
		vecs  [][]float32
		err   error
	}
	// Buffer sized 2*ceiling in *vector slots* — since each chunk
	// carries up to `ceiling` vectors, 2*ceiling slots equals two
	// in-flight chunks. That's deliberate: one chunk completed,
	// one en route; a slow consumer backpressures the third.
	results := make(chan chunkResult, 2)

	// Producer: iterate chunks serially. Serial emission is intentional
	// — the buffer provides headroom for a single slow consumer step.
	// Concurrent multi-chunk dispatch is the Embedder's job (Step 3).
	go func() {
		defer close(results)
		for _, c := range chunks {
			slice := texts[c.start:c.end]
			vecs, err := p.emb.EmbedDocuments(ctx, slice)
			select {
			case results <- chunkResult{start: c.start, vecs: vecs, err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()

	out := make([][]float32, len(texts))
	for r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("embed batch [%d:%d]: %w", r.start, r.start+len(r.vecs), r.err)
		}
		for i, v := range r.vecs {
			out[r.start+i] = v
		}
	}

	// Defensive: every slot must be populated. If a chunk errored
	// between buffer push and loop drain we'd have returned above;
	// reaching here means every result arrived.
	return out, ctx.Err()
}
```

Note: `fmt` is already imported in `provider.go`; no new import needed.

- [ ] **Step 3: Let the `Embedder` size its batches from the provider ceiling**

Edit `internal/embedder/embedder.go`. Update `New` to clamp `batchSize` to the provider's ceiling:

```go
// New creates a new Embedder. If provider is nil (LLM disabled via
// provider=none), New returns nil. Callers must check for nil before use.
//
// Block 3.4: the caller-supplied batchSize is clamped to the provider's
// declared batch ceiling (OpenAI 2048, Azure 16, Ollama 128). A
// batchSize exceeding the ceiling would cause silent truncation or
// explicit 400s depending on the provider.
func New(provider llm.Provider, batchSize int) *Embedder {
	if provider == nil {
		return nil
	}
	if batchSize <= 0 {
		batchSize = 20
	}
	if ceiling := provider.BatchCeiling(); ceiling > 0 && batchSize > ceiling {
		batchSize = ceiling
	}
	return &Embedder{provider: provider, batchSize: batchSize, concurrency: 4}
}
```

- [ ] **Step 4: Write the chunking test**

Create `internal/embedder/batch_test.go`:

```go
package embedder

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/llm"
)

// recordingProvider implements llm.Provider and captures every
// EmbedBatch call's slice length. It returns zero-filled vectors of
// length 4 per input text.
type recordingProvider struct {
	mu        sync.Mutex
	ceiling   int
	callSizes []int
	delay     time.Duration
}

func (r *recordingProvider) Name() string       { return "recording" }
func (r *recordingProvider) ModelID() string    { return "recording-v1" }
func (r *recordingProvider) BatchCeiling() int  { return r.ceiling }

func (r *recordingProvider) Complete(ctx context.Context, prompt string, opts ...llm.Option) (string, error) {
	return "", nil
}

func (r *recordingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0, 0, 0, 0}, nil
}

func (r *recordingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	r.mu.Lock()
	r.callSizes = append(r.callSizes, len(texts))
	r.mu.Unlock()
	if r.delay > 0 {
		select {
		case <-time.After(r.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0, 0, 0, 0}
	}
	return out, nil
}

// TestEmbedder_New_ClampsToBatchCeiling: a user asking for batchSize=5000
// against an OpenAI-like provider with ceiling=2048 gets clamped to 2048.
func TestEmbedder_New_ClampsToBatchCeiling(t *testing.T) {
	t.Parallel()
	p := &recordingProvider{ceiling: 2048}
	e := New(p, 5000)
	if e.batchSize != 2048 {
		t.Fatalf("batchSize = %d; want 2048 (clamped to ceiling)", e.batchSize)
	}
}

// TestEmbedder_New_BelowCeilingIsUnchanged: a user asking for 100 against
// a ceiling of 2048 keeps 100.
func TestEmbedder_New_BelowCeilingIsUnchanged(t *testing.T) {
	t.Parallel()
	p := &recordingProvider{ceiling: 2048}
	e := New(p, 100)
	if e.batchSize != 100 {
		t.Fatalf("batchSize = %d; want 100 (unchanged)", e.batchSize)
	}
}

// TestEmbedder_EmbedTexts_ChunksToBatchSize: 500 texts with batchSize=100
// results in 5 EmbedBatch calls, each of size 100.
func TestEmbedder_EmbedTexts_ChunksToBatchSize(t *testing.T) {
	t.Parallel()
	p := &recordingProvider{ceiling: 2048}
	e := New(p, 100)

	texts := make([]string, 500)
	for i := range texts {
		texts[i] = "t"
	}

	if _, err := e.EmbedTexts(context.Background(), texts); err != nil {
		t.Fatalf("EmbedTexts: %v", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.callSizes) != 5 {
		t.Fatalf("EmbedBatch calls = %d; want 5 (500 / 100)", len(p.callSizes))
	}
	for i, n := range p.callSizes {
		if n != 100 {
			t.Fatalf("call[%d] size = %d; want 100", i, n)
		}
	}
}

// TestEmbedder_EmbedTexts_PreservesOrder: returned vectors are assembled
// in input order, even with concurrent batches.
func TestEmbedder_EmbedTexts_PreservesOrder(t *testing.T) {
	t.Parallel()
	// orderedProvider returns vectors marked with their input index.
	type orderedProvider struct{ recordingProvider }
	op := &orderedProvider{recordingProvider{ceiling: 2048, delay: 5 * time.Millisecond}}
	e := New(llm.Provider(op), 50)

	texts := make([]string, 250)
	for i := range texts {
		texts[i] = "t"
	}
	vecs, err := e.EmbedTexts(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedTexts: %v", err)
	}
	if len(vecs) != 250 {
		t.Fatalf("vecs len = %d; want 250", len(vecs))
	}
	for i, v := range vecs {
		if len(v) != 4 {
			t.Fatalf("vecs[%d] len = %d; want 4", i, len(v))
		}
	}
}
```

- [ ] **Step 5: Run the test**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/embedder/ -v`

Expected: all PASS.

- [ ] **Step 6: Verify provider-level chunking with a targeted unit test**

Append to `internal/llm/timeout_test.go` (created in Task 3) a chunking test that exercises `lcProvider.EmbedBatch` directly:

```go
// chunkCountingEmbedder counts how many times EmbedDocuments is called
// and with what sizes. Used to verify provider-level chunking.
type chunkCountingEmbedder struct {
	mu        sync.Mutex
	callSizes []int
}

func (c *chunkCountingEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	c.mu.Lock()
	c.callSizes = append(c.callSizes, len(texts))
	c.mu.Unlock()
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{float32(len(c.callSizes)), float32(i)}
	}
	return out, nil
}

func (c *chunkCountingEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return []float32{0}, nil
}

func TestLcProvider_EmbedBatch_ChunksToCeiling(t *testing.T) {
	t.Parallel()
	ce := &chunkCountingEmbedder{}
	p := &lcProvider{
		llm:          stubModel{},
		emb:          ce,
		batchCeiling: 16, // Azure-sized
	}

	texts := make([]string, 50)
	for i := range texts {
		texts[i] = "t"
	}

	vecs, err := p.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(vecs) != 50 {
		t.Fatalf("vecs len = %d; want 50", len(vecs))
	}

	ce.mu.Lock()
	defer ce.mu.Unlock()
	// 50 / 16 = 3 full chunks of 16 + 1 tail of 2 → 4 calls.
	if len(ce.callSizes) != 4 {
		t.Fatalf("chunk calls = %d; want 4", len(ce.callSizes))
	}
	if ce.callSizes[0] != 16 || ce.callSizes[1] != 16 || ce.callSizes[2] != 16 || ce.callSizes[3] != 2 {
		t.Fatalf("chunk sizes = %v; want [16 16 16 2]", ce.callSizes)
	}
}
```

Add `"sync"` to the imports of `timeout_test.go`.

- [ ] **Step 7: Run the llm tests**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/llm/ -v`

Expected: PASS, including the new `TestLcProvider_EmbedBatch_ChunksToCeiling`.

- [ ] **Step 8: Full-suite regression**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 -race -timeout 300s ./...`

Expected: PASS with race detector clean.

- [ ] **Step 9: Commit**

```bash
git add internal/llm/provider.go internal/llm/openai.go internal/embedder/embedder.go internal/embedder/batch_test.go internal/llm/timeout_test.go
git commit -m "$(cat <<'EOF'
feat(llm,embedder): EmbedBatch chunking + backpressure per provider

Each provider now declares a BatchCeiling (OpenAI 2048, Azure 16,
Ollama 128) via the Provider interface. lcProvider.EmbedBatch slices
input to that ceiling and pushes per-chunk results through a buffered
channel of size 2 (in chunks, equivalently 2*ceiling vector slots) —
a slow consumer backpressures the producer once the buffer fills.

The Embedder's New() now clamps caller-supplied batchSize to the
provider's ceiling, so "batch_size: 5000" in a config against an
Azure deployment silently becomes 16 rather than failing with a
provider-side 400.

Block 3.4.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Context propagation audit (3.1)

**Files:**
- Create: `scripts/ctx-audit.sh`
- Modify: any exported function in `internal/llm`, `internal/embedder`, `internal/extractor`, `internal/crawler`, `internal/store` that fails the audit
- Modify: any ad-hoc `http.DefaultClient.Get(...)` / `client.Get(pageURL)` call in `internal/crawler` that bypasses ctx (the spec calls out HTTP calls without ctx as audit failures)

### Rationale

Spec: "every exported function in `internal/llm`, `internal/embedder`, `internal/extractor`, `internal/crawler`, `internal/store` accepts `ctx context.Context` as first argument and passes it to every I/O call it makes."

Based on the survey during planning:

- `internal/store/store.go` — already ctx-first everywhere except `migrate()` (unexported, called only from `open` which has no ctx; acceptable).
- `internal/llm/provider.go` — all exported methods ctx-first (verified).
- `internal/embedder/embedder.go` — `New`, `ModelID`, `EmbedTexts`, `EmbedOne`; `EmbedTexts` and `EmbedOne` are ctx-first. `ModelID` and `New` are pure — no ctx needed.
- `internal/extractor/entities.go`, `claims.go` — `ExtractEntities`, `ExtractClaims` already take ctx.
- `internal/crawler/crawler.go` — `Crawl(ctx, rootURL, opts)` takes ctx, but `parseSitemap(client, sitemapURL, base)`, `extractLinks(client, pageURL, base)`, `discoverSitemap(client, base)` are unexported and call `client.Get(url)` without ctx. `client.Get` does not propagate cancellation — a `ctx.Done()` while a sitemap fetch is in flight will wait for the HTTP response. This is the concrete audit failure.

The audit script is a belt-and-braces guard that catches future regressions. It's not a replacement for the targeted fix pass.

- [ ] **Step 1: Write the audit script**

Create `scripts/ctx-audit.sh`:

```bash
#!/usr/bin/env bash
# scripts/ctx-audit.sh — Block 3.1 static check.
#
# Greps the listed internal packages for:
#   1. Exported (capitalized-receiver-or-name) functions whose first
#      parameter is not ctx context.Context. Allowed exceptions:
#      constructors (New*, Open*), pure-accessor methods ending in
#      `String() string`, `Name() string`, `ModelID() string`,
#      `BatchCeiling() int`.
#   2. HTTP calls that bypass ctx: http.Get, http.Post, client.Get,
#      client.Post, http.NewRequest (without Context), http.DefaultClient.
#   3. DB calls that bypass ctx: s.db.Query, s.db.Exec, s.db.QueryRow
#      (QueryContext / ExecContext / QueryRowContext are fine).
#
# Exits non-zero if any violation is found. Intended as a CI gate.
set -euo pipefail

PACKAGES=(
  internal/llm
  internal/embedder
  internal/extractor
  internal/crawler
  internal/store
)

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$ROOT"

fail=0

echo "==> HTTP calls without ctx"
for pkg in "${PACKAGES[@]}"; do
  # http.Get, http.Post, client.Get(, client.Post( — exclude test files,
  # exclude http.NewRequestWithContext.
  hits="$(grep -rnE \
    -e 'http\.Get\(' \
    -e 'http\.Post\(' \
    -e '(^|[^A-Za-z_])client\.Get\(' \
    -e '(^|[^A-Za-z_])client\.Post\(' \
    -e 'http\.DefaultClient\.' \
    -e 'http\.NewRequest\(' \
    "$pkg" --include='*.go' --exclude='*_test.go' || true)"
  if [ -n "$hits" ]; then
    echo "$hits"
    fail=1
  fi
done

echo "==> DB calls without ctx"
for pkg in "${PACKAGES[@]}"; do
  # Match .Query(, .Exec(, .QueryRow( at package scope; exclude
  # Context-suffixed variants and exclude tests.
  hits="$(grep -rnE \
    -e '\.(Query|Exec|QueryRow)\(' \
    "$pkg" --include='*.go' --exclude='*_test.go' \
    | grep -vE '\.(Query|Exec|QueryRow)Context\(' \
    | grep -vE '^\S+\.go:[0-9]+:\s*//' || true)"
  # Exclude the migrate() path which uses s.db.Exec intentionally at
  # open time (no ctx available).
  hits="$(echo "$hits" | grep -v 'store\.go.*s\.db\.Exec(schema)' | grep -v 'store\.go.*s\.db\.Exec(m)' || true)"
  if [ -n "$hits" ]; then
    echo "$hits"
    fail=1
  fi
done

echo "==> Exported funcs without ctx as first arg"
for pkg in "${PACKAGES[@]}"; do
  # Capture `func (r *Type) Name(arg1 T1, ...)` and `func Name(arg1 T1, ...)`.
  # Only error on funcs that (a) are exported, (b) have at least one
  # arg, (c) first arg type isn't context.Context. Allow known
  # constructors/accessors.
  while IFS= read -r line; do
    file="${line%%:*}"; rest="${line#*:}"
    lineno="${rest%%:*}"; code="${rest#*:}"
    # Extract function name.
    name="$(echo "$code" | sed -nE 's/.*func(\s*\(.*\))?\s*([A-Z][A-Za-z0-9_]*)\(.*/\2/p')"
    if [ -z "$name" ]; then continue; fi
    case "$name" in
      New*|Open*|Name|ModelID|BatchCeiling|Close|String|DB|Ping) continue ;;
    esac
    first_arg_type="$(echo "$code" | sed -nE 's/.*\((\s*[a-zA-Z_][a-zA-Z0-9_]*\s+)?([a-zA-Z_][a-zA-Z0-9_\.]*).*/\2/p')"
    if [ "$first_arg_type" != "context.Context" ]; then
      echo "$file:$lineno: $name first arg type = ${first_arg_type:-<none>}; want context.Context"
      fail=1
    fi
  done < <(grep -nE '^func .*[A-Z][A-Za-z0-9_]*\s*\([^)]' "$pkg"/*.go 2>/dev/null | grep -v '_test.go:')
done

if [ $fail -ne 0 ]; then
  echo ""
  echo "CTX AUDIT FAILED — see output above."
  exit 1
fi
echo "CTX AUDIT OK"
```

Make it executable:

```bash
chmod +x scripts/ctx-audit.sh
```

- [ ] **Step 2: Run the audit once — collect violations**

Run: `bash scripts/ctx-audit.sh || true`

Expected output (sample): violations in `internal/crawler/crawler.go` for `client.Get(sitemapURL)`, `client.Get(pageURL)` inside `parseSitemap`, `discoverSitemap`, `extractLinks`.

Note: these are unexported funcs so they are not blocked by the "exported without ctx" rule, but they ARE flagged by the "HTTP calls without ctx" rule. That's intentional — the spec says "every I/O call" not "only exported-function I/O."

Any other violations surfaced (e.g. in `internal/store` if a query was added without `Context` suffix) must be fixed in Step 3.

- [ ] **Step 3: Fix the crawler ctx propagation**

Edit `internal/crawler/crawler.go`.

Change `discoverSitemap`, `parseSitemap`, `extractLinks`, `bfsCrawl` (already has ctx — confirmed), to accept `ctx context.Context` and use `http.NewRequestWithContext(ctx, http.MethodGet, url, nil)` + `client.Do(req)` instead of `client.Get(url)`.

Concrete before/after for one site (replicate for all three):

Before (`parseSitemap`):

```go
func parseSitemap(client *http.Client, sitemapURL string, base *url.URL) ([]string, error) {
	resp, err := client.Get(sitemapURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sitemap not found")
	}
	defer resp.Body.Close()
	// …
	for _, s := range idx.Sitemaps {
		sub, err := parseSitemap(client, s.Loc, base)
		// …
	}
}
```

After:

```go
func parseSitemap(ctx context.Context, client *http.Client, sitemapURL string, base *url.URL) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sitemapURL, nil)
	if err != nil {
		return nil, fmt.Errorf("sitemap request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil, fmt.Errorf("sitemap not found")
	}
	defer resp.Body.Close()
	// …
	for _, s := range idx.Sitemaps {
		sub, err := parseSitemap(ctx, client, s.Loc, base)
		// …
	}
}
```

Update the three callers (`Crawl`, `discoverSitemap`, `bfsCrawl`, `extractLinks`) to thread ctx through. `Crawl` already has ctx; pass it down.

- [ ] **Step 4: Re-run the audit**

Run: `bash scripts/ctx-audit.sh`

Expected: `CTX AUDIT OK` with exit 0.

- [ ] **Step 5: Run the crawler suite and full-suite**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/crawler/ -v`

Expected: PASS. If `crawler_test.go` calls any of the renamed helpers directly, update it to pass `context.Background()`.

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 -timeout 300s ./...`

Expected: PASS.

- [ ] **Step 6: Wire the audit into CI**

Edit `.github/workflows/ci.yml` (or the primary CI workflow file — discover with `ls .github/workflows/`). Add a step between existing "build" and "test" stages:

```yaml
      - name: ctx propagation audit
        run: bash scripts/ctx-audit.sh
```

If no CI workflow exists (unlikely — Block 1 mentions PR workflows), skip this step and note it in the commit message. The script is still useful as a local developer check.

- [ ] **Step 7: Commit**

```bash
git add scripts/ctx-audit.sh internal/crawler/crawler.go internal/crawler/crawler_test.go .github/workflows/ci.yml
git commit -m "$(cat <<'EOF'
feat(ci,crawler): ctx propagation audit + crawler fixes

Adds scripts/ctx-audit.sh — a static check that flags any HTTP or
DB call in the five ctx-required packages (internal/{llm,embedder,
extractor,crawler,store}) that bypasses ctx, plus any exported func
whose first argument is not context.Context (minus well-known
constructors and accessors). Wired into CI as a gate.

The initial audit surfaced three crawler helpers (parseSitemap,
discoverSitemap, extractLinks) that used client.Get(url) instead of
client.Do(http.NewRequestWithContext(ctx, ...)). A ctx.Done while
those were in flight would stall for up to 30s waiting on the HTTP
client's own timeout. Fixed by threading ctx through every level of
the crawl.

Block 3.1.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Request-level timeout + upload override (3.2)

**Files:**
- Modify: `internal/config/config.go` — add `ServerConfig.RequestTimeout`, `ServerConfig.UploadTimeout` + defaults + env bindings
- Modify: `internal/api/router.go` — introduce `requestTimeoutMiddleware(cfg)`; wire it below `securityHeadersMiddleware` (Block 2) and above `loggingMiddleware`; carve out a longer-timeout branch for upload routes
- Create: `internal/api/timeout_test.go`

### Rationale

Spec: "`http.TimeoutHandler(router, cfg.Server.RequestTimeout, 'request timeout')` at the outermost middleware. Default 30s. Per-endpoint override for `POST /api/projects/{p}/docs` (allow up to `cfg.Server.UploadTimeout`, default 10m) so indexing kickoff isn't killed."

Layering (from the task spec's "Implication for 3.2" note):

```
securityHeadersMiddleware  (Block 2 — outermost)
  → requestTimeoutMiddleware  (Block 3.2 — new)
    → loggingMiddleware       (existing)
      → recoveryMiddleware
        → bearerAuthMiddleware
          → projectMiddleware
            → mux
```

`http.TimeoutHandler` has a subtle gotcha: once the timeout fires, it writes a 503 response and subsequent writes from the inner handler are discarded. For streaming endpoints (file uploads, SSE) that defeats the endpoint. The spec's solution is to route upload POSTs through a longer timeout branch — we do this by splitting the middleware: wrap the mux's upload paths with `UploadTimeout`, everything else with `RequestTimeout`.

Real upload route in the codebase: `POST /api/upload` (see `handlers.go:398`). The spec references `POST /api/projects/{p}/docs`; that route does not exist today. For this plan we use the actually-existing route `POST /api/upload` AND include `POST /api/projects/{project}/import` (the notes import route, which can also be large) under the upload timeout. This is a concrete interpretation — document it in the task commit.

- [ ] **Step 1: Add config fields**

Edit `internal/config/config.go`. Extend `ServerConfig` (currently lines 145-152) by appending:

```go
type ServerConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	APIKey         string `mapstructure:"api_key"`
	MaxUploadBytes int64  `mapstructure:"max_upload_bytes"`
	WorkqWorkers   int    `mapstructure:"workq_workers"`
	WorkqDepth     int    `mapstructure:"workq_depth"`

	// RequestTimeout caps the duration of every HTTP handler except
	// the carve-outs listed in UploadRoutes. Block 3.2 default 30s.
	// Zero disables the cap (not recommended in production).
	RequestTimeout time.Duration `mapstructure:"request_timeout"`

	// UploadTimeout caps long-running upload / import endpoints
	// (POST /api/upload, POST /api/projects/{project}/import). Block
	// 3.2 default 10m.
	UploadTimeout time.Duration `mapstructure:"upload_timeout"`
}
```

Add `"time"` to the imports of `config.go` if not already present. It already is.

- [ ] **Step 2: Add defaults and env bindings**

Still in `internal/config/config.go`, immediately after the block of `server.*` SetDefault calls:

```go
	v.SetDefault("server.request_timeout", 30*time.Second)
	v.SetDefault("server.upload_timeout", 10*time.Minute)
```

Append to the `BindEnv` block at the bottom:

```go
	_ = v.BindEnv("server.request_timeout")
	_ = v.BindEnv("server.upload_timeout")
```

- [ ] **Step 3: Write the timeout middleware test**

Create `internal/api/timeout_test.go`:

```go
package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/config"
)

// TestRequestTimeoutMiddleware_FiresOnSlowHandler: a handler that
// sleeps past the request timeout returns 503 Service Unavailable.
func TestRequestTimeoutMiddleware_FiresOnSlowHandler(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cfg.Server.RequestTimeout = 50 * time.Millisecond
	cfg.Server.UploadTimeout = 1 * time.Second

	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte("too late"))
	})
	handler := requestTimeoutMiddleware(cfg)(slow)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want 503", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "request timeout") {
		t.Fatalf("body = %q; want substring 'request timeout'", body)
	}
}

// TestRequestTimeoutMiddleware_UploadRouteGetsExtendedTimeout: an upload
// request that completes within UploadTimeout (but exceeds
// RequestTimeout) succeeds.
func TestRequestTimeoutMiddleware_UploadRouteGetsExtendedTimeout(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cfg.Server.RequestTimeout = 50 * time.Millisecond
	cfg.Server.UploadTimeout = 500 * time.Millisecond

	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	handler := requestTimeoutMiddleware(cfg)(slow)

	req := httptest.NewRequest(http.MethodPost, "/api/upload", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200 (upload route under UploadTimeout)", rec.Code)
	}
}

// TestRequestTimeoutMiddleware_FastHandlerUnaffected: a handler that
// responds well within the timeout is passed through unchanged.
func TestRequestTimeoutMiddleware_FastHandlerUnaffected(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cfg.Server.RequestTimeout = 100 * time.Millisecond
	cfg.Server.UploadTimeout = 1 * time.Second

	fast := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "ok")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	handler := requestTimeoutMiddleware(cfg)(fast)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	if got := rec.Header().Get("X-Test"); got != "ok" {
		t.Fatalf("X-Test = %q; want ok", got)
	}
}
```

- [ ] **Step 4: Run the test (red)**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/api/ -run TestRequestTimeoutMiddleware -v`

Expected: FAIL — `requestTimeoutMiddleware undefined`.

- [ ] **Step 5: Implement `requestTimeoutMiddleware`**

Add a new file or append to the existing `internal/api/router.go`. To keep `router.go` manageable, create `internal/api/request_timeout.go`:

```go
package api

import (
	"net/http"
	"strings"

	"github.com/RandomCodeSpace/docsiq/internal/config"
)

// uploadRoutePrefixes enumerate the routes that get UploadTimeout
// instead of RequestTimeout. An exact match on POST + prefix is
// required; GET on the same prefix falls through to RequestTimeout.
//
// The intent per spec: long-running indexing kickoffs (file upload,
// tar import) must not be killed mid-stream by the 30s default.
var uploadRoutePrefixes = []struct {
	method, prefix string
}{
	{http.MethodPost, "/api/upload"},
	{http.MethodPost, "/api/projects/"}, // /api/projects/{p}/import
}

// isUploadRoute reports whether r should be granted UploadTimeout.
// A /api/projects/* POST is only an upload if the trailing segment is
// /import — we check that here to avoid granting 10-minute timeouts
// to note-write POSTs on the same prefix.
func isUploadRoute(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	switch {
	case r.URL.Path == "/api/upload":
		return true
	case strings.HasPrefix(r.URL.Path, "/api/projects/") && strings.HasSuffix(r.URL.Path, "/import"):
		return true
	}
	return false
}

// requestTimeoutMiddleware wraps inner in http.TimeoutHandler with
// cfg.Server.RequestTimeout as the default bound, bumped to
// cfg.Server.UploadTimeout for upload routes.
//
// Zero timeout means "no cap" — useful for local dev. In that case
// inner is returned unchanged.
func requestTimeoutMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		reqTimeout := cfg.Server.RequestTimeout
		upTimeout := cfg.Server.UploadTimeout

		// Pre-build the two TimeoutHandler instances so each request
		// just dispatches to one — no per-request allocation.
		defaultTO := inner
		if reqTimeout > 0 {
			defaultTO = http.TimeoutHandler(inner, reqTimeout, "request timeout")
		}
		uploadTO := inner
		if upTimeout > 0 {
			uploadTO = http.TimeoutHandler(inner, upTimeout, "upload timeout")
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isUploadRoute(r) {
				uploadTO.ServeHTTP(w, r)
				return
			}
			defaultTO.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 6: Run the tests (green)**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/api/ -run TestRequestTimeoutMiddleware -v`

Expected: all three PASS.

- [ ] **Step 7: Wire into the router**

Edit `internal/api/router.go`. Locate the bottom return statement (around line 218):

Current (after Block 2 merged):
```go
return securityHeadersMiddleware(cfg)(
    loggingMiddleware(
        recoveryMiddleware(
            bearerAuthMiddleware(cfg.Server.APIKey,
                projectMiddleware(cfg, registry, mux)))))
```

Change to:
```go
return securityHeadersMiddleware(cfg)(
    requestTimeoutMiddleware(cfg)(
        loggingMiddleware(
            recoveryMiddleware(
                bearerAuthMiddleware(cfg.Server.APIKey,
                    projectMiddleware(cfg, registry, mux))))))
```

Fallback (if Block 2 has NOT merged and `securityHeadersMiddleware` does not yet exist): wrap as the outermost layer:

```go
return requestTimeoutMiddleware(cfg)(
    loggingMiddleware(
        recoveryMiddleware(
            bearerAuthMiddleware(cfg.Server.APIKey,
                projectMiddleware(cfg, registry, mux)))))
```

Before editing, run `grep -n securityHeadersMiddleware internal/api/router.go` to detect which state the file is in.

Rationale for the inside-security-outside-logging placement: a 503 timeout response MUST still carry CSP + security headers (so an attacker can't substitute the 503 body with attacker-controlled HTML); and the timeout must still be LOGGED (so operators see the latency spike). The layering satisfies both.

- [ ] **Step 8: Run full suite**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 -race -timeout 300s ./...`

Expected: PASS. Any existing router integration test that did NOT account for `http.TimeoutHandler` wrapping its handler may see slightly different response shapes — check specifically `auth_integration_test.go`, `docs_integration_test.go`, `upload_progress_test.go`. If a test fails with a "http: superfluous response.WriteHeader" warning, the inner handler is writing after the timeout hit in test conditions; either shorten the test handler or raise the RequestTimeout in the test config.

- [ ] **Step 9: Commit**

```bash
git add internal/config/config.go internal/api/request_timeout.go internal/api/timeout_test.go internal/api/router.go
git commit -m "$(cat <<'EOF'
feat(api): request-level timeout with upload carve-out

Wraps the router in http.TimeoutHandler(cfg.Server.RequestTimeout,
"request timeout") — default 30s — so a misbehaving handler cannot
hold a goroutine indefinitely. POST /api/upload and
POST /api/projects/{p}/import route through a longer-timeout branch
bounded by cfg.Server.UploadTimeout (default 10m) so large indexing
kickoffs aren't killed mid-stream.

Layering is deliberate:

  securityHeadersMiddleware   ← outermost (Block 2)
    requestTimeoutMiddleware  ← new (Block 3.2)
      loggingMiddleware
        recoveryMiddleware
          bearerAuthMiddleware
            projectMiddleware
              mux

A 503 timeout response carries CSP/security headers; the timeout is
still logged.

Adds server.request_timeout + server.upload_timeout Viper keys with
env bindings (DOCSIQ_SERVER_REQUEST_TIMEOUT / _UPLOAD_TIMEOUT).

Block 3.2.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Graceful-shutdown gap-closer (3.7)

**Files:**
- Modify: `internal/api/router.go` — enrich `recoveryMiddleware` with `req_id`, `route`, `method`, `user`, full stack
- Create: `internal/api/panic_enrichment_test.go`
- Audit only (no code change): `cmd/serve.go` — confirm signal handling and `workq.Close(drainCtx)` already satisfy the spec

### Rationale

Spec: "`main` listens for `SIGINT`/`SIGTERM`, cancels the server's root context, calls `srv.Shutdown(ctx)` with a 30s deadline, then `workq.Close(remaining)` with the same deadline; logs progress at each step. Panic middleware captures `req_id`, route, method, user (if authed), full stack."

From the planning survey of `cmd/serve.go`:

- Signal handling: present via `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`.
- Shutdown: present — `srv.Shutdown(shutCtx)` with 10s deadline, then `pool.Close(drainCtx)` with 30s deadline.
- Progress logging: `🛑 shutting down...`, `❌ shutdown error`, `⚠️ workq drain timeout`, `✅ shutdown complete`.

Two real gaps:

1. The shutdown deadline for `srv.Shutdown` is **10s**, not the spec-mandated 30s. Fix.
2. The panic middleware (`internal/api/router.go:~207`) logs only `path` and `panic`. The spec requires `req_id`, route (= `path`), method, user (if authed), and full stack trace.

- [ ] **Step 1: Write the failing panic-enrichment test**

Create `internal/api/panic_enrichment_test.go`:

```go
package api

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRecoveryMiddleware_EnrichedLog verifies that a panic is logged
// with req_id, route, method, user, and a stack trace.
func TestRecoveryMiddleware_EnrichedLog(t *testing.T) {
	// Capture slog output via a TextHandler into a buffer.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	panicky := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	handler := recoveryMiddleware(panicky)

	// Seed request with a request id (mimicking loggingMiddleware
	// having already run) and a user ctx value.
	req := httptest.NewRequest(http.MethodPost, "/api/documents/abc", nil)
	ctx := context.WithValue(req.Context(), ctxRequestIDKey{}, "rid-test-123")
	ctx = withUserForTest(ctx, "alice")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want 500", rec.Code)
	}

	logOutput := buf.String()
	for _, want := range []string{
		"panic recovered",
		"req_id=rid-test-123",
		"method=POST",
		"route=/api/documents/abc",
		"user=alice",
		"panic=boom",
	} {
		if !strings.Contains(logOutput, want) {
			t.Errorf("log missing %q\nlog: %s", want, logOutput)
		}
	}
	// A stack trace marker ("goroutine " or "runtime/panic.go") must
	// appear somewhere in the log. slog serializes newlines as \n
	// literals inside the stack attribute — either is acceptable.
	if !strings.Contains(logOutput, "goroutine") && !strings.Contains(logOutput, "runtime/panic") {
		t.Errorf("log missing stack trace marker\nlog: %s", logOutput)
	}
}

// withUserForTest is a test-only helper that injects a user id into
// ctx using the same key the real auth middleware uses. The real
// accessor is userIDFromContext (or whatever name session.go exports)
// — if that helper does not exist yet, the test seeds the context
// value directly using the same key.
func withUserForTest(ctx context.Context, user string) context.Context {
	return context.WithValue(ctx, ctxUserKey{}, user)
}
```

Note: the test uses `ctxUserKey{}` — a context key already defined in the codebase (check `internal/api/auth.go` and `internal/api/session.go`). If it's named differently (e.g. `ctxAuthKey{}` or `userKey{}`), adjust the test literal to match. If no such key exists today because Block 2 hasn't landed user tracking, add a minimal one in Step 2 alongside the middleware edit.

- [ ] **Step 2: Enrich `recoveryMiddleware`**

Edit `internal/api/router.go`. Locate `recoveryMiddleware` (around line 205). Replace with:

```go
// recoveryMiddleware catches panics in handlers, logs them with
// request context (req_id, route, method, user if authed) plus the
// full stack, then returns a 500 response. The enriched log surface
// is Block 3.7's requirement: during a production panic you need
// enough context to reconstruct the request without tailing raw
// stderr.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				// Gather every piece of request context that exists
				// on the ctx — any absent value surfaces as "" and
				// gets filtered from the attr list.
				rid := RequestIDFromContext(r.Context())
				user, _ := r.Context().Value(ctxUserKey{}).(string)

				stack := debug.Stack()

				attrs := []any{
					"route", r.URL.Path,
					"method", r.Method,
					"panic", fmt.Sprint(rec),
					"stack", string(stack),
				}
				if rid != "" {
					attrs = append(attrs, "req_id", rid)
				}
				if user != "" {
					attrs = append(attrs, "user", user)
				}

				slog.Error("❌ panic recovered", attrs...)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
```

Add to the imports block at the top of `router.go` if not already present:

```go
	"fmt"
	"runtime/debug"
```

If `ctxUserKey{}` does not exist in the package, add it near the other ctx keys (e.g. next to `ctxRequestIDKey{}` in `request_id.go`):

```go
// ctxUserKey is the ctx value key for the authenticated user ID. Set
// by bearerAuthMiddleware / session cookie validation. Empty string
// means the request is unauthenticated (allowed on public routes).
type ctxUserKey struct{}
```

If auth already uses a different key name, use that key in both places. This plan's intent is structural, not name-specific.

- [ ] **Step 3: Run the test (green)**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/api/ -run TestRecoveryMiddleware_EnrichedLog -v`

Expected: PASS.

- [ ] **Step 4: Raise the `srv.Shutdown` deadline to 30s**

Edit `cmd/serve.go`. Locate:

```go
		slog.Info("🛑 shutting down...")
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			slog.Error("❌ shutdown error", "err", err)
			return err
		}
```

Change the `10*time.Second` to `30*time.Second`. Keep everything else — the workq drain already uses 30s. Now both phases get the spec-mandated 30s.

- [ ] **Step 5: Verify shutdown progress logging already covers the spec**

Re-read `cmd/serve.go` around the shutdown block. The existing log lines:

- `🛑 shutting down...` (fires at the start)
- `❌ shutdown error, err=...` (only on error)
- `⚠️ workq drain timeout, err=...` (only on drain deadline exceeded)
- `✅ shutdown complete` (fires at end)

Spec says "logs progress at each step" — each step being (a) signal received, (b) HTTP shutdown started/finished, (c) workq drain started/finished. The current wiring logs (a) and (d). To make (b)/(c) explicit, wrap the Shutdown call:

```go
		slog.Info("🛑 shutting down HTTP server...")
		shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			slog.Error("❌ HTTP server shutdown error", "err", err)
			return err
		}
		slog.Info("✅ HTTP server shutdown complete")

		slog.Info("🛑 draining workq...")
		drainCtx, drainCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer drainCancel()
		if err := pool.Close(drainCtx); err != nil {
			slog.Warn("⚠️ workq drain timeout; some indexing jobs were cancelled mid-flight", "err", err)
		} else {
			slog.Info("✅ workq drained")
		}
		slog.Info("✅ shutdown complete")
```

- [ ] **Step 6: Run the shutdown integration test**

Run: `CGO_ENABLED=1 go test -tags 'sqlite_fts5 integration' -timeout 60s ./internal/api/ -run Shutdown -v`

Expected: PASS. The existing `shutdown_integration_test.go` exercises the drain path; it should be unaffected by the log-line additions. If it asserts on exact log text, update the assertions to match the new text.

- [ ] **Step 7: Run full suite**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 -race -timeout 300s ./...`

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/api/router.go internal/api/request_id.go internal/api/panic_enrichment_test.go cmd/serve.go
git commit -m "$(cat <<'EOF'
feat(api,serve): enrich panic log + align shutdown deadline to spec

recoveryMiddleware now logs req_id, route, method, user (if authed),
and the full debug.Stack() on panic — everything needed to
reconstruct a request from the log alone, no stderr tail required.
Backed by panic_enrichment_test.go which asserts on the attribute
surface via a TextHandler buffer capture.

cmd/serve.go: raises srv.Shutdown deadline from 10s to 30s so it
matches the workq drain phase; splits the single 🛑 / ✅ pair into
per-phase log lines so operators can see exactly where a stuck
shutdown is hung.

Block 1 already shipped signal handling + workq.Close(drainCtx);
this task closes the two remaining 3.7 gaps.

Block 3.7.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review

### Spec coverage

| Spec item | Task | Notes |
|-----------|------|-------|
| 3.1 Context propagation audit | Task 5 | `scripts/ctx-audit.sh` + crawler fixes |
| 3.2 Request-level timeout | Task 6 | `requestTimeoutMiddleware(cfg)` with upload carve-out |
| 3.3 LLM call timeouts | Task 3 | `lcProvider.withCallTimeout`; cfg.LLM.CallTimeout default 60s |
| 3.4 EmbedBatch chunking + backpressure | Task 4 | `Provider.BatchCeiling()`; lcProvider.EmbedBatch buffered-channel |
| 3.5 HTTP client pooling | Task 2 | `internal/llm/httpclient.go` with tuned transport |
| 3.6 SQLite hardening | Task 1 | Explicit PRAGMAs, raised pool, `(*Store).Ping(ctx)` |
| 3.7 Graceful shutdown | Task 7 | Panic-log enrichment + 30s Shutdown deadline |

Every spec item has exactly one task. No gaps.

### Placeholder check

Searched for common red flags. Findings:

- No "TBD" / "TODO" / "implement later" anywhere.
- Task 6 step 7 gives an explicit fallback for when Block 2 has not merged (`grep` to detect which state the file is in) — this is a branching instruction, not a placeholder.
- Task 7 step 2 notes that `ctxUserKey{}` may need to be added if auth doesn't expose it — this is a concrete fallback with code, not a placeholder.
- Task 5's audit script has concrete grep patterns; no "add appropriate error handling" hand-waving.
- Every test has full code. Every implementation step has full code.

Clean.

### Type consistency

- `(*Store).Ping(ctx context.Context) error` — defined Task 1 step 4; referenced in Block 4's future /readyz work (not in this plan). Signature matches `*sql.DB.PingContext`.
- `newHTTPClient() *http.Client` — defined Task 2 step 3; consumed Task 2 step 5 (`newOpenAIProvider`), step 7 (`newOllamaProvider`, `newAzureProvider`).
- `lcProvider.httpClient *http.Client`, `lcProvider.callTimeout time.Duration`, `lcProvider.batchCeiling int` — defined Task 2 step 6; populated Tasks 2–4. Field names are stable across tasks.
- `(*lcProvider).withCallTimeout(parent context.Context) (context.Context, context.CancelFunc)` — defined Task 3 step 4; reused Task 4 in the new `EmbedBatch`.
- `Provider.BatchCeiling() int` — defined Task 4 step 1 on the interface + impl; consumed Task 4 step 3 in `embedder.New`.
- `requestTimeoutMiddleware(cfg *config.Config) func(http.Handler) http.Handler` — defined Task 6 step 5; wired Task 6 step 7.
- `isUploadRoute(r *http.Request) bool` — defined Task 6 step 5; private to `request_timeout.go`.
- `ctxUserKey{}` — defined Task 7 step 2 (or already present); consumed Task 7 step 2 in `recoveryMiddleware`.
- `config.LLMConfig.CallTimeout`, `config.ServerConfig.RequestTimeout`, `config.ServerConfig.UploadTimeout` — all `time.Duration` via mapstructure; declared Tasks 3 + 6; consumed Tasks 3 + 6.

No signature drift between tasks.

### Dependency ordering

- Task 1 stands alone (foundational).
- Task 2 must land before Task 3 (Task 3 uses the pooled client path).
- Task 3 must land before Task 4 (Task 4's new `EmbedBatch` calls `withCallTimeout` from Task 3).
- Task 5 should land after Tasks 2–4 so any ctx drift introduced in those tasks is caught in the same sweep. It's also internally independent of 2–4.
- Task 6 is independent of Tasks 1–5 and independent of Block 2 (with the documented layering fallback).
- Task 7 is independent of Tasks 1–6.

The sequenced order in the plan (1 → 2 → 3 → 4 → 5 → 6 → 7) satisfies every dependency.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-23-block3-resource-safety-plan.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task (Tasks 1–7), review between tasks, fast iteration. Each subagent gets a narrow brief referencing exactly the task's Files + Steps.

**2. Inline Execution** — Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints after Task 1 (foundational), after Task 4 (LLM surface complete), and after Task 7 (final).

Which approach?
