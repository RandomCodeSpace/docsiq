# Changelog

Curated release notes for each version are published on
**[GitHub Releases](https://github.com/RandomCodeSpace/docsiq/releases)**.

Every release includes:

- A human-readable summary of changes (the release body).
- A `CHANGELOG.md` asset attached to the release, containing the same
  curated notes.
- Signed binaries (cosign keyless + Rekor), a signed `SHA256SUMS`, and
  SLSA build provenance.

## Release procedure

Release notes are provided at release time, not maintained in-repo:

```sh
gh workflow run release.yml --ref main \
  -f bump=patch \
  -f notes=$'### Changed\n\n- Describe major changes...\n\n### Upgrade impact\n\nDrop-in replacement — no schema/API changes.'
```

The workflow uses the `notes` input verbatim as the release body and
also uploads it as `CHANGELOG.md` on the release page. The repository
never auto-commits a CHANGELOG entry — this file is static.

The project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html) and each
release is identified by its immutable `vX.Y.Z` tag.
