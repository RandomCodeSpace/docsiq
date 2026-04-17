# REST API

Every route registered by [`internal/api/router.go`](../internal/api/router.go).
All paths are relative to the server's listen address (default
`http://127.0.0.1:8080`).

## Auth model

`/api/*` and `/mcp` are gated by bearer auth when `server.api_key`
(env: `DOCSIQ_API_KEY` or `DOCSIQ_SERVER_API_KEY`) is non-empty.
Expected header:

```
Authorization: Bearer <api_key>
```

When `server.api_key` is empty the middleware is a zero-overhead
no-op — auth is disabled. The following paths are **always public**:

- `GET /health`
- `GET /metrics`
- Any path outside `/api/` and `/mcp` (UI + static assets)
- `OPTIONS` requests (CORS preflight)

Token comparison is constant-time. Scheme match is case-sensitive
(`Bearer`, not `bearer`).

## Project scoping

Every `/api/*` handler runs through the project middleware, which
resolves a slug from (in order):

1. The `{project}` path parameter, when present.
2. The `project` query string parameter.
3. The `X-Project` request header.
4. The configured `default_project` (ships as `_default`).

An unknown slug returns `404 {"error":"..."}`.

## Endpoints

### Public

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Liveness probe. Returns `{"status":"ok"}`. |
| `GET` | `/metrics` | Prometheus scrape endpoint. |

### Docs pipeline

All require bearer auth (when configured) and accept a project slug.

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/stats` | Index statistics for the resolved project. |
| `GET` | `/api/documents` | List documents. Query: `limit`, `offset`, `doc_type`. |
| `GET` | `/api/documents/{id}` | Fetch one document by ID. |
| `GET` | `/api/documents/{id}/versions` | Version history for a document. |
| `POST` | `/api/search` | Unified search — see body shape below. |
| `GET` | `/api/graph/neighborhood` | Node/edge subgraph. Query: `entity_name`, `depth`, `max_nodes`. |
| `GET` | `/api/entities` | List entities. Query: `type`, `limit`, `offset`. |
| `GET` | `/api/communities` | List community summaries. Query: `level`, `limit`, `offset`. |
| `GET` | `/api/communities/{id}` | Fetch community + members. |
| `GET` | `/api/entities/{id}/claims` | Claims for an entity. |
| `GET` | `/api/claims` | List claims. Query: `status`, `limit`, `offset`. |
| `POST` | `/api/upload` | Multipart file upload. Kicks off a background index job. |
| `GET` | `/api/upload/progress` | Server-Sent Events stream of upload progress. |

#### `POST /api/search`

Body:
```json
{
  "query": "...",           // required
  "mode": "local",          // "local" | "global"
  "top_k": 5,               // default 5
  "graph_depth": 2          // default 2 (local only)
}
```

Response (local): `{chunks, entities, relationships}`. Response
(global): `{answer, communities}`. On bad input: `400 {"error":"..."}`.

#### `POST /api/upload`

Multipart form-data with one or more `file` parts. Returns
`{"job_id": "..."}` immediately; follow progress via
`GET /api/upload/progress?job_id=...` (SSE).

### Projects (registry)

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/projects` | List every registered project. |

Returns `[{slug, name, remote, created_at}, ...]`.

### Notes

All notes endpoints take the slug as a path parameter.

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/projects/{project}/notes` | List note keys in a project. |
| `GET` | `/api/projects/{project}/notes/{key...}` | Read a note. |
| `PUT` | `/api/projects/{project}/notes/{key...}` | Create / update a note. Body: `{content, author, tags}`. |
| `DELETE` | `/api/projects/{project}/notes/{key...}` | Delete a note. |
| `GET` | `/api/projects/{project}/tree` | Folder tree of notes. |
| `GET` | `/api/projects/{project}/search` | Full-text search. Query: `q`, `limit`. |
| `GET` | `/api/projects/{project}/graph` | Wikilink-derived notes graph. |
| `GET` | `/api/projects/{project}/export` | Tarball export of all notes. |
| `POST` | `/api/projects/{project}/import` | Import a tarball of notes. |

### Hooks

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/hook/SessionStart` | Resolve a git remote to a project and return context. |

Body:
```json
{
  "remote": "git@github.com:owner/repo.git",   // required
  "cwd": "/home/user/src/repo",                // optional
  "session_id": "..."                          // optional
}
```

Body cap: 64 KiB.

Responses:
- `200 {"project": "...", "additionalContext": "..."}` — resolved.
- `204 No Content` — remote not registered (hook stays silent).
- `400 {"error": "..."}` — malformed body.

### MCP

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/mcp` | Streamable HTTP MCP transport. |
| `GET` | `/mcp` | SSE response stream (companion to POST). |

See [mcp-tools.md](./mcp-tools.md) for the tool catalog.

## Status codes — common cases

| Code | When |
|------|------|
| `200` | Success, response body present. |
| `204` | Success, no content (hook silence; some notes deletes). |
| `400` | Malformed JSON / missing required field. |
| `401` | Missing / wrong bearer token. |
| `404` | Unknown project slug or unknown resource ID. |
| `413` | Request body exceeds the handler's cap (e.g. hook 64 KiB). |
| `500` | Store open failure, disk error, unexpected panic (caught by recovery middleware). |

Every error response body is JSON: `{"error": "...", "request_id": "..."}`.
The `request_id` is echoed as `X-Request-Id` on the response.
