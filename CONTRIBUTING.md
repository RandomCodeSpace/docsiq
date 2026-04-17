# Contributing to DocsContext

## Prerequisites

DocsContext requires a **C toolchain at build time** because it uses the
CGO-backed `github.com/mattn/go-sqlite3` driver (with FTS5) and ships the
`sqlite-vec` extension as a loadable `.so` / `.dylib`. Pure-Go builds
(`CGO_ENABLED=0`) are no longer supported.

| OS      | Requirement                                             |
|---------|---------------------------------------------------------|
| Linux   | `build-essential` (gcc, make) — `apt-get install build-essential` |
| macOS   | Xcode Command Line Tools — `xcode-select --install`     |
| Windows | **Not supported.** Do not open issues for Windows.      |

You also need Go **≥ 1.22** and Node.js 20+ for the UI build.

Build locally:

```bash
make build      # CGO_ENABLED=1 go build -tags sqlite_fts5 ./...
make vet test   # same tags, CGO on
```

`go install github.com/RandomCodeSpace/docscontext@latest` continues to
work for end users provided they have the C toolchain listed above.

## sqlite-vec prebuilt binaries

The `sqlite-vec` loadable extension is embedded into the Go binary via
`internal/sqlitevec/assets/`. Contributors DO NOT need these binaries for
day-to-day development (the runtime gracefully falls back to in-memory
HNSW / brute-force search when the embedded asset is a 0-byte placeholder).
However, release builds must ship the real artefacts — see
`internal/sqlitevec/assets/README.md` for the download / drop-in
procedure.

## Committing `ui/dist/` (built UI assets)

DocsContext is distributed via `go install`, which cannot run `npm` or `vite`.
The Go binary therefore embeds the pre-built UI from `ui/dist/` via
`ui/embed.go` (`//go:embed dist`), and those build outputs **must be committed
to git**. The root `.gitignore` explicitly un-ignores `ui/dist/` for this
reason.

Whenever you change anything under `ui/src/` (or any file that affects the
Vite build), run the following before committing and include the regenerated
assets in the same commit:

```
npm --prefix ui run build && git add ui/dist
```

CI enforces this via the `ui-dist freshness` workflow
(`.github/workflows/ui-freshness.yml`), which rebuilds the UI and fails the
job if the committed `ui/dist/` has drifted from the build output.
