# Changelog

All notable changes to docsiq are documented here in a human-readable
form. The full per-commit history is available on
[GitHub Releases](https://github.com/RandomCodeSpace/docsiq/releases),
but this file is the curated summary.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Each release ships signed binaries (cosign keyless + Rekor), a signed
`SHA256SUMS`, and SLSA build provenance.

> **Contributors:** add bullets under `## [Unreleased]` as part of any
> PR worth mentioning in release notes. When the release workflow runs,
> it promotes `[Unreleased]` → `[vX.Y.Z] — YYYY-MM-DD` automatically and
> uses that section as the GitHub release body. If no non-empty
> `[Unreleased]` section exists at release time, the workflow fails.

## [Unreleased]

### Added
- `CODE_OF_CONDUCT.md`, `GOVERNANCE.md`, `.github/CODEOWNERS`,
  `.github/release.yml`, `docs/ACCESSIBILITY.md` — project governance
  and community files (OpenSSF BestPractices passing tier).
- `.bestpractices.json` tracking the full OpenSSF BestPractices matrix
  at repo root (78 Met / 10 N/A / 0 Unknown).

### Changed
- `SECURITY.md`: added a "Report archive" section clarifying that
  GitHub Issues archives non-sensitive reports and Security Advisories
  archives coordinated-disclosure reports.
- Release pipeline: dropped GoReleaser (its `prebuilt` builder is a
  Pro-only feature and wasn't parsing in OSS goreleaser). The release
  job now computes SHA256SUMS, signs with cosign keyless, and creates
  the GitHub release directly — signing, provenance, and categorised
  release notes are all preserved.
- CI: dropped macOS from the test matrix; Linux-only is sufficient to
  gate PRs. The release workflow still builds darwin-arm64 binaries
  natively on macOS runners.
- CI: removed `push: main` trigger from `ci.yml` and `fuzz.yml`;
  `pull/N/merge` already validates the merged tree. Saves ~2 min of
  runner time per merged PR. `codeql.yml` still runs on push to main
  (the Security tab's default-branch data requires it).

## [0.0.2] — 2026-04-23

### Changed

- **Scorecard workflow cadence.** `scorecard.yml` now runs on release
  completion and weekly on schedule instead of firing on every push to
  `main`. The policy being scored is unchanged; this simply stops
  re-scoring commits that don't move any Scorecard-visible state.

### Upgrade impact

Safe drop-in upgrade from v0.0.1. No API, CLI, or on-disk schema
changes — replace the binary in place.

GitHub Release: <https://github.com/RandomCodeSpace/docsiq/releases/tag/v0.0.2>

## [0.0.1] — 2026-04-23

First non-beta release. Establishes the feature set and API surface
that subsequent 0.0.x patches will maintain back-compat against.

### Added

- **GraphRAG indexing pipeline** — five-phase ingestion: chunk, extract
  entities/relationships/claims, community-detect (Louvain), embed,
  persist.
- **Document loaders** — PDF (langchaingo), DOCX, TXT, Markdown, and a
  polite web crawler with robots.txt + allow-list + MIME checks.
- **Multi-provider LLM layer** — Azure OpenAI, OpenAI, and Ollama
  behind a single `internal/llm` abstraction.
- **Query engine** — hybrid local (vector + FTS5) and global
  (community-summary) search.
- **Surfaces** — CLI (`docsiq index|search|serve`), REST API, MCP
  server, and an embedded React SPA served by `docsiq serve`.
- **Storage** — single SQLite file with `sqlite_fts5` and `sqlite-vec`
  for vector search. No external DB to deploy.
- **Signed releases** — cosign keyless via Sigstore (Rekor-anchored),
  signed `SHA256SUMS`, and SLSA build provenance.

### Known limitations

- Darwin support is limited to `arm64`; `amd64` is not built (cgo +
  sqlite-vec cross-compile complexity).
- Pre-1.0: APIs and on-disk schema are not yet frozen.

GitHub Release: <https://github.com/RandomCodeSpace/docsiq/releases/tag/v0.0.1>
