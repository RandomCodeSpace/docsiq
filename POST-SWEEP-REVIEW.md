# Post-Sweep Review — docsiq

Review against HEAD after the 51-commit quality sweep. Gate check before UI refresh.

## Summary

- **16 findings total: 0 new P0 / 3 new P1 / 4 new P2**
- **All 9 pre-sweep P0/P1 fixes verified as present and correct.**
- No regressions introduced by the sweep itself.
- **Readiness: Ready for UI work, with 3 P1s worth addressing first.**

---

## Pre-sweep fixes — verification (all ✅)

| ID | File | Evidence | Status |
|---|---|---|---|
| P0-1 | `internal/store/store.go:306-323` | Uses `docSelect` (12-col), `scanDocRow` matches | ✅ |
| P0-2 | `internal/api/handlers.go:419-434` | `filepath.Base` + absolute-path containment check | ✅ |
| P0-3 | `internal/api/notes_handlers.go:29-35,471-513` | `MaxImportEntries=10000`, `MaxImportTotalBytes=500<<20` | ✅ |
| P0-4 | `internal/api/vector_indexes.go:65-92` | `singleflight.Do` + double-checked lock | ✅ |
| P1-1 | `internal/api/handlers.go:511-522` | `progressForJob(jobID)` + `clearProgress` on terminal | ✅ |
| P1-2 | `internal/store/store.go:38` | DSN contains `_busy_timeout=5000` | ✅ |
| P1-3 | `internal/api/notes_handlers.go:119-120` | `ErrInvalidKey → StatusBadRequest` | ✅ |
| P1-4 | `internal/notes/history.go:67-70` | `sync.OnceValue` wrapping `gitLookupFn` | ✅ |
| P1-5 | `internal/mcp/notes_tools.go:66-68` | Empty `q` → `toolError("query required")` | ✅ |

---

## New findings

### P1 — should fix

#### NF-P1-1: REST `GET /api/projects/{project}/search` silently accepts empty query — inconsistent with MCP contract

**File:** `internal/api/notes_handlers.go:329-350`

REST `searchNotes` does not reject empty `q`. Passes `q=""` to `st.SearchNotes`, which returns `[]` per `store/notes.go:88-89`. HTTP response is `200 {"hits":[]}` with no error. MCP `search_notes` rejects empty `q` with a tool error.

Asymmetric contract: same logic, different validation per transport.

**Fix:** add `if q == "" { writeError(w, r, http.StatusBadRequest, "query required", nil); return }` before the store call.

**Confidence: 88.**

#### NF-P1-2: Git commit author not sanitised — newline injection into commit messages

**File:** `internal/notes/history.go:144-145`

`buildCommitMessage` concatenates caller-supplied `author` into `Co-Authored-By:` git trailer without stripping `\n`/`\r`. Author value comes from PUT body + MCP `write_note` arg.

Injected newline corrupts the commit log. Doesn't execute code (git trailers aren't commands) but confuses parsers of `git log --pretty`. `TODO(docsiq): P2-4` at line 143 acknowledges this but classified P2 — P1 is more appropriate because `author` is direct API input.

**Fix:** `author = strings.Map(func(r rune) rune { if r == '\n' || r == '\r' { return -1 }; return r }, author)` before assembly.

**Confidence: 85.**

#### NF-P1-3: `docs/rest-api.md` documents `request_id` in JSON error body — code never emits it

**File:** `docs/rest-api.md:157`

Doc states: _"Every error response body is JSON: `{"error": "...", "request_id": "..."}`."_ `writeError` (`handlers.go:73-78`) emits only `{"error": msg}`. Request ID is only in the `X-Request-ID` response header.

Clients parsing the JSON body per docs get missing field. Important to fix **before the UI bakes in assumptions**.

**Fix:** either thread `RequestIDFromContext` into `writeError` and include in JSON, or correct the doc to say the field is header-only.

**Confidence: 95.**

---

### P2 — nice to have / defer

#### NF-P2-1: `docs/mcp-tools.md` says "Docs tools (12)" — code registers 13

**File:** `docs/mcp-tools.md:8`

Summary line off by one. Body lists all 13 correctly. `get_entity_claims` is tool 13.

#### NF-P2-2: `VectorIndexes.ForProject` uses `context.Background()` — drops caller cancellation on 60s build

**File:** `internal/api/vector_indexes.go:75`

```go
buildCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
```

If HTTP client disconnects during cold-start build, build runs 60s consuming CPU + SQLite read bandwidth. Thread `ctx` parameter through `ForProject` to let cancellation propagate.

#### NF-P2-3: MCP `write_note` silently drops `IndexNote` errors (stale FTS)

**File:** `internal/mcp/notes_tools.go:145`

Already TODO-marked (P2-3). Mismatches REST handler which logs WARN. Consequence: MCP write reports success but note doesn't appear in `search_notes`.

#### NF-P2-4: `_ = st.DeleteNote(...)` in REST + MCP delete — stale FTS entries

**Files:** `internal/api/notes_handlers.go:251`, `internal/mcp/notes_tools.go:169`

FTS5 delete failure leaves stale entry — surfaces deleted note in search. Rare in practice (simple DELETE), but inconsistent with `IndexNote` error handling.

**Fix:** log at WARN level.

---

## File-size / separation assessment

| File | LoC | Verdict |
|---|---|---|
| `internal/store/store.go` | 1273 | Coherent. Every method is a thin SQL wrapper. No split warranted. |
| `internal/pipeline/pipeline.go` | 992 | **Highest split candidate.** Phases 2+3 (embed + extract) have enough complexity to warrant `pipeline/embed.go` + `pipeline/extract.go`. Not blocking UI work. |
| `internal/api/handlers.go` | 586 | Upload logic (progress, bg goroutine, tmp-dir) could move to `upload.go`. Rest is coherent. Not urgent. |
| `internal/api/notes_handlers.go` | 529 | Clean single responsibility. Not a split candidate. |
| `internal/mcp/tools.go` | 422 | 13 tool registrations; splitting by domain would produce 3 small files with no shared structure. Acceptable. |

---

## Test skip audit

| Location | Assessment |
|---|---|
| `docs_integration_test.go:96,99,135` — "upload pipeline did not complete" | **Potentially masking a bug.** FakeProvider is deterministic, so pipeline should complete. If it times out consistently in CI, the skip silently passes. Recommendation: assert `waitUploadDone == "done"` in CI, convert to `t.Fatalf` once confirmed reliable. |
| `history_integration_test.go:67,94` — "fewer than 2 entries" | Legitimate. Secondary guard for git-missing envs. |
| `notes_import_limits_test.go:81`, `notes_test.go:230`, `hnsw_test.go:216` — `-short` guards | Correct use of `-short`. Slow tests, not functional logic. |
| `registry_test.go:47,50` — Windows / root chmod | Legitimate platform-conditional skips. |
| `installer_test.go:265` — symlink on Windows | Legitimate. |

---

## Doc drift

| Doc | Claim | Actual | Severity |
|---|---|---|---|
| `docs/rest-api.md:157` | Error JSON has `request_id` field | Only `{"error":"..."}` emitted | P1 (NF-P1-3) |
| `docs/mcp-tools.md:8` | "Docs tools (12)" | 13 doc tools registered | P2 (NF-P2-1) |
| `docs/config.md` | Fields match `config.go` | No drift | ✅ |
| `docs/rest-api.md` route table | All routes listed | All `router.go` routes covered | ✅ |

---

## Error-swallowing assessment

| Site | Verdict |
|---|---|
| `mcp/notes_tools.go:145` `_ = st.IndexNote` | Bug-worthy (P2-3 TODO). Fix: log WARN. |
| `mcp/notes_tools.go:169` `_ = st.DeleteNote` | Bug-worthy (NF-P2-4). Fix: log WARN. |
| `api/notes_handlers.go:251` `_ = st.DeleteNote` | Same as above for REST. |
| `api/notes_handlers.go:524` `_ = st.IndexNote` (import path) | Acceptable: best-effort; re-import retries. |
| `notes/watcher.go:71` `_ = filepath.WalkDir` | Acceptable: per-entry errors absorbed; next poll retries. |
| `api/auth.go:83` — 401 response write | Acceptable: client already disconnected. |
| `api/router.go:168` — SPA fallback write | Acceptable: headers may be sent. |
| `api/metrics.go:159` — Prometheus text write | Acceptable: scraper disconnect. |

---

## Concurrency correctness

All hotspots re-read:
- `internal/notes/history.go` per-project mutex map — safe; `perProjectLocksMu` guards all access.
- `internal/notes/watcher.go` start/stop — safe; channel-close-once pattern.
- `internal/api/stores.go` Get — safe; mutex held for full get+insert.
- `internal/sqlitevec/load.go` extension loading — safe with `MaxOpenConns=1`.

---

## Context cancellation

| Site | Assessment |
|---|---|
| `handlers.go:467` `bgCtx := context.Background()` | Intentional (background goroutine must outlive HTTP response). Correct. |
| `vector_indexes.go:75` `context.Background()` | Drops caller ctx on 60s build. **NF-P2-2.** |
| `sqlitevec/load.go:55` `context.Background()` | Startup-only; acceptable. |
| `loader/pdf.go:30` `context.Background()` | Drops pipeline ctx. Low priority. |
| All upload/search handlers | `r.Context()` threaded correctly. |

---

## Security surface

- **Bearer auth:** `/health` + `/metrics` public by design; `/api/*` + `/mcp` gated. No new bypass.
- **Path traversal:** Upload filenames, tar entries, note keys, hook installer paths — all addressed.
- **CSRF:** No state-changing GETs. Token-auth mitigates.
- **Error leakage:** `writeError` sends only human `msg` to client; internal details go to slog. One minor: `resolveStore` sends `"open project store: "+err.Error()` which can include a filesystem path in a 500 body (low severity, auth-gated).

---

## API contract sanity

- `GET /api/projects/{project}/search` accepts empty `q` (NF-P1-1 — inconsistent with MCP).
- Project middleware returns `http.Error` plain text on invalid slug instead of standard JSON error shape. Minor.
- `/health` + `/api/stats` policies correct.

---

## Dependency health

- `go.uber.org/goleak v1.3.0` — used in `itest/harness.go`. Justified.
- `golang.org/x/sync v0.20.0` — used for `singleflight` (P0-4 fix). Correct.
- Indirect deps via `langchaingo` — not directly imported.
- `go 1.25.0` in go.mod — valid (Go 1.25 released Aug 2025).

---

## Readiness for UI work

**Ready, with caveats.**

The 9 pre-sweep P0/P1 fixes are all present and correct. No new P0 found. Three new P1s are correctness gaps, not blockers:

1. **NF-P1-1** (REST search empty query) — UI may rely on consistent validation behavior
2. **NF-P1-2** (git author sanitization) — not UI-facing but security-adjacent
3. **NF-P1-3** (`request_id` doc drift) — **UI should NOT bake in assumptions about the doc'd error shape; fix first**

Recommend fixing all 3 before starting UI refresh to avoid contract churn mid-UI-work.
