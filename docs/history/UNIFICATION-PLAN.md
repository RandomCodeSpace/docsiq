# docsiq — Unification Review & Plan

> **STATUS: Implemented.** This plan was executed 2026-04-17 across commits 3d2d2ce..HEAD on main of the docsiq repo (quality-sweep work is ongoing). Retained for historical reference.

**Date:** 2026-04-17
**Scope:** Deep review of `docsiq/` and `kgraph/`, assessment of feature completeness,
and a plan for combining them into a single coherent product.
**Status:** Planning only — no code changes proposed in this document.

---

## 1. Executive Summary

Two sibling repos live under `/home/dev/projects/docsiq/`:

| Repo | Language | Role | LoC (approx) |
|---|---|---|---|
| `docsiq/` | Go 1.22 + React/Vite UI | GraphRAG **document indexer** — ingest PDFs/DOCX/MD/web, build entity graph, community summaries, serve MCP + REST + UI | ~6–8k Go + UI |
| `kgraph/` | TypeScript (Bun) + React/Vite UI | Per-project **AI-session memory** — markdown notes with `[[wikilinks]]`, graph UI, MCP tools, SessionStart hook | 4,134 |

They solve **two halves of the same problem**: docsiq turns *source documents* into a
queryable knowledge graph; kgraph captures *agent-authored notes and decisions* during coding
sessions into a linked graph. Today they duplicate ~40% of surface area (MCP server, REST API,
graph UI, SQLite store, project config, hooks-adjacent concerns) but with incompatible schemas,
runtimes, and storage layouts. A merger unlocks the "read your docs **and** remember what the
team decided" experience neither delivers alone.

**Constraints (locked):**
- Single programming language — **Go**.
- Distribution — **`go install github.com/<you>/docsiq@latest`**. No prebuilt binaries shipped.
- **CGO enabled.** Unlocks `mattn/go-sqlite3` + SQLite C extensions (notably `sqlite-vec`).
- **Supported platforms: Linux and macOS.** Windows is **unsupported — not tested, not
  documented, not actively blocked.** A Windows user who installs their own C toolchain
  (TDM-GCC or MinGW-w64) will probably get `go install` to succeed, but they're on their
  own: no Windows CI, no Windows-specific bug fixes, no guarantee that future phases stay
  Windows-buildable. Build-at-your-own-risk.
- Go ≥ 1.22 **plus a C toolchain** on every user's machine:
  - Linux: `gcc` (pre-installed on most distros or via `build-essential`)
  - macOS: Xcode Command Line Tools (`xcode-select --install`)
  - Windows (unsupported): TDM-GCC or MinGW-w64 — user-installed, not on the happy path

**Recommendation:** consolidate on Go. Keep `docsiq` as the base; port kgraph's feature
set (per-project identity, wikilink notes, frontmatter, hook installer, bearer auth, ZIP
export/import, FTS5 notes index, notes UI) into it. Retire the TypeScript codebase once
feature parity lands. Rationale: docsiq holds the harder-to-port pieces (PDF/DOCX
loaders, Louvain, LLM extraction, langchaingo); kgraph is 4.1k LoC of mostly small,
self-contained modules that map cleanly to Go. The end state is one binary, one language,
one build, installable anywhere Go runs.

---

## 2. Project A — `docsiq` (Go)

### 2.1 What it is
A self-contained GraphRAG pipeline: load → chunk → embed → extract entities/relationships/claims
→ detect Louvain communities → LLM-summarize communities → expose via MCP/REST/UI. Single static
binary, SQLite as the only datastore.

### 2.2 Architecture

```
cmd/        Cobra CLI (root, index, serve, stats, version)
internal/
  loader/    PDF, DOCX, TXT, MD, web crawler
  chunker/   Token-aware splitting w/ overlap
  embedder/  Batched embeddings (Azure, Ollama)
  extractor/ LLM JSON-mode entity/relationship/claim extraction
  community/ Pure-Go Louvain + LLM report summarizer
  search/    local (vec + graph walk) & global (community aggregation)
  pipeline/  5-phase orchestrator
  store/     SQLite schema (documents, chunks, embeddings, entities,
             relationships, claims, communities, community_reports)
  llm/       Provider abstraction (Azure OpenAI, Ollama)
  mcp/       12 MCP tools over /mcp/sse
  api/       REST handlers + router
ui/         React + Vite + vis-network, embedded via embed.go
```

### 2.3 Strengths
- Solid 5-phase GraphRAG with hierarchical Louvain — unusual to see in pure Go without CGO.
- 12 MCP tools covering both vector and graph queries, plus community-aggregated global search.
- Good separation of concerns; pipeline is replayable with `--finalize`.
- Single static binary; operationally trivial.
- Structured doc summaries (`get_document_structure`) + MCP console/UI for debugging tool calls.

### 2.4 Gaps & weaknesses
| # | Gap | Impact |
|---|---|---|
| D1 | **No multi-project scoping** — one SQLite DB per install; all docs share a namespace | Cannot serve multiple repos/teams on one server; collides with kgraph's per-project model |
| D2 | **No auth on REST or MCP** | Cannot deploy beyond loopback without a reverse proxy |
| D3 | **No incremental / delta re-indexing** visible in CLI docs — only `--force` | Re-indexing a doc corpus is expensive; changed-file detection missing |
| D4 | **Providers limited to Azure + Ollama** (HuggingFace removed). No OpenAI direct, no Anthropic, no Bedrock, no Gemini | Users without Azure contracts must run Ollama locally |
| D5 | **No vector index** (HNSW/ivfflat). `embeddings.vector` is a BLOB — brute-force scan at query time | Query latency grows linearly with chunk count |
| D6 | **No reranker** after vector retrieval | Precision ceiling on local search |
| D7 | **Web crawler** exists (`internal/crawler/`) but no scheduling / refresh / sitemap handling documented | Web sources get stale |
| D8 | **No export / import** of the graph/data | Cannot migrate between machines or back up portably (cf. kgraph's ZIP export) |
| D9 | **UI has graph, search, stats, upload** but no edit/annotate/curate flow — read-only explorer | Cannot correct bad entity extractions |
| D10 | **Tests**: only `testdata/sample.md` visible; no `_test.go` files in the tree listing | Unknown test coverage; CI might be thin |
| D11 | **No hooks** — docsiq does not participate in AI session lifecycle | Doesn't feed context into coding agents; requires a separate mechanism |
| D12 | **No structured logging / metrics endpoint** (`/metrics`, tracing) | Hard to operate at scale |
| D13 | **CLAUDE.md** mentions completed integrations but not a user-facing changelog/roadmap | Unclear what's in flight |
| D14 | **Claims table** is populated but no MCP/REST surface to query claims directly | Dead data path |

---

## 3. Project B — `kgraph` (TypeScript/Bun)

### 3.1 What it is
A per-git-repo "memory" service. At session start, a hook resolves the current git remote to a
project and returns a message telling the agent to use MCP tools. Notes are plain markdown with
YAML frontmatter and `[[wikilinks]]`; SQLite is a derived FTS5 index over the notes folder.

### 3.2 Architecture

```
src/
  core/
    db.ts       registry (remotes→projects) + per-project FTS5
    project.ts  project dir layout, open DB
    notes.ts    read/write/delete markdown + frontmatter
    graph.ts    build graph from wikilinks, related, tree
    remote.ts   normalize git remote URLs
    watcher.ts  (small, likely file-watch for notes→index sync)
  api/
    routes.ts   REST: projects, notes, graph, tree, search, export/import
    hooks.ts    POST /api/hook/SessionStart
  mcp.ts        MCP server: list_projects, list_notes, search_notes,
                read_note, write_note, delete_note, get_graph, ...
  server.ts     Bun HTTP entrypoint (auth, routing, CORS)
  index.ts      CLI (init, serve, etc.)
hooks/          install.ts (426 LoC!), hook.mjs, hook.sh
ui/             React SPA: FolderTree, Graph, NoteEditor, NoteView, LinkPanel, TopBar
tests/          10 files, ~1.6k LoC — meaningful coverage
deploy/         systemd setup.sh
```

### 3.3 Strengths
- Per-project isolation via git-remote identity — the *right* abstraction for agent workflows.
- Markdown-on-disk as source of truth, SQLite as derived index — portable, diff-friendly,
  editable outside the app.
- Wikilinks as first-class graph edges — intuitive, user-authored graph structure.
- ZIP export/import, Docker image, systemd installer, Caddy guide — deployment story is richer
  than docsiq.
- Bearer-token auth already implemented for both REST and MCP.
- Cross-client hook installer (Claude Code, Cursor, Copilot, Codex) — 426 LoC in `hooks/install.ts`.
- **Healthy test suite** (10 test files covering api, db, graph, wikilinks, project, remote,
  frontmatter, hooks, notes, integration).
- Recent git history shows intentional minimalism ("lean session context 5 lines, 639 chars",
  "move context injection from hook to MCP instructions").

### 3.4 Gaps & weaknesses
| # | Gap | Impact |
|---|---|---|
| K1 | **No vector search / embeddings** — only SQLite FTS5 | Semantic queries fail; "find notes about auth" matches literal tokens only |
| K2 | **No LLM integration whatsoever** — no extraction, no summarization, no auto-tagging | Notes are whatever the agent writes; no cross-note synthesis |
| K3 | **No community / cluster view** — graph is raw wikilink edges | Hard to navigate once >100 notes |
| K4 | **No PDF/DOCX/web ingestion** — notes only | Cannot absorb spec docs, tickets, design docs as first-class nodes |
| K5 | **Bun-only runtime** (Node support was dropped per git log) | Platform lock-in; some ops shops won't install Bun |
| K6 | **Graph-UI scales poorly** past a few hundred nodes (sim worker is naive, no LOD) | UX ceiling |
| K7 | **No query aggregation / "global" search** across projects | Cross-repo knowledge fragmented |
| K8 | **No metrics / observability** | Same ops concern as D12 |
| K9 | **Hooks focus on SessionStart only**; Stop hook was removed — notes only land via explicit MCP `write_note` | Agents that forget to call the tool lose context |
| K10 | **`hooks/install.ts` at 426 LoC** is the biggest file — likely over-engineered multi-client installer, fragile | Maintenance burden |
| K11 | **No versioning / history** of notes (git-in-git could help but isn't wired) | Decisions get overwritten silently |
| K12 | **Web UI uses custom theme + sim worker**; no shared design system with docsiq UI | Duplicated UI work if merged |

---

## 4. Overlap & Complementarity Matrix

| Concern | docsiq | kgraph | Overlap | Merge implication |
|---|---|---|---|---|
| Datastore | SQLite (1 DB, global) | SQLite (per project) + FS notes | Both SQLite | Adopt kgraph's per-project layout; nest docsiq tables inside each project DB |
| Project scoping | ❌ none | ✅ git-remote based | **kgraph wins** | Use kgraph identity model |
| Documents as input | ✅ PDF/DOCX/TXT/MD/web | ❌ | docsiq wins | Ingestion pipeline = docsiq |
| Notes as input | ❌ (could index MD) | ✅ first-class | kgraph wins | Note authoring = kgraph |
| Vector search | ✅ brute-force | ❌ | docsiq | Unify embeddings across both |
| Graph extraction | ✅ LLM-entity | ❌ (wikilinks only) | — | Keep both; merge at query time |
| Community detection | ✅ Louvain | ❌ | docsiq | Run over combined graph |
| MCP server | ✅ 12 tools, Go | ✅ 6 tools, TS | **both** | Single MCP server, merged toolset |
| REST API | ✅ | ✅ | **both** | Merge under /api/v1 |
| Web UI | React/Vite (ui/src) | React/Vite (ui/) | **both** | Single SPA, unified design tokens |
| Auth | ❌ | ✅ bearer | kgraph | Adopt kgraph scheme everywhere |
| Hooks | ❌ | ✅ multi-client installer | kgraph | Extend to inject doc-context too |
| Export/Import | ❌ | ✅ ZIP | kgraph | Extend ZIP to include docs + embeddings |
| Deploy docs | ❌ Makefile only | ✅ Docker+systemd+Caddy | kgraph | Absorb |
| Tests | thin | healthy | kgraph | Raise bar for Go side |

---

## 5. Language Decision — **Go only**

User constraint: one language. The two honest choices are "Go-only" or "TS-only"; the
effort and risk profiles are very different.

### Port kgraph → Go (CHOSEN)
Rewrite kgraph's 4,134 LoC in Go, folding it into docsiq. kgraph is mostly thin modules
with clear contracts — most files are <350 LoC.

| kgraph module | LoC | Go target | Notes |
|---|---|---|---|
| `src/core/db.ts` | 348 | `internal/store/notes.go` | SQLite FTS5 via **pure-Go** `modernc.org/sqlite` — already viable no-CGO |
| `src/core/graph.ts` | 126 | `internal/notes/graph.go` | Wikilink edge builder |
| `src/core/notes.ts` | 129 | `internal/notes/notes.go` | md read/write + frontmatter |
| `src/core/project.ts` | 100 | `internal/project/project.go` | Per-project dir layout |
| `src/core/remote.ts` | 28 | `internal/project/remote.go` | Git remote normalization |
| `src/core/watcher.ts` | 53 | `internal/notes/watcher.go` | `fsnotify` |
| `src/utils/*` | 50 | `internal/notes/{frontmatter,wikilinks}.go` | `yaml.v3` + regex |
| `src/api/routes.ts` | 316 | extend `internal/api/handlers.go` | Add notes/projects/export/import endpoints |
| `src/api/hooks.ts` | 38 | `internal/api/hooks.go` | SessionStart handler |
| `src/mcp.ts` | 221 | extend `internal/mcp/tools.go` | Add write_note/read_note/search_notes/list_projects |
| `src/server.ts` | 146 | fold into `cmd/serve.go` | Bearer auth middleware |
| `src/index.ts` | 339 | extend `cmd/*.go` | New cobra cmds: `init`, `projects`, `notes` |
| `hooks/install.ts` | **426** | `cmd/hooks.go` + `internal/hookinstaller/` | Rewrite per-client installer (Claude Code, Cursor, Copilot, Codex). Biggest single item. |
| `tests/*.ts` | ~1.6k | `*_test.go` | Rewrite (Go-idiomatic, table-driven) |
| `ui/` (React) | — | merge into `docsiq/ui/src/` | **No language change** — both are already React/Vite |

**Effort estimate:** 3–5 weeks of focused work for core + API + MCP + installer; UI merge
in parallel; tests alongside each module. Total net new Go: ~3–4k LoC.

**Pros**
- One static binary (no CGO — `modernc.org/sqlite` handles FTS5 without C).
- Cross-compiles to Linux/macOS/Windows from one toolchain.
- Docscontext's ops story (single binary) extends to the agent-memory features.
- Existing Go test idioms cover the new code.
- One `Makefile`, one CI, one release pipeline.

**Cons**
- ~4 weeks of disciplined porting work before feature parity.
- Must rewrite the multi-client hook installer (the trickiest file).
- Temporary feature regression during the port — mitigated by phased migration (§9).

### Port docsiq → TS (REJECTED)
- 6–8k LoC Go rewrite, including PDF/DOCX parsing, Louvain, embeddings math, langchaingo
  Azure/Ollama wiring.
- PDF/DOCX TS libraries are noticeably flakier than Go equivalents (`ledongthuc/pdf`,
  `unidoc`, `nguyenthenguyen/docx`).
- Loses the single-static-binary story; introduces Bun (or Node) runtime dependency.
- ~2.5–3 months of work vs. 3–5 weeks the other way.

**Net verdict: Go.**

### Shared UI note
Both projects already ship React + Vite frontends. Merging the SPAs is **not a language
change** — kgraph's `ui/components/{Graph, FolderTree, NoteEditor, NoteView, LinkPanel}` get
copied into `docsiq/ui/src/components/notes/` and wired through the existing
`App.tsx` + `TopNav`. Embedded via existing `ui/embed.go`.

---

## 6. Recommended Unified Architecture (single Go binary)

```
                       ┌──────────────────────────────────────┐
  AI coding agent ────▶│  docsiq (Go binary)  :8080           │
  (Claude/Cursor/etc.) │                                      │
                       │  ─ cmd/ (cobra CLI)                  │
                       │     init · serve · index · notes     │
                       │     projects · hooks · stats         │
                       │                                      │
                       │  ─ MCP server (one unified toolset)  │
                       │     doc tools + note tools + graph   │
                       │                                      │
                       │  ─ REST /api/v1                      │
                       │  ─ Embedded React SPA (ui/embed.go)  │
                       │  ─ Bearer auth middleware            │
                       │  ─ SessionStart hook endpoint        │
                       │  ─ Multi-client hook installer       │
                       │                                      │
                       │  ─ Ingestion pipeline (5-phase)      │
                       │  ─ Notes (md on disk + wikilinks)    │
                       │  ─ Embeddings · Louvain · extraction │
                       │                                      │
                       │  ─ Per-project SQLite:               │
                       │    $DATA_DIR/projects/<slug>/        │
                       │       index.db       (docs+entities) │
                       │       notes.db       (FTS5 notes)    │
                       │       notes/*.md     (source-of-truth)│
                       └──────────────────────────────────────┘
```

Optionally the two SQLite files can collapse into one once the schema is stable.

**Unified primitives:**
- **Project identity** — normalized git remote (adopted from kgraph), persisted in a root
  `$DATA_DIR/registry.db`.
- **Storage layout** — `$DATA_DIR/projects/<slug>/` holds markdown notes on disk (kgraph
  model, keeps them diff-able) plus the SQLite DB that now also hosts docsiq tables.
- **Embeddings** — the same embedder that vectorizes doc chunks also vectorizes notes,
  so a single vector query can hit both.
- **Graph** — wikilink edges from notes + LLM-extracted entity edges from docs merge into
  one graph table, with a `source` column (`note | doc-entity | doc-relationship`).
- **MCP toolset** — one server exposes both families:
  - docs: `search_documents`, `local_search`, `global_search`, `query_entity`,
    `find_relationships`, `get_graph_neighborhood`, `get_document_structure`,
    `list_entities`, `list_documents`, `get_community_report`, `get_chunk`, `stats`
  - notes: `list_projects`, `list_notes`, `search_notes`, `read_note`, `write_note`,
    `delete_note`, `get_graph` (per project)
- **Auth** — single bearer key (`DOCSIQ_API_KEY`) in front of REST + MCP + hooks.
- **UI** — the docsiq React SPA gains a Notes module (folder tree, editor, wikilink
  preview) imported from kgraph's UI components; one build, one bundle.

---

## 7. Combined Feature Checklist (gaps to close on the way to "complete")

Legend: ✅ present · 🟡 partial · ❌ missing

### Ingestion & indexing
- ✅ PDF / DOCX / TXT / MD loaders (docsiq)
- ✅ Web crawler (docsiq) — 🟡 needs sitemap, scheduled refresh, robots.txt respect
- ❌ Incremental / changed-file re-index (mtime + content-hash)
- ❌ Repository-aware code indexer (treat `.py/.ts/.go` files, with symbol extraction)
- ❌ Issue tracker / Linear / GitHub ingestion
- ❌ Notes → embedded in the same vector space as docs (merge K1 + D1)

### Retrieval
- ✅ Vector brute-force search (docsiq)
- ❌ **Vector index (HNSW via sqlite-vec or Chromem-go)** — address D5
- ❌ **Hybrid search** (BM25/FTS5 + vector + reciprocal-rank-fusion)
- ❌ **Reranker** (cross-encoder or LLM)
- ✅ GraphRAG local / global search
- ❌ Cross-project search ("global" across all projects)
- ❌ Temporal filtering ("decisions from last quarter")

### Knowledge structure
- ✅ Entities / relationships / claims (docsiq) — 🟡 claims unused at surface (D14)
- ✅ Louvain communities (docsiq)
- ✅ Wikilinks (kgraph)
- ❌ Unified node taxonomy across notes + entities
- ❌ Deduplication / entity resolution across docs + notes
- ❌ Note versioning / history (K11)

### Agent integration
- ✅ MCP server (both — must merge)
- ✅ SessionStart hook (kgraph)
- ❌ Stop/PreCompact hook to auto-persist decisions (was removed; reconsider with guardrails)
- ❌ Per-tool-call attribution (which note/entity grounded each answer)
- ❌ Feedback loop (agent can flag wrong extractions; UI curation)

### LLM providers
- ✅ Azure OpenAI (docsiq)
- ✅ Ollama (docsiq)
- ❌ OpenAI direct, Anthropic, Google Vertex, Bedrock, Groq
- ❌ Per-project provider override

### Operations
- ✅ Docker (kgraph) — 🟡 docsiq has no Dockerfile visible
- ✅ systemd installer (kgraph)
- ❌ `/metrics` (Prometheus) or OTel tracing
- ❌ Structured JSON logs with request IDs
- ❌ Backup / snapshot command (extend kgraph ZIP export to include docsiq DB)
- ❌ Rate limiting on API / MCP
- 🟡 Bearer auth (kgraph only; missing in docsiq)
- ❌ Role-based scopes (read-only key vs. write key)
- ❌ Multi-user / multi-tenant

### UX
- ✅ Graph explorer (both — must merge)
- ✅ Note editor (kgraph)
- ✅ Document upload (docsiq)
- ✅ Stats (docsiq)
- ❌ Unified search bar that hits both note & doc indexes
- ❌ Entity curation (merge, rename, reject)
- ❌ "Why this answer" citations in UI
- ❌ Mobile-responsive (kgraph has partial mobile; docsiq doesn't)

### Quality
- ✅ TS tests (kgraph, 10 files)
- 🟡/❌ Go tests (docsiq — none visible)
- ❌ E2E ingestion fixture suite
- ❌ Eval harness for retrieval quality (recall@k on a labeled set)

---

## 8. Per-Project Improvement Recommendations (applicable even if you don't merge)

### docsiq
1. **Add per-project scoping** (`--project <name>`, `$DATA_DIR/projects/<slug>/`). This is a
   precondition for merging and also useful standalone.
2. **Add bearer auth** mirroring kgraph (`DOCSIQ_API_KEY`). Minimal diff.
3. **Add vector index** via `sqlite-vec` or `chromem-go` to kill brute-force scan (D5).
4. **Add Go tests** for `internal/pipeline`, `internal/search`, `internal/store` — start with
   table-driven cases on a small fixture corpus.
5. **Surface claims** as an MCP tool + REST endpoint (D14).
6. **Add Dockerfile + GH action** publishing `ghcr.io/.../docsiq`.
7. **Incremental re-index** keyed on content SHA + mtime.
8. **Export/import** (reuse kgraph's ZIP format shape).

### kgraph
1. **Embeddings-backed search.** Either call out to docsiq (after merge) or embed
   in-process via a small ONNX model. Addresses K1 and unblocks semantic search.
2. **Shrink `hooks/install.ts`** (426 LoC is a smell). Split per-client modules + a shared core.
3. **Restore Stop/PreCompact hook as opt-in,** with explicit agent-authored scratchpad only
   (the reason it was removed was unbounded auto-write).
4. **Note history** via git-commit-on-write in `projects/<name>/notes/` — cheap durable
   history, diff-able in UI.
5. **LOD/clustering** in the graph view for >300 nodes — swap sim worker for
   d3-force-3d or pixi-based canvas.
6. **Re-add Node.js support** (behind a runtime shim) — Bun-only cuts adoption.
7. **Observability:** `/metrics`, request-id logs.

---

## 9. Phased Merge Roadmap (Go-only)

The kgraph repo stays read-only once Phase 1 starts — all new code lands in docsiq
(which is renamed to `docsiq` when Phase 4 ships). kgraph is archived after Phase 4.

### Phase 0 — **Foundations** (week 1) — **COMPLETE (2026-04-17)**

Delivered (unstaged in `docsiq/` repo, ready for review + commit):
- ✅ Env-var migration — `DOCSIQ_*` canonical, `DOCSIQ_*` deprecated aliases with
  per-var + summary WARN logs. Environ-scan approach in `internal/config/config.go`
  (+102 LoC). Config file search adds `~/.docsiq/` ahead of legacy `~/.docsiq/`.
  Data dir default migrated to `~/.docsiq/data` with no-auto-move warn for legacy users.
  `ServerConfig.APIKey` field added. 37 config subtests.
- ✅ Bearer auth middleware — `internal/api/auth.go` (~85 LoC). Policies: UI + `/health`
  public, `/api/*` + `/mcp` gated, OPTIONS bypass, case-sensitive `"Bearer "` scheme,
  `crypto/subtle` constant-time compare, JSON 401, `slog.Warn` on failure (never logs
  token). 34-case adversarial test suite + 1 benchmark.
- ✅ `/health` endpoint — always-public `{"status":"ok"}`.
- ✅ `cmd/version.go` → `runtime/debug.ReadBuildInfo()` with `-ldflags` kept as override.
  Preserves `make build` path; unlocks correct version strings for `go install`. 7 tests.
- ✅ Real `ui/dist/` built and committed (308 KB, Vite content-hash assets).
- ✅ `.github/workflows/ui-freshness.yml` — inlined workflow (NOT the external reusable
  pipeline). Rebuilds `ui/dist/` on every PR and fails if the committed bundle drifts.
- ✅ Test baseline: **0 → 88 subtests** across `cmd`, `internal/api`, `internal/config`,
  `internal/store`. `go build ./...`, `go vet ./...`, `go test ./...` all clean.
- ✅ `CONTRIBUTING.md` created explaining the `ui/dist/` commit requirement.

**Deferred deliberately (audit-driven revisions to the original Phase 0 scope):**
- ⏸ Repo rename `docsiq/` → `docsiq/` — kept for a later phase once import-path
  churn can ride alongside the kgraph port.
- ⏸ **SQLite driver swap to `mattn/go-sqlite3` + CGO** — moved to Phase 5 where
  `sqlite-vec` actually lands. Phases 0–4 stay on `modernc.org/sqlite` (pure-Go, no
  CGO). Strict improvement for early-release UX: Go-only install, no C toolchain.
- ⏸ CI CGO matrix (Linux + macOS) — unnecessary until the driver swap in Phase 5.

**Findings surfacing follow-up work (tracked as separate tasks):**
- 🐛 `internal/store/store.go:21` DSN pragmas `?_journal_mode=WAL&_foreign_keys=on`
  are `mattn/go-sqlite3` syntax and **silently ignored by `modernc.org/sqlite`**.
  Runtime state observed: `journal_mode=delete`, `foreign_keys=0`. Despite schema
  declaring `ON DELETE CASCADE`, FKs are not actually enforced today. Fix before
  Phase 1 (per-project DBs where cascade semantics matter more).
- 🐛 `go test ./...` walks `ui/node_modules/flatted/golang/pkg/flatted` (a transitive
  npm dep shipping a Go package). Harmless today but a deterministic-build hazard.
- ⚠ 3 npm audit findings (1 moderate, 2 high) — transitive dev deps.

**Original Phase 0 items retained for reference (now carried into later phases):**
- `go install` distribution model: confirmed. Docs prereq is Go only while on modernc.
  C-toolchain prereq added at Phase 5 alongside the driver swap.
- `ReadBuildInfo` for version: shipped ✅

### Phase 1 — **Per-project scope** (weeks 2–3)
- New root: `$DATA_DIR/registry.db` + `$DATA_DIR/projects/<slug>/index.db`.
- Add `internal/project/` (project.go, remote.go) — git-remote normalization + slug.
- Every REST/MCP/CLI surface gains an optional `project` param (defaults to a `_default`
  slug so existing users aren't broken).
- Migrate existing single-DB users via `docsiq migrate --into _default`.
- New cobra commands: `docsiq projects list|register|delete`, `docsiq init` (git-remote-aware).
- Go tests for project isolation (two projects, no cross-read).

### Phase 2 — **Notes subsystem** (weeks 3–5)
Port kgraph's notes functionality.
- `internal/notes/`: `notes.go` (md read/write), `frontmatter.go` (yaml.v3), `wikilinks.go`
  (regex + graph edges), `graph.go` (build graph from on-disk notes), `watcher.go` (fsnotify).
- `internal/store/notes.go`: SQLite FTS5 tables for notes, indexed from the md files.
- REST: `/api/projects/:p/notes/*key` (GET/PUT/DELETE), `/api/projects/:p/tree`,
  `/api/projects/:p/search`, `/api/projects/:p/export`, `/api/projects/:p/import`.
- MCP: add `list_projects`, `list_notes`, `search_notes`, `read_note`, `write_note`,
  `delete_note`, `get_graph` alongside the existing 12 doc tools.
- Notes-on-disk is source of truth; FTS5 is a rebuildable index (matches kgraph's model).
- Port kgraph's `tests/notes.test.ts`, `wikilinks.test.ts`, `frontmatter.test.ts` as Go
  tests.

### Phase 3 — **Hooks + cross-client installer** (weeks 5–7)
The highest-risk port. kgraph's `hooks/install.ts` is 426 LoC covering Claude Code, Cursor,
Copilot, Codex — each with its own config file format and MCP registration quirk.
- `cmd/hooks.go`: `docsiq hooks install [--client=claude|cursor|copilot|codex|all]`.
- `internal/hookinstaller/`: one file per client. Small shared helpers for JSON merging
  and config-file location detection.
- `/api/hook/SessionStart` endpoint — returns `{additionalContext}` pointing agents at MCP
  tools (same shape kgraph uses, so existing hook.sh/hook.mjs keep working).
- **Hook runtime script stays language-agnostic** and rides in the binary via `embed.FS`:
  - `hook.sh` (bash + curl) — the only shipped hook script.
  - `hook.mjs` is dropped (Node.js requirement eliminated on client machines).
  - End-users of the AI clients need only a POSIX shell.
  - Windows users (unsupported path) can run `hook.sh` under Git Bash / WSL, or BYO a
    `hook.ps1` equivalent — we don't ship one.
- `docsiq hooks install` writes these scripts to `$DATA_DIR/hooks/` and registers them in
  the selected AI client's config.
- Port kgraph's `tests/hooks.test.ts` + `project.test.ts` as Go tests.

### Phase 4 — **UI merge + unified retrieval** (weeks 7–10)
- Copy kgraph's UI components (`FolderTree`, `Graph`, `NoteEditor`, `NoteView`, `LinkPanel`,
  `simWorker`) into `ui/src/components/notes/`. Both sides already use React + Vite, so this
  is file-level integration, not a rewrite.
- Add Notes tab to `TopNav`. Unified search bar hits notes FTS5 + doc hybrid retrieval;
  results labeled `[note]/[doc]/[entity]`.
- Embed notes in the same vector space as doc chunks — one embedder, one search path.
- Merge wikilink edges and LLM-extracted entity edges into a single graph rendering; legend
  distinguishes sources.
- Rebrand: binary name becomes `docsiq`, `docsiq` kept as a symlink for one release.
- Archive the kgraph repo (README pointing to docsiq).

### Phase 5 — **Ops + retrieval quality** (weeks 10–12)
Now that everything is in one codebase, knock down the gaps from §7:
- **Vector index — `sqlite-vec`** loaded via `db.Exec("SELECT load_extension('vec0')")`.
  CGO lets us use the mature C extension directly; ANN queries are milliseconds at 1M+
  vectors. Ship `vec0.so` (Linux) and `vec0.dylib` (macOS) embedded via `embed.FS`,
  extracted to `$DATA_DIR/ext/` on first run.
- Hybrid search (FTS5 + vector + RRF) and optional LLM reranker.
- Incremental re-index (content-hash + mtime).
- Surface `claims` via MCP/REST (D14).
- `/metrics` (Prometheus) + structured JSON logs with request IDs.
- Eval harness (recall@k) with a labeled fixture set.
- Per-project LLM provider override in config.

### Phase 6 — **Optional: stretch** (beyond week 12)
- Additional providers (OpenAI direct, Anthropic, Vertex, Bedrock, Groq) — pluggable via
  the existing `internal/llm/provider.go` interface.
- Note history via auto-commit-on-write into a hidden `projects/<slug>/notes/.git`.
- Entity resolution / deduplication (note "JWT" ↔ doc entity "JSON Web Token").
- Multi-user auth (RBAC, per-project scopes) — only if deployment model demands it.

---

## 10. Risks, Open Questions, Decisions to Make

### Risks
- **Multi-client hook installer port** is the single riskiest item — 426 LoC covering four
  AI clients each with quirks. Mitigation: keep the hook runtime as language-agnostic glue
  (`hook.sh` + new `hook.ps1`) embedded via `embed.FS`; only port the *installer* logic
  (file discovery + JSON merging), not the hook runtime itself.
- **Stale `ui/dist/` in git** — `go install` has no way to rebuild the UI, so a committed
  `ui/dist/` that drifts from `ui/src/` will ship broken UIs to users. Mitigation: CI job
  rebuilds `ui/dist/` on every PR and fails if the committed bundle doesn't match; optional
  pre-commit hook locally.
- **`go install` first-run cost** — each user pays a 10–30s Go+C compile the first time
  (and every version bump). CGO roughly doubles this vs. pure Go. Mitigation: accept it;
  document it; target audience already runs `go install` for other tools.
- **CGO breaks `go install` on machines with no C compiler at all** (bare minimal Docker
  images, some CI environments). Mitigation: documented Linux/macOS prereqs.
- **`sqlite-vec` extension file distribution** — `.so` and `.dylib` must be shipped with
  the binary. Embedding via `embed.FS` and extracting to `$DATA_DIR/ext/` on first run
  works, but adds ~2 MB to the binary (one per supported OS). Mitigation: accept — it's
  the price of ANN. (Windows users on the unsupported path get a build-time error telling
  them to drop a `vec0.dll` into `$DATA_DIR/ext/` themselves.)
- **Feature regression during port** — current kgraph users lose functionality until Phase 2
  ships. Mitigation: don't archive kgraph repo until Phase 4 is green; users stay on kgraph
  until docsiq reaches parity.
- **SQLite driver swap** (`mattn/go-sqlite3` → `modernc.org/sqlite`) is a behavior change
  even though SQL is identical — modernc has no CGO but different error messages and
  marginally different performance. Mitigation: run the existing test corpus against both
  drivers once before committing to the swap.
- **LLM cost inflation** once notes are auto-embedded. Mitigation: embed on demand, dedupe
  by content hash.
- **Entity dedup / resolution** at graph merge time ("JWT" note ↔ "JSON Web Token" entity).
  Mitigation: start with vector similarity threshold; defer true ER to Phase 6.

### Open questions (for the human to decide)
1. **Target deployment model** — single laptop / team VM / hosted SaaS? Drives auth and
   multi-tenancy priorities.
2. **Provider preference** — is Azure + Ollama enough, or must Phase 5/6 ship
   OpenAI/Anthropic/Bedrock?
3. **Naming** — keep the umbrella as `docsiq`, rename the Go binary to `docsiq`, retire both
   `docsiq` and `kgraph` names? Or keep `docsiq` as the binary and "docsiq" only as
   the project label?
4. **Migration path** — existing kgraph users on `~/.kgraph/` — do we ship a one-shot
   `docsiq migrate --from-kgraph ~/.kgraph` importer?
5. **Licensing** — docsiq has a LICENSE; kgraph's license status should match.
6. **Who uses this** — solo devs, teams, enterprises? Drives RBAC vs. single-key auth.
7. **Notes versioning now or later** — is `.git`-on-notes-dir worth wiring in Phase 2, or
   defer to Phase 6?

---

## 11. Quick-Win Punch List (can start Monday, no merge needed)

| # | Task | Phase | Effort | Value |
|---|---|---|---|---|
| 1 | Add bearer auth middleware to REST+MCP | 0 | S | Unblocks remote deploy, parity with kgraph |
| 2 | Settle on `mattn/go-sqlite3` + `sqlite_fts5` build tag (CGO on) | 0 | S | Mature SQLite + enables `sqlite-vec` later |
| 3 | CI matrix: Linux/macOS with `CGO_ENABLED=1` | 0 | S | Catches toolchain regressions pre-release |
| 4 | Commit pre-built `ui/dist/` + CI freshness check | 0 | S | Mandatory for `go install` UI shipping |
| 5 | Switch version string to `runtime/debug.ReadBuildInfo()` | 0 | S | Version info survives `go install` |
| 6 | Add `project` scope everywhere | 1 | M | Precondition for notes merge |
| 7 | Port kgraph notes core (notes.go, frontmatter, wikilinks) | 2 | M | First visible "merged" feature |
| 8 | Port kgraph's test fixtures to `_test.go` | 2 | M | Baseline coverage |
| 9 | Port multi-client hook installer; keep only `hook.sh` (drop Node/PowerShell) | 3 | L | Agent integration, zero client-side runtime deps |
| 10 | Merge UI (copy kgraph React components into `ui/src/`) | 4 | M | One SPA |
| 11 | Surface `claims` via MCP/REST | 5 | S | Uses existing data |
| 12 | Wire `sqlite-vec` (embed extension + loadExtension on boot) | 5 | M | Proper ANN index, scales past 1M vectors |

---

## 12. Appendix — File & LoC Snapshot

### docsiq (key files)
- `cmd/`: root, serve, index, stats, version
- `internal/store/store.go` (schema: documents, chunks, embeddings, entities, relationships,
  claims, communities, community_reports)
- `internal/pipeline/pipeline.go` (5-phase orchestration)
- `internal/mcp/{server,tools}.go` (12 tools)
- `internal/api/{router,handlers}.go`
- `internal/search/{local,global}.go`
- `internal/community/{louvain,summarizer}.go`
- `internal/extractor/{entities,claims}.go`
- `ui/src/components/docs/*` + `ui/src/components/mcp/*` React SPA

### kgraph (total 4,134 LoC)
- `src/core/db.ts` (348) — registry + FTS5
- `src/index.ts` (339) — CLI
- `src/api/routes.ts` (316) — REST surface
- `src/mcp.ts` (221) — MCP tools
- `src/server.ts` (146) — HTTP entrypoint
- `src/core/{graph,notes,project,remote,watcher}.ts`
- `src/utils/{frontmatter,wikilinks,runtime}.ts`
- `hooks/install.ts` (426) — cross-client installer (**largest file**)
- `tests/*.ts` (10 files, ~1.6k LoC)
- `ui/components/{TopBar,Graph,FolderTree,NoteView,NoteEditor,LinkPanel,simWorker}.tsx`

### Git state
- docsiq: clean, on `main`, up-to-date with origin.
- kgraph: clean, on `main`; recent history shows deliberate pruning (MCP-over-hook migration,
  Bun-only pivot).

---

**End of plan. No source files modified.**
