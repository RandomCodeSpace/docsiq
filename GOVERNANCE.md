# Governance

## Model

docsiq follows a **lead-maintainer** governance model. The project currently
has one lead maintainer, **Amit Kumar** (GitHub: [@aksOps](https://github.com/aksOps),
email: ak.nitrr13@gmail.com), who holds final authority on technical direction,
release timing, security policy, and contributor access.

The project welcomes additional co-maintainers as the community grows. Any
contributor who demonstrates sustained, high-quality involvement may be invited
to join as a co-maintainer.

## Roles

| Role | Who | Responsibilities |
|------|-----|-----------------|
| Lead maintainer | @aksOps | Merges PRs, cuts releases, triages issues, manages repo settings |
| Security contact | @aksOps | Receives private vulnerability reports, coordinates disclosure |
| Reviewer | Contributors invited ad-hoc | Reviews PRs; cannot merge without lead-maintainer approval |

## Decision-making

1. **Routine changes** (bug fixes, dependency bumps, doc improvements) — the
   lead maintainer merges after CI passes.
2. **Significant changes** (new features, breaking API changes, new dependencies) —
   a GitHub Discussion or PR is opened for at least 72 hours of community input
   before merging.
3. **Security-sensitive changes** — handled privately via GitHub Security
   Advisories; disclosed publicly after a fix ships.

## Access continuity

- Repository admin access is held by @aksOps.
- All build, signing, and release artifacts are fully reproducible from committed
  source (`go build -tags sqlite_fts5 ./`). If maintainer access is lost, any
  fork can reproduce and redistribute identical artifacts.
- `.github/CODEOWNERS` is configured so GitHub automatically requests review
  from the lead maintainer on every PR.

## Continuity and resilience

docsiq is currently a single-maintainer project. Continuity risk is reduced by:

- **Reproducible builds** — the full binary can be rebuilt by anyone from source.
- **Cosign keyless signing** — release signatures are anchored to the GitHub OIDC
  identity and the Rekor transparency log, not a private key held by one person.
- **Open governance** — this document and all project infrastructure are
  publicly committed; a new maintainer can take over without institutional
  knowledge gaps.

If the lead maintainer becomes unavailable for more than 90 days without notice,
interested contributors should open a GitHub Issue to coordinate next steps.

## Amendments

This document may be updated by the lead maintainer via a normal PR. Significant
governance changes will be announced in the release notes.
