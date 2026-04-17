# DocsContext

[![Security Scan](https://github.com/RandomCodeSpace/docscontext/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/RandomCodeSpace/docscontext/actions/workflows/ci.yml)
[![OpenSSF Scan](https://github.com/RandomCodeSpace/docscontext/actions/workflows/ci.yml/badge.svg?branch=main&label=OpenSSF%20scan)](https://github.com/RandomCodeSpace/docscontext/actions/workflows/ci.yml)
[![OpenSSF Score](https://api.scorecard.dev/projects/github.com/RandomCodeSpace/docscontext/badge)](https://scorecard.dev/viewer/?uri=github.com/RandomCodeSpace/docscontext)
[![Release](https://img.shields.io/github/v/release/RandomCodeSpace/docscontext)](https://github.com/RandomCodeSpace/docscontext/releases)
[![Beta](https://img.shields.io/github/v/release/RandomCodeSpace/docscontext?include_prereleases&label=beta)](https://github.com/RandomCodeSpace/docscontext/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/RandomCodeSpace/docscontext)](https://github.com/RandomCodeSpace/docscontext/blob/main/go.mod)
[![Frontend Version](https://img.shields.io/badge/frontend-none-lightgrey)](https://github.com/RandomCodeSpace/docscontext)

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

A **C toolchain** is required at install time (DocsContext links SQLite
via CGO and bundles the `sqlite-vec` loadable extension):

- **Linux**: `sudo apt-get install build-essential` (or equivalent)
- **macOS**: `xcode-select --install`
- **Windows**: not supported

Go ≥ 1.22 is required.

```bash
go install github.com/RandomCodeSpace/docscontext@latest
```

Or build from source:

```bash
git clone https://github.com/RandomCodeSpace/docscontext.git
cd DocsContext
CGO_ENABLED=1 go build -tags sqlite_fts5 -o docscontext .
```

## Quick Start

```bash
# 1. Create config
mkdir -p ~/.docscontext
cp config.example.yaml ~/.docscontext/config.yaml
# Edit ~/.docscontext/config.yaml — set your LLM provider

# 2. Index documents (Phases 1-2: load, chunk, embed, extract entities)
docscontext index ./your-docs/ --workers 4

# 3. Build knowledge graph (Phases 3-4: community detection + summaries)
docscontext index --finalize

# 4. Check stats
docscontext stats

# 5. Start server
docscontext serve --port 8080
```

Open **http://localhost:8080** for the Web UI.

## Configuration

Copy `config.example.yaml` to `~/.docscontext/config.yaml` and edit:

```yaml
data_dir: ~/.docscontext/data

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

All config keys can be set via environment variables with the `DOCSCONTEXT_` prefix.
Dots become underscores; the prefix is case-insensitive.

```bash
# Provider
export DOCSCONTEXT_LLM_PROVIDER=azure

# Azure shared (used by both chat and embed unless overridden)
export DOCSCONTEXT_LLM_AZURE_ENDPOINT=https://myresource.openai.azure.com
export DOCSCONTEXT_LLM_AZURE_API_KEY=sk-...
export DOCSCONTEXT_LLM_AZURE_API_VERSION=2024-08-01

# Azure chat (overrides shared values for chat completions)
export DOCSCONTEXT_LLM_AZURE_CHAT_ENDPOINT=https://chat-deployment.openai.azure.com
export DOCSCONTEXT_LLM_AZURE_CHAT_MODEL=gpt-4o

# Azure embed (overrides shared values for embeddings)
export DOCSCONTEXT_LLM_AZURE_EMBED_ENDPOINT=https://embed-deployment.openai.azure.com
export DOCSCONTEXT_LLM_AZURE_EMBED_MODEL=text-embedding-3-small

# Ollama
export DOCSCONTEXT_LLM_OLLAMA_BASE_URL=http://localhost:11434
export DOCSCONTEXT_LLM_OLLAMA_CHAT_MODEL=llama3.2
export DOCSCONTEXT_LLM_OLLAMA_EMBED_MODEL=nomic-embed-text

# Indexing
export DOCSCONTEXT_INDEXING_WORKERS=4
export DOCSCONTEXT_INDEXING_CHUNK_SIZE=512

# Server
export DOCSCONTEXT_SERVER_PORT=9090
```

## CLI

```bash
# Index a file or directory
docscontext index ./docs/ [--force] [--workers 4] [--verbose]

# Run community detection + LLM summaries
docscontext index --finalize

# Show statistics
docscontext stats
docscontext stats --json

# Start MCP + Web UI server
docscontext serve [--port 8080] [--host 127.0.0.1]
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

All data lives in a single SQLite file at `$DATA_DIR/docscontext.db`.

## Supported File Types

| Format | Extensions | Notes |
|---|---|---|
| PDF | `.pdf` | Text extraction via langchaingo (ledongthuc/pdf); scanned/image-only PDFs yield no text |
| Word | `.docx` | Open XML format (Office 2007+); legacy `.doc` not supported |
| Markdown | `.md`, `.markdown` | Heading `# Title` used as document title |
| Plain text | `.txt`, `.text` | UTF-8 encoding expected |

> **Tip:** For best graph quality, prefer documents with clear structure (headings, named entities, factual prose). Scanned PDFs or heavily formatted spreadsheets will produce sparse graphs.