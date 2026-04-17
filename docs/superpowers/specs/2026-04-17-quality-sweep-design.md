# docsiq — Quality Sweep Design Spec

**Date:** 2026-04-17
**Status:** Ready for user review
**Author:** Claude Opus 4.7 (session orchestrator)
**Source of requirements:** brainstorming session (scope A + Medium testing depth approved by user)

---

## 1. Goal

Close all known correctness, test-coverage, and documentation gaps surfaced by the port of kgraph into docsiq (the 7 feature commits `3d2d2ce..790810f` on `main`), without expanding feature scope. The result: a product that is *correct* and *defensible*, not a product that is *larger*.

**Explicitly in scope:** code review + remediation, frontend tests, integration tests, hook schema verification against real client docs, MCP global_search per-project LLM adoption, docs cleanup (CLAUDE.md, UNIFICATION-PLAN.md, kgraph archive pointer), user-facing docs directory, lint modernization sweep.

**Explicitly out of scope** (deferred to future cycles, even if requested mid-execution):
- Additional LLM providers beyond the shipped three (Azure / Ollama / OpenAI direct)
- Markdown renderer features beyond what Wave B's tests mandate
- Entity resolution / deduplication
- Multi-user auth / RBAC
- `goreleaser` / prebuilt binary releases
- Windows support
- Dropping real `sqlite-vec` binaries (user action)
- Renaming the GitHub repo `docscontext` → `docsiq` (user action)

---

## 2. Architecture & Execution Model

**4 parallel initial waves + 2 sequential remediation waves + final verification.**

```
Wave A (solo, read-only)      Wave B (solo, ui/**)
  → code-review                 → Vitest + RTL + tests
  → REVIEW.md                   → component coverage ≥ 70%

Wave C (solo, *_integration_test.go)   Wave D (solo, 4 sub-steps)
  → httptest harness                     D1: hook schema fixes
  → 60+ integration subtests             D2: MCP per-project LLM
                                         D3: content cleanup
                                         D4: docs/ directory
             ↓ all 4 land
Wave E (serial, reads REVIEW.md)
  → fixes P0/P1; adds TODO(docsiq) for P2
Wave F (serial, lint sweep)
  → rangeint, stringsseq, mapsloop, b.Loop, any, min/max
Final verification → commit + push
```

### File-isolation contracts

Prevent merge conflicts between the parallel waves A–D:

| Wave | Writes only to |
|---|---|
| A | `REVIEW.md` (repo root) |
| B | `ui/**` (source, config, new `__tests__/` dirs), `ui/package.json`, `ui/package-lock.json` |
| C | `internal/**/*_integration_test.go`, new package `internal/api/itest/**`, `Makefile` (add `test-integration` target) |
| D | `internal/hookinstaller/**`, `internal/mcp/**` (D2 only), `README.md`, `CLAUDE.md`, `docs/**`, `../UNIFICATION-PLAN.md` (parent dir), `../kgraph/ARCHIVED.md` (sibling repo) |

Waves E + F run *after* A–D, in serial, and may touch any file.

### Severity rubric

- **P0** — bugs, data-loss risks, auth bypass, race conditions → must fix
- **P1** — correctness gaps, wrong error handling, misleading docs → must fix
- **P2** — style, refactor opportunities, nice-to-haves → comment + defer

### Agent assignments

| Wave | Agent type | Solo or parallel |
|---|---|---|
| A | `feature-dev:code-reviewer` | solo |
| B | `general-purpose` | parallel with A, C, D |
| C | `general-purpose` | parallel with A, B, D |
| D | `general-purpose` | parallel with A, B, C |
| E | `general-purpose` | serial after A–D |
| F | `general-purpose` | serial after E |

---

## 3. Wave A — Code Review

**Agent:** `feature-dev:code-reviewer`, read-only.

**Scope:** All 7 feature commits + rename commit since session start (~12,000 LoC).

**Review axes (explicit checklist the agent follows):**
- **Correctness** — race conditions (per-project store cache, HNSW index map, note auto-commit mutexes), silent error-drop paths, FTS5 snippet/rank off-by-one, goroutine lifecycle on shutdown
- **Security** — auth bypass, path traversal (note keys, tar import, hook installer paths), token logging, CSRF-equivalent on REST
- **Concurrency** — `projectStores` + `VectorIndexes` cache races, note auto-commit under concurrent writers, shutdown ordering
- **Resource leaks** — unclosed `*sql.DB` / `*sql.Rows` / readers, tmpdir cleanup, goroutine leaks in upload-progress endpoint
- **API contract** — HTTP status codes per endpoint, JSON shape per response, MCP tool arg validation
- **Test coverage gaps** — paths not exercised by existing 424 subtests
- **Consistency** — style drift between sub-agents' work, duplicated logic, naming

**Deliverable — `REVIEW.md` at repo root:**

```
# Code Review — docsiq quality sweep

## Summary
- N findings total: X P0 / Y P1 / Z P2
- Packages audited: [list]
- Lines audited: ~12k

## P0 — must fix
### [P0-1] <title> — <file:line>
**What:** one-sentence description
**Impact:** concrete consequence
**Evidence:** code excerpt or test scenario
**Recommended fix:** approach, not code

## P1 — should fix
... same shape ...

## P2 — nice to have / defer
... same shape ...

## What looks good
<short intentional non-findings to confirm reviewer saw something and decided it was OK>
```

**Wave E consumes this doc directly.** P0 and P1 are fixed in code; P2 gets a `// TODO(docsiq): P2-<N> <short>` comment at the site.

**Acceptance:** `REVIEW.md` exists with either findings or explicit "clean" statement. Severity distribution is plausible (>0 findings expected in 12k LoC).

---

## 4. Wave B — Frontend Tests

**Stack additions** (all dev-only, zero production-bundle impact):
- `vitest` — Vite-native runner with jsdom env
- `@testing-library/react`
- `@testing-library/user-event`
- `@testing-library/jest-dom`
- `jsdom`

**Layout:** `ui/src/**/__tests__/<Component>.test.tsx` colocated with the component.

**Coverage targets:**

| Component | Required test cases |
|---|---|
| `FolderTree` | renders tree; click loads note; `+` opens modal; folder context menu; Enter submits; Esc cancels; invalid key inline-rejected |
| `NoteView` | markdown: headings, bold, italic, code, lists, wikilinks; **new**: links, images, blockquotes, tables, HR, inline math; wikilink click → onNavigate; empty body; frontmatter stripped |
| `NoteEditor` | body textarea updates; tag input parses comma-separated; save → writeNote; dirty-flag warns |
| `LinkPanel` | inbound + outbound links; empty state; click navigates |
| `NotesGraphView` | renders N nodes; empty state; note-accent color applied |
| `NotesSearchPanel` | input debounces; results render; snippet highlighting; click navigates; count + ms |
| `UnifiedSearchPanel` | both /api/search + /api/projects/{p}/search in parallel; merged labels; empty results |
| `TopNav` | tabs + project selector render; `?tab=` + `?project=` URL sync |
| `App` | initial tab from URL; switching tabs updates URL; switching projects reloads hooks |
| `useNotes`, `useProjects`, `useNotesSearch`, `useNotesGraph`, `useNotesTree` | correct URLs; error + loading states; reload refetches |

**Coverage floor:** 70% statements / 60% branches on `ui/src/components/{notes,nav,shared}/**` and `ui/src/hooks/**`, enforced via `vitest --coverage` with `fail-under`.

**Wiring:**
- `npm --prefix ui test` → Makefile `check` target
- `ci.yml` → add step after `ui-freshness` job's build

---

## 5. Wave C — Integration Tests

**Goal:** catch bugs that unit tests can't — middleware ordering, ctx propagation, store cache races, MCP JSON-RPC round-trips, concurrent-request isolation.

**Structure:**
- New package `internal/api/itest/` for helpers (`harness.go`, `doubles.go`)
- `*_integration_test.go` files under existing packages, guarded by `//go:build integration` tag
- Makefile adds `test-integration` target: `CGO_ENABLED=1 go test -tags "sqlite_fts5 integration" -timeout 600s ...`

**Harness API** (not prescribing implementation, just shape):

```go
type TestEnv struct {
    Server   *httptest.Server   // full router + auth + project middleware
    DataDir  string             // tempdir, auto-cleaned
    Registry *project.Registry
    Stores   api.Storer
    APIKey   string             // random, set on server
    Client   *http.Client       // pre-configured with Authorization: Bearer
}
func New(t *testing.T, opts ...Opt) *TestEnv
// Convenience helpers per endpoint
func (e *TestEnv) PUTNote(project, key, content string, tags []string) *http.Response
// ... etc
// MCP over JSON-RPC against the real streamable HTTP server
func (e *TestEnv) MCPCall(tool string, args map[string]any) (any, error)
```

**Test suites — one file per concern:**

| File | Verifies |
|---|---|
| `auth_integration_test.go` | bearer required on `/api/*` + `/mcp`; UI + `/health` public; OPTIONS bypass; concurrent auth-failure no race |
| `project_integration_test.go` | `?project=` isolation E2E; `_default` auto-registers; unknown project → 404 |
| `notes_integration_test.go` | PUT→GET→DELETE; wikilink graph updates on write; FTS5 search finds new notes; tar export/import round-trip preserves tree |
| `docs_integration_test.go` | upload → index → search (with mock LLM); per-project doc isolation |
| `mcp_integration_test.go` | every MCP tool via JSON-RPC: note tools, doc tools, `stats`, claims |
| `concurrency_integration_test.go` | 100 parallel PUTs to distinct keys in same project; 50 parallel reads during upload; store cache safety; per-project mutex prevents note-history git races |
| `history_integration_test.go` | write twice → history has 2 commits; delete creates commit; missing git binary → write still succeeds |
| `hooks_integration_test.go` | POST /api/hook/SessionStart with registered + unknown remotes; response matches Claude Code hook spec |
| `shutdown_integration_test.go` | SIGTERM mid-request → graceful drain; all stores close; no goroutine leaks via `go.uber.org/goleak` |
| `metrics_integration_test.go` | `/metrics` parses as Prometheus text; counters increment on requests; label cardinality bounded |

**Scope limits (keeps Medium, not Deep):**
- Harness wraps `api.NewRouter` in `httptest.NewServer`. **No subprocess.**
- LLM provider is a test double.
- Embedder is deterministic stub.
- Real git binary used for history tests; skipped with `t.Skip` if missing.

**Target:** 60+ integration subtests.

**Potential new dep:** `go.uber.org/goleak` for shutdown leak detection. Single well-known dep; justified.

---

## 6. Wave D — Hooks + MCP + Docs

One agent, 4 sub-steps sequentially.

### D1 — Hook schema verification

For each AI client, WebFetch the authoritative docs:

| Client | Docs URL pattern |
|---|---|
| Claude Code | `docs.claude.com/en/docs/claude-code/hooks` |
| Cursor | `docs.cursor.com/` (search hooks / rules / MCP config) |
| GitHub Copilot CLI | `docs.github.com/en/copilot/` + `github.com/github/gh-copilot` |
| OpenAI Codex CLI | `github.com/openai/codex` README + configs |

For each:
1. **If docs confirm the current Go installer schema** → add fixture-based test, done.
2. **If docs show a different schema** → fix `internal/hookinstaller/<client>.go` + update fixture + add migration note in comment.
3. **If the client has no documented hook API** → mark with prominent `// UNVERIFIED — <client> does not publicly document a SessionStart hook API` header, install-time WARN log, AND a "Hook support matrix" in `README.md` with per-client status.

All installers get `internal/hookinstaller/fixtures/<client>/{before,after}.json` test fixtures.

### D2 — MCP `global_search` per-project LLM adoption

Thread `project` arg through `mcp.globalSearch` handler, resolve `provider := llm.ProviderForProject(cfg, project)`, pass into `search.GlobalSearch`. Add MCP integration test (runs in Wave C) that two projects with different provider overrides produce different provider-name fingerprints in the search output.

### D3 — Content cleanup

- `CLAUDE.md` — rebrand every `DocsContext` / `docscontext` / `~/.docscontext/` / `DOCSCONTEXT_*` reference; drop the "Recent Changes (already committed)" section about the superseded fix branch.
- `/home/dev/projects/docsiq/UNIFICATION-PLAN.md` — rebrand all references; add a `STATUS: Implemented` banner at the top listing the commit range (`3d2d2ce..790810f`).
- `/home/dev/projects/docsiq/kgraph/ARCHIVED.md` — new file:
  > This TypeScript codebase was ported into Go as `docsiq` on 2026-04-17. See `github.com/RandomCodeSpace/docsiq`. This repo is kept for historical reference only; all active development happens in docsiq.

### D4 — User-facing docs directory

New `docs/` directory at repo root:

| File | Contents |
|---|---|
| `docs/README.md` | entry point; links to every guide |
| `docs/getting-started.md` | install prerequisites (Go, C toolchain), `docsiq init`, first project |
| `docs/cli-reference.md` | every subcommand with flags + examples |
| `docs/mcp-tools.md` | all 19 MCP tools (12 doc + 7 notes) with arg/return JSON shapes |
| `docs/rest-api.md` | every REST endpoint with method, path, body shape, status codes |
| `docs/config.md` | every env var + config field |
| `docs/hooks.md` | hook support matrix + install instructions + troubleshooting |
| `docs/architecture.md` | one Mermaid / ASCII diagram + 2-page explanation (per-project layout, store + registry + HNSW flow) |

Where possible, content is generated from source (MCP tool list scraped from `internal/mcp/tools.go`, REST routes scraped from `internal/api/router.go`) to minimize drift. Agent implements this as manual first-pass; a future codegen is out of scope.

---

## 7. Wave E — Review Remediation

Serial, after A–D land.

**Agent reads `REVIEW.md` and:**
- For every **P0** finding: implement recommended fix + add regression test (if possible) + one commit per finding for traceability.
- For every **P1** finding: implement fix + test, grouped thematically into commits.
- For every **P2** finding: add `// TODO(docsiq): P2-<N> <short summary>` comment at the referenced site.
- If the agent believes a finding is incorrect: push back inline in `REVIEW.md` under the finding's `**Status:**` line with reasoning. User sees this in spec review.
- Updates `REVIEW.md` per-finding: `**Status:** fixed in <sha>` / `**Status:** deferred (P2)` / `**Status:** disputed — <reason>`.

**Constraint:** every P0/P1 fix that can be covered by a test MUST be. No "fixed but untested" P0 or P1 ships.

---

## 8. Wave F — Lint Modernization

Serial, after Wave E.

One agent sweeps Go 1.24 style hints:
- `rangeint` — `for i := 0; i < n; i++` → `for i := range n`
- `stringsseq` — `strings.Split` in range → `strings.SplitSeq`
- `mapsloop` — manual copy loop → `maps.Copy`
- `b.N` in benchmarks → `b.Loop()`
- `interface{}` → `any`
- Unused user-defined `min`/`max` → builtins
- `unusedfunc` false positives → `//nolint:unusedfunc` where the function is only referenced via composition

Mechanical, surgical, no logic changes. Runs `make vet && make test` after each file batch. Zero regression tolerated.

---

## 9. Completion Criteria (Ship Gates)

Before pushing to `origin/main`, ALL of the following must be true:

- ✅ 0 P0 findings open in REVIEW.md
- ✅ 0 P1 findings open in REVIEW.md
- ✅ Every P0/P1 fix has a regression test (or explicit justification in commit msg)
- ✅ `make test` green (existing + new)
- ✅ `make test-integration` green (60+ new integration subtests)
- ✅ `npm --prefix ui test` green; coverage ≥ 70% statements on `ui/src/components/notes/**` and `ui/src/hooks/**`
- ✅ `make vet` clean
- ✅ All Wave D content cleanup landed: CLAUDE.md / UNIFICATION-PLAN.md / kgraph/ARCHIVED.md / `docs/**`
- ✅ Hook installer support matrix in README.md with per-client status (`verified` / `unverified`)
- ✅ `grep -r 'docscontext' internal/ cmd/` returns 0 (rename-consistency guard)
- ✅ `grep -r 'TODO(docsiq): P[01]' internal/ cmd/` returns 0 (P0/P1s all addressed)
- ✅ `git log` shows sensible commit messages; P0 commits reference `REVIEW.md` finding IDs
- ✅ CI green after push (watched, not assumed)

---

## 10. Risks & Mitigations

- **Risk:** Wave A returns 30+ findings; Wave E balloons in scope.
  - *Mitigation:* Severity rubric is strict. Only P0/P1 fix-mandatory. P2 → TODOs, even if numerous. If P0+P1 combined > 20 findings, this is itself a signal the codebase needs a second review cycle — pause and escalate to user before executing Wave E.

- **Risk:** Integration test harness exposes pre-existing bugs that weren't covered by the 424 unit subtests.
  - *Mitigation:* Any such bug is filed as P0 and fixed in Wave E, not ignored. This is exactly what the quality sweep is meant to surface.

- **Risk:** WebFetch for hook docs returns stale or inaccurate info; real clients behave differently.
  - *Mitigation:* Installers that can't be verified get the `UNVERIFIED` marker + README matrix entry. Zero silent acceptance of unverified behavior.

- **Risk:** Wave B's coverage floor fails because of a mis-configured vitest — wastes time debugging tooling instead of closing gaps.
  - *Mitigation:* Start Wave B with a smoke test (trivial component) to validate the vitest + coverage reporter pipeline BEFORE writing all component tests.

- **Risk:** `go.uber.org/goleak` introduces false positives from stdlib goroutines on some platforms.
  - *Mitigation:* Use `goleak.IgnoreCurrent()` at test start to baseline. Dep is only used in one test file; trivial to remove if problematic.

- **Risk:** Parallel waves A–D race on LSP/indexer state; merge conflicts surface despite file-isolation contracts.
  - *Mitigation:* File-isolation contracts are explicit and enforced by agent instructions. If a conflict arises, serialize the offending wave rather than trying to merge. Every wave must finish with `git status --short` confirmation that only its allowed files changed.

---

## 11. Handoff to writing-plans

Once the user approves this spec, I invoke the `superpowers:writing-plans` skill to break each Wave into numbered, testable implementation steps with explicit acceptance criteria per step. That plan doc becomes the live execution artifact — a checklist the orchestrator (me) runs against.
