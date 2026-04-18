# CLAUDE.md — docsiq Development Guide

## Project Overview

docsiq is a GraphRAG-powered documentation search tool written in Go. It indexes documents (PDF, DOCX, TXT, MD, web pages) into a knowledge graph with entity extraction, community detection, and vector embeddings, then answers queries using a combination of graph search and vector similarity.

## Build & Test

```bash
go build ./...
go test ./...
go run . --help
```

## Architecture

```
cmd/           CLI commands (cobra): index, serve, search, version
internal/
  api/         REST API handlers
  chunker/     Text splitting into overlapping chunks
  community/   Louvain community detection + summarization
  config/      Viper-based YAML config loading
  crawler/     Web page crawler
  embedder/    Batched text → vector embedding
  extractor/   LLM-based entity/relationship/claims extraction
  llm/         LLM provider abstraction (Azure OpenAI, OpenAI, Ollama)
  loader/      Document loaders (PDF, DOCX, TXT, MD, web)
  mcp/         Model Context Protocol server
  pipeline/    5-phase GraphRAG indexing pipeline
  search/      Query engine (local + global search)
  store/       SQLite storage layer
```

## Supported LLM Providers

**Azure OpenAI**, **OpenAI**, and **Ollama** are supported. HuggingFace was removed.

## Completed Integrations

**langchaingo** is used for:
- **LLM providers** — `internal/llm/provider.go` wraps langchaingo for Azure OpenAI, OpenAI, and Ollama (replaced custom HTTP clients)
- **Text splitting** — `internal/chunker/chunker.go` uses `textsplitter.RecursiveCharacter`
- **PDF loading** — `internal/loader/pdf.go` uses `documentloaders.NewPDF()` (replaced pdfcpu Tj/TJ parser)

## Config & Environment

- Config dir: `~/.docsiq/`
- Env var prefix: `DOCSIQ_`
- Supports both `.yaml` and `.yml` config files

## Code Style

- Use `slog` for logging with emoji prefixes (📄 ✅ ⚠️ ❌ 🔗 🧩 💾 🌐 ⏭️ ⚙️)
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Concurrency: use semaphore channels (`make(chan struct{}, N)`) for limiting parallelism
- Config: Viper with `mapstructure` tags, env prefix `DOCSIQ_`
