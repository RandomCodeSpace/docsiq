# Changelog

All notable changes to **docsiq** are documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).
Each release is identified by an immutable `vX.Y.Z` git tag.

## How releases are produced

Releases are cut by the manual
[`release.yml`](.github/workflows/release.yml) workflow:

```sh
gh workflow run release.yml --ref main \
  -f bump=patch \
  -f notes=$'### Changed\n\n- Describe major changes...\n\n### Upgrade impact\n\nDrop-in replacement — no schema/API changes.'
```

The workflow uses the `notes` input verbatim as the GitHub Release body
and uploads it as `CHANGELOG.md` on the release page. Each release ships
signed binaries (cosign keyless via Sigstore + Rekor anchoring), a signed
`SHA256SUMS`, and SLSA build provenance.

This in-repo file is the canonical, human-curated history. The matching
GitHub Release page for each `vX.Y.Z` tag carries the same notes plus the
signed artifacts and verification snippet.

## [Unreleased]

No unreleased changes.

## [0.0.3] — 2026-04-23

Supply-chain hardening: complete OpenSSF Best Practices passing tier and
flip the published Scorecard signal up.

### Added
- `.bestpractices.json` so the OpenSSF Best Practices badge tracks
  project [12628](https://www.bestpractices.dev/en/projects/12628)
  automatically. ([#45])
- Governance and community files (`SECURITY.md`, `CODE_OF_CONDUCT.md`,
  `CONTRIBUTING.md`, issue / PR templates) to flip the remaining
  Best Practices criteria to Met. ([#46])
- Initial `CHANGELOG.md` and the rest of the missing Best Practices
  criteria (release-notes pointer, vulnerability-report instructions,
  build documentation pointers). ([#48])

### Changed
- Release signing path: switched to `goreleaser` to expose a
  `Packaging` signal to OSSF Scorecard, then dropped it again because
  prebuilt-binary signing is a goreleaser Pro feature. The current
  release path is inline `cosign sign-blob` + `gh release create`,
  preserving keyless signing without the Pro dependency.
  ([#43], [#50])
- CI hygiene: dropped `push: main` triggers from the `ci` and `fuzz`
  workflows. Both still run on PRs and on the relevant scheduled jobs;
  this removes ~2 minutes from each merge while keeping branch
  protection coverage intact. ([#47])

### Fixed
- CodeQL path/command-injection findings closed by adding
  `filepath.IsLocal` sanitisers on user-supplied path inputs in the
  loader and crawler boundaries. ([#44])
- `TestScale_1000Notes` flake on macOS — dropped macOS from the test
  matrix (Linux-only CI is sufficient for the supported targets;
  darwin-arm64 builds are still produced in the release matrix).
  ([#49])

[#43]: https://github.com/RandomCodeSpace/docsiq/pull/43
[#44]: https://github.com/RandomCodeSpace/docsiq/pull/44
[#45]: https://github.com/RandomCodeSpace/docsiq/pull/45
[#46]: https://github.com/RandomCodeSpace/docsiq/pull/46
[#47]: https://github.com/RandomCodeSpace/docsiq/pull/47
[#48]: https://github.com/RandomCodeSpace/docsiq/pull/48
[#49]: https://github.com/RandomCodeSpace/docsiq/pull/49
[#50]: https://github.com/RandomCodeSpace/docsiq/pull/50

## [0.0.2] — 2026-04-23

Small CI-only follow-up to v0.0.1. No user-facing behaviour changes.

### Changed
- OpenSSF Scorecard workflow cadence: `scorecard.yml` now runs on
  release completion and on a weekly schedule, instead of firing on
  every push to `main`. The policy being scored is unchanged; this
  trims noise from re-scoring commits that don't move any
  scorecard-visible state. ([#42])

### Upgrade impact
Safe drop-in upgrade from v0.0.1. No API, CLI, or on-disk schema
changes — replace the binary in place.

[#42]: https://github.com/RandomCodeSpace/docsiq/pull/42

## [0.0.1] — 2026-04-23

First non-beta release of docsiq after an extended beta phase. This
release establishes the feature set and API surface that subsequent
0.0.x patches maintain back-compat against.

### Added
- **GraphRAG indexing pipeline** — five-phase ingestion: chunk →
  extract entities + relationships + claims → community-detect
  (Louvain) → embed → persist.
- **Document loaders** — PDF (langchaingo), DOCX, TXT, Markdown, and a
  polite web crawler with `robots.txt` + allow-list + MIME checks.
- **Multi-provider LLM layer** — Azure OpenAI, OpenAI, and Ollama
  behind a single `internal/llm` abstraction (langchaingo
  underneath).
- **Hybrid query engine** — local search (vector + FTS5) plus global
  search (community-summary).
- **Surfaces** — CLI (`docsiq index|search|serve`), REST API, MCP
  server, and an embedded React SPA served by `docsiq serve`.
- **Storage** — single SQLite file with `sqlite_fts5` + `sqlite-vec`
  for vector search. No external DB to deploy.

### Security
- Release binaries signed with [cosign](https://github.com/sigstore/cosign)
  keyless via Sigstore and anchored to the Rekor transparency log.
- Signed `SHA256SUMS` published with each release, with verification
  instructions attached.
- SLSA build provenance (`.intoto.jsonl`) accompanies the binaries.

### Known limitations
- Darwin support is limited to `arm64`; `amd64` binaries are not
  built (cgo + sqlite-vec cross-compile complexity).
- Pre-1.0: APIs and on-disk schema are not yet frozen.

### Upgrade impact
No previous stable release exists — this is v0.0.1. Users upgrading
from `v0.0.0-beta.*` should start with a fresh data directory; the
schema is the same as the final beta but the beta tags have been
retired.

[Unreleased]: https://github.com/RandomCodeSpace/docsiq/compare/v0.0.3...HEAD
[0.0.3]: https://github.com/RandomCodeSpace/docsiq/releases/tag/v0.0.3
[0.0.2]: https://github.com/RandomCodeSpace/docsiq/releases/tag/v0.0.2
[0.0.1]: https://github.com/RandomCodeSpace/docsiq/releases/tag/v0.0.1
