# Contributing to DocsContext

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
