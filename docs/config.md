# Configuration

docsiq's config lives at `~/.docsiq/config.yaml` (the current working
directory is also searched as a fallback). Every field can also be
supplied as an environment variable with the `DOCSIQ_` prefix and dots
replaced by underscores. Missing fields fall back to the defaults
listed below.

Source: [`internal/config/config.go`](../internal/config/config.go).

## Top-level

| Field | YAML key | Env var | Type | Default | Description |
|-------|----------|---------|------|---------|-------------|
| DataDir | `data_dir` | `DOCSIQ_DATA_DIR` | string | `~/.docsiq/data` | Root directory for every per-project SQLite DB (`$DATA_DIR/projects/<slug>/docsiq.db`) and the project registry (`$DATA_DIR/registry.db`). |
| DefaultProject | `default_project` | `DOCSIQ_DEFAULT_PROJECT` | string | `_default` | Slug used when a request omits `?project=` / `X-Project`. |
| LLMOverrides | `llm_overrides` | _(YAML only)_ | `map<slug, LLMConfig>` | `{}` | Per-project LLM overrides. Missing slug → fall back to top-level `llm`. Env binding deliberately omitted (flat env vars don't nest well into a map). |

## `llm`

The active LLM provider is selected by `llm.provider`.

| Field | YAML key | Env var | Type | Default | Description |
|-------|----------|---------|------|---------|-------------|
| Provider | `llm.provider` | `DOCSIQ_LLM_PROVIDER` | string | `ollama` | One of `ollama`, `azure`, `openai`. |

### `llm.ollama`

| Field | YAML key | Env var | Type | Default |
|-------|----------|---------|------|---------|
| BaseURL | `llm.ollama.base_url` | `DOCSIQ_LLM_OLLAMA_BASE_URL` | string | `http://localhost:11434` |
| ChatModel | `llm.ollama.chat_model` | `DOCSIQ_LLM_OLLAMA_CHAT_MODEL` | string | `llama3.2` |
| EmbedModel | `llm.ollama.embed_model` | `DOCSIQ_LLM_OLLAMA_EMBED_MODEL` | string | `nomic-embed-text` |

### `llm.azure`

Shared top-level fields act as defaults; `llm.azure.chat.*` and
`llm.azure.embed.*` override specific fields when set.

| Field | YAML key | Env var | Type | Default |
|-------|----------|---------|------|---------|
| Endpoint (shared) | `llm.azure.endpoint` | `DOCSIQ_LLM_AZURE_ENDPOINT` | string | `""` |
| APIKey (shared) | `llm.azure.api_key` | `DOCSIQ_LLM_AZURE_API_KEY` | string | `""` |
| APIVersion (shared) | `llm.azure.api_version` | `DOCSIQ_LLM_AZURE_API_VERSION` | string | `2024-08-01` |
| Chat.Endpoint | `llm.azure.chat.endpoint` | `DOCSIQ_LLM_AZURE_CHAT_ENDPOINT` | string | `""` |
| Chat.APIKey | `llm.azure.chat.api_key` | `DOCSIQ_LLM_AZURE_CHAT_API_KEY` | string | `""` |
| Chat.APIVersion | `llm.azure.chat.api_version` | `DOCSIQ_LLM_AZURE_CHAT_API_VERSION` | string | `""` |
| Chat.Model | `llm.azure.chat.model` | `DOCSIQ_LLM_AZURE_CHAT_MODEL` | string | `gpt-4o` |
| Embed.Endpoint | `llm.azure.embed.endpoint` | `DOCSIQ_LLM_AZURE_EMBED_ENDPOINT` | string | `""` |
| Embed.APIKey | `llm.azure.embed.api_key` | `DOCSIQ_LLM_AZURE_EMBED_API_KEY` | string | `""` |
| Embed.APIVersion | `llm.azure.embed.api_version` | `DOCSIQ_LLM_AZURE_EMBED_API_VERSION` | string | `""` |
| Embed.Model | `llm.azure.embed.model` | `DOCSIQ_LLM_AZURE_EMBED_MODEL` | string | `text-embedding-3-small` |

### `llm.openai`

Direct OpenAI (api.openai.com), not Azure OpenAI.

| Field | YAML key | Env var | Type | Default |
|-------|----------|---------|------|---------|
| APIKey | `llm.openai.api_key` | `DOCSIQ_LLM_OPENAI_API_KEY` | string | `""` |
| BaseURL | `llm.openai.base_url` | `DOCSIQ_LLM_OPENAI_BASE_URL` | string | `https://api.openai.com/v1` |
| ChatModel | `llm.openai.chat_model` | `DOCSIQ_LLM_OPENAI_CHAT_MODEL` | string | `gpt-4o-mini` |
| EmbedModel | `llm.openai.embed_model` | `DOCSIQ_LLM_OPENAI_EMBED_MODEL` | string | `text-embedding-3-small` |
| Organization | `llm.openai.organization` | `DOCSIQ_LLM_OPENAI_ORGANIZATION` | string | `""` |

## `indexing`

| Field | YAML key | Env var | Type | Default | Description |
|-------|----------|---------|------|---------|-------------|
| ChunkSize | `indexing.chunk_size` | `DOCSIQ_INDEXING_CHUNK_SIZE` | int | `512` | Target chunk size in tokens. |
| ChunkOverlap | `indexing.chunk_overlap` | `DOCSIQ_INDEXING_CHUNK_OVERLAP` | int | `50` | Overlap between adjacent chunks. |
| BatchSize | `indexing.batch_size` | `DOCSIQ_INDEXING_BATCH_SIZE` | int | `20` | Embedding batch size. |
| Workers | `indexing.workers` | `DOCSIQ_INDEXING_WORKERS` | int | `4` | Parallel indexing workers. |
| ExtractGraph | `indexing.extract_graph` | `DOCSIQ_INDEXING_EXTRACT_GRAPH` | bool | `true` | Run entity + relationship extraction. |
| ExtractClaims | `indexing.extract_claims` | `DOCSIQ_INDEXING_EXTRACT_CLAIMS` | bool | `true` | Run claims extraction. |
| MaxGleanings | `indexing.max_gleanings` | `DOCSIQ_INDEXING_MAX_GLEANINGS` | int | `1` | Extra gleaning passes for entity extraction (0 = single pass). |

## `community`

| Field | YAML key | Env var | Type | Default | Description |
|-------|----------|---------|------|---------|-------------|
| MinCommunitySize | `community.min_community_size` | `DOCSIQ_COMMUNITY_MIN_COMMUNITY_SIZE` | int | `2` | Drop communities smaller than this during finalize. |
| MaxLevels | `community.max_levels` | `DOCSIQ_COMMUNITY_MAX_LEVELS` | int | `3` | Max Louvain hierarchy depth. |

## `server`

| Field | YAML key | Env var | Type | Default | Description |
|-------|----------|---------|------|---------|-------------|
| Host | `server.host` | `DOCSIQ_SERVER_HOST` | string | `127.0.0.1` | Listen host. |
| Port | `server.port` | `DOCSIQ_SERVER_PORT` | int | `8080` | Listen port. |
| APIKey | `server.api_key` | `DOCSIQ_SERVER_API_KEY` / `DOCSIQ_API_KEY` | string | `""` | Bearer API key. Empty ⇒ auth disabled. |

## Per-project LLM overrides

YAML-only. Keyed by project slug. Any slug missing from the map (or
with an empty `provider`) falls back to the top-level `llm` block.

```yaml
llm:
  provider: ollama
  ollama:
    chat_model: llama3.2

llm_overrides:
  secret-docs:
    provider: azure
    azure:
      endpoint: https://my-azure.openai.azure.com
      api_key: ${AZURE_OPENAI_API_KEY}
      chat:
        model: gpt-4o
```

Applies to: `POST /api/search`, every MCP docs tool that synthesizes
(notably `global_search`), and the server-side pipeline.

## Deprecated env vars

`DOCSCONTEXT_*` env vars are auto-migrated to `DOCSIQ_*` at startup
with a `slog.Warn`. Migrate your shell rc files before v2.0.

## Full example

See [`config.example.yaml`](../config.example.yaml) in the repo root.
