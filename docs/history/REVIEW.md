# Code Review — docsiq quality sweep

## Summary
- **14 findings total: 4 P0 / 5 P1 / 5 P2**
- **Wave-E remediation: all 4 P0 and all 5 P1 findings fixed (9/9). P2s deferred per Wave-E scope.**
- Packages audited: `internal/api`, `internal/notes`, `internal/store`, `internal/vectorindex`, `internal/mcp`, `internal/hookinstaller`, `internal/sqlitevec`, `cmd`
- Lines audited: ~12,000

---

## P0 — must fix

### [P0-1] `GetDocumentVersions` column-count mismatch causes runtime error on every call — `internal/store/store.go:304`

**What:** `GetDocumentVersions` uses an inline `SELECT` that omits `indexed_mtime` (11 columns), but passes rows into `scanDocRow` which calls `rows.Scan` with 12 destinations.

**Impact:** Every call to `GET /api/documents/{id}/versions` fails with `sql: expected 12 destination arguments in Scan, not 11`. The endpoint is completely broken.

**Evidence:**
```
store.go:304-308  — inline query, 11 cols, NO indexed_mtime:
  SELECT id,path,title,doc_type,file_hash,structured,version,canonical_id,is_latest,created_at,updated_at
  FROM documents WHERE id=? OR canonical_id=? ORDER BY version ASC

store.go:331-332  — scanDocRow, 12 Scan destinations:
  rows.Scan(&d.ID,&d.Path,&d.Title,&d.DocType,&d.FileHash,&d.Structured,
      &d.Version,&canonicalID,&isLatest,&indexedMtime,&d.CreatedAt,&d.UpdatedAt)

store.go:324     — docSelect is the correct 12-col query, but
  GetDocumentVersions does NOT use docSelect.
```

**Recommended fix:** Replace the inline query with `docSelect + " WHERE id=? OR canonical_id=? ORDER BY version ASC"` to match every other doc query function.

**Status:** fixed in 0cd9a97

---

### [P0-2] Path traversal via multipart filename in upload handler — `internal/api/handlers.go:409`

**What:** The upload handler writes each file to `filepath.Join(tmpDir, fh.Filename)` without sanitizing `fh.Filename`. A multipart upload with `filename="../../etc/cron.d/evil"` escapes the temp directory.

**Impact:** An authenticated attacker can write arbitrary files to any path writable by the server process, enabling privilege escalation or persistent backdoor installation.

**Evidence:**
```go
// handlers.go:408-409
for _, fh := range files {
    dst := filepath.Join(tmpDir, fh.Filename)  // no containment check
    // ...
    out, err := os.Create(dst)
```
`filepath.Join("/tmp/docsiq-upload-abc", "../../etc/cron.d/pwn")` evaluates to `/etc/cron.d/pwn`. No path containment assertion follows before `os.Create(dst)`.

**Recommended fix:** Sanitize `fh.Filename` to `filepath.Base(fh.Filename)` (stripping all directory components) before joining, AND assert `strings.HasPrefix(absDst, absTmpDir+string(os.PathSeparator))` as defense-in-depth. Reject entries whose sanitized name is empty, `.`, or `..`.

**Status:** fixed in 42109f7

---

### [P0-3] No total decompressed-size cap in tar import enables OOM — `internal/api/notes_handlers.go:448-502`

**What:** `importTar` enforces a per-entry cap of `MaxNoteBytes` (10 MB) but imposes no limit on the number of entries or total decompressed bytes across the archive. A crafted `.tar.gz` with thousands of near-maximum-size entries can exhaust server memory.

**Impact:** Any authenticated user can crash the server by uploading a crafted archive (a gzip bomb or simply a very large tar), causing OOM and denial of service.

**Evidence:**
```go
// notes_handlers.go:479-488 — per-entry limit only:
data, err := io.ReadAll(io.LimitReader(tr, MaxNoteBytes+1))
if len(data) > MaxNoteBytes { ... return }
// No entry count limit, no running total of bytes decompressed.
// A tar with 10,000 entries × 10MB = 100GB of decompressed data.
```

**Recommended fix:** Add a counter for total bytes decompressed AND a maximum entry count. Suggest `MaxImportEntries = 10_000` and `MaxImportTotalBytes = 500 << 20` (500 MB). Return 413 when either limit trips.

**Status:** fixed in 06960fc

---

### [P0-4] TOCTOU race in `VectorIndexes.ForProject` — `internal/api/vector_indexes.go:35-63`

**What:** `ForProject` checks the cache under the mutex, releases it, builds the index (a potentially 60-second operation), then re-acquires. Two concurrent goroutines for the same slug both pass the first empty-cache check and both call `vectorindex.BuildFromStore` simultaneously. The store has `MaxOpenConns=1`, so both builds compete for the single SQLite connection, blocking each other and all other requests to that project for up to 120s combined.

**Impact:** Under concurrent first-search load for a project (common at startup with multiple simultaneous clients), API requests to that project stall for the full 60-second build window × number of racing goroutines.

**Evidence:**
```go
// vector_indexes.go:39-44 — check-then-unlock:
v.mu.Lock()
if idx, ok := v.indexes[slug]; ok {
    v.mu.Unlock()
    return idx
}
v.mu.Unlock()   // <-- released here; another goroutine enters the same block

// Both call BuildFromStore concurrently:
idx, err := vectorindex.BuildFromStore(buildCtx, st)  // up to 60s, uses the single DB conn
```

**Recommended fix:** Use `golang.org/x/sync/singleflight` keyed by slug so only one build runs for a given slug at a time; all other callers wait on the same result. Alternative: an in-cache placeholder `*Index` guarded by a sync.WaitGroup so concurrent callers block on the in-flight build.

**Status:** fixed in 9ea058e

---

## P1 — should fix

### [P1-1] `uploadProgress` SSE stream: `jobProgress` map never pruned and multi-job tracking is broken — `internal/api/handlers.go:469-517`

**What:** Completed job entries are never removed from `h.jobProgress`, so the map grows without bound. When multiple uploads run concurrently, the SSE poll loop picks an arbitrary job's status (Go map iteration is non-deterministic) and terminates the stream when any job reaches `"done"` — even if the caller's own job is still running.

**Impact:** (a) Memory leak in long-running servers with many uploads. (b) Client polling `GET /api/upload/progress` with two concurrent jobs may receive premature `done` for a job that hasn't finished, or miss their own job's error events.

**Evidence:**
```go
// handlers.go:499-514 — picks last value from unordered map iteration:
for _, v := range h.jobProgress {
    msg = v   // non-deterministic when len > 1
}
if msg == "done" || strings.HasPrefix(msg, "error:") {
    return    // terminates for any job, not just caller's
}
// setProgress() at line 469 adds entries; nothing ever deletes them.
```

**Recommended fix:** Add a `?job_id=` query parameter to `uploadProgress`, filter `jobProgress` by that ID, delete the entry from the map when `"done"` or `"error:"` is emitted. Return the `job_id` from the upload response so clients can correlate.

**Status:** fixed in aabb50c

---

### [P1-2] SQLite DSN missing `_busy_timeout`; concurrent requests return immediate `SQLITE_BUSY` — `internal/store/store.go:35`

**What:** The SQLite connection string sets WAL and foreign keys but does not set `_busy_timeout`. With `MaxOpenConns=1`, any second concurrent database operation on the same project store gets an immediate `database is locked` / `SQLITE_BUSY` error instead of retrying.

**Impact:** Under concurrent API load (search + upload finalize), one operation fails with a 500 error that wouldn't occur with a busy timeout set.

**Evidence:**
```go
// store.go:35 — no _busy_timeout:
db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
```

**Recommended fix:** Add `&_busy_timeout=5000` (5 seconds) to the DSN in `open()`.

**Status:** fixed in 619243a

---

### [P1-3] `ErrInvalidKey` mapped to HTTP 403 Forbidden instead of 400 Bad Request — `internal/api/notes_handlers.go:105-116`

**What:** The `notesError` helper maps `notes.ErrInvalidKey` to `http.StatusForbidden` (403). An invalid note key (contains `..`, null byte, etc.) is a client input validation error, not an authorization failure.

**Impact:** Clients and monitoring systems interpret 403 as "you don't have permission" rather than "your request is malformed". Client retry logic behaves incorrectly.

**Evidence:**
```go
// notes_handlers.go:109-110:
case errors.Is(err, notes.ErrInvalidKey):
    writeError(w, r, http.StatusForbidden, "invalid key: "+err.Error(), err)
```
Same mapping appears in `writeNote` at line 172 and `deleteNote` at line 233.

**Recommended fix:** Change `http.StatusForbidden` → `http.StatusBadRequest` (400) for `ErrInvalidKey` cases. Reserve 403 for authorization failures.

**Status:** fixed in 619243a

---

### [P1-4] `gitAvailable()` calls `exec.LookPath` on every note write — `internal/notes/history.go:56-60`

**What:** `autoCommit` calls `gitAvailable()` on every note write or delete, which calls `exec.LookPath("git")`. This is a filesystem `stat` syscall chain executed synchronously on the hot write path.

**Impact:** On systems where git is not on `PATH` (common in containers), this performs a full `PATH` directory scan on every write. Latency spikes in profiling.

**Evidence:**
```go
// history.go:56-60:
func gitAvailable() bool {
    _, err := exec.LookPath("git")
    return err == nil
}
// Called at history.go:138 inside autoCommit, per Write() and Delete().
```

**Recommended fix:** Cache the result in a `sync.Once` at package level. Git binary location doesn't change during server lifetime.

**Status:** fixed in 619243a

---

### [P1-5] `search_notes` MCP tool passes empty query without validation — `internal/mcp/notes_tools.go:61-79`

**What:** The `search_notes` MCP tool extracts `query` and calls `st.SearchNotes(ctx, q, limit)` even when `q` is empty, relying on the store's internal short-circuit. Diverges from all other MCP tools (`search_documents`, `local_search`) which return `toolError("query required")` for empty queries.

**Impact:** MCP client gets silent `{"hits":[]}` instead of a clear error. Inconsistent behavior across tools.

**Evidence:**
```go
// notes_tools.go:63-65 — no guard:
q := stringArg(args, "query", "")
limit := intArg(args, "limit", 20)

// Compare search_documents (tools.go:49-51):
if query == "" {
    return toolError(fmt.Errorf("query required")), nil
}
```

**Recommended fix:** Add `if q == "" { return toolError(fmt.Errorf("query required")), nil }` after extracting `q`.

**Status:** fixed in 619243a

---

## P2 — nice to have / defer

### [P2-1] `ParseMultipartForm` memory limit does not bound total upload size — `internal/api/handlers.go:384`

**What:** `r.ParseMultipartForm(128 << 20)` buffers up to 128 MB in memory. No `http.MaxBytesReader` wraps `r.Body`, so a client can stream an arbitrarily large upload body. Files exceeding 128 MB spill to disk rather than being rejected.

**Recommended fix:** Wrap `r.Body` with `http.MaxBytesReader(w, r.Body, maxBytes)` before `ParseMultipartForm`. Make the max configurable via `cfg.Server.MaxUploadBytes`.

**Status:** deferred (P2), TODO planted at internal/api/handlers.go:384

---

### [P2-2] `/metrics` endpoint is unauthenticated while exposing operational details — `internal/api/router.go:84`, `internal/api/metrics.go:130`

**What:** Prometheus `/metrics` is mounted outside the auth wrap. It exposes project names, note counts, request paths, and latency distributions to any unauthenticated caller.

**Recommended fix:** Either document this as intentional (Prometheus convention) or add an optional separate scrape token via `cfg.Server.MetricsKey`.

**Status:** deferred (P2), TODO planted at internal/api/router.go:84 and internal/api/metrics.go:130

---

### [P2-3] `write_note` MCP tool silently swallows `IndexNote` errors — `internal/mcp/notes_tools.go:141`

**What:** MCP `write_note` calls `_ = st.IndexNote(ctx, n)`, discarding the error. REST handler logs `slog.WarnContext` on the same error. Inconsistent — FTS5 index failures invisible through MCP.

**Recommended fix:** Add `slog.WarnContext(ctx, "notes FTS index failed", "key", key, "err", err)` to match the REST handler.

**Status:** deferred (P2), TODO planted at internal/mcp/notes_tools.go:144

---

### [P2-4] `buildCommitMessage` does not sanitize author for git trailer injection — `internal/notes/history.go:119-127`

**What:** Commit message includes `Co-Authored-By: <author> <<author>@local>`. `author` comes from API request body. A value containing newlines can inject additional git trailer lines.

**Recommended fix:** Strip `\n` and `\r` from author before embedding. Alternatively, reject author values containing control chars at the handler level.

**Status:** deferred (P2), TODO planted at internal/notes/history.go:142

---

### [P2-5] `VectorIndexes.ForProject` builds index outside the mutex without singleflight — duplicate work — `internal/api/vector_indexes.go:46-62`

**What:** Even post-P0-4 fix, absent singleflight two goroutines racing for the first build both complete their builds and the second result is discarded. Wasted CPU/IO for a 60s build.

**Recommended fix:** Use `golang.org/x/sync/singleflight` keyed by slug to deduplicate concurrent builds. (P0-4's fix likely subsumes this — verify.)

**Status:** resolved by P0-4 fix in 9ea058e (singleflight.Group now coalesces concurrent first-touch builds in `ForProject`; no TODO planted).

---

## What looks good

- **`projectStores` cache** (`internal/api/stores.go`): mutex held for full get-or-open, no TOCTOU gap. `Close()` drains the map under the same lock.
- **`ValidateKey` + `resolvePath`** (`internal/notes/notes.go:51-104`): thorough traversal defense — segment-by-segment `..` check + post-`Abs` containment assertion. Defense-in-depth correctly layered.
- **`importTar` traversal rejection** (`internal/api/notes_handlers.go:457-473`): `filepath.Clean` + double containment check correctly blocks tar path traversal attacks.
- **`bearerAuthMiddleware`** (`internal/api/auth.go:64`): `crypto/subtle.ConstantTimeCompare` used correctly; never logs the submitted token; case-sensitive scheme intentional and documented.
- **`Write` atomicity** (`internal/notes/notes.go:209-231`): temp file + `Sync` + rename correctly implemented; temp cleaned on all error paths; per-project mutex serializes write + auto-commit.
- **HNSW concurrency** (`internal/vectorindex/hnsw.go`): `Add` and `Search` acquire the appropriate mutex; `rng` only accessed from `Add` holding the write lock.
- **Schema migration** (`internal/store/store.go:82-103`): idempotent `ALTER TABLE` with duplicate-column error catch handles SQLite's error message correctly.
- **Hook installer atomicity** (`internal/hookinstaller/installer.go:100-143`): temp-file + rename prevents corrupt config on crash; symlink resolution lands temp file on the same filesystem as target.
