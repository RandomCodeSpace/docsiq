# docsiq Production-Polish Roadmap — Design Spec

**Date:** 2026-04-23
**Scope:** Polish existing features for production-readiness. **No new features.**
**Priority:** (A) Hardened self-hosted / air-gapped deployment — *highest*. (B) OSS release-quality polish — *next*. (C) Enterprise / multi-tenant — *deferred*.
**Deployment topology assumed:** Small-team self-hosted — 2–10 users sharing one single-binary host; basic bearer-token auth; SQLite on local disk.

---

## 1. Goals & Non-Goals

### Goals

- Close correctness, resource-safety, and security gaps that would cause the service to crash, leak, hang, or silently mis-serve in production.
- Make air-gap deployment trustworthy: no public CDN, no phone-home, reproducible offline builds, deterministic startup.
- Make every route bound in time, memory, and concurrent workload.
- Give operators observability (metrics, structured logs, health/readiness) sufficient to diagnose incidents without code access.
- Tighten the UI's empty / loading / error surfaces so the product feels finished, not scaffolded.
- Bring the OSS-facing surface (README, SECURITY, CONTRIBUTING, example config) to a first-impression bar equal to Linear / Supabase / Raycast.

### Non-Goals

- Multi-tenant project isolation, per-user permissions, OIDC/SSO, audit log ingestion, SLO dashboards — explicitly deferred (C-priority).
- Any new user-visible feature — no new routes, no new API endpoints, no new MCP tools.
- Framework migrations, rewrites, or speculative refactors outside files already touched by the roadmap items.

### Guiding Principles (inherited from `~/.claude/rules/*.md`)

- Measure before optimizing; bound every external call in time and size.
- No secret in code, config, log, or HTML. Parameterized queries only.
- Minimal diffs; follow existing patterns; no silent unrelated refactoring.
- All deps pinned and vendored; no runtime public-internet calls.
- WCAG AA; honor `prefers-reduced-motion` and `prefers-color-scheme`.
- Never report done on "should work" — verify by executing.

---

## 2. Execution Strategy

**Block 1 (Critical)** ships first, sequentially, one PR per item. Target: done in 1–2 days.
**Blocks 2–6 (A-priority themed sweeps)** run in any order after Block 1 lands. Blocks are independent and PR-sized; most items inside a block are also independent. Parallel subagent execution is expected.
**Block 7 (B-priority OSS polish)** runs after Blocks 1–6 are green. Primarily documentation and repo-surface work.

Every item follows the standing "Done = Done" bar: code self-reviewed via full-diff read; tests added for new behavior; type-checks, linters, and builds pass; dep audit clean; short summary delivered.

---

## 3. Block 1 — Critical (ship-blocker severity, severity-first)

### 1.1 — API key leaked into every HTML response

- **File:** `internal/api/router.go:184-191`
- **What exists:** When `server.api_key` is non-empty, the SPA handler injects `<meta name="docsiq-api-key" content="…">` into `index.html` so browser-side fetch can read and send it in `Authorization`.
- **Gap:** The key is visible in View Source, cached by the browser, transmitted on every page load, and read by any injected script / extension / XSS. There is no rotation path; compromising any browser that ever loaded the UI compromises the key.
- **Fix shape:** Replace the meta-tag contract with an `httpOnly; Secure; SameSite=Strict` session cookie set on login. Browser `fetch` uses `credentials: "include"` — no JS ever touches the key. A single new `POST /api/session` endpoint exchanges the bearer key (sent once via `Authorization: Bearer <key>` over the loopback during first-run, or via config-file-provisioned cookie in team deploys) for the cookie. Delete the meta-tag code path entirely.
- **Tests:** unit — cookie set with correct attributes on success, 401 on bad key; integration — `/api/*` returns 401 without cookie; existing UI client code updated and still green in Playwright.
- **Severity:** Critical. **Effort:** M.

### 1.2 — Unbounded multipart upload

- **File:** `internal/api/handlers.go:391`
- **What exists:** `r.ParseMultipartForm(maxMemory)` is called with no outer cap on request-body size.
- **Gap:** An attacker can stream arbitrary bytes; Go will spill to temp files on disk with no ceiling. Both memory and disk are DoS targets.
- **Fix shape:** Wrap request body with `http.MaxBytesReader(w, r.Body, cfg.Server.MaxUploadBytes)` before `ParseMultipartForm`. Add `server.max_upload_mb` config key, default 100 (= 100 MiB). Handler returns `413 Payload Too Large` with structured JSON error on overflow.
- **Tests:** unit — 413 on body larger than cap; unit — success at exactly cap; config — default honored, override honored.
- **Severity:** Critical. **Effort:** S.

### 1.3 — Fire-and-forget indexing goroutine

- **File:** `internal/api/handlers.go:475`
- **What exists:** After a successful upload the handler returns 202 and spawns `go indexPipeline(context.Background(), …)`. No queue, no worker cap, no timeout, no shutdown join.
- **Gap:** Arbitrary concurrent pipeline runs (resource blowup), zero backpressure, in-flight jobs are abandoned on SIGTERM with partial writes to SQLite.
- **Fix shape:** New `internal/workq` package — a bounded worker pool with fixed worker count (default `runtime.NumCPU()`), fixed-depth job queue (default 64), and a `Submit(job)` API that returns `ErrQueueFull` when saturated. Jobs receive a `context.Context` derived from the server's shutdown context. `Close()` drains or times out according to server shutdown deadline. Upload handler rejects with `503 Service Unavailable` + `Retry-After` when queue is full.
- **Tests:** unit — submit blocks/returns correct error when full; unit — shutdown joins in-flight, cancels queued; integration — SIGTERM during pipeline waits for current job.
- **Severity:** Critical. **Effort:** M.

### 1.4 — Auth-disabled-by-default

- **Files:** `internal/api/auth.go:25-28`, `internal/config/config.go:210`
- **What exists:** `bearerAuthMiddleware` short-circuits and returns `next` unwrapped when `apiKey == ""`. Default for `server.api_key` is `""`.
- **Gap:** A plain `docsiq serve` exposes every endpoint to anyone who can reach the port. Silent no-auth is the default.
- **Fix shape:** At startup, if `server.api_key` is empty AND `server.bind` is not loopback (`127.0.0.1` / `localhost` / `::1`), `Serve()` returns an error explaining the unsafe configuration and refusing to bind. If bound to loopback, emit a single prominent `slog.Warn` line on startup: `"auth disabled (empty api_key); only loopback bind allowed"`.
- **Tests:** unit — startup error on non-loopback + empty key; unit — startup succeeds with warning on loopback + empty key.
- **Severity:** Critical. **Effort:** S.

### 1.5 — Entity full-scan per local search

- **File:** `internal/search/local.go:84`
- **What exists:** `localSearch()` computes the top-hit document IDs via vector search, then fetches *all* entities from the store and filters in Go.
- **Gap:** Linear-in-corpus memory + latency per query; unacceptable past a few thousand entities. The top-hit doc IDs are already known when the fetch runs — the scope is discarded.
- **Fix shape:** Change the entity fetch to `SELECT … FROM entities WHERE doc_id IN (?, ?, …)` using the top-hit doc IDs. Chunk the IN-list at 999 (SQLite's default `SQLITE_MAX_VARIABLE_NUMBER`). Return an empty slice on empty scope.
- **Tests:** unit — scoped query returns only entities for provided doc IDs; unit — chunking correctness at boundary sizes (0, 1, 998, 999, 1000, 2000); benchmark — demonstrate sub-linear scaling on synthetic corpus of 10k entities.
- **Severity:** Critical. **Effort:** M.

---

## 4. Block 2 — Security & auth hardening

- **2.1 CSP header** — `Content-Security-Policy: default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; font-src 'self'; frame-ancestors 'none'; base-uri 'self'`. Enforces air-gap (no CDN), blocks inline scripts. Apply via middleware; exempt `OPTIONS`.
- **2.2 Baseline security headers** — `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy: camera=(), microphone=(), geolocation=(), payment=(), usb=()`, `Strict-Transport-Security: max-age=31536000` (when `server.tls_cert` is configured).
- **2.3 Secret scrubbing in debug logs** — audit every `slog.Debug`/`slog.Info` site in `internal/config`, `internal/llm`, and `internal/api`. Wrap logged config values through a `Redact()` helper that zeroes fields tagged `secret:"true"` in the `Config` struct. Add a test that parses config with known secrets and asserts no occurrence in captured log output.
- **2.4 Config validation** — on `config.Load()`, reject unknown top-level keys (Viper: `v.UnmarshalExact` equivalent), validate LLM provider triples are internally consistent (chat endpoint + chat key + chat model all present or all absent), fail fast with a pointer at the offending key.
- **2.5 Request-ID middleware** — generate `X-Request-ID` (ULID) if absent; echo on response; thread into `slog.With("req_id", id)` for the request's handler tree.

**Severity:** Important. **Effort:** 5× S.

---

## 5. Block 3 — Resource safety & correctness

- **3.1 Context propagation audit** — every exported function in `internal/llm`, `internal/embedder`, `internal/extractor`, `internal/crawler`, `internal/store` accepts `ctx context.Context` as first argument and passes it to every I/O call it makes. Static check: `go vet -vettool=$(which shadow)` + a tiny custom linter script that greps for HTTP/DB calls missing a ctx argument.
- **3.2 Request-level timeout** — `http.TimeoutHandler(router, cfg.Server.RequestTimeout, "request timeout")` at the outermost middleware. Default 30s. Per-endpoint override for `POST /api/projects/{p}/docs` (allow up to `cfg.Server.UploadTimeout`, default 10m) so indexing kickoff isn't killed.
- **3.3 LLM call timeouts** — every provider wraps its outbound call in `ctx, cancel := context.WithTimeout(parent, cfg.LLM.CallTimeout)`, default 60s. Retry wrapper (if any) counts retries against the total timeout, not resets it.
- **3.4 `EmbedBatch` chunking + backpressure** — each provider declares its batch ceiling (OpenAI 2048, Azure 16 per deployment default, Ollama 128 heuristic). Caller chunks input to that ceiling. Provider pushes results through a buffered channel of size `2 * batchCeiling`; slow consumers get backpressure.
- **3.5 HTTP client pooling** — one `*http.Client` per LLM provider, stored on the provider struct. Transport tuned: `MaxIdleConns=100`, `MaxIdleConnsPerHost=10`, `IdleConnTimeout=90s`, `TLSHandshakeTimeout=10s`, `ResponseHeaderTimeout=60s`.
- **3.6 SQLite hardening** — on open: `PRAGMA journal_mode=WAL`, `PRAGMA busy_timeout=5000`, `PRAGMA synchronous=NORMAL`, `PRAGMA foreign_keys=ON`. `db.SetMaxOpenConns(4)`, `db.SetMaxIdleConns(2)`, `db.SetConnMaxLifetime(1h)`. Add `Ping(ctx)` to store interface.
- **3.7 Graceful shutdown** — `main` listens for `SIGINT`/`SIGTERM`, cancels the server's root context, calls `srv.Shutdown(ctx)` with a 30s deadline, then `workq.Close(remaining)` with the same deadline; logs progress at each step. Panic middleware captures `req_id`, route, method, user (if authed), full stack.

**Severity:** Critical / Important mix. **Effort:** 7× S–M.

---

## 6. Block 4 — Observability & ops

- **4.1 Prometheus `/metrics`** — expose: `docsiq_pipeline_stage_duration_seconds{stage}`, `docsiq_embed_latency_seconds{provider}`, `docsiq_llm_tokens_total{provider,kind}`, `docsiq_workq_depth`, `docsiq_workq_rejected_total`, `docsiq_http_requests_total{route,method,status}`, `docsiq_http_request_duration_seconds{route}`. Use `github.com/prometheus/client_golang` (already vendored if not — verify).
- **4.2 Structured-log schema** — standard fields on every log: `req_id`, `project`, `user_id` (if authed), `route`, `method`, `status`, `duration_ms`, `err` (when non-nil). Production log format drops emoji prefixes (keep in dev format for human readability). Format switch via `log.format=json|text`.
- **4.3 Health endpoints** — `/healthz` — liveness, no dependencies, always 200 if process is running. `/readyz` — readiness, checks (a) SQLite ping, (b) LLM provider reachable via cheap call (cache result for 10s to avoid hammering). Return JSON body with per-check result.
- **4.4 Access log middleware** — one log line per request: `req_id`, `method`, `path`, `status`, `duration_ms`, `bytes_out`, `user_id`. Emitted even on panic (via deferred capture).
- **4.5 Version endpoint** — `GET /api/version` returns JSON: `{version, commit, build_date, go_version, deps: {key: version}}`. Wired from `-ldflags -X main.commit=…` at build time.

**Severity:** Important. **Effort:** 5× M.

---

## 7. Block 5 — UI polish

- **5.1 Error boundary** — new `<RouteBoundary>` at the Suspense fallback level in `App.tsx`. Catches render errors, shows a card with: error message (sanitized), "Reload this view" button (resets the boundary), "Report" button (opens mailto: or GitHub issue template). Covered by a Vitest test that throws from a mounted child.
- **5.2 Loading / empty / error state pattern** — one reusable trio (`<EmptyState>`, `<LoadingSkeleton>`, `<ErrorState>`) with consistent copy + visuals. Apply to every route that fetches: Home, Notes list/view, Documents list/view, Graph, MCPConsole.
- **5.3 Dynamic `document.title`** — `useDocumentTitle(parts)` hook already exists per Shell; fill it in per route (`Home`, `Notes`, `{noteKey}`, `Documents`, `{doc.title}`, `Graph`, `MCP Console`). All suffixed with ` — docsiq`.
- **5.4 iOS safe-area insets** — Shell header `padding-top: max(var(--header-pad), env(safe-area-inset-top))`; sidebar `padding-left: env(safe-area-inset-left)`. Add viewport meta `viewport-fit=cover` (already present per index.html).
- **5.5 "Maximum update depth exceeded"** — trace via React DevTools Profiler + Playwright console capture. Most likely culprit: a `useEffect` in `Home` or `StatsStrip` whose dep array includes an object created per-render. Fix by memoizing the dep or lifting state.
- **5.6 Axe violations** — Playwright console reports `button-name` on shadcn `SelectTrigger` — add `aria-label` to every `SelectTrigger` callsite lacking a visible label. Re-run axe via a Playwright audit test that asserts `violations.length === 0`.
- **5.7 Reduced-motion** — audit every Framer Motion `<motion.*>` callsite. Gate non-essential animation behind `useReducedMotion()` (hook already exists). Ensure `transition: { duration: 0 }` path when reduced.
- **5.8 Focus management** — command palette returns focus to the invoking element on close; sheet/dialog focus-trap verified; skip-link lands on `main#main` and is the first tab target.
- **5.9 Theme-flash** — inline script in `index.html` `<head>` reading `localStorage.getItem('theme')` (matches Zustand persist key) and applying `document.documentElement.classList` before React hydrates. No FOUC.
- **5.10 Mobile viewport pass** — 375px width manual + Playwright check: sidebar collapses, header tap targets ≥44×44 CSS px, palette fills viewport, tables scroll horizontally instead of overflowing.

**Severity:** Important. **Effort:** 10× S–M.

---

## 8. Block 6 — Testing & CI

- **6.1 Playwright smokes added** — 404 route shows NotFound, unauthed API call triggers expected UI state, upload happy-path end-to-end with stubbed backend.
- **6.2 Pipeline integration test** — full `index` command over a small markdown corpus with a mock LLM provider; asserts SQLite row counts, entity/edge counts, search returns expected hits.
- **6.3 Fuzz targets** — `FuzzSearchTokenize`, `FuzzMCPToolArgs`. Add to existing fuzz-smoke CI job.
- **6.4 `govulncheck` CI job** — matches `~/.claude/rules/security.md` requirement. Runs on every PR that touches Go code.
- **6.5 `npm audit --audit-level=moderate` CI step** — inside the existing UI job. Fails the build on moderate+ CVE.
- **6.6 Flake register** — any `t.Skip` or `test.skip` gets a `// TODO(#issue): <why>` comment with a live issue link, enforced by a CI grep.

**Severity:** Important. **Effort:** 6× S–M.

---

## 9. Block 7 — OSS polish (B-priority)

- **7.1 README** — refactor so the first screen is (i) one-line description, (ii) single-command install from a release binary, (iii) single-command first-index on a sample corpus. Target: user indexes and queries in under 3 minutes. Screenshots of Home and Graph inline.
- **7.2 `CONTRIBUTING.md`** — local dev loop (Go + UI), test commands, pre-commit hook setup, conventional commit style, PR template pointer.
- **7.3 `SECURITY.md`** — report channel (email alias or GitHub private advisory), disclosure policy, supported versions, fix SLA.
- **7.4 `configs/docsiq.example.yaml`** — every option present with inline comments describing purpose, default, env-var override.
- **7.5 Quickstart doc** — `docs/quickstart.md` walks a user through indexing a small sample (`docs/samples/`) and running a search, top-down.
- **7.6 Screenshot gallery** — `docs/screenshots/` with fresh captures of Home, Notes, Documents, Graph, MCP. Referenced from README.
- **7.7 Badge row** — README top: CodeQL, OpenSSF Best Practices (already earned), build, coverage, license, go report card.

**Severity:** Polish. **Effort:** 7× S.

---

## 10. Risks & Open Questions

- **Cookie-based auth (1.1)** — first-run team deploys need a provisioning path for the cookie. Options: CLI subcommand `docsiq login --key X` that sets the cookie in a local file and prints a `curl` recipe for headless use, or a one-time auth page. Need to decide; default: CLI subcommand.
- **workq default size (1.3)** — `runtime.NumCPU()` workers × 64-deep queue is a guess. Real sizing needs a load test; defaults should be documented as starting points.
- **Metrics exporter cost (4.1)** — `prometheus/client_golang` adds ~3–4 MB to the binary. Acceptable given air-gap already vendors dependencies. Confirm before wiring.
- **Secret scrubbing (2.3)** — requires every `Config` field holding a secret to be tagged. One-time audit; easy to regress without a lint. Add a go-vet-style check if feasible.

---

## 11. Dependencies Between Blocks

- 1.3 (workq) and 3.6 (graceful shutdown) share the shutdown-drain path — land 1.3 first, extend in 3.6.
- 1.1 (cookie auth) changes the UI fetch contract — schedule Playwright smoke updates in the same PR.
- 4.1 (metrics) depends on 2.5 (request-ID) for per-request labels.
- 5.5 ("max update depth") is a debugging task; effort may exceed S once root cause is found — re-evaluate after investigation.

---

## 12. Out of Scope (explicitly deferred)

- OIDC/SSO, per-user permissions, per-project ACLs.
- Multi-tenant isolation beyond project namespace.
- Audit-log ingestion into SIEM.
- SLO dashboards, error budget tracking.
- Distributed deployment, HA, replication.
- Any net-new feature, route, or MCP tool.
- Framework or language version bumps not already tracked by Dependabot.

---

## 13. Acceptance

This roadmap is complete when:
- Block 1 all merged and running green on main for 7 days without regression.
- Blocks 2–6 all merged; `govulncheck`, `npm audit`, Playwright, vitest, go test, and UI bundle-budget all green on the HEAD commit.
- Block 7 merged; README first-screen passes the "installs and indexes in 3 minutes on a fresh VM" check.
- User confirms each block before proceeding to the next.
