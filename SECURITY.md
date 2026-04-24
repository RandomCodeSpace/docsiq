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

We will acknowledge your report within **72 hours** and provide a
remediation plan within **7 days of triage**.

## Disclosure policy

docsiq follows **coordinated disclosure**. The default embargo window is
**90 days** from the acknowledgement date, during which we will work
with you on a fix, a CVE request (where applicable), and a public
advisory. We are happy to credit you in the advisory — tell us how you
would like to be named.

If a fix ships before the 90-day window ends, we will publish the
advisory at release time. If we need more time (e.g. upstream dependency
fix required), we will tell you why and propose a revised date.

Once a fix ships in a release, we publish a
[GitHub Security Advisory](https://github.com/RandomCodeSpace/docsiq/security/advisories)
crediting the reporter unless they request anonymity.

## Supported versions

We issue security fixes for:

- **The latest released tag** on the `main` branch (see
  [Releases](https://github.com/RandomCodeSpace/docsiq/releases)).
- **`main` branch HEAD** — security fixes land here first and are
  included in the next tagged release.

docsiq is pre-1.0; older `v0.0.0-beta.N` prereleases are not patched.
Please upgrade to the latest release.

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

- The `docsiq` binary and all Go packages under `internal/` and `cmd/`.
- The embedded React SPA in `ui/`.
- The MCP server and REST API exposed by `docsiq serve`.
- Build, release, and CI workflows under `.github/`.
- Default configuration as shipped.
- Vulnerabilities in our direct dependencies that are reachable through
  docsiq.

Out of scope:

- Third-party LLM providers (Azure OpenAI, OpenAI, Ollama) — report
  upstream.
- Upstream vulnerabilities in transitive dependencies that are not
  reachable from docsiq. Please report those to the upstream project;
  we will track and upgrade when a patched version ships.
- Misconfigurations introduced by a downstream user (e.g. binding a
  public port with no API key set).
- Vulnerabilities that require a compromised local shell or filesystem
  access.
- Denial of service via resource exhaustion on a self-hosted instance
  the attacker already has network access to.

## Safe harbor

We will not pursue legal action against researchers who act in good
faith, follow this policy, stay within scope, avoid privacy violations,
and do not degrade service for other users. If in doubt, ask first.

## Report archive

Non-sensitive bug reports and their full discussion history are archived
publicly as [GitHub Issues](https://github.com/RandomCodeSpace/docsiq/issues).
Security reports are archived as
[GitHub Security Advisories](https://github.com/RandomCodeSpace/docsiq/security/advisories)
after coordinated disclosure.
