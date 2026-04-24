# Block 7 — OSS Polish Implementation Plan

> **REQUIRED SUB-SKILL** — The executor of this plan MUST invoke
> `superpowers:executing-plans` before starting work. That skill enforces
> the checkpoint discipline, full-diff reads, and verification steps this
> plan assumes. Do not skip it.

---

## Goal

Bring the docsiq repository to public-facing OSS polish: a README whose
first screen delivers a working index in under 3 minutes, a contributor
on-ramp, a responsible-disclosure policy, a fully documented example
config, a top-down quickstart, a fresh screenshot gallery, and the badge
row that reflects our already-earned OpenSSF Best Practices, CodeQL, CI,
license, and Go Report Card signals. This is the last block of the
production-polish roadmap and is almost entirely documentation — the only
executable artifacts are a Playwright screenshot script and an
image-optimization step.

## Architecture

docsiq is a single Go binary (`docsiq`) that embeds a React 19 SPA via
`//go:embed ui/dist`. Public-facing polish therefore has two surfaces:
the repo root (README, CONTRIBUTING, SECURITY, example config) and
`docs/` (quickstart, screenshots). Screenshot capture requires booting
the real binary with `./docsiq serve` against a small fixture corpus and
driving Playwright against `http://localhost:37778`; no mock UI is used
because we want authentic screenshots that match what a user will see
after running the quickstart.

## Tech Stack

This block is **~95% markdown and YAML**. Executable pieces:

- **Playwright 1.54+** (already a UI dev dep) — `ui/e2e/screenshots.spec.ts`
  to capture the 5 PNGs.
- **sharp 0.33+** (already a UI dev dep) — `ui/scripts/optimize-screenshots.mjs`
  to compress each PNG to < 500 KB while preserving pixel density.
- **docsiq itself** — booted in a fixture data dir for the screenshot run.

No new dependencies. No Go code changes in this block.

## Constraints (read before every task)

- **No `.md` or config files committed unless listed in this plan.**
  Block 7 is scoped strictly to: `README.md`, `CONTRIBUTING.md`,
  `SECURITY.md`, `config.example.yaml` (rename target
  `configs/docsiq.example.yaml`), `docs/quickstart.md`, `docs/samples/*`,
  `docs/screenshots/*.png`, `ui/e2e/screenshots.spec.ts`,
  `ui/scripts/optimize-screenshots.mjs`.
- **Do not rewrite existing sections wholesale** unless a task explicitly
  says so. Preserve the voice, emoji usage (📄 ✅ ⚠️), and architecture
  list from the current README.
- **Air-gapped compatibility**: the README install command assumes a
  release binary downloaded from GitHub Releases. Confirm the release
  asset naming matches the current CI upload pattern before pasting the
  command as-is — in particular, that `docsiq-linux-amd64` (and not
  `docsiq_linux_amd64` or a tarball) is the emitted artifact name. If it
  differs, update the plan's README snippet to match before writing.
- **OpenSSF Best Practices badge URL** in the current README is
  `https://www.bestpractices.dev/projects/12628/badge` — keep that exact
  URL in the new badge row; do not re-derive the project ID.
- **Repo URL is `github.com/RandomCodeSpace/docsiq`** everywhere.
- **License is MIT**, file at `LICENSE`.
- **Conventional Commit style** is expected — feat/fix/docs/chore/refactor.
  The CONTRIBUTING.md must state this and PR titles should match.

## Task ordering rationale

The seven tasks are ordered by dependency fan-in, not by spec number:

1. **Task 1 — 7.3 SECURITY.md** — smallest, zero dependencies.
2. **Task 2 — 7.2 CONTRIBUTING.md** — small, self-contained.
3. **Task 3 — 7.4 configs/docsiq.example.yaml** — reference material the
   quickstart uses.
4. **Task 4 — 7.5 docs/quickstart.md** — consumes the example config,
   introduces sample-corpus concept used by screenshots.
5. **Task 5 — 7.6 Screenshots** — needs a running UI; fixture corpus set
   up in Task 4.
6. **Task 6 — 7.7 Badge row** — small README patch that references the
   screenshots committed in Task 5.
7. **Task 7 — 7.1 README refactor** — biggest; integrates everything
   above (badges from Task 6, screenshots from Task 5, links to
   CONTRIBUTING/SECURITY/quickstart from Tasks 1/2/4).

Each task is independently committable. Do not squash across tasks.

---

## Task 1: Write SECURITY.md (7.3)

**Spec:** report channel (email alias or GitHub private advisory),
disclosure policy, supported versions, fix SLA.

**File:** `/home/dev/projects/docsiq/SECURITY.md` (overwrite existing)

### Steps

- [ ] **Step 1** — Read the existing `SECURITY.md` at repo root to see
      what the current policy promises. Do not delete any explicit
      commitments already made to the community; if a stricter SLA is
      already stated, keep the stricter one.
- [ ] **Step 2** — Confirm the GitHub private security advisory URL is
      reachable at
      `https://github.com/RandomCodeSpace/docsiq/security/advisories/new`
      (visit once in a browser; this is a static GitHub surface so no
      code check is required).
- [ ] **Step 3** — Overwrite `SECURITY.md` with the exact content below.
- [ ] **Step 4** — Self-review: read the full diff. Confirm: (a) the
      advisory URL is exact, (b) supported-versions section lists both
      "latest released tag" and "main branch HEAD", (c) the SLA table
      has three severity rows.
- [ ] **Step 5** — Commit with message:
      `docs(security): publish disclosure policy, supported versions, and fix SLA`

### Exact content for SECURITY.md

```markdown
# Security Policy

Thanks for helping keep docsiq and its users safe. This document
describes how to report a security issue, what you can expect from us,
and which versions receive fixes.

## Reporting a vulnerability

**Please do not open a public issue.** Use one of the following private
channels:

1. **GitHub private security advisory** (preferred) —
   <https://github.com/RandomCodeSpace/docsiq/security/advisories/new>.
   This is the fastest path; the maintainers are notified directly and
   the report stays private until a fix ships.
2. **Encrypted email** — if you cannot use GitHub advisories, email the
   maintainers with the subject prefix `[SECURITY] docsiq:`. Contact
   details are on the project's GitHub profile. PGP keys available on
   request.

When reporting, please include:

- A description of the issue and its impact.
- Steps to reproduce, ideally with a minimal proof of concept.
- The affected version, commit SHA, and platform.
- Any suggested mitigation or patch you have in mind.

We will acknowledge your report within **3 business days**.

## Disclosure policy

docsiq follows **coordinated disclosure**. The default embargo window is
**90 days** from the acknowledgement date, during which we will work
with you on a fix, a CVE request (where applicable), and a public
advisory. We are happy to credit you in the advisory — tell us how you
would like to be named.

If a fix ships before the 90-day window ends, we will publish the
advisory at release time. If we need more time (e.g. upstream dependency
fix required), we will tell you why and propose a revised date.

## Supported versions

We issue security fixes for:

- **The latest released tag** on the `main` branch (see
  [Releases](https://github.com/RandomCodeSpace/docsiq/releases)).
- **`main` branch HEAD** — security fixes land here first and are
  included in the next tagged release.

Older tags are not patched; please upgrade to the latest release.

## Fix SLA

| Severity  | Target fix window | Notes                                                                 |
|-----------|-------------------|-----------------------------------------------------------------------|
| Critical  | 7 days            | Remote code execution, auth bypass, data corruption at rest.          |
| High      | 30 days           | Privilege escalation, unauthenticated read of sensitive data.         |
| Medium    | 90 days           | Authenticated flaws with limited blast radius.                        |
| Low       | Best effort       | Hardening improvements, defence-in-depth, theoretical issues.         |

These are targets, not guarantees. We will tell you up front if we
cannot meet one and why.

## Scope

In scope:

- The `docsiq` binary and everything under this repository.
- Default configuration as shipped.
- Vulnerabilities in our direct dependencies that are reachable through
  docsiq.

Out of scope:

- Upstream vulnerabilities in transitive dependencies that are not
  reachable from docsiq. Please report those to the upstream project;
  we will track and upgrade when a patched version ships.
- Misconfigurations introduced by a downstream user (e.g. binding a
  public port with no API key set).
- Denial of service via resource exhaustion on a self-hosted instance
  the attacker already has network access to.

## Safe harbor

We will not pursue legal action against researchers who act in good
faith, follow this policy, stay within scope, avoid privacy violations,
and do not degrade service for other users. If in doubt, ask first.
```

### Commit

```bash
git add SECURITY.md
git commit -m "$(cat <<'EOF'
docs(security): publish disclosure policy, supported versions, and fix SLA

Documents the preferred GitHub private advisory channel, the 90-day
coordinated-disclosure default, and per-severity fix targets. Establishes
that only the latest tag and main HEAD receive patches.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Success criteria

- `SECURITY.md` exists at repo root with the content above.
- No other files touched in this commit.
- `git log -1 --name-only` shows a single-file change.

---

## Task 2: Write CONTRIBUTING.md (7.2)

**Spec:** local dev loop (Go + UI), test commands, pre-commit hook
setup, conventional commit style, PR template pointer.

**File:** `/home/dev/projects/docsiq/CONTRIBUTING.md` (overwrite
existing)

### Steps

- [ ] **Step 1** — Read the existing `CONTRIBUTING.md`. Note any
      project-specific conventions already documented (e.g. how tests
      are run, required tags) that must be preserved.
- [ ] **Step 2** — Verify the PR template path. Check whether
      `.github/pull_request_template.md` exists. If it does not, the
      "PR template" reference in CONTRIBUTING becomes a note "PR
      template is not configured; use a clear, conventional-commit-style
      title". If it does exist, reference it by path.
- [ ] **Step 3** — Verify the Go build tags referenced in CLAUDE.md
      still apply: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./...`. This
      is the hard truth for the repo — do not drift.
- [ ] **Step 4** — Verify that the UI test command is
      `npm --prefix ui test -- --run --coverage` (matches existing
      README) and that typecheck is `npm --prefix ui run typecheck`.
- [ ] **Step 5** — Check if a `.pre-commit-config.yaml` or equivalent
      hook scaffolding exists in the repo. If it does, document the
      exact `pre-commit install` command. If it does not, provide a
      minimal `pre-commit-config.yaml` snippet in the doc (gofmt, go
      vet, prettier on ui/) and a one-line bootstrap that the
      contributor runs locally — do not commit a new
      `.pre-commit-config.yaml` as part of this block; the doc is
      instructional.
- [ ] **Step 6** — Overwrite `CONTRIBUTING.md` with the exact content
      below. If Step 2 determined that no PR template exists, adjust
      the "PR checklist" section to remove the template path reference
      and replace with the note above.
- [ ] **Step 7** — Self-review: read the full diff. Confirm: Go +
      UI build commands are executable as-pasted, conventional commit
      prefixes match what the repo already uses
      (`git log --oneline -30` to sanity-check), the PR template link
      is correct.
- [ ] **Step 8** — Commit with message:
      `docs(contributing): write local dev loop, test commands, and commit style guide`

### Exact content for CONTRIBUTING.md

```markdown
# Contributing to docsiq

Welcome — and thank you for considering a contribution. This document is
the concrete guide for getting docsiq building on your machine, making a
change, and submitting it.

## TL;DR

```bash
# 1. Fork + clone
git clone https://github.com/<your-user>/docsiq && cd docsiq

# 2. Install UI deps and build the SPA (needed by the Go embed)
npm --prefix ui ci
npm --prefix ui run build

# 3. Build and test the Go binary
CGO_ENABLED=1 go build -tags sqlite_fts5 -o docsiq ./
CGO_ENABLED=1 go test  -tags sqlite_fts5 ./...

# 4. Run the UI tests
npm --prefix ui run typecheck
npm --prefix ui test -- --run --coverage
```

If these all pass, you're ready to make changes.

## Prerequisites

- **Go** — version from `go.mod` (`go mod edit -json | jq -r .Go`) or
  newer. A working CGO toolchain is required (`gcc` on Linux, Xcode CLT
  on macOS, mingw on Windows).
- **Node.js** — 20.x or newer, for the Vite-based UI.
- **SQLite FTS5 + sqlite-vec** — both are linked into the binary via the
  `sqlite_fts5` build tag and the vendored `sqlite-vec` extension. No
  separate install is needed; CGO pulls them in.
- **Git** — 2.30+.

## Local dev loop

docsiq has two surfaces that move at different speeds.

### Backend (Go)

```bash
# Build a binary
CGO_ENABLED=1 go build -tags sqlite_fts5 -o docsiq ./

# Run all unit tests (fast)
CGO_ENABLED=1 go test -tags sqlite_fts5 ./...

# Run integration tests with race detector (slow)
CGO_ENABLED=1 go test -tags "sqlite_fts5 integration" -race -timeout 1200s ./...

# Format + vet
gofmt -s -w .
go vet -tags sqlite_fts5 ./...
```

### Frontend (React SPA)

```bash
cd ui

# Dev server with HMR against a running `docsiq serve`
npm run dev

# Typecheck (tsc --noEmit)
npm run typecheck

# Unit tests with coverage
npm test -- --run --coverage

# Production build (output to ui/dist/)
npm run build
```

The Go binary embeds `ui/dist/` via `//go:embed`, so **any UI change
requires `npm --prefix ui run build` before the change shows up in a
built binary**. For iterative UI work, run `./docsiq serve` in one
terminal and `npm --prefix ui run dev` in another; Vite will proxy API
calls to the backend.

### End-to-end

```bash
# Boot the binary against a fixture data dir
DOCSIQ_DATA_DIR=/tmp/docsiq-dev ./docsiq serve --port 37778 &
cd ui && npm run test:e2e
```

## Pre-commit hooks (optional but recommended)

We recommend [pre-commit](https://pre-commit.com/) to keep `gofmt`,
`go vet`, and `prettier` from slipping. A minimal config:

```yaml
# .pre-commit-config.yaml — keep on your fork, or PR it to the repo
repos:
  - repo: https://github.com/dnephin/pre-commit-golang
    rev: v0.5.1
    hooks:
      - id: go-fmt
      - id: go-vet
  - repo: https://github.com/pre-commit/mirrors-prettier
    rev: v3.1.0
    hooks:
      - id: prettier
        files: \.(ts|tsx|js|jsx|css|md)$
```

Then:

```bash
pip install pre-commit   # or: brew install pre-commit
pre-commit install
```

## Commit style

We use **Conventional Commits**. The subject line is:

```
<type>(<scope>): <summary>
```

Types we use:

- `feat` — user-facing new capability.
- `fix` — user-visible bug fix.
- `refactor` — internal reshuffle, no behaviour change.
- `perf` — measured speedup or memory reduction.
- `docs` — documentation only.
- `test` — tests only, no production code change.
- `chore` — tooling, CI, dependency bumps.
- `build` — build system, Makefile, CI pipeline.

Scope (optional) is usually a directory or subsystem —
`feat(extractor): …`, `fix(ui): …`, `chore(deps): …`.

**Message body** — wrap at 72 chars, explain *why*, not *what*. The diff
already shows *what*.

**Footer** — when the commit is authored or pair-coded with an AI agent,
include a `Co-Authored-By:` trailer. Do **not** force-push to shared
branches (`main`, `release/*`) — see the project's Git discipline rules
at [`~/.claude/rules/git.md`](https://github.com/RandomCodeSpace/docsiq/blob/main/.claude/rules/README.md)
if you're contributing under the same conventions.

## Pull requests

### Before opening

- Branch from `main`. Use a descriptive branch name:
  `feat/community-summaries`, `fix/mcp-handshake-timeout`, etc.
- Rebase on top of the latest `main` before opening the PR.
- Run the full test suite (Go + UI) — `go test`, `npm test`, typecheck,
  build. The CI will run them anyway; running locally is faster feedback.
- Re-read your own diff. `git diff main...HEAD` in the terminal, top to
  bottom. Most self-inflicted review comments are caught this way.

### PR template

Use a clear, specific title following the Conventional Commits format,
then describe:

1. **Problem / motivation** — one or two sentences on what this PR
   changes and why.
2. **Approach** — key design choices you made and anything you
   explicitly rejected.
3. **Tests** — which layers are covered (unit / integration / e2e) and
   what remains untested.
4. **Screenshots** — for UI changes, include before/after. See
   `docs/screenshots/` for capture conventions.
5. **Follow-ups** — known limitations or deferred items (with a link
   to an issue where possible).

If `.github/pull_request_template.md` exists in your fork, it will be
pre-filled when you open the PR.

### During review

- Be responsive but not hasty — take time to think about comments.
- If a reviewer is wrong, push back with a clear reason. We prefer
  disagreement over silent compliance; see
  [`~/.claude/rules/git.md`](https://github.com/RandomCodeSpace/docsiq)
  for the general tone.

### After merge

- Delete your branch.
- If your PR earned a release-worthy mention, add a line to the next
  release's draft notes (maintainers can help).

## Coding conventions

- **Go** — follow `gofmt`; `go vet` must pass; error wrapping is
  `fmt.Errorf("context: %w", err)`; logging via `slog` with the emoji
  prefixes used elsewhere (📄 ✅ ⚠️ ❌ 🔗 🧩 💾 🌐 ⏭️ ⚙️); concurrency
  uses semaphore channels (`make(chan struct{}, N)`) for bounded
  parallelism.
- **TypeScript** — strict mode on; prefer explicit types over `any`;
  CSS lives in `globals.css` `@layer components`, JSX uses semantic
  class names only — Tailwind utilities stay inside shadcn primitives.
- **Tests** — write tests for new logic, including failure paths, not
  just happy path. Flaky tests are broken tests — fix, quarantine, or
  delete in the same PR.

## License and CLA

By contributing, you agree your contributions will be licensed under the
same MIT license as the project (see [LICENSE](LICENSE)). We do not
require a separate CLA.

## Code of conduct

Be kind. See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## Questions

Open a GitHub discussion or a draft PR. Small questions are welcome —
nobody was born knowing GraphRAG.
```

### Commit

```bash
git add CONTRIBUTING.md
git commit -m "$(cat <<'EOF'
docs(contributing): write local dev loop, test commands, and commit style guide

Covers both the Go (CGO + sqlite_fts5 tag) and UI (Vite + Vitest)
surfaces, the recommended pre-commit setup, and the Conventional Commits
format the project uses. Replaces the previous stub.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Success criteria

- `CONTRIBUTING.md` exists with the content above.
- A new contributor can run all five commands in the TL;DR block and
  they all work on a fresh clone.
- Conventional commit types listed match what the repo's history uses
  (verify with `git log --format=%s -100 | head -30`).

---

## Task 3: Write configs/docsiq.example.yaml (7.4)

**Spec:** every option present with inline comments describing purpose,
default, env-var override.

**File:** `/home/dev/projects/docsiq/configs/docsiq.example.yaml` (new)

### Steps

- [ ] **Step 1** — Verify the full set of config fields from
      `internal/config/config.go`: `data_dir`, `default_project`,
      `llm.provider`, `llm.azure.{endpoint,api_key,api_version,chat.*,embed.*}`,
      `llm.openai.{api_key,base_url,chat_model,embed_model,organization}`,
      `llm.ollama.{base_url,chat_model,embed_model}`,
      `indexing.{chunk_size,chunk_overlap,batch_size,workers,extract_graph,extract_claims,max_gleanings}`,
      `community.{min_community_size,max_levels}`,
      `server.{host,port,api_key,max_upload_bytes,workq_workers,workq_depth}`,
      `llm_overrides.<slug>.*`. Verify by re-reading the `Load()`
      function in `internal/config/config.go` — every `v.SetDefault`
      call must have a corresponding key in the example YAML.
- [ ] **Step 2** — Create the `configs/` directory if it does not
      already exist: `mkdir -p configs`.
- [ ] **Step 3** — Write the file with the exact content below.
- [ ] **Step 4** — Keep the legacy `config.example.yaml` at the repo
      root as a symlink or a pointer. Recommended: overwrite
      `config.example.yaml` with a two-line pointer
      (`# This file has moved to configs/docsiq.example.yaml`) so
      existing bookmarks still resolve. Do not delete it outright —
      that's a breaking change for anyone who linked to it.
- [ ] **Step 5** — Self-review: every default value in the YAML
      matches a `v.SetDefault(...)` call in `config.go`. Every env var
      name matches the `DOCSIQ_<KEY>` replacer output (dots →
      underscores, uppercased). Inline comments explain purpose and
      cross-reference the struct field.
- [ ] **Step 6** — Commit with message:
      `docs(config): publish fully annotated example config with env-var overrides`

### Exact content for configs/docsiq.example.yaml

```yaml
# docsiq example configuration
#
# Copy this file to one of the locations docsiq checks on startup:
#   - ~/.docsiq/config.yaml         (global, per-user)
#   - ./config.yaml                 (current working directory)
#
# Every key listed here can be overridden by an environment variable.
# The rule is: prefix with DOCSIQ_, replace dots with underscores,
# uppercase everything. For example:
#   server.port                     → DOCSIQ_SERVER_PORT
#   llm.azure.chat.endpoint         → DOCSIQ_LLM_AZURE_CHAT_ENDPOINT
#   indexing.workers                → DOCSIQ_INDEXING_WORKERS
#
# Two convenience aliases exist:
#   DOCSIQ_API_KEY                  → server.api_key
#   DOCSIQ_SERVER_API_KEY           → server.api_key
#
# Fields are documented with: (default), purpose, env var.

# ---------------------------------------------------------------------------
# Storage
# ---------------------------------------------------------------------------

# Root directory for the per-project SQLite stores and notes.
# Default: ~/.docsiq/data
# Env:     DOCSIQ_DATA_DIR
data_dir: ~/.docsiq/data

# The project slug used when a request does not specify ?project= or
# X-Project. Must match a slug registered via `docsiq projects register`.
# Default: _default
# Env:     DOCSIQ_DEFAULT_PROJECT
default_project: _default

# ---------------------------------------------------------------------------
# LLM provider
# ---------------------------------------------------------------------------

llm:
  # One of: azure | openai | ollama | none
  # "none" disables all LLM-backed endpoints; notes/graph still work.
  # Default: ollama
  # Env:     DOCSIQ_LLM_PROVIDER
  provider: ollama

  # Azure OpenAI. Shared endpoint/api_key/api_version apply unless the
  # chat.* or embed.* sub-blocks override them.
  azure:
    # Shared defaults — leave empty if you configure chat/embed separately.
    endpoint: ""         # Env: DOCSIQ_LLM_AZURE_ENDPOINT
    api_key: ""          # Env: DOCSIQ_LLM_AZURE_API_KEY
    api_version: "2024-08-01"  # Env: DOCSIQ_LLM_AZURE_API_VERSION

    chat:
      endpoint: ""       # Env: DOCSIQ_LLM_AZURE_CHAT_ENDPOINT
      api_key: ""        # Env: DOCSIQ_LLM_AZURE_CHAT_API_KEY
      api_version: ""    # Env: DOCSIQ_LLM_AZURE_CHAT_API_VERSION
      model: gpt-4o      # Env: DOCSIQ_LLM_AZURE_CHAT_MODEL

    embed:
      endpoint: ""       # Env: DOCSIQ_LLM_AZURE_EMBED_ENDPOINT
      api_key: ""        # Env: DOCSIQ_LLM_AZURE_EMBED_API_KEY
      api_version: ""    # Env: DOCSIQ_LLM_AZURE_EMBED_API_VERSION
      model: text-embedding-3-small
                         # Env: DOCSIQ_LLM_AZURE_EMBED_MODEL

  # Direct OpenAI (api.openai.com), distinct from Azure OpenAI.
  openai:
    # Required when provider is "openai". Use env, not the YAML.
    api_key: ""                         # Env: DOCSIQ_LLM_OPENAI_API_KEY

    # For custom proxies or gateways; leave as default for api.openai.com.
    base_url: https://api.openai.com/v1 # Env: DOCSIQ_LLM_OPENAI_BASE_URL

    # Chat completion model.
    chat_model: gpt-4o-mini             # Env: DOCSIQ_LLM_OPENAI_CHAT_MODEL

    # Embedding model for vector search.
    embed_model: text-embedding-3-small # Env: DOCSIQ_LLM_OPENAI_EMBED_MODEL

    # Optional OpenAI-Organization header for billing routing.
    organization: ""                    # Env: DOCSIQ_LLM_OPENAI_ORGANIZATION

  # Ollama (self-hosted). Default when nothing else is configured.
  ollama:
    base_url: http://localhost:11434    # Env: DOCSIQ_LLM_OLLAMA_BASE_URL
    chat_model: llama3.2                # Env: DOCSIQ_LLM_OLLAMA_CHAT_MODEL
    embed_model: nomic-embed-text       # Env: DOCSIQ_LLM_OLLAMA_EMBED_MODEL

# ---------------------------------------------------------------------------
# Per-project LLM overrides (YAML only — env vars cannot nest like this)
# ---------------------------------------------------------------------------
# llm_overrides:
#   my-project-slug:
#     provider: openai
#     openai:
#       chat_model: gpt-4o
#   another-slug:
#     provider: ollama
#     ollama:
#       chat_model: mistral

# ---------------------------------------------------------------------------
# Indexing pipeline
# ---------------------------------------------------------------------------

indexing:
  # Target size (in characters, not tokens) per chunk.
  # Default: 512. Env: DOCSIQ_INDEXING_CHUNK_SIZE
  chunk_size: 512

  # Overlap between successive chunks, in characters.
  # Default: 50. Env: DOCSIQ_INDEXING_CHUNK_OVERLAP
  chunk_overlap: 50

  # How many chunks to embed per LLM batch request.
  # Default: 20. Env: DOCSIQ_INDEXING_BATCH_SIZE
  batch_size: 20

  # Parallel workers for document-level operations (load → chunk → embed).
  # Default: 4. Env: DOCSIQ_INDEXING_WORKERS
  workers: 4

  # Run entity + relationship extraction during indexing.
  # Set false to build vector-only index (faster, no graph queries).
  # Default: true. Env: DOCSIQ_INDEXING_EXTRACT_GRAPH
  extract_graph: true

  # Extract covariates / claims associated with entity mentions.
  # Default: true. Env: DOCSIQ_INDEXING_EXTRACT_CLAIMS
  extract_claims: true

  # Number of "continue extracting" passes over the same chunk. Higher
  # values recover more entities at the cost of LLM tokens.
  # Default: 1. Env: DOCSIQ_INDEXING_MAX_GLEANINGS
  max_gleanings: 1

# ---------------------------------------------------------------------------
# Community detection
# ---------------------------------------------------------------------------

community:
  # Minimum number of nodes in a detected community before it's reported.
  # Default: 2. Env: DOCSIQ_COMMUNITY_MIN_COMMUNITY_SIZE
  min_community_size: 2

  # Depth of hierarchical Louvain clustering. Higher → more layers of
  # nested communities, at the cost of longer indexing time.
  # Default: 3. Env: DOCSIQ_COMMUNITY_MAX_LEVELS
  max_levels: 3

# ---------------------------------------------------------------------------
# HTTP server (API + UI + MCP)
# ---------------------------------------------------------------------------

server:
  # Bind address. Use 0.0.0.0 for LAN access. 127.0.0.1 binds loopback.
  # Default: 127.0.0.1. Env: DOCSIQ_SERVER_HOST
  host: 127.0.0.1

  # Listen port.
  # Default: 8080. Env: DOCSIQ_SERVER_PORT
  port: 8080

  # If set, every API + MCP request must carry
  # "Authorization: Bearer <key>". Leave empty to disable.
  # Default: "". Env: DOCSIQ_SERVER_API_KEY (alias: DOCSIQ_API_KEY)
  api_key: ""

  # Maximum upload size in bytes for POST /api/upload. 0 or negative
  # disables the cap (not recommended).
  # Default: 104857600 (100 MiB). Env: DOCSIQ_SERVER_MAX_UPLOAD_BYTES
  max_upload_bytes: 104857600

  # Number of background workers servicing the indexing work queue.
  # 0 → runtime.NumCPU().
  # Default: 0. Env: DOCSIQ_SERVER_WORKQ_WORKERS
  workq_workers: 0

  # Maximum queued jobs before /api/upload starts returning 429.
  # Default: 64. Env: DOCSIQ_SERVER_WORKQ_DEPTH
  workq_depth: 64
```

### Commit

```bash
git add configs/docsiq.example.yaml config.example.yaml
git commit -m "$(cat <<'EOF'
docs(config): publish fully annotated example config with env-var overrides

Every ServerConfig, LLMConfig (Azure/OpenAI/Ollama), IndexingConfig,
and CommunityConfig field is present with its default and the
DOCSIQ_ env-var override. The old root-level config.example.yaml now
points at the new location.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Success criteria

- Every `v.SetDefault` call in `internal/config/config.go` is
  represented by a key in the YAML.
- `docsiq serve --config configs/docsiq.example.yaml` starts without
  errors (defaults are internally consistent).
- `DOCSIQ_SERVER_PORT=9999 docsiq serve --config configs/docsiq.example.yaml`
  binds on :9999, proving env overrides work.

---

## Task 4: Write docs/quickstart.md and sample corpus (7.5)

**Spec:** walks a user through indexing a small sample (`docs/samples/`)
and running a search, top-down.

**Files:**

- `/home/dev/projects/docsiq/docs/quickstart.md` (new)
- `/home/dev/projects/docsiq/docs/samples/README.md` (new)
- `/home/dev/projects/docsiq/docs/samples/*.md` — 3-5 tiny markdown
  files to form the corpus

### Steps

- [ ] **Step 1** — Create `docs/samples/`:
      `mkdir -p /home/dev/projects/docsiq/docs/samples`.
- [ ] **Step 2** — Author three sample documents. Keep each under 1 KB;
      the point is a corpus small enough to index in <30 seconds on a
      laptop, with enough entity density to produce interesting
      graph/community output. Use the exact content for each file below.
- [ ] **Step 3** — Add `docs/samples/README.md` explaining what the
      samples are for and that they are suitable for screenshots + the
      quickstart. Exact content below.
- [ ] **Step 4** — Author the quickstart at `docs/quickstart.md`. Exact
      content below. Cross-references:
      (a) `configs/docsiq.example.yaml` from Task 3;
      (b) `CONTRIBUTING.md` from Task 2 for dev setup;
      (c) `README.md` for the badge row (to be added in Task 6).
- [ ] **Step 5** — Verify the commands actually work end-to-end. On a
      machine with docsiq built, run every shell block in the
      quickstart and confirm each produces the output stated. If any
      step does not match reality, update the quickstart — do not
      update reality to match the quickstart.
- [ ] **Step 6** — Self-review: the 3-minute target is real. Time
      yourself (or a clean-vm equivalent) following the quickstart from
      scratch. If it takes longer, trim, preconfigure more, or explain
      the wait.
- [ ] **Step 7** — Commit with message:
      `docs(quickstart): add top-down quickstart with sample corpus`

### Exact content for docs/samples/README.md

```markdown
# Sample corpus

These tiny markdown documents exist to:

1. Back the [quickstart](../quickstart.md) — a user can index this
   directory in under 30 seconds and ask a meaningful question.
2. Populate the screenshots in [../screenshots/](../screenshots/) with
   realistic but non-proprietary data.

The corpus is intentionally tiny. For a real workload, point `docsiq
index` at a folder of your actual documents (PDF, DOCX, TXT, MD, or the
output of `docsiq crawl`).
```

### Exact content for docs/samples/roman-aqueducts.md

```markdown
# Roman Aqueducts

Roman aqueducts were a network of structures that supplied water to
cities across the Roman Empire. The first, the Aqua Appia, was
constructed in 312 BCE under the censor Appius Claudius Caecus. By the
first century CE, eleven major aqueducts served Rome, delivering an
estimated one million cubic metres of water per day.

Gravity, not pumping, moved the water. Engineers held the gradient
between 1:200 and 1:500 over distances exceeding a hundred kilometres,
using arcades to bridge valleys and inverted siphons where terrain
required pressure flow.

The Pont du Gard in modern-day France is the most famous surviving
example; the Aqua Claudia and Anio Novus, both completed in 52 CE under
the emperor Claudius, are the longest. Maintenance was the
responsibility of the curator aquarum — the water commissioner — an
office held by Sextus Julius Frontinus, whose treatise *De Aquaeductu*
remains the principal source on the subject.
```

### Exact content for docs/samples/graphrag.md

```markdown
# GraphRAG

GraphRAG is a retrieval-augmented generation (RAG) technique introduced
by Microsoft Research in 2024 that augments vector search with an
explicit knowledge graph. Instead of retrieving only the top-k chunks
semantically similar to a query, GraphRAG extracts entities, relations,
and claims from each chunk, builds a graph, runs Louvain community
detection on it, and then serves queries against either local (entity
neighbourhood) or global (community summary) views.

The key claim is that graph-derived structure recovers global context
that pure vector search cannot: "who are the main actors in this
corpus", "what are the dominant themes", questions that require a view
of the whole rather than a handful of passages.

docsiq is a Go implementation of this technique, shipping as a single
binary with an embedded React UI. It supports Azure OpenAI, OpenAI, and
Ollama as LLM providers, storing everything in SQLite with FTS5 and the
sqlite-vec extension for ANN vector search.
```

### Exact content for docs/samples/louvain.md

```markdown
# Louvain community detection

Louvain is a greedy algorithm for community detection in large networks,
published by Blondel, Guillaume, Lambiotte, and Lefebvre in 2008. It
optimises modularity — a scalar that rewards dense within-community
links and sparse between-community links — through two alternating
phases: local move (every node is considered for reassignment to the
community that most increases modularity) and aggregation (each detected
community becomes a super-node in the next iteration).

The algorithm terminates when no move increases modularity. Runtime is
roughly linear in the number of edges, which is why Louvain scales to
graphs of tens of millions of nodes where spectral methods do not.

GraphRAG (see graphrag.md) uses Louvain to partition its entity graph
into nested communities. Each community gets an LLM-generated summary,
which is what the "global" search mode retrieves.
```

### Exact content for docs/quickstart.md

```markdown
# Quickstart

Go from zero to a queryable knowledge graph in under three minutes.

## What you'll do

1. Install the `docsiq` binary.
2. Register the current directory as a docsiq project.
3. Index a small sample corpus of three markdown documents.
4. Ask a question.
5. Open the UI and see the graph.

The sample corpus lives at [`docs/samples/`](samples/); it's three
short markdown files about Roman aqueducts, GraphRAG, and Louvain
community detection. Small enough to index in ~30 seconds, dense enough
to produce interesting entities and a multi-community graph.

## 1. Install

Download the latest release for your platform. Replace
`docsiq-linux-amd64` with the asset name matching your OS if needed
(macOS arm64, Windows amd64 assets are published alongside).

```bash
curl -LO https://github.com/RandomCodeSpace/docsiq/releases/latest/download/docsiq-linux-amd64
chmod +x docsiq-linux-amd64
mv docsiq-linux-amd64 ~/.local/bin/docsiq   # or any directory on your PATH
```

Verify:

```bash
docsiq version
```

Building from source is also supported and takes about a minute
end-to-end; see [CONTRIBUTING.md](../CONTRIBUTING.md) for the build
instructions.

## 2. Register a project

```bash
cd ~/path/to/any/directory     # or stay in the docsiq repo for the demo
docsiq init
```

`docsiq init` registers the current directory as a project and creates a
scope-specific SQLite store at `~/.docsiq/data/projects/<slug>/`. If
you're in a git repo, the slug is derived from the repo's remote origin;
otherwise you'll be prompted for a name.

## 3. Index the sample corpus

From the repository root (so that `docs/samples/` resolves):

```bash
docsiq index docs/samples/
```

You will see log lines for each phase:

```
⚙️ loaded config file path=/home/you/.docsiq/config.yaml
📄 loading documents count=3
🧩 chunking chunks=12
🌐 embedding batches=1
🔗 extracting entities entities=18 relationships=24
🧩 detecting communities levels=3 communities=5
✅ index complete duration=21.4s
```

If you are running without an LLM configured
(`DOCSIQ_LLM_PROVIDER=none` or `llm.provider: none` in the config),
entity extraction and embedding steps are skipped; you'll still get a
keyword-searchable corpus and a notes graph.

## 4. Ask a question

```bash
docsiq search "Who built the first Roman aqueduct?"
```

Expected (with an LLM configured):

```
Answer: Appius Claudius Caecus built the first Roman aqueduct, the
Aqua Appia, in 312 BCE in his role as censor.

Sources:
  roman-aqueducts.md (chunk 0)
```

For a corpus-scale question, try:

```bash
docsiq search "What are the main themes in this corpus?"
```

This triggers the global search path, which consults community
summaries rather than individual chunks.

## 5. Open the UI

```bash
docsiq serve
# → http://localhost:8080
```

Navigate to `http://localhost:8080`. You should see:

- **Home** — project picker, recent indexing activity.
- **Notes** — wikilinked markdown, even without any LLM configured.
- **Documents** — the three sample files with chunk counts.
- **Graph** — force-directed entity/community visualisation.
- **MCP** — inspector-style console for the 12+ MCP tools docsiq
  exposes at `/mcp`.

Screenshots of each view are in [`docs/screenshots/`](screenshots/).

## Where to next

- **Configure an LLM** — see [`configs/docsiq.example.yaml`](../configs/docsiq.example.yaml)
  for every option, default, and env-var override.
- **Integrate with Claude Desktop / Cursor** — run
  `docsiq hooks install --client claude-desktop`.
- **Index a real corpus** — `docsiq index /path/to/your/docs` accepts
  PDF, DOCX, TXT, and Markdown. Web pages can be fetched with
  `docsiq crawl <url>`.
- **Read the architecture overview** — [README.md](../README.md#architecture).
- **Contribute** — [CONTRIBUTING.md](../CONTRIBUTING.md).
```

### Commit

```bash
git add docs/quickstart.md docs/samples/
git commit -m "$(cat <<'EOF'
docs(quickstart): add top-down quickstart with sample corpus

docs/quickstart.md walks a new user from zero-install to first search
in five numbered steps. docs/samples/ ships a 3-document markdown
corpus (aqueducts, graphrag, louvain) that indexes in <30s and
produces a non-trivial entity graph for screenshots.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Success criteria

- A user on a fresh machine can follow the quickstart from scratch and
  have a queryable index in under 3 minutes (assuming Ollama is
  already installed with the default models, or an OpenAI key is
  exported).
- `docs/samples/` contains the three `.md` files and a README.
- No internal links in `docs/quickstart.md` are broken (every relative
  path resolves).

---

## Task 5: Capture and commit the screenshot gallery (7.6)

**Spec:** `docs/screenshots/` with fresh captures of Home, Notes,
Documents, Graph, MCP. Referenced from README.

**Files:**

- `/home/dev/projects/docsiq/ui/e2e/screenshots.spec.ts` (new)
- `/home/dev/projects/docsiq/ui/scripts/optimize-screenshots.mjs` (new)
- `/home/dev/projects/docsiq/docs/screenshots/home.png` (new)
- `/home/dev/projects/docsiq/docs/screenshots/notes.png` (new)
- `/home/dev/projects/docsiq/docs/screenshots/documents.png` (new)
- `/home/dev/projects/docsiq/docs/screenshots/graph.png` (new)
- `/home/dev/projects/docsiq/docs/screenshots/mcp.png` (new)

### Investigation note

Screenshots cannot be generated from within this plan file — they
require a running UI against a populated store. This task is therefore
an *investigative / procedural* task with concrete success criteria
(5 PNGs committed, each < 500 KB, referenced from README). The executor
runs the procedure end-to-end on their machine.

### Steps

- [ ] **Step 1** — Create the output directory:
      `mkdir -p docs/screenshots`.
- [ ] **Step 2** — Verify Playwright is already a UI dev dep. If not,
      install it with `npm --prefix ui install -D @playwright/test`
      and commit `ui/package.json` + `ui/package-lock.json` as part of
      this task.
- [ ] **Step 3** — Verify `sharp` is a UI dev dep (per the plan
      assumption). If not, add it with
      `npm --prefix ui install -D sharp`.
- [ ] **Step 4** — Author `ui/e2e/screenshots.spec.ts` — content below.
- [ ] **Step 5** — Author `ui/scripts/optimize-screenshots.mjs` —
      content below.
- [ ] **Step 6** — Prepare the fixture data directory. In a shell:

      ```bash
      FIX=/tmp/docsiq-screenshots-fixture
      rm -rf "$FIX" && mkdir -p "$FIX"
      DOCSIQ_DATA_DIR="$FIX" ./docsiq init --name screenshots-demo
      DOCSIQ_DATA_DIR="$FIX" ./docsiq index docs/samples/
      ```

      The fixture does not need an LLM if you're just checking layout;
      with an LLM configured you get real community summaries and graph
      edges, which is what we want for screenshots.
- [ ] **Step 7** — Boot the server against the fixture:

      ```bash
      DOCSIQ_DATA_DIR="$FIX" ./docsiq serve --port 37778 &
      SERVE_PID=$!
      ```

      Wait ~1s for it to bind, then verify `curl -s localhost:37778/api/health`
      returns 200.
- [ ] **Step 8** — Run the Playwright script:

      ```bash
      BASE_URL=http://localhost:37778 \
        npx --prefix ui playwright test ui/e2e/screenshots.spec.ts
      ```

      This produces 5 raw PNGs in `docs/screenshots/`.
- [ ] **Step 9** — Kill the server: `kill $SERVE_PID`.
- [ ] **Step 10** — Optimize the PNGs through sharp:

      ```bash
      node ui/scripts/optimize-screenshots.mjs
      ```

      Confirm each PNG is < 500 KB after optimization:

      ```bash
      ls -lh docs/screenshots/*.png
      ```
- [ ] **Step 11** — Visually review each PNG. Reject any with:
      - Placeholder or "no data" empty states (the fixture corpus
        should produce data on every screen).
      - Debug toolbars, React Query devtools, or console-open state
        visible.
      - Personal data leakage (no usernames, no emails, no paths that
        include the executor's home directory).
- [ ] **Step 12** — Commit with message:
      `docs(screenshots): capture home, notes, documents, graph, and mcp`.
      Include the test spec and optimizer script in the same commit —
      future recaptures should be reproducible by anyone.

### Exact content for ui/e2e/screenshots.spec.ts

```typescript
// Capture the five canonical docsiq screenshots used in the README and
// docs/screenshots/. Runs against a live server on $BASE_URL (default
// http://localhost:37778) seeded with the docs/samples/ fixture corpus.
//
// Usage:
//   DOCSIQ_DATA_DIR=/tmp/fixture ./docsiq serve --port 37778 &
//   BASE_URL=http://localhost:37778 \
//     npx playwright test ui/e2e/screenshots.spec.ts

import { test, expect } from "@playwright/test";
import path from "node:path";

const BASE_URL = process.env.BASE_URL ?? "http://localhost:37778";
const OUT_DIR = path.resolve(__dirname, "..", "..", "docs", "screenshots");

// Desktop viewport. Matches the typical reviewer's Retina display;
// the sharp script downscales as needed.
test.use({
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: 2,
  colorScheme: "dark", // dark theme is the default in the app
});

async function settle(page: import("@playwright/test").Page) {
  // Wait for network idle + a small buffer for any d3 transitions.
  await page.waitForLoadState("networkidle");
  await page.waitForTimeout(500);
}

test("home", async ({ page }) => {
  await page.goto(`${BASE_URL}/`);
  await settle(page);
  await page.screenshot({
    path: path.join(OUT_DIR, "home.png"),
    fullPage: true,
  });
});

test("notes", async ({ page }) => {
  await page.goto(`${BASE_URL}/notes`);
  await settle(page);
  await page.screenshot({
    path: path.join(OUT_DIR, "notes.png"),
    fullPage: true,
  });
});

test("documents", async ({ page }) => {
  await page.goto(`${BASE_URL}/documents`);
  await settle(page);
  await page.screenshot({
    path: path.join(OUT_DIR, "documents.png"),
    fullPage: true,
  });
});

test("graph", async ({ page }) => {
  await page.goto(`${BASE_URL}/graph`);
  await settle(page);
  // Graph has an SVG force simulation — give it a bit longer to settle.
  await page.waitForTimeout(2000);
  await page.screenshot({
    path: path.join(OUT_DIR, "graph.png"),
    fullPage: true,
  });
});

test("mcp", async ({ page }) => {
  await page.goto(`${BASE_URL}/mcp`);
  await settle(page);
  await page.screenshot({
    path: path.join(OUT_DIR, "mcp.png"),
    fullPage: true,
  });
});
```

### Exact content for ui/scripts/optimize-screenshots.mjs

```javascript
// Optimise docs/screenshots/*.png in place via sharp.
//
// Targets:
//   - Keep pixel dimensions (don't downscale — we use @2x captures).
//   - Re-encode PNG with compression level 9 and palette where possible.
//   - Refuse to exit 0 if any resulting file is > 500 KB — that's our
//     published budget per screenshot.
//
// Usage:  node ui/scripts/optimize-screenshots.mjs

import { readdir, stat } from "node:fs/promises";
import path from "node:path";
import sharp from "sharp";

const DIR = path.resolve(
  path.dirname(new URL(import.meta.url).pathname),
  "..",
  "..",
  "docs",
  "screenshots",
);
const BUDGET_BYTES = 500 * 1024;

const entries = (await readdir(DIR)).filter((f) => f.endsWith(".png"));
if (entries.length === 0) {
  console.error("no PNGs found in", DIR);
  process.exit(1);
}

let failed = false;
for (const name of entries) {
  const p = path.join(DIR, name);
  const before = (await stat(p)).size;
  const buf = await sharp(p)
    .png({ compressionLevel: 9, palette: true, effort: 10 })
    .toBuffer();
  await sharp(buf).toFile(p);
  const after = (await stat(p)).size;
  const flag = after > BUDGET_BYTES ? "OVER BUDGET" : "ok";
  console.log(
    `${name.padEnd(18)}  ${Math.round(before / 1024)} KB → ${Math.round(after / 1024)} KB  [${flag}]`,
  );
  if (after > BUDGET_BYTES) failed = true;
}

process.exit(failed ? 2 : 0);
```

### Commit

```bash
git add ui/e2e/screenshots.spec.ts \
        ui/scripts/optimize-screenshots.mjs \
        docs/screenshots/
git commit -m "$(cat <<'EOF'
docs(screenshots): capture home, notes, documents, graph, and mcp

Five fresh @2x PNGs of the embedded SPA against the docs/samples/
fixture corpus. Reproducible via ui/e2e/screenshots.spec.ts and
ui/scripts/optimize-screenshots.mjs; each output is compressed below
the 500 KB per-image budget.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Success criteria

- Five PNGs exist under `docs/screenshots/`:
  `home.png`, `notes.png`, `documents.png`, `graph.png`, `mcp.png`.
- Each is < 500 KB.
- Each shows real fixture data, not an empty state.
- No personal paths, tokens, or secrets visible in any screenshot.
- The screenshot spec and optimizer script are committed alongside the
  PNGs so future recaptures are reproducible.

### Fallback

If Playwright cannot run headlessly in the executor's environment,
capture the screenshots manually using Chrome DevTools' "Capture full
size screenshot" command against a running server, then still run them
through the sharp optimizer. Document the manual path in the commit
message so the next recapture knows which mode was used.

---

## Task 6: Add the badge row (7.7)

**Spec:** README top: CodeQL, OpenSSF Best Practices (already earned),
build, coverage, license, Go Report Card.

**File:** `/home/dev/projects/docsiq/README.md` (patch, lines 1-10)

### Steps

- [ ] **Step 1** — Confirm the current badges in README (lines 3-7 per
      the snapshot used to write this plan):
      - Security Scan (CI)
      - OpenSSF Best Practices — keep the exact URL
        (`https://www.bestpractices.dev/projects/12628/badge`)
      - OpenSSF Scorecard
      - Release
      - Go Version
- [ ] **Step 2** — Confirm CodeQL workflow exists at
      `.github/workflows/codeql.yml`. The badge URL is
      `https://github.com/RandomCodeSpace/docsiq/actions/workflows/codeql.yml/badge.svg`.
- [ ] **Step 3** — Confirm Codecov status. Run `grep -r codecov
      .github/workflows/` — if codecov is uploaded, the badge is
      `https://codecov.io/gh/RandomCodeSpace/docsiq/branch/main/graph/badge.svg`.
      If not configured, defer coverage badge and add a follow-up note
      in the PR description.
- [ ] **Step 4** — The full post-Task 6 badge row replaces lines 3-7
      of the existing README. Exact content below.
- [ ] **Step 5** — Use the `Edit` tool with the exact
      find-and-replace pair below — do not rewrite the whole README in
      this task (that happens in Task 7).
- [ ] **Step 6** — Self-review: open the README on GitHub (preview
      mode locally via `gh browse -- README.md` after push, or a local
      markdown renderer). Every badge must render, every link must
      resolve.
- [ ] **Step 7** — Commit with message:
      `docs(readme): add CodeQL, license, Go report card, and coverage badges`

### Find in README.md (the existing badge block)

```
[![Security Scan](https://github.com/RandomCodeSpace/docsiq/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/RandomCodeSpace/docsiq/actions/workflows/ci.yml)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/12628/badge)](https://www.bestpractices.dev/projects/12628)
[![OpenSSF Score](https://api.scorecard.dev/projects/github.com/RandomCodeSpace/docsiq/badge)](https://scorecard.dev/viewer/?uri=github.com/RandomCodeSpace/docsiq)
[![Release](https://img.shields.io/github/v/release/RandomCodeSpace/docsiq?include_prereleases&sort=semver)](https://github.com/RandomCodeSpace/docsiq/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/RandomCodeSpace/docsiq)](https://github.com/RandomCodeSpace/docsiq/blob/main/go.mod)
```

### Replace with

```
[![CI](https://github.com/RandomCodeSpace/docsiq/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/RandomCodeSpace/docsiq/actions/workflows/ci.yml)
[![CodeQL](https://github.com/RandomCodeSpace/docsiq/actions/workflows/codeql.yml/badge.svg?branch=main)](https://github.com/RandomCodeSpace/docsiq/actions/workflows/codeql.yml)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/12628/badge)](https://www.bestpractices.dev/projects/12628)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/RandomCodeSpace/docsiq/badge)](https://scorecard.dev/viewer/?uri=github.com/RandomCodeSpace/docsiq)
[![Go Report Card](https://goreportcard.com/badge/github.com/RandomCodeSpace/docsiq)](https://goreportcard.com/report/github.com/RandomCodeSpace/docsiq)
[![License: MIT](https://img.shields.io/github/license/RandomCodeSpace/docsiq)](LICENSE)
[![Release](https://img.shields.io/github/v/release/RandomCodeSpace/docsiq?include_prereleases&sort=semver)](https://github.com/RandomCodeSpace/docsiq/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/RandomCodeSpace/docsiq)](https://github.com/RandomCodeSpace/docsiq/blob/main/go.mod)
```

If Step 3 confirmed Codecov is configured, also insert this badge
between CodeQL and OpenSSF Best Practices:

```
[![Coverage](https://codecov.io/gh/RandomCodeSpace/docsiq/branch/main/graph/badge.svg)](https://codecov.io/gh/RandomCodeSpace/docsiq)
```

### Commit

```bash
git add README.md
git commit -m "$(cat <<'EOF'
docs(readme): add CodeQL, license, Go report card, and coverage badges

Extends the existing OpenSSF + Scorecard badge row with CodeQL
(now running on every PR), an explicit MIT license badge, and a Go
Report Card link. Coverage badge included only if Codecov is wired;
otherwise tracked as follow-up.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Success criteria

- README's badge row renders with every badge present and green.
- No broken badge URLs (each returns a 200 with a valid SVG).
- Badge ordering is stable and readable: status signals (CI, CodeQL)
  first; community signals (OpenSSF) next; project metadata (Go
  Report, License, Release, Go Version) last.

---

## Task 7: Refactor README first screen (7.1)

**Spec:** first screen is (i) one-line description, (ii) single-command
install from a release binary, (iii) single-command first-index on a
sample corpus. Target: user indexes and queries in under 3 minutes.
Screenshots of Home and Graph inline.

**File:** `/home/dev/projects/docsiq/README.md` (overwrite)

### Steps

- [ ] **Step 1** — Re-read the current README end to end (you should
      have it cached from Task 6). Identify content that must survive:
      - The architecture tree (lines 100-122 of the current version).
      - The UI section (Keyboard shortcuts table, PWA note).
      - The MCP section and `docsiq hooks install` example.
      - The "No LLM?" note about `provider: none`.
      - The LICENSE reference.
- [ ] **Step 2** — Verify the release asset URL shape. Run
      `gh release list -R RandomCodeSpace/docsiq -L 1`
      then `gh release view <tag> -R RandomCodeSpace/docsiq
      --json assets --jq '.assets[].name'`
      to get the actual asset names. If the file is
      `docsiq-linux-amd64`, use that verbatim. If it is
      `docsiq_linux_amd64.tar.gz` or similar, adjust the install
      snippet to match (download, extract, chmod, move).
- [ ] **Step 3** — Overwrite the entire README with the content below.
      **Do not lose any information captured in Step 1**; the new
      README integrates it all in a rearranged order, with the
      "three-command onboarding" promoted above everything else.
- [ ] **Step 4** — Validate every link. In the new README:
      - `CONTRIBUTING.md` → Task 2 output.
      - `SECURITY.md` → Task 1 output.
      - `docs/quickstart.md` → Task 4 output.
      - `configs/docsiq.example.yaml` → Task 3 output.
      - `docs/screenshots/{home,graph}.png` → Task 5 output.
      - `LICENSE` → existing.
      - `CODE_OF_CONDUCT.md` → existing.
- [ ] **Step 5** — Self-review: read the whole diff. Confirm the first
      screen (top 40 lines) hits the three-bullet target:
      one-line description, single install command, single first-index
      command. The 3-minute claim must be defensible given what the
      quickstart says.
- [ ] **Step 6** — Commit with message:
      `docs(readme): refactor first screen for 3-minute onboarding`

### Exact content for README.md

```markdown
# docsiq

[![CI](https://github.com/RandomCodeSpace/docsiq/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/RandomCodeSpace/docsiq/actions/workflows/ci.yml)
[![CodeQL](https://github.com/RandomCodeSpace/docsiq/actions/workflows/codeql.yml/badge.svg?branch=main)](https://github.com/RandomCodeSpace/docsiq/actions/workflows/codeql.yml)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/12628/badge)](https://www.bestpractices.dev/projects/12628)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/RandomCodeSpace/docsiq/badge)](https://scorecard.dev/viewer/?uri=github.com/RandomCodeSpace/docsiq)
[![Go Report Card](https://goreportcard.com/badge/github.com/RandomCodeSpace/docsiq)](https://goreportcard.com/report/github.com/RandomCodeSpace/docsiq)
[![License: MIT](https://img.shields.io/github/license/RandomCodeSpace/docsiq)](LICENSE)
[![Release](https://img.shields.io/github/v/release/RandomCodeSpace/docsiq?include_prereleases&sort=semver)](https://github.com/RandomCodeSpace/docsiq/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/RandomCodeSpace/docsiq)](https://github.com/RandomCodeSpace/docsiq/blob/main/go.mod)

**A single-binary GraphRAG knowledge base — index documents, extract an
entity graph, ask questions across it, and browse the result in an
embedded React UI over MCP.**

## Three-minute onboarding

```bash
# 1. Install (Linux amd64 shown; macOS arm64 + Windows amd64 are published alongside)
curl -LO https://github.com/RandomCodeSpace/docsiq/releases/latest/download/docsiq-linux-amd64
chmod +x docsiq-linux-amd64 && sudo mv docsiq-linux-amd64 /usr/local/bin/docsiq

# 2. Index the sample corpus
git clone https://github.com/RandomCodeSpace/docsiq && cd docsiq
docsiq init && docsiq index docs/samples/

# 3. Ask a question
docsiq search "What are the main themes in this corpus?"
```

For a UI session:

```bash
docsiq serve
# → http://localhost:8080
```

Full walk-through with expected output: [docs/quickstart.md](docs/quickstart.md).

## Screenshots

| Home | Graph |
|---|---|
| ![Home view](docs/screenshots/home.png) | ![Graph view](docs/screenshots/graph.png) |

More: [Notes](docs/screenshots/notes.png) ·
[Documents](docs/screenshots/documents.png) ·
[MCP Console](docs/screenshots/mcp.png).

## What it does

docsiq is a GraphRAG-powered knowledge base that runs as a single Go
binary. It ingests unstructured documents, builds a knowledge graph
with community detection, persists wikilinked markdown notes, and
exposes the whole thing over **MCP + an embedded React SPA** on one
port.

Inspired by [Microsoft GraphRAG](https://github.com/microsoft/graphrag);
storage is CGO-backed SQLite (`mattn/go-sqlite3` with FTS5) + the
[`sqlite-vec`](https://github.com/asg017/sqlite-vec) extension for ANN
vector search.

## Features

- **GraphRAG pipeline** — load → chunk → embed → extract entities /
  relationships / claims → detect communities, all in one
  `docsiq index` run.
- **Notes subsystem** — markdown on disk with `[[wikilinks]]`, project
  scopes, cross-project references, and a live note graph view. Works
  without any LLM configured.
- **Interactive graph** — SVG force-directed viz with d3-zoom
  (pinch/wheel pan/zoom 0.1×–40×), hover-to-highlight neighbourhood,
  degree-scaled nodes.
- **Community detection** — pure-Go Louvain, hierarchical, no external
  deps.
- **Three LLM providers** — Azure OpenAI, OpenAI, Ollama — via
  [`tmc/langchaingo`](https://github.com/tmc/langchaingo). Set
  `provider: "none"` to run the server in notes-only mode with no LLM.
- **MCP server** — 12+ tools (local/global search, graph walk,
  community reports, note read/write, …) exposed at `/mcp` via
  Streamable HTTP transport with session handshake.
- **Embedded SPA** — React 19 + Tailwind 4 + shadcn/ui, served from
  `//go:embed ui/dist`. PWA-installable with manifest + service worker.
- **Per-repo projects** — each scope has its own SQLite store + notes
  directory, addressable by slug.

## UI

- **Stack**: React 19, Vite 6, Tailwind 4, shadcn/ui primitives, Geist
  typography, Lucide icons.
- **Architecture**: CSS lives in a single `globals.css` with an
  `@layer components` section; JSX uses semantic class names only;
  shadcn primitives are the only place Tailwind utilities live inline.
- **Navigation**: labelled sidebar (Home · Notes · Documents · Graph ·
  MCP) with ⌘K command palette.
- **Responsiveness**: mobile drawer via shadcn `Sheet`; iOS safe-area
  respected; inputs forced to 16px below `sm:` to kill Safari auto-zoom.
- **PWA**: manifest + 192/512 PNG icons + minimal service worker,
  installable on Android/iOS.
- **Hard reload**: refresh button in the header purges service worker +
  CacheStorage and reloads from network — mobile-friendly `⌘⇧R` substitute.

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

docsiq speaks the MCP Streamable HTTP transport at `POST /mcp`. The
UI's MCP Console (inspector-style) gives you the same tool list with
typed argument forms. For external clients (Claude Desktop, Cursor,
etc.) register the server URL directly, or use the hooks helper:

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
an env var with prefix `DOCSIQ_` (dots → underscores, uppercased). A
fully annotated reference with every option, default, and env var is at
[`configs/docsiq.example.yaml`](configs/docsiq.example.yaml).

```yaml
server:
  host: 0.0.0.0
  port: 37778
  api_key: ""          # if set, UI + API require Authorization: Bearer <key>

llm:
  provider: ollama     # azure | openai | ollama | none
  ollama:
    base_url: http://localhost:11434
    chat_model: llama3.2
    embed_model: nomic-embed-text
```

**No LLM?** Set `provider: none`. The server still runs notes,
wikilinks, graph, tree, and notes-search. Endpoints that need the
model (`POST /api/search`, `POST /api/upload`, `/mcp` tool calls that
embed or extract) return `503 {"code": "llm_disabled"}`.

## Build from source

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

## Community

- **Contributing** — [CONTRIBUTING.md](CONTRIBUTING.md) for local dev
  loop, test commands, and commit style.
- **Security** — [SECURITY.md](SECURITY.md) for the disclosure policy
  and fix SLA.
- **Code of conduct** — [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
- **Governance** — [GOVERNANCE.md](GOVERNANCE.md).
- **Changelog** — [CHANGELOG.md](CHANGELOG.md).

## License

MIT. See [LICENSE](LICENSE).
```

### Commit

```bash
git add README.md
git commit -m "$(cat <<'EOF'
docs(readme): refactor first screen for 3-minute onboarding

Promotes a three-command onboarding block (install, index, query) to
the first screen, inlines Home/Graph screenshots, and links
downstream docs (quickstart, example config, CONTRIBUTING, SECURITY)
into a single Community section. No content lost — architecture, UI,
MCP, build, and tests sections are preserved verbatim.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Success criteria

- First screen (top 40 lines) hits the three targets verbatim: one-line
  description, single install command, single index command.
- Home and Graph screenshots render inline (not as links).
- Every outbound link resolves: `CONTRIBUTING.md`, `SECURITY.md`,
  `docs/quickstart.md`, `configs/docsiq.example.yaml`, all five
  screenshots, `CODE_OF_CONDUCT.md`, `GOVERNANCE.md`, `CHANGELOG.md`,
  `LICENSE`.
- The architecture tree, UI description, MCP section, and configuration
  block from the previous README are preserved (no content deletion,
  only reordering).
- Badge row matches Task 6's output.

---

## Block-level Self-Review

Before declaring Block 7 complete, run through this checklist. Every
item is scoped to what this block produced; do not borrow failures from
other blocks.

### Content audit

- [ ] README first screen can be read on a phone-sized viewport without
      scrolling past the install/index commands.
- [ ] Every `.md` file created in this block renders cleanly in the
      GitHub markdown viewer (preview with `gh browse`).
- [ ] Every fenced code block is syntactically valid for its language
      tag (bash, yaml, typescript, javascript, markdown).
- [ ] No broken internal links. Run
      `grep -rhoE '\]\([^)]+\)' README.md CONTRIBUTING.md SECURITY.md docs/*.md docs/samples/*.md`
      and eyeball each target.
- [ ] `configs/docsiq.example.yaml` matches `internal/config/config.go`
      field-for-field. Run `grep 'v.SetDefault' internal/config/config.go`
      and confirm every key is present in the YAML.

### Execution audit

- [ ] `docsiq serve --config configs/docsiq.example.yaml` starts
      without errors.
- [ ] A user following `docs/quickstart.md` verbatim hits a queryable
      state in under 3 minutes on a reference machine (no LLM
      configuration required; Ollama must be running locally or
      `provider: none` used).
- [ ] All five screenshots are under 500 KB and show real data.
- [ ] `ui/e2e/screenshots.spec.ts` and
      `ui/scripts/optimize-screenshots.mjs` are syntactically valid
      (`npx tsc --noEmit` and `node --check` pass).

### OSS hygiene audit

- [ ] Badge row on README renders without any broken badges.
- [ ] `SECURITY.md` advisory URL is reachable.
- [ ] `CONTRIBUTING.md` build commands work on a fresh clone.
- [ ] No secrets, tokens, personal paths, or email addresses
      committed in any file, including the screenshots.

### Commit audit

- [ ] Exactly seven commits, one per task.
- [ ] Each commit's subject line follows Conventional Commits and fits
      in 72 chars.
- [ ] Each commit has the `Co-Authored-By: Claude Opus 4.7 (1M context)
      <noreply@anthropic.com>` trailer.
- [ ] No commit touches files outside the list declared in that task.

## Execution Handoff

When executing this plan:

1. **Invoke `superpowers:executing-plans`** — the skill is required at
   the top of this document. It will read these tasks one at a time,
   enforce the "read full diff before declaring done" rule, and gate
   each commit on the listed success criteria.
2. **Run tasks in order** (1 → 7). They are ordered by dependency
   fan-in. Do not parallelize — Task 7 reads the screenshots and badges
   produced by Tasks 5 and 6.
3. **Do not batch commits.** Each task ends with its own commit. If a
   hook fails, fix and create a *new* commit — do not `--amend`.
4. **Do not push.** A controller batches Block 7 with any remaining
   work from adjacent blocks.
5. **If a step reveals a hidden constraint** (e.g., release binary is
   actually `.tar.gz`, or Codecov is not wired) — pause, update the
   relevant task's snippet, note the change under a `## Deviations`
   heading at the bottom of this plan, and continue. Do not silently
   adapt.

### Expected final state

```
# Files created
configs/docsiq.example.yaml
docs/quickstart.md
docs/samples/README.md
docs/samples/roman-aqueducts.md
docs/samples/graphrag.md
docs/samples/louvain.md
docs/screenshots/home.png
docs/screenshots/notes.png
docs/screenshots/documents.png
docs/screenshots/graph.png
docs/screenshots/mcp.png
ui/e2e/screenshots.spec.ts
ui/scripts/optimize-screenshots.mjs

# Files overwritten
README.md
CONTRIBUTING.md
SECURITY.md
config.example.yaml   (now a two-line pointer to configs/docsiq.example.yaml)
```

Seven commits. Block 7 complete.
