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

docsiq requires a **C toolchain at build time** because it uses the
CGO-backed `github.com/mattn/go-sqlite3` driver (with FTS5) and ships the
`sqlite-vec` extension as a loadable asset. Pure-Go builds
(`CGO_ENABLED=0`) are not supported.

| OS      | Requirement                                                        |
|---------|--------------------------------------------------------------------|
| Linux   | `build-essential` (gcc, make) — `apt-get install build-essential`  |
| macOS   | Xcode Command Line Tools — `xcode-select --install`                |
| Windows | **Not supported.** Do not open issues for Windows.                 |

You also need:

- **Go** — version from `go.mod` (`go mod edit -json | jq -r .Go`) or
  newer.
- **Node.js** — 20.x or newer, for the Vite-based UI.
- **SQLite FTS5 + sqlite-vec** — both are linked into the binary via the
  `sqlite_fts5` build tag and the vendored `sqlite-vec` extension. No
  separate install is needed; CGO pulls them in.
- **Git** — 2.30+.

## sqlite-vec prebuilt binaries

The `sqlite-vec` loadable extension is embedded into the Go binary via
`internal/sqlitevec/assets/`. Contributors do **not** need these
binaries for day-to-day development — the runtime gracefully falls back
to in-memory HNSW / brute-force search when the embedded asset is a
0-byte placeholder. Release builds must ship the real artefacts; see
`internal/sqlitevec/assets/README.md` for the download / drop-in
procedure.

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

A `Makefile` wraps the common targets (`make build`, `make vet test`)
with the correct tags and CGO setting.

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

`ui/dist/` is **not committed** — the repo only ships a tiny
`ui/dist/index.html` placeholder so `//go:embed ui/dist` compiles. CI
rebuilds the UI and passes `ui/dist/` to each Go job as an artifact.

### End-to-end

```bash
# Boot the binary against a fixture data dir
DOCSIQ_DATA_DIR=/tmp/docsiq-dev ./docsiq serve --port 37778 &
cd ui && npm run e2e
```

## Pre-commit hooks (optional but recommended)

We recommend [pre-commit](https://pre-commit.com/) to keep `gofmt`,
`go vet`, and `prettier` from slipping. A minimal config you can drop
into your fork (not committed to the repo):

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
branches (`main`, `release/*`).

## Pull requests

### Before opening

- Branch from `main`. Use a descriptive branch name:
  `feat/community-summaries`, `fix/mcp-handshake-timeout`, etc.
- Rebase on top of the latest `main` before opening the PR.
- Run the full test suite (Go + UI) — `go test`, `npm test`, typecheck,
  build. CI will run them anyway; running locally is faster feedback.
- Re-read your own diff. `git diff main...HEAD` in the terminal, top to
  bottom. Most self-inflicted review comments are caught this way.

### PR description

No PR template is configured at this time; use a clear,
conventional-commit-style title and describe:

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

### During review

- Be responsive but not hasty — take time to think about comments.
- If a reviewer is wrong, push back with a clear reason. We prefer
  disagreement over silent compliance.

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
