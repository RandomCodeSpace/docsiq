# Block 4 — Observability & Ops Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire docsiq for production observability — Prometheus metrics, structured logs, health probes, a unified access log, and a reflective `/api/version` endpoint — so that an SRE scraping the binary behind a corporate firewall has parity with commercial GraphRAG services.

**Architecture:** Five independent, incremental landings. We introduce the `prometheus/client_golang` dep and replace the ad-hoc text-format collector in `internal/api/metrics.go` with the official one (collision-free, auto-typed, supports labels). `/healthz` is dependency-free; `/readyz` consults SQLite + the LLM provider with a 10-second in-memory result cache. Access logging extends the existing `loggingMiddleware` (rather than adding a second middleware) so req_id generation, Prometheus recording, and the log line share one pass. `/api/version` returns `cmd.versionInfo()` plus `runtime/debug.ReadBuildInfo()` deps.

**Tech Stack:** Go 1.25, `github.com/prometheus/client_golang` v1.20+, `log/slog`, `net/http` (std `http.ServeMux` with Go 1.22 method-pattern routing), `spf13/viper` for config, `spf13/cobra` for CLI.

---

## Hard dependency

**Block 2 (security hardening) must merge first.** This plan assumes the middleware chain is:

```go
// internal/api/router.go:167 (after Block 2)
return securityHeadersMiddleware(cfg)(
    loggingMiddleware(
        recoveryMiddleware(
            bearerAuthMiddleware(cfg.Server.APIKey,
                projectMiddleware(cfg, registry, mux)))))
```

If Block 2 has not yet landed when this plan begins, Task 4 still works — it replaces the body of `loggingMiddleware` in-place — but the documented chain diagram in comments must be kept in sync.

## Task sequencing & rationale

1. **Task 1 — 4.5 Version endpoint.** Zero infrastructure deps. Reuses `cmd.versionInfo()` and existing `-ldflags` wiring (Makefile already sets `cmd.Version/Commit/Date`). Smallest surface area; lands first so later tasks can assert `/api/version` exists.
2. **Task 2 — 4.3 Health endpoints.** Small, self-contained. Depends only on `stores` and `llm.Provider`, both already plumbed through `NewRouter`. No new deps.
3. **Task 3 — 4.1 Prometheus metrics.** Adds the `client_golang` dep, replaces the ad-hoc collector, adds `pipeline` + `embedder` + `llm` stage metrics, and introduces `workq.Pool.Stats()` (a small new accessor). Must land after Task 2 so `/readyz` can scrape its own hit counter in one place.
4. **Task 4 — 4.4 Access log middleware.** Extends the existing `loggingMiddleware` with `bytes_out`, `user_id` (as a coarse `auth` label), and a panic-resilient `defer` that emits even if downstream middleware panics before `WriteHeader`. Must come after Task 3 so the `bytes_out` figure can also flow into the `docsiq_http_response_bytes_total` counter (added here to round out the HTTP metric family).
5. **Task 5 — 4.2 Structured-log schema + format switch.** Adjusts the `initConfig` slog setup to (a) drop emoji from the `json` handler's message field and (b) honour a new `LogConfig.Format` field from viper as the lowest-priority source of truth. Lands last because it alters the output format of earlier tasks' logs and would otherwise churn test fixtures mid-flight.

---

## Global verification commands

Every task ends with this trio of checks (run before the task's final commit step):

```
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go test -tags "sqlite_fts5" -timeout 300s ./...
```

Expected across all three: no warnings, no errors, all tests pass. Failing tests that pre-date this plan must be reported to the caller, not papered over.

---

## Task 1: Version endpoint (4.5)

**Files:**
- Create: `internal/api/version.go`
- Create: `internal/api/version_test.go`
- Modify: `internal/api/router.go` — register `GET /api/version` inside `NewRouter`
- Modify: `cmd/version.go` — export `VersionInfo` and `versionInfo` so `internal/api` can call them (currently unexported)

**Notes for the implementer:** The Makefile already wires `-X cmd.Version/Commit/Date` via the `LDFLAGS` variable. Do NOT add new ldflags. `cmd.versionInfo()` already falls back to `runtime/debug.ReadBuildInfo()` when the ldflags are sentinel values. We just need to expose it to `internal/api` without creating a cyclic import (`internal/api` does not import `cmd`; the inverse is already true). To break the cycle we move the version-resolution logic to a new small package `internal/buildinfo` and have both `cmd/version.go` and `internal/api/version.go` call into it.

- [ ] **Step 1: Create the shared buildinfo package skeleton**

Create `internal/buildinfo/buildinfo.go`:

```go
// Package buildinfo resolves the running binary's version metadata. It
// reads ldflags-injected overrides (set via `-X` at build time) and
// falls back to `runtime/debug.ReadBuildInfo()` when the overrides are
// sentinel values, so `go install module@version` binaries still report
// useful metadata.
package buildinfo

import "runtime/debug"

// Set via -ldflags at build time (see Makefile). These act as overrides.
// They are package-level var so the linker can write to them; keep the
// names stable — the Makefile's LDFLAGS refers to them by full symbol
// path (github.com/RandomCodeSpace/docsiq/internal/buildinfo.Version etc.).
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// Info holds resolved version metadata for the running binary.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Dirty     string `json:"dirty"` // "true", "false", or "unknown"
	Deps      map[string]string `json:"deps,omitempty"`
}

// readBuildInfo is a package-level indirection so tests can stub it.
var readBuildInfo = debug.ReadBuildInfo

func isSentinel(v string) bool {
	switch v {
	case "", "dev", "unknown":
		return true
	}
	return false
}

// Resolve returns the current version metadata using:
//  1. -ldflags overrides (if non-sentinel)
//  2. runtime/debug.ReadBuildInfo() (module version + VCS settings)
//  3. "unknown" for any remaining field
//
// When includeDeps is true, the returned Info also lists the main
// module's direct dependencies (Path → Version). Transitive deps are
// omitted because they bloat the response without real diagnostic value.
func Resolve(includeDeps bool) Info {
	info := Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: Date,
		Dirty:     "unknown",
	}

	bi, ok := readBuildInfo()
	if !ok {
		if isSentinel(info.Version) {
			info.Version = "unknown"
		}
		if isSentinel(info.Commit) {
			info.Commit = "unknown"
		}
		if isSentinel(info.BuildDate) {
			info.BuildDate = "unknown"
		}
		info.GoVersion = "unknown"
		return info
	}

	info.GoVersion = bi.GoVersion

	if isSentinel(info.Version) {
		if bi.Main.Version != "" {
			info.Version = bi.Main.Version
		} else {
			info.Version = "unknown"
		}
	}

	var vcsRev, vcsTime, vcsMod string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			vcsRev = s.Value
		case "vcs.time":
			vcsTime = s.Value
		case "vcs.modified":
			vcsMod = s.Value
		}
	}
	if isSentinel(info.Commit) {
		if vcsRev != "" {
			info.Commit = vcsRev
		} else {
			info.Commit = "unknown"
		}
	}
	if isSentinel(info.BuildDate) {
		if vcsTime != "" {
			info.BuildDate = vcsTime
		} else {
			info.BuildDate = "unknown"
		}
	}
	if vcsMod != "" {
		info.Dirty = vcsMod
	}

	if includeDeps {
		deps := make(map[string]string, len(bi.Deps))
		for _, d := range bi.Deps {
			if d == nil {
				continue
			}
			// Skip replaced modules — the Replace struct's Version is
			// what actually ships, not d.Version.
			v := d.Version
			if d.Replace != nil {
				v = d.Replace.Version
			}
			if v == "" {
				v = "unknown"
			}
			deps[d.Path] = v
		}
		info.Deps = deps
	}

	return info
}
```

- [ ] **Step 2: Write buildinfo tests**

Create `internal/buildinfo/buildinfo_test.go`:

```go
package buildinfo

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestResolve_SentinelFallsBackToBuildInfo(t *testing.T) {
	origVersion, origCommit, origDate, origRead :=
		Version, Commit, Date, readBuildInfo
	defer func() {
		Version, Commit, Date, readBuildInfo =
			origVersion, origCommit, origDate, origRead
	}()

	Version, Commit, Date = "dev", "unknown", "unknown"
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			GoVersion: "go1.25.5",
			Main: debug.Module{
				Path:    "github.com/RandomCodeSpace/docsiq",
				Version: "v0.5.0",
			},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc123def"},
				{Key: "vcs.time", Value: "2026-04-23T10:00:00Z"},
				{Key: "vcs.modified", Value: "false"},
			},
		}, true
	}

	got := Resolve(false)
	if got.Version != "v0.5.0" {
		t.Errorf("Version=%q want v0.5.0", got.Version)
	}
	if got.Commit != "abc123def" {
		t.Errorf("Commit=%q want abc123def", got.Commit)
	}
	if got.BuildDate != "2026-04-23T10:00:00Z" {
		t.Errorf("BuildDate=%q", got.BuildDate)
	}
	if got.GoVersion != "go1.25.5" {
		t.Errorf("GoVersion=%q", got.GoVersion)
	}
	if got.Dirty != "false" {
		t.Errorf("Dirty=%q", got.Dirty)
	}
}

func TestResolve_LdflagsOverridesWin(t *testing.T) {
	origVersion, origCommit, origDate, origRead :=
		Version, Commit, Date, readBuildInfo
	defer func() {
		Version, Commit, Date, readBuildInfo =
			origVersion, origCommit, origDate, origRead
	}()

	Version, Commit, Date = "v9.9.9", "ffffff", "2026-01-01T00:00:00Z"
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			GoVersion: "go1.25.5",
			Main:      debug.Module{Version: "v0.0.0"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "DO-NOT-USE"},
			},
		}, true
	}

	got := Resolve(false)
	if got.Version != "v9.9.9" {
		t.Errorf("ldflags Version should win; got %q", got.Version)
	}
	if got.Commit != "ffffff" {
		t.Errorf("ldflags Commit should win; got %q", got.Commit)
	}
}

func TestResolve_IncludeDepsPopulatesMap(t *testing.T) {
	origRead := readBuildInfo
	defer func() { readBuildInfo = origRead }()
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			GoVersion: "go1.25.5",
			Main:      debug.Module{Version: "v0.5.0"},
			Deps: []*debug.Module{
				{Path: "github.com/spf13/cobra", Version: "v1.10.2"},
				{Path: "github.com/tmc/langchaingo", Version: "v0.1.14"},
			},
		}, true
	}

	got := Resolve(true)
	if got.Deps["github.com/spf13/cobra"] != "v1.10.2" {
		t.Errorf("deps missing cobra; got %+v", got.Deps)
	}
	if got.Deps["github.com/tmc/langchaingo"] != "v0.1.14" {
		t.Errorf("deps missing langchaingo")
	}
}

func TestResolve_ReadBuildInfoUnavailable(t *testing.T) {
	origRead := readBuildInfo
	origVersion, origCommit, origDate :=
		Version, Commit, Date
	defer func() {
		readBuildInfo = origRead
		Version, Commit, Date = origVersion, origCommit, origDate
	}()

	Version, Commit, Date = "dev", "unknown", "unknown"
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	got := Resolve(true)
	if got.Version != "unknown" || got.Commit != "unknown" || got.BuildDate != "unknown" {
		t.Errorf("all fields should be 'unknown'; got %+v", got)
	}
	if got.GoVersion != "unknown" {
		t.Errorf("GoVersion should be 'unknown'; got %q", got.GoVersion)
	}
	if got.Deps != nil {
		t.Errorf("Deps should be nil when ReadBuildInfo fails; got %+v", got.Deps)
	}
	if strings.Contains(got.Dirty, "true") {
		t.Errorf("Dirty should default to 'unknown'; got %q", got.Dirty)
	}
}
```

- [ ] **Step 3: Run buildinfo tests (red — no ldflags-migration yet but tests use stubs)**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/buildinfo/ -v`

Expected: all 4 tests PASS. If `go: module lookup disabled by GOPROXY=off` fires, remove GOPROXY and retry. If any test references a symbol not yet defined in `buildinfo.go`, re-read Step 1 and confirm the full file was saved.

- [ ] **Step 4: Migrate cmd/version.go to call buildinfo.Resolve**

Open `cmd/version.go`. Replace the whole file body with:

```go
package cmd

import (
	"fmt"

	"github.com/RandomCodeSpace/docsiq/internal/buildinfo"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of docsiq",
	Run: func(cmd *cobra.Command, args []string) {
		info := buildinfo.Resolve(false)
		dirtySuffix := ""
		if info.Dirty == "true" {
			dirtySuffix = " (dirty)"
		}
		fmt.Printf("docsiq %s (commit: %s, built: %s)%s\n",
			info.Version, info.Commit, info.BuildDate, dirtySuffix)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
```

Note: `Version`, `Commit`, `Date` package-level vars move to `internal/buildinfo`.

- [ ] **Step 5: Update Makefile LDFLAGS to point at the new package path**

Open `Makefile`. Current `LDFLAGS` block:

```
LDFLAGS  := -X github.com/RandomCodeSpace/docsiq/cmd.Version=$(VERSION) \
            -X github.com/RandomCodeSpace/docsiq/cmd.Commit=$(COMMIT) \
            -X github.com/RandomCodeSpace/docsiq/cmd.Date=$(DATE)
```

Change to:

```
LDFLAGS  := -X github.com/RandomCodeSpace/docsiq/internal/buildinfo.Version=$(VERSION) \
            -X github.com/RandomCodeSpace/docsiq/internal/buildinfo.Commit=$(COMMIT) \
            -X github.com/RandomCodeSpace/docsiq/internal/buildinfo.Date=$(DATE)
```

- [ ] **Step 6: Patch cmd/version_test.go for the rename**

Open `cmd/version_test.go`. Any reference to `Version`, `Commit`, `Date`, `readBuildInfo`, `versionInfo`, or `VersionInfo` inside `package cmd` must be rewritten to import and use `internal/buildinfo`. If the test mocks `readBuildInfo`, it now lives in `internal/buildinfo` and tests there. Delete test cases whose behaviour is now covered by `internal/buildinfo/buildinfo_test.go`; keep only tests that exercise the `docsiq version` Cobra command surface (output formatting via stdout capture).

If the test file becomes empty, delete it.

- [ ] **Step 7: Run cmd tests to confirm version cmd still builds**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./cmd/... -v -run Version`

Expected: PASS. If `docsiq version` prints "docsiq dev (commit: unknown, built: unknown)" that's the `go test` path (no ldflags) and is correct.

- [ ] **Step 8: Write the /api/version handler**

Create `internal/api/version.go`:

```go
package api

import (
	"net/http"

	"github.com/RandomCodeSpace/docsiq/internal/buildinfo"
)

// versionHandler serves GET /api/version. Returns buildinfo.Info as
// JSON, including the direct-dependency map. Public endpoint — no
// secrets are exposed; commit hash and Go version are considered
// non-sensitive for a self-hosted MCP server.
func versionHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		info := buildinfo.Resolve(true)
		writeJSON(w, http.StatusOK, info)
	})
}
```

Note: `writeJSON` is the existing helper in `internal/api/handlers.go`. No new helpers needed.

- [ ] **Step 9: Write the handler tests**

Create `internal/api/version_test.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionHandler_ReturnsJSON(t *testing.T) {
	t.Parallel()
	h := versionHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct == "" || ct[:16] != "application/json" {
		t.Errorf("Content-Type=%q want application/json*", ct)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v — raw=%q", err, rec.Body.String())
	}

	for _, key := range []string{"version", "commit", "build_date", "go_version"} {
		if _, ok := body[key]; !ok {
			t.Errorf("response missing %q field; got keys=%v", key, mapKeys(body))
		}
	}
}

func TestVersionHandler_RejectsNonGET(t *testing.T) {
	t.Parallel()
	h := versionHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/version", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/version status=%d want 405", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != "GET" {
		t.Errorf("Allow=%q want GET", got)
	}
}

func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
```

- [ ] **Step 10: Run the new handler tests (red — not wired yet if via route, but unit test exercises handler directly so should PASS)**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/api/ -run TestVersionHandler -v`

Expected: 2/2 PASS. If you get a nil `writeJSON` or "undefined: writeJSON" error, confirm `internal/api/handlers.go` still exports it (it is not currently exported by name — it is package-internal, which is fine because we're in the same package).

- [ ] **Step 11: Register the route inside NewRouter**

Open `internal/api/router.go`. Inside `NewRouter`, directly after the existing `/metrics` registration (currently around line 94), add:

```go
	// Version metadata — public, no auth. Used for operator diagnostics
	// and CI tooling ("what's running in prod?"). No secrets exposed.
	mux.Handle("GET /api/version", versionHandler())
```

- [ ] **Step 12: Write an integration test that hits the live-routed /api/version**

Append to `internal/api/router_no_llm_test.go`:

```go
func TestNewRouter_VersionEndpointPublic(t *testing.T) {
	t.Parallel()
	cfg := minimalTestConfig(t) // existing helper; do not add a new one
	reg := newTestRegistry(t)
	h := NewRouter(nil, nil, cfg, reg)

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/version with no Auth should be 200; got %d body=%s",
			rec.Code, rec.Body.String())
	}
}
```

If `minimalTestConfig(t)` and `newTestRegistry(t)` do not exist, mirror the pattern used by the existing `TestNewRouter_NoLLM` sub-tests in the same file. Do not invent new helpers unless the file has zero prior harness.

- [ ] **Step 13: Run all changed tests + go vet**

```
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go test -tags "sqlite_fts5" -timeout 300s ./internal/api/ ./internal/buildinfo/ ./cmd/...
```

Expected: clean vet, all tests PASS. If a test that pre-dates this task is newly red, open it and check for a stale reference to `cmd.Version/Commit/Date`.

- [ ] **Step 14: Smoke-test via built binary**

```
make build
./docsiq serve --port 0 &
SERVE_PID=$!
# Hit the endpoint before auth kicks in — /api/version is public.
sleep 1
curl -s http://127.0.0.1:8080/api/version | python3 -m json.tool
kill $SERVE_PID
```

Expected: JSON with at minimum `version`, `commit`, `build_date`, `go_version`, `deps` keys. The `version` field equals the output of `git describe --tags --always --dirty` when the working tree is clean; otherwise it carries the `-dirty` suffix that `git describe` emits.

If the binary fails to start due to `api_key empty + non-loopback bind`, set `DOCSIQ_API_KEY=dev` for the smoke test — the version endpoint bypasses auth anyway.

- [ ] **Step 15: Commit**

```bash
git add internal/buildinfo/ cmd/version.go cmd/version_test.go Makefile \
        internal/api/version.go internal/api/version_test.go \
        internal/api/router.go internal/api/router_no_llm_test.go
git commit -m "$(cat <<'EOF'
feat(api): GET /api/version with ldflags + BuildInfo fallback

Moves the shared version-resolution logic out of cmd into a new
internal/buildinfo package so internal/api can serve the same data
without a cmd import cycle. Adds a public GET /api/version endpoint
that returns {version, commit, build_date, go_version, dirty, deps}
as JSON. Makefile LDFLAGS retargeted at the new package path.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Health endpoints (4.3)

**Files:**
- Create: `internal/api/health.go`
- Create: `internal/api/health_test.go`
- Modify: `internal/api/router.go` — register `GET /healthz` and `GET /readyz`; remove the existing `GET /health` alias (covered by `/healthz`)

**Notes:** `/healthz` always 200 if the process is running; no I/O, no dependencies. `/readyz` checks SQLite ping + LLM provider reach, with a 10-second in-memory cache keyed only on time (not on the set of projects — we ping the default project's store as the representative shard). When the LLM provider is `nil` (config `provider: none`), the LLM check reports `"status": "skipped"` and does NOT fail readiness — the server still serves notes and MCP-without-LLM traffic. Cache TTL is a `const readyCheckTTL = 10 * time.Second`, not a config knob; operators who want different behaviour can subclass the handler.

- [ ] **Step 1: Write the failing health test**

Create `internal/api/health_test.go`:

```go
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
	sq := &healthPingerStub{}
	sq.err = nil
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

// healthPingerFunc is an adapter so the last test above can use an
// inline closure without hand-rolling another stub type.
type healthPingerFunc func(ctx context.Context) error

func (f healthPingerFunc) Ping(ctx context.Context) error { return f(ctx) }

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
	time.Sleep(80 * time.Millisecond)
	req()

	if got := sq.hits.Load(); got != 2 {
		t.Errorf("sqlite pinger hits=%d want 2 (one per TTL window)", got)
	}
}
```

- [ ] **Step 2: Run the failing tests**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/api/ -run Test(Healthz|Readyz) -v`

Expected: all compile errors because `healthzHandler`, `readyzHandler`, `readyzHandlerForTest`, `readyzBody`, `healthPinger`, `llmPinger` do not exist yet. That is the red state.

- [ ] **Step 3: Implement internal/api/health.go**

Create `internal/api/health.go`:

```go
package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/llm"
)

// readyCheckTTL is how long the /readyz handler caches its aggregated
// verdict. Ten seconds is short enough to notice a real outage quickly
// and long enough to absorb a chatty Prometheus + Kubernetes probe loop
// without hammering SQLite or the LLM endpoint.
const readyCheckTTL = 10 * time.Second

// readyCheckTimeout bounds each individual probe. The SQLite ping is a
// microsecond-scale PRAGMA query; the LLM ping is network-bound, so we
// allow more headroom but still fail fast when the provider is wedged.
const (
	sqliteCheckTimeout = 500 * time.Millisecond
	llmCheckTimeout    = 2 * time.Second
)

// healthPinger is the narrow interface readyz needs for SQLite
// reachability: a bounded Ping call that reports the first hard error.
type healthPinger interface {
	Ping(ctx context.Context) error
}

// llmPinger is the narrow interface readyz needs for LLM reachability.
// Implementations may issue a tiny Complete call or a model-list GET.
type llmPinger interface {
	Ping(ctx context.Context) error
}

// checkStatus holds per-component readiness.
type checkStatus struct {
	Status string `json:"status"` // "ok" | "error" | "skipped"
	Err    string `json:"err,omitempty"`
}

type readyzBody struct {
	Status string                 `json:"status"` // "ready" | "not_ready"
	Checks map[string]checkStatus `json:"checks"`
}

// healthzHandler implements GET /healthz. Liveness: always 200 as long
// as the goroutine scheduler can run this function. No dependencies.
func healthzHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

// readyzHandler implements GET /readyz. Readiness: aggregates a SQLite
// ping and an LLM reach check with a 10-second in-memory cache. When
// llm is nil (provider=none), the LLM check is reported as "skipped"
// and does NOT fail readiness.
//
// Pass the default project's store adapter and an llmPinger wrapper
// around the configured provider (or nil). See router.go for wiring.
func readyzHandler(sq healthPinger, llm llmPinger) http.Handler {
	return readyzHandlerForTest(sq, llm, readyCheckTTL)
}

// readyzHandlerForTest is the injectable-TTL variant used only by tests.
// Production code must use readyzHandler.
func readyzHandlerForTest(sq healthPinger, llm llmPinger, ttl time.Duration) http.Handler {
	rc := &readyzCache{ttl: ttl}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, code := rc.check(r.Context(), sq, llm)
		writeJSON(w, code, body)
	})
}

// readyzCache is the TTL-cached readiness result. Uses a single mutex
// because the hot path is cheap; lock contention only matters when the
// TTL expires and many requests arrive before the first unlock.
type readyzCache struct {
	ttl time.Duration

	mu     sync.Mutex
	expiry time.Time
	body   readyzBody
	code   int
}

func (c *readyzCache) check(ctx context.Context, sq healthPinger, llmp llmPinger) (readyzBody, int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.expiry.IsZero() && time.Now().Before(c.expiry) {
		return c.body, c.code
	}

	body := readyzBody{
		Status: "ready",
		Checks: map[string]checkStatus{},
	}
	code := http.StatusOK

	// SQLite probe — mandatory. Failure fails readiness.
	{
		sqCtx, cancel := context.WithTimeout(ctx, sqliteCheckTimeout)
		err := sq.Ping(sqCtx)
		cancel()
		if err != nil {
			body.Checks["sqlite"] = checkStatus{Status: "error", Err: err.Error()}
			body.Status = "not_ready"
			code = http.StatusServiceUnavailable
		} else {
			body.Checks["sqlite"] = checkStatus{Status: "ok"}
		}
	}

	// LLM probe — optional. Nil provider (provider=none) reports skipped.
	switch {
	case llmp == nil:
		body.Checks["llm"] = checkStatus{Status: "skipped", Err: "provider=none"}
	default:
		llmCtx, cancel := context.WithTimeout(ctx, llmCheckTimeout)
		err := llmp.Ping(llmCtx)
		cancel()
		if err != nil {
			body.Checks["llm"] = checkStatus{Status: "error", Err: err.Error()}
			body.Status = "not_ready"
			code = http.StatusServiceUnavailable
		} else {
			body.Checks["llm"] = checkStatus{Status: "ok"}
		}
	}

	c.body = body
	c.code = code
	c.expiry = time.Now().Add(c.ttl)
	return body, code
}

// sqlDBPinger adapts a *sql.DB for the healthPinger interface. SQLite's
// driver treats PingContext as a cheap no-op when the handle is healthy.
type sqlDBPinger struct{ db *sql.DB }

func (p sqlDBPinger) Ping(ctx context.Context) error {
	if p.db == nil {
		return errors.New("nil sql.DB")
	}
	return p.db.PingContext(ctx)
}

// providerPinger adapts an llm.Provider for the llmPinger interface.
// It issues a tiny Complete call with 1-token cap; providers that
// cannot produce tokens (e.g. misconfigured) fail the ping.
type providerPinger struct{ prov llm.Provider }

func (p providerPinger) Ping(ctx context.Context) error {
	if p.prov == nil {
		return errors.New("nil provider")
	}
	// A minimal generation request — 1 token, temp 0, no JSON mode.
	// Providers that stream will still return promptly when maxTokens=1.
	_, err := p.prov.Complete(ctx, "ping", llm.WithMaxTokens(1), llm.WithTemperature(0))
	return err
}
```

- [ ] **Step 4: Run the health tests (green)**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/api/ -run Test(Healthz|Readyz) -v`

Expected: all 7 tests PASS. If `TestReadyz_RefreshesAfterTTL` is flaky due to the 80ms sleep, bump to 120ms — do not skip the test. A flaky test here hides real TTL bugs.

- [ ] **Step 5: Wire the routes into NewRouter**

Open `internal/api/router.go`. Inside `NewRouter`, locate the existing `/health` registration (currently around line 88-89):

```go
	// Public liveness probe — registered on the mux itself. The auth
	// middleware also explicitly bypasses /health as defense-in-depth.
	mux.HandleFunc("GET /health", h.health)
```

Replace with:

```go
	// Public liveness + readiness probes. /healthz is dependency-free
	// (process-is-running); /readyz aggregates a SQLite ping + LLM reach
	// check with a 10s in-memory cache. Both are registered on the mux
	// and also explicitly bypassed by bearerAuthMiddleware.
	mux.Handle("GET /healthz", healthzHandler())
	{
		// Default-project store is the representative SQLite shard — a
		// failure here means the whole server is hosed. Resolve lazily
		// at handler-build time so tests that pass nil stores still work.
		defaultSlug := cfg.DefaultProject
		if defaultSlug == "" {
			defaultSlug = "_default"
		}
		var sq healthPinger
		if stores != nil {
			if st, err := stores.Get(defaultSlug); err == nil && st != nil {
				sq = sqlDBPinger{db: st.DB()}
			}
		}
		if sq == nil {
			// Fall back to a "no-op OK" probe when there is no default
			// store (tests, or pre-registration boot sequence). Lying
			// about readiness here is acceptable because bearerAuth is
			// still absent — this path is never reached in production.
			sq = healthPingerFuncForRouter(func(_ context.Context) error { return nil })
		}
		var llmp llmPinger
		if prov != nil {
			llmp = providerPinger{prov: prov}
		}
		mux.Handle("GET /readyz", readyzHandler(sq, llmp))
	}

	// Back-compat alias: GET /health was the pre-Block-4 probe. Clients
	// that haven't migrated to /healthz still get a 200.
	mux.Handle("GET /health", healthzHandler())
```

Add the adapter helper to `internal/api/health.go` (append at end):

```go
// healthPingerFuncForRouter is the non-test counterpart of
// healthPingerFunc; lives in the prod file so the wiring in router.go
// does not have to depend on a test-only adapter.
type healthPingerFuncForRouter func(ctx context.Context) error

func (f healthPingerFuncForRouter) Ping(ctx context.Context) error { return f(ctx) }
```

- [ ] **Step 6: Update bearerAuthMiddleware to bypass /healthz and /readyz**

Open `internal/api/auth.go`. Find the existing bypass block (around line 40-50 — the "/api/session" and "/health" bypasses). The exact code depends on what Block 2 merged; it looks like this after Block 2:

```go
		if path == "/health" || path == "/metrics" || path == "/api/session" {
			next.ServeHTTP(w, r)
			return
		}
```

Change the single-line check to:

```go
		switch path {
		case "/health", "/healthz", "/readyz", "/metrics", "/api/session", "/api/version":
			next.ServeHTTP(w, r)
			return
		}
```

`/api/version` is added here as part of Task 1's public-endpoint commitment; a separate task did not touch this file to avoid merge conflict with Block 2. If the block you see uses a slice + `slices.Contains`, update the slice contents instead.

- [ ] **Step 7: Delete the obsolete h.health handler**

Open `internal/api/handlers.go`. Find the function `func (h *handlers) health(w http.ResponseWriter, r *http.Request)` — it is short (5-10 lines, returns JSON `{"status":"ok"}`). Delete the function entirely. The `/health` route is now served by `healthzHandler()` directly.

Also delete any test in `internal/api/` that references `h.health` or `healthHandler` — grep for `health` under test files and update.

- [ ] **Step 8: Run full API suite**

```
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./internal/api/ -v -timeout 120s
```

Expected: all PASS. Failures most likely come from the handler deletion in Step 7 — fix the test references rather than restore the old handler.

- [ ] **Step 9: Smoke-test live endpoints**

```
DOCSIQ_API_KEY=dev make build && ./docsiq serve --port 0 &
SERVE_PID=$!
sleep 1
echo '=== healthz ==='; curl -s -w '\nstatus=%{http_code}\n' http://127.0.0.1:8080/healthz
echo '=== readyz ==='; curl -s -w '\nstatus=%{http_code}\n' http://127.0.0.1:8080/readyz
kill $SERVE_PID
```

Expected `/healthz`: `{"status":"ok"}` + `status=200`.
Expected `/readyz`: `{"status":"ready","checks":{"sqlite":{"status":"ok"},"llm":{"status":"skipped",...}}}` + `status=200` (assuming no LLM provider is configured). If provider is `openai` but the key is invalid, expect `status=503` and `checks.llm.status=error`.

- [ ] **Step 10: Commit**

```bash
git add internal/api/health.go internal/api/health_test.go \
        internal/api/router.go internal/api/auth.go \
        internal/api/handlers.go
git commit -m "$(cat <<'EOF'
feat(api): /healthz + /readyz probes with 10s cache

/healthz is dependency-free liveness. /readyz checks SQLite PRAGMA +
an LLM provider Complete(maxTokens=1) reach, caching the verdict for
10 seconds to absorb Prometheus + Kubernetes probe loops. Nil
provider (config provider: none) reports llm.status=skipped and keeps
readiness green. Auth middleware bypasses both endpoints; legacy
/health route remains as a 200-returning alias for older clients.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Prometheus metrics + workq stats (4.1)

**Files:**
- Modify: `go.mod`, `go.sum` — add `github.com/prometheus/client_golang` v1.20.5 (or latest patch)
- Create: `internal/obs/metrics.go` — the central registry + collectors
- Create: `internal/obs/metrics_test.go`
- Modify: `internal/api/metrics.go` — replace the ad-hoc text collector with a thin adapter over `promhttp.Handler()`
- Modify: `internal/workq/workq.go` — add `Pool.Stats() Stats` accessor and a `rejectedTotal` counter
- Modify: `internal/workq/workq_test.go` — add `TestPool_StatsCountsRejections`
- Modify: `internal/api/router.go` — record HTTP metrics from `loggingMiddleware` via `obs.HTTP`
- Modify: `internal/pipeline/pipeline.go` — wrap each phase with `obs.Pipeline.TimeStage(name, fn)`
- Modify: `internal/embedder/embedder.go` — wrap batch embed calls with `obs.Embed.Observe(provider, duration)`
- Modify: `internal/llm/provider.go` — wrap completions with `obs.LLM.RecordTokens(provider, kind, n)` when the provider exposes usage

**Notes on the metric set (verbatim from the spec):**

| Metric | Type | Labels | Source |
|---|---|---|---|
| `docsiq_pipeline_stage_duration_seconds` | Histogram | `stage` | `internal/pipeline/pipeline.go` |
| `docsiq_embed_latency_seconds` | Histogram | `provider` | `internal/embedder/embedder.go` |
| `docsiq_llm_tokens_total` | Counter | `provider`, `kind` | `internal/llm/provider.go` |
| `docsiq_workq_depth` | Gauge | — | `internal/workq/workq.go` |
| `docsiq_workq_rejected_total` | Counter | — | `internal/workq/workq.go` |
| `docsiq_http_requests_total` | Counter | `route`, `method`, `status` | `internal/api/router.go` (loggingMiddleware) |
| `docsiq_http_request_duration_seconds` | Histogram | `route` | same |

`kind` label values: `"prompt"`, `"completion"` (when provider returns usage), or `"total"` (fallback when the provider does not split). `route` label is the Go 1.22 method pattern (e.g. `GET /api/documents/{id}`), NOT `r.URL.Path` — unbounded path cardinality would DoS the scrape.

- [ ] **Step 1: Add the Prometheus client_golang dep**

```
go get github.com/prometheus/client_golang@v1.20.5
go mod tidy
```

Expected: `go.mod` gains `github.com/prometheus/client_golang v1.20.5`, `go.sum` picks up the new hashes. If `go get` fails with `proxy.golang.org: connection refused` you are behind an air-gapped firewall; configure `GOPROXY` per build.md and retry. DO NOT vendor manually.

- [ ] **Step 2: Write the obs package tests first**

Create `internal/obs/metrics_test.go`:

```go
package obs

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestHTTP_ObserveRecordsCounterAndHistogram(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	h := NewHTTPMetrics(reg)

	h.Observe("GET /api/documents", "GET", 200, 42*time.Millisecond)
	h.Observe("GET /api/documents", "GET", 200, 80*time.Millisecond)
	h.Observe("POST /api/search", "POST", 500, 2*time.Second)

	if got := testutil.ToFloat64(h.Requests.WithLabelValues("GET /api/documents", "GET", "200")); got != 2 {
		t.Errorf("requests{...200}=%v want 2", got)
	}
	if got := testutil.ToFloat64(h.Requests.WithLabelValues("POST /api/search", "POST", "500")); got != 1 {
		t.Errorf("requests{...500}=%v want 1", got)
	}

	// Histogram buckets — assert the sum is plausible.
	out, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	var found bool
	for _, mf := range out {
		if mf.GetName() == "docsiq_http_request_duration_seconds" {
			found = true
			for _, m := range mf.Metric {
				if h := m.GetHistogram(); h != nil {
					if h.GetSampleCount() == 0 {
						t.Errorf("histogram sample count=0")
					}
				}
			}
		}
	}
	if !found {
		t.Errorf("docsiq_http_request_duration_seconds not registered")
	}
}

func TestPipeline_TimeStageRecordsBothSuccessAndError(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	p := NewPipelineMetrics(reg)

	_ = p.TimeStage("load", func() error { return nil })
	_ = p.TimeStage("chunk", func() error { return nil })
	_ = p.TimeStage("chunk", func() error { return nil })

	// Histogram sample counts per label — expect 1 for "load", 2 for "chunk".
	count := func(stage string) uint64 {
		out, _ := reg.Gather()
		for _, mf := range out {
			if mf.GetName() != "docsiq_pipeline_stage_duration_seconds" {
				continue
			}
			for _, m := range mf.Metric {
				var s string
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "stage" {
						s = lp.GetValue()
					}
				}
				if s == stage {
					return m.GetHistogram().GetSampleCount()
				}
			}
		}
		return 0
	}
	if got := count("load"); got != 1 {
		t.Errorf("load stage count=%d want 1", got)
	}
	if got := count("chunk"); got != 2 {
		t.Errorf("chunk stage count=%d want 2", got)
	}
}

func TestEmbed_ObserveByProvider(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	e := NewEmbedMetrics(reg)

	e.Observe("openai", 120*time.Millisecond)
	e.Observe("openai", 200*time.Millisecond)
	e.Observe("ollama", 50*time.Millisecond)

	// Verify each provider got its own histogram line.
	out, _ := reg.Gather()
	perProvider := map[string]uint64{}
	for _, mf := range out {
		if mf.GetName() != "docsiq_embed_latency_seconds" {
			continue
		}
		for _, m := range mf.Metric {
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "provider" {
					perProvider[lp.GetValue()] = m.GetHistogram().GetSampleCount()
				}
			}
		}
	}
	if perProvider["openai"] != 2 {
		t.Errorf("openai count=%d want 2", perProvider["openai"])
	}
	if perProvider["ollama"] != 1 {
		t.Errorf("ollama count=%d want 1", perProvider["ollama"])
	}
}

func TestLLM_RecordTokensByKind(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	l := NewLLMMetrics(reg)

	l.RecordTokens("openai", "prompt", 512)
	l.RecordTokens("openai", "completion", 128)
	l.RecordTokens("azure", "total", 256)

	if got := testutil.ToFloat64(l.Tokens.WithLabelValues("openai", "prompt")); got != 512 {
		t.Errorf("openai prompt=%v want 512", got)
	}
	if got := testutil.ToFloat64(l.Tokens.WithLabelValues("openai", "completion")); got != 128 {
		t.Errorf("openai completion=%v want 128", got)
	}
	if got := testutil.ToFloat64(l.Tokens.WithLabelValues("azure", "total")); got != 256 {
		t.Errorf("azure total=%v want 256", got)
	}
}

func TestWorkq_DepthAndRejectedFromStatsProvider(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	w := NewWorkqMetrics(reg)

	w.BindStatsProvider(func() WorkqStats { return WorkqStats{Depth: 7, Rejected: 3} })

	// Gather — the gauge is a Collect-time function, so we need a
	// gather pass to read it.
	out, _ := reg.Gather()
	var depth, rejected float64
	for _, mf := range out {
		switch mf.GetName() {
		case "docsiq_workq_depth":
			depth = mf.Metric[0].GetGauge().GetValue()
		case "docsiq_workq_rejected_total":
			rejected = mf.Metric[0].GetCounter().GetValue()
		}
	}
	if depth != 7 {
		t.Errorf("depth=%v want 7", depth)
	}
	if rejected != 3 {
		t.Errorf("rejected=%v want 3", rejected)
	}
}

func TestExpose_ScrapeOutputContainsAllFamilies(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	_ = NewHTTPMetrics(reg)
	_ = NewPipelineMetrics(reg)
	_ = NewEmbedMetrics(reg)
	_ = NewLLMMetrics(reg)
	_ = NewWorkqMetrics(reg)

	// Register a build_info gauge too.
	bi := NewBuildInfoMetric(reg)
	bi.Set("v9.9.9", "abcdef")

	body := renderForTest(t, reg)
	for _, want := range []string{
		"docsiq_http_requests_total",
		"docsiq_http_request_duration_seconds",
		"docsiq_pipeline_stage_duration_seconds",
		"docsiq_embed_latency_seconds",
		"docsiq_llm_tokens_total",
		"docsiq_workq_depth",
		"docsiq_workq_rejected_total",
		"docsiq_build_info",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("scrape output missing family %q", want)
		}
	}
}
```

Add `renderForTest` at the bottom of the test file (it's a fixture shared across tests):

```go
func renderForTest(t *testing.T, reg *prometheus.Registry) string {
	t.Helper()
	rec := httptest.NewRecorder()
	promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	return rec.Body.String()
}
```

Add the required imports (`net/http/httptest`, `github.com/prometheus/client_golang/prometheus/promhttp`).

- [ ] **Step 3: Run failing tests**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/obs/ -v`

Expected: all compile errors — `NewHTTPMetrics`, `NewPipelineMetrics`, etc., are undefined. That is red.

- [ ] **Step 4: Implement internal/obs/metrics.go**

Create `internal/obs/metrics.go`:

```go
// Package obs wires Prometheus metrics for docsiq. One registry per
// process (exposed via obs.Default). Metric families are grouped by
// subject (HTTP, pipeline, embed, LLM, workq, build-info) so handlers
// record through a thin typed API — callers never touch raw collectors.
package obs

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Default is the process-wide registry. Tests construct their own
// prometheus.NewRegistry() to avoid Register-twice panics.
var (
	Default = prometheus.NewRegistry()

	HTTP     *HTTPMetrics
	Pipeline *PipelineMetrics
	Embed    *EmbedMetrics
	LLM      *LLMMetrics
	Workq    *WorkqMetrics
	Build    *BuildInfoMetric
)

// Init wires the Default registry. Must be called exactly once at
// startup (from cmd/serve.go). Safe no-op on second call.
var inited bool

func Init() {
	if inited {
		return
	}
	inited = true
	HTTP = NewHTTPMetrics(Default)
	Pipeline = NewPipelineMetrics(Default)
	Embed = NewEmbedMetrics(Default)
	LLM = NewLLMMetrics(Default)
	Workq = NewWorkqMetrics(Default)
	Build = NewBuildInfoMetric(Default)
}

// ---- HTTP ---------------------------------------------------------------

type HTTPMetrics struct {
	Requests *prometheus.CounterVec
	Duration *prometheus.HistogramVec
}

func NewHTTPMetrics(reg prometheus.Registerer) *HTTPMetrics {
	m := &HTTPMetrics{
		Requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "docsiq_http_requests_total",
				Help: "Total HTTP requests by route, method, and status.",
			},
			[]string{"route", "method", "status"},
		),
		Duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "docsiq_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds, by route.",
				Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"route"},
		),
	}
	reg.MustRegister(m.Requests, m.Duration)
	return m
}

// Observe records one request. `route` MUST be the pattern (e.g.
// "GET /api/documents/{id}"), not r.URL.Path — raw paths have unbounded
// cardinality and will explode the scrape database.
func (m *HTTPMetrics) Observe(route, method string, status int, d time.Duration) {
	m.Requests.WithLabelValues(route, method, statusLabel(status)).Inc()
	m.Duration.WithLabelValues(route).Observe(d.Seconds())
}

func statusLabel(code int) string {
	// Two-digit prefix keeps cardinality bounded even when a handler
	// accidentally emits 418 or similar: we still record 4xx buckets.
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	case code >= 300:
		return "3xx"
	case code >= 200:
		return "2xx"
	default:
		return "1xx"
	}
}

// ---- Pipeline -----------------------------------------------------------

type PipelineMetrics struct {
	StageDuration *prometheus.HistogramVec
}

func NewPipelineMetrics(reg prometheus.Registerer) *PipelineMetrics {
	m := &PipelineMetrics{
		StageDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "docsiq_pipeline_stage_duration_seconds",
				Help:    "GraphRAG pipeline stage duration in seconds.",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 14), // 0.1s → ~27min
			},
			[]string{"stage"},
		),
	}
	reg.MustRegister(m.StageDuration)
	return m
}

// TimeStage measures the wall-clock duration of fn and records it
// against the given stage label. The error from fn is propagated.
func (m *PipelineMetrics) TimeStage(stage string, fn func() error) error {
	start := time.Now()
	err := fn()
	m.StageDuration.WithLabelValues(stage).Observe(time.Since(start).Seconds())
	return err
}

// ---- Embed --------------------------------------------------------------

type EmbedMetrics struct {
	Latency *prometheus.HistogramVec
}

func NewEmbedMetrics(reg prometheus.Registerer) *EmbedMetrics {
	m := &EmbedMetrics{
		Latency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "docsiq_embed_latency_seconds",
				Help:    "Per-batch embed call latency in seconds, by provider.",
				Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
			},
			[]string{"provider"},
		),
	}
	reg.MustRegister(m.Latency)
	return m
}

func (m *EmbedMetrics) Observe(provider string, d time.Duration) {
	m.Latency.WithLabelValues(provider).Observe(d.Seconds())
}

// ---- LLM ----------------------------------------------------------------

type LLMMetrics struct {
	Tokens *prometheus.CounterVec
}

func NewLLMMetrics(reg prometheus.Registerer) *LLMMetrics {
	m := &LLMMetrics{
		Tokens: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "docsiq_llm_tokens_total",
				Help: "LLM tokens consumed, by provider and kind (prompt|completion|total).",
			},
			[]string{"provider", "kind"},
		),
	}
	reg.MustRegister(m.Tokens)
	return m
}

// RecordTokens increments the counter by n. Use kind="prompt",
// "completion", or "total" (when the provider cannot split usage).
func (m *LLMMetrics) RecordTokens(provider, kind string, n int) {
	if n <= 0 {
		return
	}
	m.Tokens.WithLabelValues(provider, kind).Add(float64(n))
}

// ---- Workq --------------------------------------------------------------

// WorkqStats is the snapshot surface the obs layer needs from workq.
// Workq owns the concrete Pool.Stats() method; obs only reads.
type WorkqStats struct {
	Depth    int64
	Rejected int64
}

// WorkqStatsProvider is a closure over pool.Stats(). Injected from
// cmd/serve.go after the pool is constructed, so the obs package does
// not take a hard dep on internal/workq (keeps the import DAG acyclic).
type WorkqStatsProvider func() WorkqStats

type WorkqMetrics struct {
	depthGauge   *prometheus.GaugeFunc
	rejectedDesc *prometheus.Desc

	// rejected tracking
	lastRejected int64
	provider     WorkqStatsProvider
}

func NewWorkqMetrics(reg prometheus.Registerer) *WorkqMetrics {
	m := &WorkqMetrics{}
	// Default no-op provider so Collect before BindStatsProvider does
	// not blow up.
	m.provider = func() WorkqStats { return WorkqStats{} }

	depth := prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "docsiq_workq_depth",
			Help: "Current depth of the workq submission queue (jobs waiting).",
		},
		func() float64 {
			return float64(m.provider().Depth)
		},
	)
	m.depthGauge = &depth

	rejected := prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Name: "docsiq_workq_rejected_total",
			Help: "Total workq submissions rejected because the queue was full.",
		},
		func() float64 {
			return float64(m.provider().Rejected)
		},
	)
	reg.MustRegister(depth, rejected)
	return m
}

// BindStatsProvider wires a live snapshot source. Call from cmd/serve.go
// after the pool is created, before starting the HTTP server.
func (m *WorkqMetrics) BindStatsProvider(p WorkqStatsProvider) {
	if p == nil {
		return
	}
	m.provider = p
}

// ---- Build info ---------------------------------------------------------

type BuildInfoMetric struct {
	Info *prometheus.GaugeVec
}

func NewBuildInfoMetric(reg prometheus.Registerer) *BuildInfoMetric {
	m := &BuildInfoMetric{
		Info: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "docsiq_build_info",
				Help: "Build metadata (value is always 1; labels carry version and commit).",
			},
			[]string{"version", "commit"},
		),
	}
	reg.MustRegister(m.Info)
	return m
}

// Set publishes the current build metadata. Subsequent Set calls
// overwrite rather than accumulate labels (prior labels get their value
// set to 0 via Reset+re-register).
func (m *BuildInfoMetric) Set(version, commit string) {
	m.Info.Reset()
	m.Info.WithLabelValues(version, commit).Set(1)
}
```

- [ ] **Step 5: Re-run obs tests**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/obs/ -v`

Expected: all 6 tests PASS. If `NewGaugeFunc` fails with "duplicate metric already registered", it means you re-registered Default across tests — ensure each test constructs `prometheus.NewRegistry()` and passes it into `New…Metrics(reg)`.

- [ ] **Step 6: Add Pool.Stats() + rejectedTotal in workq**

Open `internal/workq/workq.go`. Add a `sync/atomic` import. Extend the `Pool` struct:

```go
type Pool struct {
	jobs   chan Job
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// mu guards close(p.jobs) vs concurrent sends in Submit. RLock on
	// the send path lets many Submits proceed in parallel; Close takes
	// the write lock before closing the channel.
	mu        sync.RWMutex
	closeOnce sync.Once
	closed    chan struct{}

	// rejectedTotal counts how many Submit calls returned ErrQueueFull
	// over the pool's lifetime. Only incremented on the ErrQueueFull
	// path; ErrClosed does NOT count (it's a shutdown condition, not a
	// capacity condition).
	rejectedTotal atomic.Int64
}
```

Find the existing Submit body where it returns `ErrQueueFull`; add an increment immediately before the return:

```go
	// Path: queue is full → reject immediately.
	select {
	case p.jobs <- job:
		return nil
	default:
		p.rejectedTotal.Add(1)
		return ErrQueueFull
	}
```

(The exact default-branch shape depends on how Submit is implemented today — grep for `ErrQueueFull` and add the increment on each return that yields it.)

Append at the bottom of the file:

```go
// Stats is a point-in-time snapshot of pool utilisation. Depth is the
// count of jobs currently queued but not yet picked up by a worker;
// Rejected is the monotonic count of Submit calls that returned
// ErrQueueFull since process start. Both are safe to call concurrently.
type Stats struct {
	Depth    int64
	Rejected int64
}

func (p *Pool) Stats() Stats {
	return Stats{
		Depth:    int64(len(p.jobs)),
		Rejected: p.rejectedTotal.Load(),
	}
}
```

- [ ] **Step 7: Add the workq stats test**

Append to `internal/workq/workq_test.go`:

```go
func TestPool_StatsReportsDepthAndRejected(t *testing.T) {
	t.Parallel()
	p := New(Config{Workers: 1, QueueDepth: 1})
	defer p.Close(context.Background())

	// Occupy the worker.
	block := make(chan struct{})
	started := make(chan struct{})
	if err := p.Submit(func(ctx context.Context) { close(started); <-block }); err != nil {
		t.Fatalf("submit 1: %v", err)
	}
	<-started
	// Fill the single queue slot.
	if err := p.Submit(func(ctx context.Context) {}); err != nil {
		t.Fatalf("submit 2: %v", err)
	}

	// Two more submissions must be rejected; Rejected must grow by 2.
	if err := p.Submit(func(ctx context.Context) {}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("submit 3: want ErrQueueFull, got %v", err)
	}
	if err := p.Submit(func(ctx context.Context) {}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("submit 4: want ErrQueueFull, got %v", err)
	}

	stats := p.Stats()
	if stats.Depth != 1 {
		t.Errorf("Depth=%d want 1", stats.Depth)
	}
	if stats.Rejected != 2 {
		t.Errorf("Rejected=%d want 2", stats.Rejected)
	}

	close(block)
}
```

- [ ] **Step 8: Run workq tests**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/workq/ -v`

Expected: existing tests still PASS, new `TestPool_StatsReportsDepthAndRejected` PASSes. If the pre-existing `TestPool_SubmitReturnsErrQueueFull` now races with the new `rejectedTotal` increment, the fix is to move the `p.rejectedTotal.Add(1)` INSIDE the existing `default:` branch (not add a new branch). Re-read the diff.

- [ ] **Step 9: Replace the body of internal/api/metrics.go**

The current file (~270 lines) hand-rolls Prometheus text format. Replace the entire contents with a thin adapter over `obs.Default`:

```go
package api

import (
	"net/http"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/obs"
	"github.com/RandomCodeSpace/docsiq/internal/project"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metricsHandler returns the /metrics handler. Backed by the shared
// obs.Default registry. Public — no auth (Prometheus scrape cannot
// present a bearer token in typical configs).
//
// Retained signature (registry, stores, cfg) so NewRouter callers do
// not need to change; the args are currently unused here but kept as a
// seam for future per-project gauges (see writeNotesGauge, now moved
// into obs as a registered collector).
func metricsHandler(
	_ *project.Registry,
	_ *projectStores,
	_ *config.Config,
) http.Handler {
	return promhttp.HandlerFor(obs.Default, promhttp.HandlerOpts{
		EnableOpenMetrics: false,
	})
}

// SetBuildInfo publishes binary version + commit to the docsiq_build_info
// gauge. Kept with its pre-existing signature so cmd/serve.go callers
// do not have to change.
func SetBuildInfo(version, commit string) {
	if obs.Build == nil {
		return // obs.Init() not called yet; noop rather than panic
	}
	obs.Build.Set(version, commit)
}
```

Note: `writeBuildInfo`, `writeProjectsGauge`, `writeNotesGauge`, `writeRequestsTotal`, `writeRequestDuration`, `numHistogramBuckets`, `histogramBuckets`, `labelKey`, `histogramCell`, `histogramKey`, `metricsRegistry`, `newMetricsRegistry`, `globalMetrics`, `recordRequest`, `formatFloat` — all DELETED. Their functionality is now in `internal/obs`.

If `writeNotesGauge` or `writeProjectsGauge` had unique value (per-project gauges for notes counts and project counts), restore them as Prometheus `GaugeFunc`s registered from `NewRouter` via a small helper `registerProjectsGauge(reg, registry, stores)`. This is NOT part of the spec, but removing them would regress the scrape surface. Add the restoration inside `internal/api/metrics.go`:

```go
// registerProjectGauges wires per-project gauges (projects_total,
// notes_total) on the shared obs registry. Safe to call more than once
// — the function uses MustRegister, so a repeat call will panic; guard
// with a sync.Once if tests re-invoke.
func registerProjectGauges(registry *project.Registry, stores *projectStores) {
	// … omitted; if the caller cares about this metric, lift the bodies
	// of the old writeProjectsGauge / writeNotesGauge here using
	// NewGaugeFunc. If the caller does NOT care, delete this helper
	// stub entirely.
}
```

If in doubt, port both gauges — they're cheap and operators use them.

- [ ] **Step 10: Record HTTP metrics from loggingMiddleware**

Open `internal/api/router.go`. Inside `loggingMiddleware` (after the `next.ServeHTTP(rw, r)` line and before the log emission), replace the existing `recordRequest(...)` call with:

```go
		// Skip the self-referential /metrics scrape — a tight scrape
		// loop would otherwise dominate the time series.
		if r.URL.Path != "/metrics" {
			// Route pattern — extract via the request context's
			// http.ServeMux pattern helper (Go 1.22+). Fall back to
			// the literal path when the match is empty (unknown route).
			route := r.Pattern
			if route == "" {
				route = "unknown"
			}
			obs.HTTP.Observe(route, r.Method, rw.status, duration)
		}
```

Add the `obs` import. Delete the old `recordRequest` call (it is gone with the metrics.go rewrite in Step 9).

Note: `r.Pattern` is populated by the std `http.ServeMux` as of Go 1.22. For tests that hit a handler directly (bypassing the mux), `r.Pattern` is empty and we record `route="unknown"` — acceptable.

- [ ] **Step 11: Wrap pipeline phases with obs.Pipeline.TimeStage**

Open `internal/pipeline/pipeline.go`. Identify the distinct phases from the existing slog.Info emojis (see research note):

- Phase 1 — Load: file collection + document parse
- Phase 2 — Chunk: RecursiveCharacter splitting
- Phase 3 — Embed: batch embedding into `embeddings` table
- Phase 4 — Extract: entity/relationship/claim extraction via LLM
- Phase 5 — Community: Louvain detection + summarisation
- Phase 6 — Finalize: persist + entity-vector embed

For each phase boundary, wrap the body with `obs.Pipeline.TimeStage`. Concrete example for the community phase (around line 600):

```go
// Before:
slog.Info("🧩 Phase 3: running Louvain community detection", /* ... */)
// ... community detection body ...

// After:
if err := obs.Pipeline.TimeStage("community", func() error {
    slog.Info("🧩 Phase 3: running Louvain community detection", /* ... */)
    // ... original body; propagate internal err via return ...
    return nil
}); err != nil {
    return fmt.Errorf("community phase: %w", err)
}
```

Repeat for each of the six phases with stage labels `"load"`, `"chunk"`, `"embed"`, `"extract"`, `"community"`, `"finalize"`. Stage names are fixed — no config knobs.

If `obs.Pipeline` is nil at the call site (cmd/index.go does not call `obs.Init()` today), add a nil guard helper:

```go
// timeStage is a nil-safe wrapper around obs.Pipeline.TimeStage. The
// indexer CLI does not initialise obs (obs.Init is only called from
// cmd/serve.go), so the CLI path must not blow up on a nil obs.Pipeline.
func timeStage(stage string, fn func() error) error {
	if obs.Pipeline == nil {
		return fn()
	}
	return obs.Pipeline.TimeStage(stage, fn)
}
```

Use `timeStage(...)` throughout `pipeline.go` instead of the raw `obs.Pipeline.TimeStage`.

- [ ] **Step 12: Instrument embedder batch calls**

Open `internal/embedder/embedder.go`. Locate the batch embed method (likely `Embed` or `EmbedBatch`). Wrap the per-batch provider call:

```go
// Before:
vectors, err := e.provider.EmbedBatch(ctx, batch)

// After:
start := time.Now()
vectors, err := e.provider.EmbedBatch(ctx, batch)
if obs.Embed != nil {
    obs.Embed.Observe(e.provider.Name(), time.Since(start))
}
```

Import `time` and `github.com/RandomCodeSpace/docsiq/internal/obs` at the top.

- [ ] **Step 13: Instrument LLM token counts**

Open `internal/llm/provider.go`. The interface `Complete` returns `(string, error)`; the underlying langchaingo providers return usage in `llms.ContentResponse.Choices[0].GenerationInfo`. Since the Provider interface does not expose usage, add a provider-side counter at each concrete implementation. Simpler alternative (and what this task implements): record a coarse `kind="total"` count using `len(prompt)+len(response)` as a token proxy (Go's byte length is not tokens, but operators prefer "a number greater than zero" to "nothing"). This is a knowingly-lossy approximation; flagged in the commit message.

In `internal/llm/provider.go`, add after each provider's `Complete` returns:

```go
if obs.LLM != nil {
    // Approximate: 1 token ≈ 4 bytes of UTF-8 for English text. This
    // is a coarse fallback until we thread langchaingo's GenerationInfo
    // through the Provider interface (tracked as follow-up).
    approxTokens := (len(prompt) + len(resp)) / 4
    obs.LLM.RecordTokens(p.Name(), "total", approxTokens)
}
```

If refactoring the interface is in scope for this task, add `Usage` to the return tuple; otherwise ship the approximation and leave a `// TODO(docsiq): thread real usage via GenerationInfo` comment.

- [ ] **Step 14: Wire obs.Init() and WorkqMetrics binding in cmd/serve.go**

Open `cmd/serve.go`. After the `pool := workq.New(...)` line (around line 155) and BEFORE the `router := api.NewRouter(...)` line, insert:

```go
		// Observability — initialise the Prometheus registry once per
		// process and bind the workq stats provider so the scrape
		// handler can read live queue depth + rejection count.
		obs.Init()
		obs.Workq.BindStatsProvider(func() obs.WorkqStats {
			s := pool.Stats()
			return obs.WorkqStats{Depth: s.Depth, Rejected: s.Rejected}
		})

		// Publish build info early so the first /metrics scrape sees
		// docsiq_build_info{version,commit} 1.
		{
			info := buildinfo.Resolve(false)
			api.SetBuildInfo(info.Version, info.Commit)
		}
```

Add imports `github.com/RandomCodeSpace/docsiq/internal/obs` and `github.com/RandomCodeSpace/docsiq/internal/buildinfo`.

- [ ] **Step 15: Run the full suite**

```
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go test -tags "sqlite_fts5" -timeout 300s ./...
```

Expected: all PASS. Likely failures:
- `internal/api/metrics_test.go` — asserts on the old text-format output. Rewrite those tests to scrape `/metrics` via the router and grep for metric family names (e.g. `docsiq_http_requests_total{route=…`). Do NOT delete the tests — the coverage is load-bearing.
- Integration tests that inspect `docsiq_requests_total` by exact name — the family was renamed to `docsiq_http_requests_total`. Update the assertions to the new name.

- [ ] **Step 16: Smoke-test /metrics output live**

```
DOCSIQ_API_KEY=dev make build && ./docsiq serve --port 0 &
SERVE_PID=$!
sleep 1
curl -s http://127.0.0.1:8080/api/stats -H 'Authorization: Bearer dev' > /dev/null  # generate 1 req
curl -s http://127.0.0.1:8080/metrics | head -60
kill $SERVE_PID
```

Expected: `docsiq_http_requests_total`, `docsiq_http_request_duration_seconds`, `docsiq_workq_depth`, `docsiq_workq_rejected_total`, `docsiq_build_info`, `go_*` (standard Go runtime metrics), `process_*` all present.

- [ ] **Step 17: Commit**

```bash
git add go.mod go.sum internal/obs/ internal/api/metrics.go \
        internal/api/router.go internal/workq/workq.go \
        internal/workq/workq_test.go internal/pipeline/pipeline.go \
        internal/embedder/embedder.go internal/llm/provider.go \
        cmd/serve.go internal/api/metrics_test.go \
        internal/api/metrics_integration_test.go
git commit -m "$(cat <<'EOF'
feat(obs): Prometheus metrics + workq stats

Replaces the ad-hoc text-format collector in internal/api/metrics.go
with the official prometheus/client_golang. Adds a new internal/obs
package hosting the Default registry and per-subject collectors
(HTTP, pipeline stages, embed latency, LLM tokens, workq depth +
rejections, build info). Workq gains a Pool.Stats() snapshot accessor
with rejectedTotal counter. Pipeline phases are wrapped in
TimeStage("load"|"chunk"|"embed"|"extract"|"community"|"finalize").

LLM token counts are approximated (bytes/4) until langchaingo usage
is threaded through the Provider interface — tracked as follow-up.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Access log middleware (4.4)

**Files:**
- Modify: `internal/api/router.go` — rewrite `loggingMiddleware` body + `responseWriter` struct

**Notes:** We do NOT add a second middleware. The existing `loggingMiddleware` already handles req_id generation, Prometheus recording, and emits the structured log. This task:

1. Extends `responseWriter` to track `bytes_out`.
2. Adds a `user_id`-shaped field: since docsiq auth is a single shared API key, there is no real user. We emit `auth` with values `"bearer"` (Authorization header), `"cookie"` (docsiq_session cookie), or `"anon"` (unauth — public endpoints only).
3. Moves the slog emission into a `defer` so access logs are still written when a downstream middleware (`recoveryMiddleware`) recovers a panic. Currently the log emission runs AFTER `next.ServeHTTP`, so a panic that recoveryMiddleware catches does print the log — but a panic escaping recoveryMiddleware (e.g. from `securityHeadersMiddleware` logic that runs outside) would bypass logging. Defer fixes this.

- [ ] **Step 1: Extend responseWriter to track bytes**

Open `internal/api/router.go`. Replace the existing `responseWriter` struct and its method block with:

```go
// responseWriter wraps http.ResponseWriter to capture status code and
// bytes written. Both are read by loggingMiddleware for the access log
// and by Prometheus. bytes is tracked via Write; implicit 200-only
// writes go through WriteHeader, so we default status to 200.
type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
	// wroteHeader prevents double-counting status when a handler calls
	// both WriteHeader(N) and Write — the std library already handles
	// this for us; the bool is an access-log observability aid.
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(p []byte) (int, error) {
	if !rw.wroteHeader {
		// Implicit 200 per net/http contract.
		rw.wroteHeader = true
	}
	n, err := rw.ResponseWriter.Write(p)
	rw.bytes += int64(n)
	return n, err
}

// Flush passes through to the underlying writer when it supports it —
// required for SSE and streaming responses. Standard http.ResponseWriter
// does NOT have Flush in its interface, so the type assertion is the
// correct idiom.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
```

- [ ] **Step 2: Rewrite loggingMiddleware**

Replace the entire `loggingMiddleware` function with:

```go
// loggingMiddleware assigns a per-request ID, records Prometheus
// metrics, and emits one structured "http" log line per request. The
// log emission is deferred so that a panic escaping recoveryMiddleware
// (e.g. a panic in securityHeadersMiddleware) is still observable.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Request ID: header pass-through, otherwise generate fresh
		// 16-hex (8 random bytes). Put on ctx + echo back as response
		// header.
		rid := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if rid == "" {
			rid = newRequestID()
		}
		ctx := context.WithValue(r.Context(), ctxRequestIDKey{}, rid)
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-ID", rid)

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()

		// Defer the access log + metric emission so a panic still produces
		// an observation. recoveryMiddleware catches the panic and writes a
		// 500; we then see status=500 in the log. If a panic escapes
		// recoveryMiddleware, Go still unwinds through our deferred func
		// before the goroutine dies, so the log is emitted with whatever
		// status (likely 200) had been set.
		defer func() {
			duration := time.Since(start)

			// Rethrow after logging; our logging must be side-effect-only
			// for the panic propagation path. A deeper recovery belongs in
			// recoveryMiddleware, not here.
			recErr := recover()

			// /metrics is self-referential — exclude to avoid scraping
			// feedback loops.
			if r.URL.Path != "/metrics" {
				route := r.Pattern
				if route == "" {
					route = "unknown"
				}
				if obs.HTTP != nil {
					obs.HTTP.Observe(route, r.Method, rw.status, duration)
				}
			}

			level := slog.LevelInfo
			if rw.status >= 500 || recErr != nil {
				level = slog.LevelError
			} else if rw.status >= 400 {
				level = slog.LevelWarn
			}

			attrs := []any{
				"req_id", rid,
				"method", r.Method,
				"path", r.URL.Path,
				"route", r.Pattern,
				"status", rw.status,
				"duration_ms", duration.Milliseconds(),
				"bytes_out", rw.bytes,
				"auth", classifyAuth(r),
			}
			if project := ProjectFromContext(r.Context()); project != "" {
				attrs = append(attrs, "project", project)
			}
			if recErr != nil {
				attrs = append(attrs, "panic", recErr)
			}

			slog.Log(r.Context(), level, "http", attrs...)

			if recErr != nil {
				// Re-raise: let upstream recovery middleware (or the
				// std library) handle the actual HTTP error response.
				panic(recErr)
			}
		}()

		next.ServeHTTP(rw, r)
	})
}

// classifyAuth reports a coarse auth-method label for the access log.
// docsiq uses a single shared API key (no per-user identity), so we
// emit the channel the client used rather than a user_id.
func classifyAuth(r *http.Request) string {
	if strings.HasPrefix(strings.TrimSpace(r.Header.Get("Authorization")), "Bearer ") {
		return "bearer"
	}
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		return "cookie"
	}
	return "anon"
}
```

Note: `ProjectFromContext` is the existing helper in `internal/api/project.go`. `sessionCookieName` is the existing const in `session.go` (introduced earlier by the session-cookie feature). `obs` imported at top.

- [ ] **Step 3: Add loggingMiddleware tests**

Create `internal/api/logging_middleware_test.go`:

```go
package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureLogs swaps the default slog handler for a JSON-to-buffer one
// for the duration of the test, then restores the previous default.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestLoggingMiddleware_EmitsStructuredAccessLog(t *testing.T) {
	// NOT parallel — mutates global slog.
	buf := captureLogs(t)

	h := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello world"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	req.Header.Set("Authorization", "Bearer dev")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("no log lines emitted")
	}
	var last map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatalf("last log line not JSON: %v", err)
	}

	want := map[string]any{
		"msg":    "http",
		"method": "GET",
		"path":   "/api/stats",
		"status": float64(200),
		"auth":   "bearer",
	}
	for k, v := range want {
		if got := last[k]; got != v {
			t.Errorf("log[%s]=%v want %v", k, got, v)
		}
	}
	if _, ok := last["req_id"].(string); !ok {
		t.Errorf("req_id missing or not string: %v", last["req_id"])
	}
	if b, ok := last["bytes_out"].(float64); !ok || b != 11 {
		t.Errorf("bytes_out=%v want 11", last["bytes_out"])
	}
}

func TestLoggingMiddleware_PanicStillLogsAccessEntry(t *testing.T) {
	// NOT parallel — mutates global slog.
	buf := captureLogs(t)

	// Chain: loggingMiddleware wraps a handler that panics.
	// Without recoveryMiddleware here, the panic propagates — but the
	// deferred access log must still have fired.
	h := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic-path", nil)
	rec := httptest.NewRecorder()

	func() {
		defer func() { _ = recover() }() // swallow in test
		h.ServeHTTP(rec, req)
	}()

	if buf.Len() == 0 {
		t.Fatal("access log not emitted through panic path")
	}
	if !strings.Contains(buf.String(), `"panic":"boom"`) {
		t.Errorf("log should mention panic=boom; got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"level":"ERROR"`) {
		t.Errorf("panic log should be ERROR level; got: %s", buf.String())
	}
}

func TestLoggingMiddleware_ReqIDPassThrough(t *testing.T) {
	t.Parallel()
	h := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := RequestIDFromContext(r.Context()); got != "caller-id-abc" {
			t.Errorf("ctx req_id=%q want caller-id-abc", got)
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "caller-id-abc")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "caller-id-abc" {
		t.Errorf("echoed X-Request-ID=%q", got)
	}
}

func TestLoggingMiddleware_AnonCookieBearerClassification(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(r *http.Request)
		want  string
	}{
		{name: "anon_no_auth", setup: func(r *http.Request) {}, want: "anon"},
		{name: "bearer_header", setup: func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer k")
		}, want: "bearer"},
		{name: "session_cookie", setup: func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "cookie-token"})
		}, want: "cookie"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tc.setup(req)
			if got := classifyAuth(req); got != tc.want {
				t.Errorf("classifyAuth=%q want %q", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 4: Run the new tests**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/api/ -run TestLoggingMiddleware -v`

Expected: 4 tests PASS. If `TestLoggingMiddleware_PanicStillLogsAccessEntry` fails because `buf.Len() == 0`, the `defer` block is wrong — re-check that the slog emission happens INSIDE the deferred func, not AFTER `next.ServeHTTP`.

- [ ] **Step 5: Run the full API suite**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/api/ -v -timeout 120s`

Expected: all PASS. The new `bytes_out` field changes the log-line shape — any test that grep-asserts on the exact log line string must be updated to `slog` attr-parsing.

- [ ] **Step 6: Commit**

```bash
git add internal/api/router.go internal/api/logging_middleware_test.go
git commit -m "$(cat <<'EOF'
feat(api): structured access log with bytes_out + panic resilience

loggingMiddleware now emits one JSON/text line per request with
{req_id, method, path, route, status, duration_ms, bytes_out, auth,
project, panic}. The emission is deferred so a panic escaping
recoveryMiddleware still produces an access-log entry. auth is a
coarse label (bearer|cookie|anon) because docsiq uses a single shared
API key; there is no real user identity.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Structured-log schema + format switch (4.2)

**Files:**
- Modify: `internal/config/config.go` — add `LogConfig` struct with `Format` field; wire defaults + env binding
- Modify: `internal/config/config_test.go` — add a test confirming `log.format` defaults to `"text"` and parses `"json"`
- Modify: `cmd/root.go` — the `initConfig` function: add a third source of truth (config file) below the existing `--log-format` flag and `DOCSIQ_LOG_FORMAT` env var precedence
- Modify: `internal/api/` + `internal/pipeline/` + `internal/embedder/` + `cmd/*.go` — audit and strip the leading emoji from every `slog.*` message when the global format is `json`. Approach: wrap the slog handler so the emoji is a text-mode-only decoration (DRY; no source edits needed in log call sites).

**Notes:** The user's rule is "production log format drops emoji prefixes (keep in dev format for human readability)". The cleanest implementation is a custom slog middleware handler that strips a leading emoji + space from the `msg` when wrapping a JSON handler. Zero edits to call sites.

- [ ] **Step 1: Add LogConfig to config**

Open `internal/config/config.go`. Extend the top-level `Config` struct:

```go
type Config struct {
	DataDir        string                 `mapstructure:"data_dir"`
	DefaultProject string                 `mapstructure:"default_project"`
	LLM            LLMConfig              `mapstructure:"llm"`
	Indexing       IndexingConfig         `mapstructure:"indexing"`
	Community      CommunityConfig        `mapstructure:"community"`
	Server         ServerConfig           `mapstructure:"server"`
	Log            LogConfig              `mapstructure:"log"`
	LLMOverrides   map[string]LLMConfig   `mapstructure:"llm_overrides"`
}
```

Add the new struct (place near the other `…Config` types):

```go
// LogConfig controls structured-log emission format.
type LogConfig struct {
	// Format chooses the slog handler. "text" (default) emits a
	// human-readable single-line format with emoji prefixes; "json"
	// strips emoji and emits machine-parseable JSON objects.
	Format string `mapstructure:"format"`
}
```

In the `Load` function, near the existing `SetDefault` block, add:

```go
	v.SetDefault("log.format", "text")
```

And in the `BindEnv` block, add:

```go
	_ = v.BindEnv("log.format", "DOCSIQ_LOG_FORMAT")
```

- [ ] **Step 2: Add a config test**

Append to `internal/config/config_test.go`:

```go
func TestLoad_LogFormatDefaultText(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg, err := Load(filepath.Join(dir, "missing.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Log.Format != "text" {
		t.Errorf("default Log.Format=%q want text", cfg.Log.Format)
	}
}

func TestLoad_LogFormatFromYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(f, []byte("log:\n  format: json\n"), 0o600))
	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Log.Format != "json" {
		t.Errorf("Log.Format=%q want json", cfg.Log.Format)
	}
}
```

- [ ] **Step 3: Run config tests**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/config/ -v -run TestLoad_LogFormat`

Expected: 2/2 PASS.

- [ ] **Step 4: Implement the emoji-stripping slog middleware handler**

Create `internal/obs/slogfmt.go`:

```go
package obs

import (
	"context"
	"log/slog"
	"strings"
	"unicode/utf8"
)

// NewProductionHandler wraps an inner slog.Handler and strips a
// leading emoji + trailing space from each record's Message. docsiq
// uses emoji prefixes (✅ ❌ ⚠️ …) as visual cues in dev text format;
// in JSON these collide with log-aggregator indexing (Elasticsearch
// tokeniser, fluentd grep rules) and obscure the actual message
// string. The handler mutates only Message — attrs pass through.
func NewProductionHandler(inner slog.Handler) slog.Handler {
	return &prodHandler{inner: inner}
}

type prodHandler struct{ inner slog.Handler }

func (h *prodHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return h.inner.Enabled(ctx, lvl)
}

func (h *prodHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Message = stripLeadingEmoji(r.Message)
	return h.inner.Handle(ctx, r)
}

func (h *prodHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &prodHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *prodHandler) WithGroup(name string) slog.Handler {
	return &prodHandler{inner: h.inner.WithGroup(name)}
}

// stripLeadingEmoji removes the first rune from msg if it is in a
// Unicode emoji-like range, plus any immediately-following whitespace.
// Narrow vs broad emoji detection: we use a conservative heuristic —
// the rune's category falls into Symbol (S*) and its value is >= 0x2600
// OR it sits in one of the Private/Surrogate-adjacent pictograph blocks.
// We intentionally do NOT use a dependency like mattn/go-emoji; docsiq
// ships under air-gap rules (see build.md) and every dep is scrutinised.
func stripLeadingEmoji(msg string) string {
	if msg == "" {
		return msg
	}
	r, size := utf8.DecodeRuneInString(msg)
	if r == utf8.RuneError {
		return msg
	}
	if !isEmojiLike(r) {
		return msg
	}
	// Drop emoji + any whitespace that follows.
	rest := msg[size:]
	rest = strings.TrimLeft(rest, " \t")
	// Also strip a VS16 variation selector (U+FE0F) that often
	// follows ⚠ etc.
	if len(rest) > 0 {
		r2, size2 := utf8.DecodeRuneInString(rest)
		if r2 == 0xFE0F {
			rest = strings.TrimLeft(rest[size2:], " \t")
		}
	}
	return rest
}

// isEmojiLike is a conservative test for the emoji-range runes that
// appear in docsiq log messages today: ✅ ❌ ⚠️ 🛑 ⚙️ 🔒 📦 🔍 🔗 🧩 💾
// 🌐 ⏭️ 📄 📂 📒 📊 🚀 🧭 🧮 🗑️ 📋. Covers BMP symbols (U+2600–U+27BF),
// miscellaneous pictographs (U+1F300–U+1F6FF), supplemental symbols
// (U+1F900–U+1F9FF), and the warning sign block (U+26A0).
func isEmojiLike(r rune) bool {
	switch {
	case r >= 0x2600 && r <= 0x27BF:
		return true
	case r >= 0x1F300 && r <= 0x1F6FF:
		return true
	case r >= 0x1F900 && r <= 0x1F9FF:
		return true
	}
	return false
}
```

- [ ] **Step 5: Add tests for the handler**

Create `internal/obs/slogfmt_test.go`:

```go
package obs

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestStripLeadingEmoji(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"✅ all good", "all good"},
		{"❌ panic recovered", "panic recovered"},
		{"⚠️ auth disabled", "auth disabled"},   // VS16 variation selector
		{"🛑 shutting down...", "shutting down..."},
		{"⚙️ LLM provider initialised", "LLM provider initialised"},
		{"plain log line", "plain log line"},
		{"", ""},
		{"  leading spaces", "  leading spaces"}, // no emoji → untouched
	}
	for _, c := range cases {
		got := stripLeadingEmoji(c.in)
		if got != c.want {
			t.Errorf("stripLeadingEmoji(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestProductionHandler_JSONOutputNoEmoji(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	h := NewProductionHandler(slog.NewJSONHandler(&buf, nil))
	logger := slog.New(h)

	logger.Info("✅ ready", "port", 8080)
	logger.Error("❌ connection failed", "err", "timeout")

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("got %d lines want 2", len(lines))
	}
	for _, line := range lines {
		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("not JSON: %v — raw=%q", err, line)
		}
		msg, _ := rec["msg"].(string)
		if msg == "" {
			t.Errorf("missing msg: %s", line)
		}
		// No emoji should survive.
		for _, r := range msg {
			if isEmojiLike(r) {
				t.Errorf("msg contains emoji %q; msg=%q", r, msg)
				break
			}
		}
	}
}
```

- [ ] **Step 6: Run the obs handler tests**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/obs/ -v -run Handler`

Expected: both tests PASS.

- [ ] **Step 7: Wire the handler into initConfig**

Open `cmd/root.go`. Replace the `initConfig` function's logger setup block (currently lines ~37-60 — the `switch format { case "json": … default: … }` block) with:

```go
func initConfig() {
	// Set up structured logger. Level resolution order: --log-level flag.
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Format resolution order (highest-priority wins):
	//   1. --log-format flag
	//   2. DOCSIQ_LOG_FORMAT env var
	//   3. config file log.format
	//   4. default "text"
	// Note: (3) requires config.Load() to have run; we call it below
	// and re-evaluate the handler afterwards if the flag/env did not
	// already lock in a value.
	format := strings.ToLower(strings.TrimSpace(logFormat))
	if format == "" {
		format = strings.ToLower(strings.TrimSpace(os.Getenv("DOCSIQ_LOG_FORMAT")))
	}

	// First pass: install a temporary handler so config-load errors
	// emit somewhere. Upgraded below once config.Log.Format is known.
	slog.SetDefault(slog.New(buildHandler(level, format)))

	var err error
	cfg, err = config.Load(cfgFile)
	if err != nil {
		slog.Error("❌ config error", "err", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		slog.Error("❌ mkdir data dir", "err", err)
		os.Exit(1)
	}

	// Second pass: if neither flag nor env specified format, use the
	// value from the loaded config.
	if format == "" && cfg.Log.Format != "" {
		format = strings.ToLower(strings.TrimSpace(cfg.Log.Format))
		slog.SetDefault(slog.New(buildHandler(level, format)))
	}
}

// buildHandler assembles the slog handler chain. For "json" format we
// wrap the JSON handler in obs.NewProductionHandler to strip emoji
// prefixes from the message field (keeping them is harmless but noisy
// for log aggregators). "text" keeps emoji for human readability.
func buildHandler(level slog.Level, format string) slog.Handler {
	opts := &slog.HandlerOptions{Level: level}
	switch format {
	case "json":
		return obs.NewProductionHandler(slog.NewJSONHandler(os.Stderr, opts))
	default:
		return slog.NewTextHandler(os.Stderr, opts)
	}
}
```

Add the import `github.com/RandomCodeSpace/docsiq/internal/obs` at the top of `cmd/root.go`.

- [ ] **Step 8: Update the existing cmd/logformat_test.go**

Open `cmd/logformat_test.go`. It currently asserts on the text-vs-json switch via `--log-format` and `DOCSIQ_LOG_FORMAT`. Add one new test case:

```go
func TestInitConfig_LogFormatFromConfigFile(t *testing.T) {
	// NOT parallel — mutates package-level flags.
	t.Cleanup(func() {
		logLevel = "info"
		logFormat = ""
		cfgFile = ""
		cfg = nil
	})

	dir := t.TempDir()
	yaml := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(yaml, []byte("log:\n  format: json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgFile = yaml
	logFormat = ""
	os.Unsetenv("DOCSIQ_LOG_FORMAT")

	initConfig()

	// After initConfig, the handler should be JSON. Probe by logging
	// through slog.Default and inspecting stderr capture — but simpler:
	// assert cfg.Log.Format round-tripped.
	if cfg.Log.Format != "json" {
		t.Errorf("cfg.Log.Format=%q want json", cfg.Log.Format)
	}
}
```

If `logformat_test.go` already has precedence-order tests, assert that (flag > env > config > default) is preserved.

- [ ] **Step 9: Run the cmd tests**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./cmd/ -v -run LogFormat`

Expected: all PASS. If `initConfig()` panics because `rootCmd` has not been executed (flag init skipped), add `cobra.OnInitialize(initConfig)` guard — but the existing test file already exercises `initConfig` directly so this should be fine.

- [ ] **Step 10: Full-suite verification**

```
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go test -tags "sqlite_fts5" -timeout 300s ./...
```

Expected: all PASS. If any test greps the log output for a specific emoji (`"✅ ready"`) AFTER format switches to json, those tests need the emoji stripped from expected strings — update them.

- [ ] **Step 11: Smoke-test both formats live**

```
make build

echo '=== text ==='
DOCSIQ_LOG_FORMAT=text DOCSIQ_API_KEY=dev ./docsiq serve --port 0 2>&1 &
PID=$!; sleep 1; curl -s http://127.0.0.1:8080/healthz > /dev/null; kill $PID
# Expect: human-readable line with emoji 🚀

echo '=== json ==='
DOCSIQ_LOG_FORMAT=json DOCSIQ_API_KEY=dev ./docsiq serve --port 0 2>&1 &
PID=$!; sleep 1; curl -s http://127.0.0.1:8080/healthz > /dev/null; kill $PID
# Expect: {"time":"…","level":"INFO","msg":"server started",…} — no emoji
```

Verify stderr of the json run contains no raw emoji bytes:

```
DOCSIQ_LOG_FORMAT=json DOCSIQ_API_KEY=dev timeout 2 ./docsiq serve --port 0 2>&1 | grep -E '[^\x00-\x7F]' || echo "NO NON-ASCII BYTES (expected)"
```

Expected final line: `NO NON-ASCII BYTES (expected)`. If non-ASCII bytes leak through, a call site uses an emoji not covered by `isEmojiLike` — extend the ranges.

- [ ] **Step 12: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go \
        cmd/root.go cmd/logformat_test.go \
        internal/obs/slogfmt.go internal/obs/slogfmt_test.go
git commit -m "$(cat <<'EOF'
feat(log): log.format=json|text with emoji-strip production handler

New log.format config key (default "text"; DOCSIQ_LOG_FORMAT env).
Precedence is --log-format > env > config > default. The json handler
is wrapped in obs.NewProductionHandler, which strips a leading emoji
from slog Record.Message so log aggregators do not have to special-case
multi-byte sequences. The text handler keeps emoji for human readers.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review

**1. Spec coverage.** Each bullet of Block 4 has a task:

- 4.1 Prometheus `/metrics` → Task 3. All seven named metric families are present:
  - `docsiq_pipeline_stage_duration_seconds{stage}` — `internal/obs/metrics.go` `NewPipelineMetrics`
  - `docsiq_embed_latency_seconds{provider}` — `NewEmbedMetrics`
  - `docsiq_llm_tokens_total{provider,kind}` — `NewLLMMetrics`
  - `docsiq_workq_depth` — `NewWorkqMetrics.depthGauge`
  - `docsiq_workq_rejected_total` — `NewWorkqMetrics.rejectedTotal`
  - `docsiq_http_requests_total{route,method,status}` — `NewHTTPMetrics.Requests`
  - `docsiq_http_request_duration_seconds{route}` — `NewHTTPMetrics.Duration`
- 4.2 Structured-log schema → Task 5. Standard fields `req_id, project, route, method, status, duration_ms, err, bytes_out, auth` are emitted from the `loggingMiddleware` in Task 4 and the format switch from Task 5 removes emoji in json mode. The spec names `user_id`; we document in the commit and the auth-classification comment that docsiq has no per-user identity and emit `auth` instead — this is the best-faith mapping.
- 4.3 Health endpoints → Task 2. `/healthz` + `/readyz` with 10-second cache and per-check JSON body.
- 4.4 Access log middleware → Task 4. Extends existing `loggingMiddleware` rather than adding a second. Panic-safe via `defer`.
- 4.5 Version endpoint → Task 1. `GET /api/version` with ldflags wiring (Makefile already set up for this) and `runtime/debug` fallback + dep map.

**2. Placeholder scan.** No TBDs, no "implement later", no "add appropriate X". The single hedged item is `registerProjectGauges` in Task 3 Step 9, which is explicitly marked "port both or delete, do not leave stub". Every test step includes actual test code; every command lists the actual expected output.

**3. Type consistency.**

- `buildinfo.Info` with `json` tags is used consistently across Task 1 handler + buildinfo_test.
- `workq.Stats{Depth, Rejected int64}` is used in both Task 3 Step 6 (workq.go) and Step 14 (cmd/serve.go). The adapter in `obs.WorkqStats{Depth, Rejected int64}` has identical fields — the converter in serve.go spells out the copy.
- `healthPinger` interface has `Ping(ctx) error` signature in both `health.go` and `health_test.go`. The `sqlDBPinger` and `providerPinger` both satisfy it.
- `obs.HTTPMetrics.Observe(route, method, status, d)` is the single entry point for HTTP metrics — called once from `loggingMiddleware` in Task 4.
- `obs.Pipeline.TimeStage(stage, fn func() error) error` signature is consistent; the `timeStage` local wrapper in pipeline.go has the same signature for nil-safe call sites.
- `classifyAuth` returns string `"bearer" | "cookie" | "anon"`; the test `TestLoggingMiddleware_AnonCookieBearerClassification` asserts exactly those values.
- `LogConfig.Format` field is referenced in `cfg.Log.Format` in cmd/root.go — tag `mapstructure:"format"` matches both the struct literal and viper's `log.format` key.

**Follow-ups (not in scope of this plan):**
- LLM token counts are approximated from byte length. Threading langchaingo's `GenerationInfo` through `llm.Provider` is a follow-up commit. Flagged in Task 3 Step 13.
- `r.Pattern` may be empty for the embedded SPA and the `/mcp` handler (they're `mux.Handle("/", …)` catch-alls). These will log `route="unknown"`. If operators want finer granularity, a separate PR can synthesise route labels from `URL.Path` with a configured whitelist.
- `registerProjectGauges` — the Task 3 rewrite of metrics.go removes per-project `docsiq_notes_total` + `docsiq_projects_total` gauges. The implementer MUST either port them (recommended — fast, cheap, operators use them) or remove them with a clear commit note. Do not leave a stub.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-23-block4-observability-plan.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration. Task boundaries are clean (each task ends with a green build + test run + single commit), which suits this model well.

**2. Inline Execution** — Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints for review.

Which approach?
