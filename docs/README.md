# docsiq — User Documentation

docsiq is a GraphRAG documentation-search tool: it ingests PDFs, DOCX,
Markdown, plain text and docs websites, builds a knowledge graph with
community detection, and exposes that graph to AI coding agents over
the Model Context Protocol (MCP) and over a local REST + Web UI.

This directory holds the user-facing guides. Developer-facing notes
(internals, commit history, per-package READMEs) live elsewhere in the
repo.

## Table of contents

- [Getting started](./getting-started.md) — prerequisites, install, first project, smoke test
- [CLI reference](./cli-reference.md) — every subcommand and flag (`docsiq init|serve|index|stats|projects|hooks|vec|version`)
- [MCP tools](./mcp-tools.md) — the 19 MCP tools exposed at `POST /mcp` (12 docs + 7 notes)
- [REST API](./rest-api.md) — every `/api/*` endpoint, auth model, query / body / response shapes
- [Configuration](./config.md) — every config field: env var, default, type, purpose
- [Hooks](./hooks.md) — SessionStart hook integration with Claude Code, Cursor, Copilot CLI, Codex CLI
- [Architecture](./architecture.md) — per-project store layout, registry, HNSW flow, a diagram

Start with **Getting started** if you're new. Jump to **CLI reference**,
**MCP tools**, or **REST API** as a lookup. The **Configuration** and
**Architecture** pages explain the shape of `~/.docsiq/config.yaml` and
the on-disk data directory.
