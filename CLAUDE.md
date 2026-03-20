# CLAUDE.md — DocsContext Development Guide

## Project Overview

DocsContext is a GraphRAG-powered documentation search tool written in Go. It indexes documents (PDF, DOCX, TXT, MD, web pages) into a knowledge graph with entity extraction, community detection, and vector embeddings, then answers queries using a combination of graph search and vector similarity.

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
  llm/         LLM provider abstraction (Azure OpenAI, Ollama)
  loader/      Document loaders (PDF, DOCX, TXT, MD, web)
  mcp/         Model Context Protocol server
  pipeline/    5-phase GraphRAG indexing pipeline
  search/      Query engine (local + global search)
  store/       SQLite storage layer
```

## Supported LLM Providers

Only **Azure OpenAI** and **Ollama** are supported. HuggingFace was removed.

## Recent Changes (already committed)

The following improvements are already committed to the branch `claude/fix-codecontext-config-DR15O`:

1. **Config fix**: Loads config from `~/.docscontext/` (lowercase) and supports both `.yaml` and `.yml`
2. **HuggingFace removal**: Dropped HuggingFace provider, config struct, and defaults
3. **GraphRAG quality improvements** (aligned with Microsoft GraphRAG):
   - **Gleanings**: Multi-pass entity extraction in `internal/extractor/entities.go` (configurable via `indexing.max_gleanings`, default: 1)
   - **Improved extraction prompt**: Few-shot examples, 10 entity types, weight guidance, implicit relationship extraction
   - **Entity name normalization**: Case-insensitive dedup in `internal/pipeline/pipeline.go`
   - **Relationship deduplication**: By (source, target, predicate) in pipeline
   - **Fixed Louvain modularity formula**: Correct ΔQ calculation in `internal/community/louvain.go`

## Remaining Task: langchaingo Integration

Replace the custom HTTP-based LLM provider implementations with [langchaingo](https://github.com/tmc/langchaingo) (v0.1.14+).

### Why

The current `internal/llm/azure.go` and `internal/llm/ollama.go` are ~250 lines of manual HTTP client code. langchaingo provides battle-tested implementations with proper error handling and retries.

### Step 1: Add langchaingo dependency

```bash
go get github.com/tmc/langchaingo@latest
```

### Step 2: Rewrite `internal/llm/provider.go`

Keep the existing `Provider` interface unchanged. Replace the implementations with a single `lcProvider` struct that wraps langchaingo:

```go
package llm

import (
    "context"
    "fmt"

    "github.com/RandomCodeSpace/docscontext/internal/config"
    "github.com/tmc/langchaingo/embeddings"
    "github.com/tmc/langchaingo/llms"
    "github.com/tmc/langchaingo/llms/ollama"
    "github.com/tmc/langchaingo/llms/openai"
)

// lcProvider adapts langchaingo to our Provider interface.
type lcProvider struct {
    llm     llms.Model
    emb     embeddings.Embedder
    name    string
    modelID string
}

func (p *lcProvider) Name() string    { return p.name }
func (p *lcProvider) ModelID() string { return p.modelID }

func (p *lcProvider) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
    o := applyOptions(opts)
    callOpts := []llms.CallOption{
        llms.WithMaxTokens(o.maxTokens),
        llms.WithTemperature(o.temperature),
    }
    if o.jsonMode {
        callOpts = append(callOpts, llms.WithJSONMode())
    }
    return llms.GenerateFromSinglePrompt(ctx, p.llm, prompt, callOpts...)
}

func (p *lcProvider) Embed(ctx context.Context, text string) ([]float32, error) {
    return p.emb.EmbedQuery(ctx, text)
}

func (p *lcProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
    return p.emb.EmbedDocuments(ctx, texts)
}
```

#### Ollama factory:

```go
func newOllamaProvider(cfg *config.LLMConfig) (Provider, error) {
    chatLLM, err := ollama.New(
        ollama.WithServerURL(cfg.Ollama.BaseURL),
        ollama.WithModel(cfg.Ollama.ChatModel),
    )
    if err != nil {
        return nil, fmt.Errorf("ollama chat LLM: %w", err)
    }
    embedLLM, err := ollama.New(
        ollama.WithServerURL(cfg.Ollama.BaseURL),
        ollama.WithModel(cfg.Ollama.EmbedModel),
    )
    if err != nil {
        return nil, fmt.Errorf("ollama embed LLM: %w", err)
    }
    emb, err := embeddings.NewEmbedder(embedLLM)
    if err != nil {
        return nil, fmt.Errorf("ollama embedder: %w", err)
    }
    return &lcProvider{llm: chatLLM, emb: emb, name: "ollama", modelID: cfg.Ollama.EmbedModel}, nil
}
```

#### Azure factory:

```go
func newAzureProvider(cfg *config.LLMConfig) (Provider, error) {
    chatLLM, err := openai.New(
        openai.WithBaseURL(cfg.Azure.Endpoint),
        openai.WithToken(cfg.Azure.APIKey),
        openai.WithAPIVersion(cfg.Azure.APIVersion),
        openai.WithAPIType(openai.APITypeAzure),
        openai.WithModel(cfg.Azure.ChatModel),
        openai.WithEmbeddingModel(cfg.Azure.EmbedModel),
    )
    if err != nil {
        return nil, fmt.Errorf("azure openai LLM: %w", err)
    }
    emb, err := embeddings.NewEmbedder(chatLLM)
    if err != nil {
        return nil, fmt.Errorf("azure openai embedder: %w", err)
    }
    return &lcProvider{llm: chatLLM, emb: emb, name: "azure", modelID: cfg.Azure.EmbedModel}, nil
}
```

### Step 3: Delete old implementations

```bash
rm internal/llm/azure.go internal/llm/ollama.go
```

### Step 4: Replace `internal/chunker/chunker.go` with langchaingo textsplitter

```go
package chunker

import (
    "unicode/utf8"
    "github.com/tmc/langchaingo/textsplitter"
)

type Chunk struct {
    Index   int
    Content string
    Tokens  int
}

type Chunker struct {
    splitter textsplitter.RecursiveCharacter
}

func New(chunkSize, chunkOverlap int) *Chunker {
    return &Chunker{
        splitter: textsplitter.NewRecursiveCharacter(
            textsplitter.WithChunkSize(chunkSize),
            textsplitter.WithChunkOverlap(chunkOverlap),
            textsplitter.WithSeparators([]string{"\n\n", "\n", ". ", " ", ""}),
        ),
    }
}

func (c *Chunker) Split(text string) []Chunk {
    parts, err := c.splitter.SplitText(text)
    if err != nil {
        return []Chunk{{Index: 0, Content: text, Tokens: estimateTokens(text)}}
    }
    chunks := make([]Chunk, len(parts))
    for i, p := range parts {
        chunks[i] = Chunk{Index: i, Content: p, Tokens: estimateTokens(p)}
    }
    return chunks
}

func estimateTokens(text string) int {
    return utf8.RuneCountInString(text) / 4
}
```

### Step 5: No changes needed for embedder

`internal/embedder/embedder.go` delegates to `Provider.EmbedBatch()` — it works as-is since the `Provider` interface is unchanged.

### Step 6: Build and verify

```bash
go mod tidy
go build ./...
go test ./...
```

### Important langchaingo API notes

- `llms.GenerateFromSinglePrompt()` — sends a single prompt and returns the text response
- `embeddings.NewEmbedder(client)` — wraps any LLM with `CreateEmbedding()` into an `Embedder`
- `embeddings.Embedder.EmbedDocuments()` returns `[][]float32` (not float64)
- `embeddings.Embedder.EmbedQuery()` returns `[]float32`
- Ollama's `LLM` and OpenAI's `LLM` both implement `CreateEmbedding(ctx, []string) ([][]float32, error)`
- OpenAI package supports Azure via `openai.WithAPIType(openai.APITypeAzure)`
- OpenAI package supports separate embedding model via `openai.WithEmbeddingModel()`

## Code Style

- Use `slog` for logging with emoji prefixes (📄 ✅ ⚠️ ❌ 🔗 🧩 💾 🌐 ⏭️ ⚙️)
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Concurrency: use semaphore channels (`make(chan struct{}, N)`) for limiting parallelism
- Config: Viper with `mapstructure` tags, env prefix `DocsContext`
