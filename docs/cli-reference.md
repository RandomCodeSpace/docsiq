# CLI reference

Every subcommand and flag that `docsiq --help` surfaces, reformatted.
Defaults are shown in parentheses.

## Global flags

These apply to every subcommand.

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--config` | string | `~/.docsiq/config.yaml` | Path to the YAML config file. |
| `--log-level` | string | `info` | One of `debug`, `info`, `warn`, `error`. |
| `--log-format` | string | `text` | One of `text` or `json`. Honors `DOCSIQ_LOG_FORMAT`. |

## `docsiq` (root)

```
docsiq ingests unstructured documents, builds a knowledge graph with
community detection, and exposes an MCP server + embedded Web UI.
```

### Subcommands

- `init` — register the current git repo as a project
- `serve` — start the MCP + Web UI server
- `index` — ingest documents or crawl a docs site
- `stats` — print index statistics for a project
- `projects` — manage registered projects
- `hooks` — install/uninstall/inspect client hooks
- `vec` — inspect the active vector backend
- `version` — print the docsiq version
- `completion` — generate shell completion scripts (bash/zsh/fish/powershell)

---

## `docsiq init`

Detects the current directory's git `origin` remote, derives a slug,
and registers the project in `$DATA_DIR/registry.db`. Also creates the
per-project SQLite database at `$DATA_DIR/projects/<slug>/docsiq.db`.

```
docsiq init [flags]
```

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--name` | string | _last path segment of origin_ | Human-readable display name. |
| `--force` | bool | `false` | Overwrite an existing registry entry with the same slug. |

Requires a git repo with an `origin` remote.

---

## `docsiq serve`

Starts the MCP + Web UI server. Middleware order: logging → recovery →
bearer auth → project scope → mux.

```
docsiq serve [flags]
```

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--host` | string | `server.host` (`127.0.0.1`) | Override listen host. |
| `--port` | int | `server.port` (`8080`) | Override listen port. |

Auth: set `server.api_key` / `DOCSIQ_API_KEY` to require
`Authorization: Bearer <key>` on `/api/*` and `/mcp`. When empty, auth
is disabled.

---

## `docsiq index`

Index documents or crawl a documentation website (Phases 1–2 of the
GraphRAG pipeline).

```
docsiq index [path] [flags]
docsiq index --url https://example.com/docs [flags]
```

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--project` | string | `default_project` | Project slug to index into. |
| `--batch-size` | int | `20` | Embedding batch size. |
| `--workers` | int | `4` | Parallel indexing workers. |
| `--force` | bool | `false` | Re-index even if file hash already exists. |
| `--prune` | bool | `false` | Remove documents whose source files no longer exist on disk. |
| `--finalize` | bool | `false` | Run community detection + summaries (Phases 3–4) after indexing. |
| `--verbose` | bool | `false` | Show per-file errors. |
| `--url` | string | _unset_ | Root URL of a docs site to crawl (MkDocs, Docusaurus, Sphinx). |
| `--max-pages` | int | `500` | Max pages to crawl (0 = unlimited). |
| `--max-depth` | int | `0` | Max BFS link depth (0 = unlimited). |
| `--skip-sitemap` | bool | `false` | Force BFS crawl even if `sitemap.xml` exists. |

Either a positional path or `--url` is required.

---

## `docsiq stats`

Show index statistics for a project.

```
docsiq stats [flags]
```

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--project` | string | `default_project` | Project slug to query. |
| `--json` | bool | `false` | Emit JSON instead of a human-readable table. |

---

## `docsiq projects`

Manage per-git-remote project scopes.

### `docsiq projects register <remote>`

Register a project by explicit remote URL (no cwd / git required).

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--name` | string | _last segment of remote_ | Display name. |

### `docsiq projects list`

List all registered projects. No flags.

### `docsiq projects show <slug>`

Show project details plus on-disk DB size. No flags.

### `docsiq projects delete <slug>`

Delete a project from the registry. Use `--purge` to also delete its
data directory.

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--purge` | bool | `false` | Also remove `$DATA_DIR/projects/<slug>/`. |

---

## `docsiq hooks`

Install / remove / inspect SessionStart hooks for AI clients. See
[hooks.md](./hooks.md) for the full support matrix.

### `docsiq hooks install`

```
docsiq hooks install [flags]
```

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--client` | string | `all` | `all` \| `claude` \| `cursor` \| `copilot` \| `codex`. |
| `--hook-url` | string | `http://127.0.0.1:$DOCSIQ_SERVER_PORT` | Override the hook server URL the client posts to. |
| `--dry-run` | bool | `false` | Print what would happen without writing files. |

### `docsiq hooks uninstall`

Remove hook registration from selected client(s). Same `--client` /
`--dry-run` flags as `install`.

### `docsiq hooks status`

Report hook installation status per client. No flags.

---

## `docsiq vec`

Inspect which vector backend is active.

### `docsiq vec status`

Reports which backend is live: sqlite-vec, in-memory HNSW, or
brute-force. No flags.

---

## `docsiq version`

Prints the embedded version string. No flags.
