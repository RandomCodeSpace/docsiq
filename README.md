# docsiq

[![Security Scan](https://github.com/RandomCodeSpace/docsiq/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/RandomCodeSpace/docsiq/actions/workflows/ci.yml)
[![OpenSSF Scan](https://github.com/RandomCodeSpace/docsiq/actions/workflows/ci.yml/badge.svg?branch=main&label=OpenSSF%20scan)](https://github.com/RandomCodeSpace/docsiq/actions/workflows/ci.yml)
[![OpenSSF Score](https://api.scorecard.dev/projects/github.com/RandomCodeSpace/docsiq/badge)](https://scorecard.dev/viewer/?uri=github.com/RandomCodeSpace/docsiq)
[![Release](https://img.shields.io/github/v/release/RandomCodeSpace/docsiq)](https://github.com/RandomCodeSpace/docsiq/releases)
[![Beta](https://img.shields.io/github/v/release/RandomCodeSpace/docsiq?include_prereleases&label=beta)](https://github.com/RandomCodeSpace/docsiq/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/RandomCodeSpace/docsiq)](https://github.com/RandomCodeSpace/docsiq/blob/main/go.mod)
[![Frontend Version](https://img.shields.io/badge/frontend-none-lightgrey)](https://github.com/RandomCodeSpace/docsiq)

> **Repo rename:** the project was previously named `docscontext`. The local
> checkout directory may still be `docscontext/`, but the Go module and
> binary are now `docsiq`. The GitHub repo rename from `docscontext` to
> `docsiq` is a manual step on github.com — badge/clone URLs above assume
> it has been done.

A GraphRAG tool inspired by [Microsoft GraphRAG](https://github.com/microsoft/graphrag). Built on CGO-backed SQLite ([`mattn/go-sqlite3`](https://github.com/mattn/go-sqlite3) with FTS5) plus the [`sqlite-vec`](https://github.com/asg017/sqlite-vec) loadable extension for ANN vector search.
Ingests unstructured documents, builds a hierarchical knowledge graph with community detection, and exposes an **MCP server + embedded Web UI** on a single port.

## Features

- **GraphRAG pipeline** — 5-phase: load → chunk → embed → graph extraction → community detection
- **Knowledge graph** — entity/relationship/claim extraction via LLM (JSON mode)
- **Louvain community detection** — pure Go, hierarchical, no external dependencies
- **Two LLM providers** — Azure OpenAI, Ollama (local), via langchaingo
- **12 MCP tools** — local search, global search, graph walk, community reports, and more
- **Embedded Web UI** — vis-network graph explorer, semantic search, document browser
- **Single binary** — ships on Linux / macOS (Windows is not supported)

## Install

### Prerequisites

A **C toolchain** is required at install time (docsiq links SQLite
via CGO and bundles the `sqlite-vec` loadable extension):

- **Linux**: `sudo apt-get install build-essential` (or equivalent)
- **macOS**: `xcode-select --install`
- **Windows**: not supported

Go ≥ 1.22 is required.

```bash
go install github.com/RandomCodeSpace/docsiq@latest
```

Or build from source:

```bash
git clone https://github.com/RandomCodeSpace/docsiq.git
cd docsiq
CGO_ENABLED=1 go build -tags sqlite_fts5 -o docsiq .
```

## Quick Start

```bash
# 1. Create config
mkdir -p ~/.docsiq
cp config.example.yaml ~/.docsiq/config.yaml
# Edit ~/.docsiq/config.yaml — set your LLM provider

# 2. Index documents (Phases 1-2: load, chunk, embed, extract entities)
docsiq index ./your-docs/ --workers 4

# 3. Build knowledge graph (Phases 3-4: community detection + summaries)
docsiq index --finalize

# 4. Check stats
docsiq stats

# 5. Start server
docsiq serve --port 8080
```

Open **http://localhost:8080** for the Web UI.

## Configuration

Copy `config.example.yaml` to `~/.docsiq/config.yaml` and edit:

```yaml
data_dir: ~/.docsiq/data

llm:
  provider: ollama          # azure | ollama

  ollama:
    base_url: http://localhost:11434
    chat_model: llama3.2
    embed_model: nomic-embed-text

  azure:
    # Shared defaults — used when chat/embed-specific values are not set.
    endpoint: https://myresource.openai.azure.com
    api_key: ${AZURE_OPENAI_API_KEY}
    api_version: "2024-08-01"

    chat:
      model: gpt-4o
      # endpoint: ...   # optional override (falls back to shared)
      # api_key: ...    # optional override (falls back to shared)
    embed:
      model: text-embedding-3-small
      # endpoint: ...   # optional override for separate deployment
      # api_key: ...    # optional override

indexing:
  chunk_size: 512
  chunk_overlap: 50
  workers: 4
  extract_graph: true
  extract_claims: true
  max_gleanings: 1

community:
  min_community_size: 2
  max_levels: 3

server:
  host: 127.0.0.1
  port: 8080
```

### Environment Variables

All config keys can be set via environment variables with the `DOCSIQ_` prefix.
Dots become underscores; the prefix is case-sensitive (upper-case).

```bash
# Provider
export DOCSIQ_LLM_PROVIDER=azure

# Azure shared (used by both chat and embed unless overridden)
export DOCSIQ_LLM_AZURE_ENDPOINT=https://myresource.openai.azure.com
export DOCSIQ_LLM_AZURE_API_KEY=sk-...
export DOCSIQ_LLM_AZURE_API_VERSION=2024-08-01

# Azure chat (overrides shared values for chat completions)
export DOCSIQ_LLM_AZURE_CHAT_ENDPOINT=https://chat-deployment.openai.azure.com
export DOCSIQ_LLM_AZURE_CHAT_MODEL=gpt-4o

# Azure embed (overrides shared values for embeddings)
export DOCSIQ_LLM_AZURE_EMBED_ENDPOINT=https://embed-deployment.openai.azure.com
export DOCSIQ_LLM_AZURE_EMBED_MODEL=text-embedding-3-small

# Ollama
export DOCSIQ_LLM_OLLAMA_BASE_URL=http://localhost:11434
export DOCSIQ_LLM_OLLAMA_CHAT_MODEL=llama3.2
export DOCSIQ_LLM_OLLAMA_EMBED_MODEL=nomic-embed-text

# Indexing
export DOCSIQ_INDEXING_WORKERS=4
export DOCSIQ_INDEXING_CHUNK_SIZE=512

# Server
export DOCSIQ_SERVER_PORT=9090
```

## CLI

```bash
# Index a file or directory
docsiq index ./docs/ [--force] [--workers 4] [--verbose]

# Run community detection + LLM summaries
docsiq index --finalize

# Show statistics
docsiq stats
docsiq stats --json

# Start MCP + Web UI server
docsiq serve [--port 8080] [--host 127.0.0.1]
```

## MCP Tools

Connect any MCP client to `http://localhost:8080/mcp/sse`.

| Tool | Description |
|---|---|
| `search_documents` | Vector similarity search over chunks |
| `local_search` | Vector + graph walk (GraphRAG local) |
| `global_search` | Community summary aggregation with LLM synthesis |
| `query_entity` | Entity details + relationships by name |
| `find_relationships` | Relationship lookup by source / target / predicate |
| `get_graph_neighborhood` | Subgraph JSON for visualization |
| `get_document_structure` | LLM-generated structured summary |
| `list_entities` | Browse entities with type filter |
| `list_documents` | Browse indexed documents |
| `get_community_report` | Community summary + member entities |
| `get_chunk` | Retrieve chunk by ID |
| `stats` | Full index statistics |

## REST API

```
GET  /api/stats
GET  /api/documents
GET  /api/documents/{id}
POST /api/search          {"query":"...","mode":"local|global","top_k":5}
GET  /api/graph/neighborhood?entity=<name>&depth=2
GET  /api/entities
GET  /api/communities
GET  /api/communities/{id}
POST /api/upload
```

## Architecture

```
Document In
    │
    ▼ Phase 1 — Text Units
  Loader (PDF/DOCX/TXT/MD) → Chunker → Embedder → SQLite

    ▼ Phase 2 — Graph Extraction  [parallel per document]
  LLM → Entities + Relationships + Claims → SQLite

    ▼ Phase 3 — Community Detection  [post-index finalization]
  Louvain algorithm → hierarchical community assignments

    ▼ Phase 4 — Community Summaries  [parallel]
  LLM → CommunityReport → embed summary → SQLite

    ▼ Phase 5 — Structured Doc
  LLM → JSON summary → SQLite
```

Per-project SQLite databases live at
`$DATA_DIR/projects/<slug>/docsiq.db`, with a tiny registry at
`$DATA_DIR/registry.db` mapping slugs to project metadata.

## Hook support matrix

docsiq registers a SessionStart hook with each AI client so GraphRAG
context is loaded when the client starts. Only Claude Code publishes a
documented SessionStart hook schema — the others are a best-effort
placeholder pinned by fixture tests. Installing an unverified hook
prints a `slog.Warn` so operators can opt out.

| Client | Config path | Schema source | Status |
|---|---|---|---|
| Claude Code | `~/.claude/settings.json` | [docs.claude.com/en/docs/claude-code/hooks](https://docs.claude.com/en/docs/claude-code/hooks) (fetched 2026-04-17) | verified |
| Cursor | `~/.cursor/docsiq-hooks.json` | no docs (docs.cursor.com/en/agent/hooks returned empty, 2026-04-17) | unverified |
| Copilot CLI | `~/.config/github-copilot/hooks.json` | no docs (github.com/copilot CLI docs publish no hook schema, 2026-04-17) | unverified |
| Codex CLI | `~/.codex/hooks.json` | no docs (github.com/openai/codex `docs/config.md` documents only a `Notify` post-turn hook, no SessionStart, 2026-04-17) | unverified |

> **Note:** Unverified hook shapes mirror the original kgraph guesses.
> When a client publishes a real schema, the corresponding installer in
> `internal/hookinstaller/` should be updated along with its fixture
> pair in `internal/hookinstaller/fixtures/<client>/`.

## Supported File Types

| Format | Extensions | Notes |
|---|---|---|
| PDF | `.pdf` | Text extraction via langchaingo (ledongthuc/pdf); scanned/image-only PDFs yield no text |
| Word | `.docx` | Open XML format (Office 2007+); legacy `.doc` not supported |
| Markdown | `.md`, `.markdown` | Heading `# Title` used as document title |
| Plain text | `.txt`, `.text` | UTF-8 encoding expected |

> **Tip:** For best graph quality, prefer documents with clear structure (headings, named entities, factual prose). Scanned PDFs or heavily formatted spreadsheets will produce sparse graphs.
