# Getting started

This guide walks you from zero to a running docsiq server with one
project indexed and the MCP endpoint answering queries.

## Prerequisites

- **Go 1.22 or newer** — required to build from source.
  ```
  go version
  # go version go1.22.x ...
  ```
- **A C toolchain** — docsiq links the `mattn/go-sqlite3` driver with
  CGO enabled, so you need a working C compiler.
  - **Linux (Debian / Ubuntu):** `sudo apt install build-essential`
  - **Linux (Fedora / RHEL):** `sudo dnf install @development-tools`
  - **macOS:** `xcode-select --install`
  - **Windows:** MinGW-w64 / MSYS2 with `gcc` on PATH, or WSL2
- **Git** — `docsiq init` reads the current repo's `origin` remote.
- **An LLM backend** — either a local Ollama daemon or an Azure OpenAI
  / OpenAI API key. The defaults point at Ollama on
  `http://localhost:11434`.

## Install

```bash
go install github.com/RandomCodeSpace/docsiq@latest
```

This drops a `docsiq` binary into `$(go env GOPATH)/bin`. Make sure
that path is on your `PATH`.

Alternatively, build from a checkout:

```bash
git clone https://github.com/RandomCodeSpace/docsiq.git
cd docsiq
CGO_ENABLED=1 go build -o docsiq .
```

## First project

1. Create a default config (optional — every field has a sensible
   default, so you can skip this step and re-run with `--config` later).

   ```bash
   mkdir -p ~/.docsiq
   cp config.example.yaml ~/.docsiq/config.yaml
   $EDITOR ~/.docsiq/config.yaml
   ```

2. From inside any git checkout, register a project:

   ```bash
   cd ~/src/my-repo
   docsiq init
   # ✅ project registered
   #   slug:   my-repo
   #   name:   my-repo
   #   remote: git@github.com:you/my-repo.git
   #   db:     ~/.docsiq/data/projects/my-repo/docsiq.db
   ```

3. Index a directory of documents (PDF / DOCX / MD / TXT) into that
   project:

   ```bash
   docsiq index ./docs --project my-repo
   docsiq index ./docs --project my-repo --finalize   # runs community detection
   ```

   Or crawl a documentation site:

   ```bash
   docsiq index --url https://example.com/docs/ --project my-repo
   ```

## Start the server

```bash
docsiq serve
# ⚙️ resolved LLM config provider=ollama
# 🌐 listening http://127.0.0.1:8080
```

Flags `--host` / `--port` override `server.host` / `server.port` in the
config. The MCP Streamable HTTP transport is served at `/mcp`; the
embedded Web UI is served at `/`.

## Smoke test

```bash
# Liveness probe — always public, never gated by auth.
curl -s http://127.0.0.1:8080/health
# {"status":"ok"}

# Project-scoped stats (bearer auth only when server.api_key is set).
curl -s -H "Authorization: Bearer $DOCSIQ_API_KEY" \
  "http://127.0.0.1:8080/api/stats?project=my-repo"
```

If `/health` returns `{"status":"ok"}` you have a working install.
Point your AI client's MCP config at `http://127.0.0.1:8080/mcp` and
optionally run `docsiq hooks install --client claude` to register the
SessionStart hook.

## Next steps

- **[Configuration](./config.md)** — swap providers, set an API key,
  tune chunk size
- **[MCP tools](./mcp-tools.md)** — what tools show up in your AI
  client and what they return
- **[Hooks](./hooks.md)** — how SessionStart hooks preload project
  context
