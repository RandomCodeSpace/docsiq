# docsiq

[![Security Scan](https://github.com/RandomCodeSpace/docsiq/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/RandomCodeSpace/docsiq/actions/workflows/ci.yml)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/12628/badge)](https://www.bestpractices.dev/projects/12628)
[![OpenSSF Score](https://api.scorecard.dev/projects/github.com/RandomCodeSpace/docsiq/badge)](https://scorecard.dev/viewer/?uri=github.com/RandomCodeSpace/docsiq)
[![Release](https://img.shields.io/github/v/release/RandomCodeSpace/docsiq?include_prereleases&sort=semver)](https://github.com/RandomCodeSpace/docsiq/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/RandomCodeSpace/docsiq)](https://github.com/RandomCodeSpace/docsiq/blob/main/go.mod)

docsiq is a GraphRAG-powered knowledge base that runs as a single Go binary.
It ingests unstructured documents, builds a knowledge graph with
community detection, persists wikilinked markdown notes, and exposes the
whole thing over **MCP + an embedded React SPA** on one port.

Inspired by [Microsoft GraphRAG](https://github.com/microsoft/graphrag);
storage is CGO-backed SQLite (`mattn/go-sqlite3` with FTS5) + the
[`sqlite-vec`](https://github.com/asg017/sqlite-vec) extension for ANN
vector search.

## Features

- **GraphRAG pipeline** — load → chunk → embed → extract entities/
  relationships/claims → detect communities, all in one `docsiq index` run.
- **Notes subsystem** — markdown on disk with `[[wikilinks]]`, project
  scopes, cross-project references, and a live note graph view. Works
  without any LLM configured.
- **Interactive graph** — SVG force-directed viz with d3-zoom (pinch/wheel
  pan/zoom 0.1×–40×), hover-to-highlight neighbourhood, degree-scaled nodes.
- **Community detection** — pure-Go Louvain, hierarchical, no external deps.
- **Three LLM providers** — Azure OpenAI, OpenAI, Ollama — via
  [`tmc/langchaingo`](https://github.com/tmc/langchaingo). Set
  `provider: "none"` to run the server in notes-only mode with no LLM.
- **MCP server** — 12+ tools (local/global search, graph walk, community
  reports, note read/write, …) exposed at `/mcp` via Streamable HTTP
  transport with session handshake.
- **Embedded SPA** — React 19 + Tailwind 4 + shadcn/ui, served from
  `//go:embed ui/dist`. PWA-installable with manifest + service worker.
- **Per-repo projects** — each scope has its own SQLite store + notes
  directory, addressable by slug.

## Quickstart

```bash
# Clone, install UI deps, build UI, build Go binary
git clone https://github.com/RandomCodeSpace/docsiq
cd docsiq
npm --prefix ui ci
npm --prefix ui run build
CGO_ENABLED=1 go build -tags sqlite_fts5 -o docsiq ./

# Register the current git repo as a project
./docsiq init

# Index a docs folder
./docsiq index ~/path/to/docs

# Start the server (UI + API + MCP)
./docsiq serve --port 37778
# → http://localhost:37778
```

## UI

- **Stack**: React 19, Vite 6, Tailwind 4, shadcn/ui primitives, Geist typography, Lucide icons
- **Architecture**: CSS lives in a single `globals.css` with an
  `@layer components` section; JSX uses semantic class names only;
  shadcn primitives are the only place Tailwind utilities live inline
- **Navigation**: labeled sidebar (Home · Notes · Documents · Graph · MCP)
  with ⌘K command palette
- **Responsiveness**: mobile drawer via shadcn `Sheet`; iOS safe-area
  respected; inputs forced to 16px below `sm:` to kill Safari auto-zoom
- **PWA**: manifest + 192/512 PNG icons + minimal service worker,
  installable on Android/iOS
- **Hard reload**: refresh button in the header purges service worker +
  CacheStorage and reloads from network — mobile-friendly `⌘⇧R` substitute

### Keyboard shortcuts

| Key | Action |
|---|---|
| `⌘K` / `Ctrl+K` | Command palette |
| `G H` | Home |
| `G N` | Notes |
| `G D` | Documents |
| `G G` | Graph |
| `G M` | MCP console |
| `⌘/` | Toggle tree drawer (Notes) |
| `⌘L` | Toggle links drawer (Notes) |

## MCP

docsiq speaks the MCP Streamable HTTP transport at `POST /mcp`. The UI's
MCP Console (inspector-style) gives you the same tool list with typed
argument forms. For external clients (Claude Desktop, Cursor, etc.)
register the server URL directly, or use the hooks helper:

```bash
docsiq hooks install --client claude-desktop
```

## Architecture

```
cmd/            CLI commands (cobra): index, serve, search, projects, init, hooks, vec
internal/
  api/          REST API + /mcp handler
  chunker/      Text splitting (textsplitter.RecursiveCharacter)
  community/    Louvain detection + summaries
  config/       Viper YAML config + env override
  crawler/      Web page crawler
  embedder/     Batched text → vector (nil-safe when provider=none)
  extractor/    LLM-based entity / relationship / claim extraction
  llm/          Provider abstraction (Azure, OpenAI, Ollama, none)
  loader/       Document loaders (PDF, DOCX, TXT, MD, web)
  mcp/          Streamable HTTP MCP server (12+ tools)
  notes/        Per-project markdown + wikilinks + graph builder
  pipeline/     5-phase indexing pipeline
  project/      Project registry (git-remote-scoped slugs)
  search/       Query engine (local + global + hybrid)
  store/        SQLite + FTS5 + vector index
  vectorindex/  HNSW ANN vector search
ui/             React 19 + Vite 6 SPA, embedded at compile time
```

## Configuration

Config lives at `~/.docsiq/config.yaml`; every key can be overridden by
an env var with prefix `DOCSIQ_` (dots → underscores, uppercased).

```yaml
server:
  host: 0.0.0.0
  port: 37778
  api_key: ""          # if set, UI + API require Authorization: Bearer <key>

llm:
  provider: ollama     # azure | openai | ollama | none
  azure:
    chat_endpoint: https://chat.openai.azure.com
    chat_api_key:  ${AZURE_OPENAI_KEY}
    chat_model:    gpt-4o
    chat_api_version: "2024-08-01"
    embed_endpoint: https://embed.openai.azure.com
    embed_api_key:  ${AZURE_OPENAI_KEY}
    embed_model:    text-embedding-3-small
    embed_api_version: "2024-08-01"
  openai:
    api_key: ${OPENAI_API_KEY}
    chat_model: gpt-4o
    embed_model: text-embedding-3-small
  ollama:
    base_url: http://localhost:11434
    chat_model: llama3.2
    embed_model: nomic-embed-text

indexing:
  workers: 4
  chunk_size: 512
  batch_size: 32

default_project: _default
```

**No LLM?** Set `provider: none`. The server still runs notes, wikilinks,
graph, tree, and notes-search. Endpoints that need the model
(`POST /api/search`, `POST /api/upload`, `/mcp` tool calls that embed or
extract) return `503 {"code": "llm_disabled"}`.

## Build

```bash
# First time on a connected machine
npm --prefix ui ci                          # install UI deps
go mod download                             # Go deps

# Build
npm --prefix ui run build                   # produces ui/dist/
CGO_ENABLED=1 go build -tags sqlite_fts5 -o docsiq ./
```

CI builds UI first and passes `ui/dist/` to each Go job as an artifact.
`ui/dist/` is **not committed**; only a tiny placeholder `ui/dist/index.html`
exists in the repo to keep `//go:embed ui/dist` happy at compile time.

## Tests

```bash
# Go
CGO_ENABLED=1 go test -tags sqlite_fts5 ./...
# Go -race integration
CGO_ENABLED=1 go test -tags "sqlite_fts5 integration" -race -timeout 1200s ./...

# UI
npm --prefix ui run typecheck
npm --prefix ui test -- --run --coverage
npm --prefix ui run build
```

## License

MIT. See [LICENSE](LICENSE).
