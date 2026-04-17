# Architecture

docsiq is a single Go binary that ingests documents, stores per-project
knowledge graphs in SQLite, and exposes that data to AI clients over
MCP + a REST API + an embedded Web UI.

## Diagram

```mermaid
flowchart LR
    subgraph Clients
        CLI["docsiq CLI"]
        Agent["AI client<br/>(Claude / Cursor / …)"]
        Browser["Web UI"]
    end

    subgraph Server["docsiq serve"]
        direction TB
        Mux["http.ServeMux"]
        Auth["bearer auth middleware"]
        Scope["project middleware<br/>resolves slug"]
        REST["REST handlers<br/>(/api/*)"]
        MCP["MCP server<br/>(/mcp)"]
        Hook["hook handler<br/>(/api/hook/SessionStart)"]
    end

    subgraph PerProject["per-project scope<br/>($DATA_DIR/projects/&lt;slug&gt;)"]
        Store[("docsiq.db<br/>SQLite + FTS5")]
        Notes["notes/ (markdown)"]
        HNSW[["HNSW index<br/>(in memory)"]]
    end

    Registry[("$DATA_DIR/registry.db<br/>slug → {name, remote}")]

    CLI -->|config + registry| Registry
    Agent -->|POST /mcp| Mux
    Agent -->|POST /api/hook/SessionStart| Mux
    Browser -->|GET /| Mux

    Mux --> Auth --> Scope
    Scope --> REST
    Scope --> MCP
    Scope --> Hook

    REST -->|stores.ForProject(slug)| Store
    MCP -->|stores.ForProject(slug)| Store
    Hook -->|registry lookup| Registry

    REST -->|vecIndexes.ForProject(slug)| HNSW
    MCP -->|vecIndexes.ForProject(slug)| HNSW

    REST -.reads/writes.-> Notes
    MCP -.reads/writes.-> Notes
```

ASCII fallback:

```
            +---------------------------------------+
  CLI ----> |        docsiq (single binary)         |
  agent --> |  mux -> auth -> project-scope -> ...  |
  browser-> |    |                                  |
            |    +-> REST handlers --> stores.ForProject(slug)
            |    +-> MCP server     --> vecIndexes.ForProject(slug)
            |    +-> hook handler   --> registry lookup
            +---------------------------------------+
                         |          |
                         v          v
                $DATA_DIR/projects/<slug>/
                    docsiq.db   notes/
                    (SQLite)    (markdown)

                $DATA_DIR/registry.db   (slug -> {name, remote})
```

## Per-project layout

Everything docsiq writes to disk lives under `$DATA_DIR` (default
`~/.docsiq/data`).

```
$DATA_DIR/
├── registry.db                       # project registry (one row per project)
├── hooks/
│   └── hook.sh                       # extracted by `docsiq hooks install`
└── projects/
    └── <slug>/
        ├── docsiq.db                 # per-project SQLite DB (graph + FTS5)
        ├── docsiq.db-wal             # SQLite WAL file
        ├── docsiq.db-shm             # SQLite shared memory
        └── notes/                    # markdown notes for this project
            ├── foo.md
            └── subdir/bar.md
```

The **registry** (`registry.db`) is tiny — one row per project with
`slug`, `name`, `remote`, `created_at`. A `UNIQUE` constraint on
`remote` prevents two projects from sharing a git origin.

Each **project DB** (`projects/<slug>/docsiq.db`) is independent and
holds that project's documents, chunks, entities, relationships,
communities, claims, and note FTS5 index. Dropping a project is
literally `rm -rf projects/<slug>/` plus a registry row delete —
that's what `docsiq projects delete --purge` does.

## The `stores` cache

`internal/api/stores.go` defines `projectStores`: a lazy, mutex-guarded
cache of `slug → *store.Store`. Policy:

- **Open on first request.** No eager open at boot.
- **Cached for process lifetime.** No TTL eviction; SQLite's per-DB
  overhead is tiny.
- **Owned by the caller, not the handler.** Handlers call
  `ForProject(slug)` and *must not* call `Close` on the returned
  handle.
- **Shared between REST and MCP.** The same cache is injected into
  both routers via `api.NewRouter(..., api.WithProjectStores(cache))`
  and `mcp.New(cache, ...)`.

This keeps the WAL happy (one writer per DB) and means a long-running
server doesn't accumulate open file descriptors as new projects are
registered.

## Vector search (HNSW)

Per-project HNSW indexes live behind
`internal/api/vector_indexes.go → VectorIndexes`, which satisfies the
narrow `VectorIndexResolver` interface used by both REST and MCP. The
resolver:

- Builds the in-memory HNSW for a slug on first demand.
- Falls back to `nil` for a slug with no embeddings — the search
  package then brute-forces against the chunks table.
- Exposes `vec status` (see CLI reference) so operators can tell
  whether sqlite-vec, HNSW, or brute-force is live.

Indexes are not persisted to disk — they're rebuilt from the SQLite
chunks table on server boot (or on first per-project access). Rebuild
cost is linear in chunk count and typically sub-second for mid-size
projects.

## Request lifecycle

A typical MCP `global_search` call:

1. **Transport** — Claude Code POSTs a JSON-RPC frame to `/mcp`. The
   `mcp-go` `StreamableHTTPServer` unwraps it.
2. **Auth** — bearer middleware checks `Authorization: Bearer ...`
   against `server.api_key` (constant-time compare).
3. **Project middleware** — extracts `project` from the request and
   attaches the slug to the `context.Context`.
4. **Tool dispatch** — mcp-go routes to `global_search`'s handler,
   which extracts args (`query`, `community_level`, `project`).
5. **Scope resolution** — `resolveDocsScope(args)` opens the per-project
   store via `s.stores.ForProject(slug)` and fetches the per-project
   HNSW via `s.vecIndexes.ForProject(slug, st)` (unused here — global
   search doesn't hit HNSW).
6. **LLM resolution** — `llm.ProviderForProject(cfg, slug)` picks the
   slug's override from `cfg.LLMOverrides` if present, else falls back
   to the root `llm.*` config.
7. **Search** — `search.GlobalSearch` loads community summaries,
   ranks them against the query embedding, and calls `prov.Complete`
   to synthesize an answer.
8. **Response** — the answer + community list are marshaled to JSON
   and wrapped in an MCP `CallToolResult` text content.

Upload and index flows are similar but write-side: `POST /api/upload`
→ `resolveStore` → `pipeline.Run(...)` → phases 1–4 (chunk → embed →
extract → communities) → atomic FTS5 rebuild.

## Why SQLite?

- One file per project makes "delete a project" trivial.
- WAL gives us readers + one writer without a separate server.
- FTS5 is the notes search backend at zero extra cost.
- `sqlite-vec` (when present) plugs in as a vector backend; absent it,
  HNSW takes over; absent that, brute-force cosine is the safety net.

No external services to deploy. No schema migrations across DBs — each
project's DB is versioned independently by `internal/store`.

## Further reading

- [`config.md`](./config.md) — every knob
- [`cli-reference.md`](./cli-reference.md) — operator-facing surface
- [`mcp-tools.md`](./mcp-tools.md) / [`rest-api.md`](./rest-api.md) —
  client-facing surface
- [`hooks.md`](./hooks.md) — SessionStart integration with AI clients
