# MCP tools

docsiq exposes 19 MCP tools over the Streamable HTTP transport at
`POST /mcp` (and `GET /mcp` for the SSE response stream). Tools fall
into two families:

- **Docs tools (12)** — query the indexed document graph
- **Notes tools (7)** — manage per-project markdown notes

Every docs tool accepts an optional `project` string argument. An
absent / empty value resolves to the project identified by
`default_project` in config (ships as `_default`). Every notes tool
requires an explicit `project`.

Source: [`internal/mcp/tools.go`](../internal/mcp/tools.go),
[`internal/mcp/notes_tools.go`](../internal/mcp/notes_tools.go).

## Docs tools

### 1. `search_documents`

Vector similarity search over indexed document chunks.

| Arg | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| `query` | string | yes | — | Search query. |
| `top_k` | number | no | 5 | Number of results. |
| `doc_type` | string | no | — | Filter by `pdf` \| `docx` \| `txt` \| `md`. |
| `project` | string | no | `_default` | Project slug. |

Returns: JSON array of chunk objects (`id`, `doc_id`, `content`, `score`, …).

Example:
```json
{"query": "how does auth work", "top_k": 3, "project": "my-repo"}
```

### 2. `local_search`

GraphRAG local search: vector similarity + graph walk.

| Arg | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| `query` | string | yes | — | Search query. |
| `top_k` | number | no | 5 | Number of chunk results. |
| `graph_depth` | number | no | 2 | Graph walk depth. |
| `project` | string | no | `_default` | Project slug. |

Returns: `{chunks: [...], entities: [...], relationships: [...]}`.

### 3. `global_search`

GraphRAG global search: community summary aggregation with LLM
synthesis. Honors per-project LLM overrides from `llm_overrides`.

| Arg | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| `query` | string | yes | — | Search query. |
| `community_level` | number | no | 0 | Community hierarchy level. |
| `project` | string | no | `_default` | Project slug. |

Returns: `{Answer: "...", Communities: [...]}`.

### 4. `query_entity`

Get entity details and relationships by name.

| Arg | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| `entity_name` | string | yes | — | Entity name. |
| `depth` | number | no | 1 | Relationship depth. |
| `project` | string | no | `_default` | Project slug. |

Returns: `{entity: {...}, relationships: [...]}` or `{"error":"entity not found: ..."}`.

### 5. `find_relationships`

Find relationships by source, target, or predicate.

| Arg | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| `from` | string | no | — | Source entity ID. |
| `to` | string | no | — | Target entity ID. |
| `predicate` | string | no | — | Relationship predicate filter. |
| `project` | string | no | `_default` | Project slug. |

Returns: JSON array of relationship objects.

### 6. `get_graph_neighborhood`

Get a subgraph (nodes + edges JSON) for visualization.

| Arg | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| `entity_name` | string | yes | — | Center entity. |
| `depth` | number | no | 2 | BFS depth. |
| `max_nodes` | number | no | 50 | Max nodes to return. |
| `project` | string | no | `_default` | Project slug. |

Returns: `{nodes: [{id, label, type, description, rank}], edges: [{id, from, to, label, weight}]}`.

### 7. `get_document_structure`

Get the LLM-generated structured summary of a document.

| Arg | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| `doc_id` | string | yes | — | Document ID. |
| `project` | string | no | `_default` | Project slug. |

Returns: the stored `structured` JSON blob, or `{"error":"..."}`.

### 8. `list_entities`

Browse graph entities with an optional type filter.

| Arg | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| `type` | string | no | — | Entity type filter. |
| `limit` | number | no | 20 | Max results. |
| `offset` | number | no | 0 | Pagination offset. |
| `project` | string | no | `_default` | Project slug. |

Returns: JSON array of entities.

### 9. `list_documents`

Browse indexed documents.

| Arg | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| `doc_type` | string | no | — | `pdf` \| `docx` \| `txt` \| `md`. |
| `limit` | number | no | 20 | Max results. |
| `offset` | number | no | 0 | Pagination offset. |
| `project` | string | no | `_default` | Project slug. |

Returns: JSON array of documents.

### 10. `get_community_report`

Get the community summary and its member entities.

| Arg | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| `community_id` | string | yes | — | Community ID. |
| `project` | string | no | `_default` | Project slug. |

Returns: `{community: {...}, members: [...]}`.

### 11. `get_chunk`

Retrieve a specific chunk by ID.

| Arg | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| `chunk_id` | string | yes | — | Chunk ID. |
| `project` | string | no | `_default` | Project slug. |

Returns: chunk object or `{"error":"chunk not found"}`.

### 12. `stats`

Get full index statistics for a project.

| Arg | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| `project` | string | no | `_default` | Project slug. |

Returns: `{documents, chunks, entities, relationships, communities, claims, …}`.

### 13. `get_entity_claims`

List every claim extracted for an entity.

| Arg | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| `entity_id` | string | yes | — | Entity ID. |
| `project` | string | no | `_default` | Project slug. |

Returns: JSON array of claim objects.

## Notes tools

### 14. `list_projects`

List every project registered in the notes registry. No args.

Returns: `{"projects": [{slug, name, remote, created_at}, ...]}`.

### 15. `list_notes`

List all note keys in a project.

| Arg | Type | Required | Description |
|-----|------|----------|-------------|
| `project` | string | yes | Project slug. |

Returns: `{"project": "...", "keys": ["..."]}`.

### 16. `search_notes`

Full-text (SQLite FTS5) search across notes in a project.

| Arg | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| `project` | string | yes | — | Project slug. |
| `query` | string | yes | — | FTS5 query text. |
| `limit` | number | no | 20 | Max hits. |

Returns: `{"project": "...", "query": "...", "hits": [...]}`.

### 17. `read_note`

Read a single note plus its wikilink outlinks.

| Arg | Type | Required | Description |
|-----|------|----------|-------------|
| `project` | string | yes | Project slug. |
| `key` | string | yes | Note key (folders via `/`). |

Returns: note object with body and outlinks.

### 18. `write_note`

Create or update a note; indexes into FTS5.

| Arg | Type | Required | Description |
|-----|------|----------|-------------|
| `project` | string | yes | Project slug. |
| `key` | string | yes | Note key (folders via `/`). |
| `content` | string | yes | Markdown body. |
| `author` | string | no | Author name. |
| `tags` | string[] | no | Tag list. |

Returns: the written note object.

### 19. `delete_note`

Delete a note from disk and the FTS5 index.

| Arg | Type | Required | Description |
|-----|------|----------|-------------|
| `project` | string | yes | Project slug. |
| `key` | string | yes | Note key. |

Returns: `{"ok": true}`.

### 20. `get_notes_graph`

Return the wikilink-derived notes graph.

| Arg | Type | Required | Description |
|-----|------|----------|-------------|
| `project` | string | yes | Project slug. |

Returns: `{nodes: [...], edges: [...]}` where edges are `[[wikilinks]]`.

> Note: the spec calls out "19 MCP tools (12 docs + 7 notes)". The
> notes family has 7 tools: `list_projects`, `list_notes`,
> `search_notes`, `read_note`, `write_note`, `delete_note`,
> `get_notes_graph`.

## Error shape

Every tool returns a single text content item. On error the MCP result
has `isError: true` and the text is a plain error string (not JSON).
