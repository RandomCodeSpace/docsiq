# Security Policy

## Reporting a Vulnerability

Please report security vulnerabilities via GitHub's
[private vulnerability reporting](https://github.com/RandomCodeSpace/docsiq/security/advisories/new).

Do **not** open a public issue for security reports.

We aim to acknowledge reports within 72 hours and provide a remediation
plan within 7 days of triage.

## Scope

In scope:

- The `docsiq` binary and all Go packages under `internal/` and `cmd/`
- The embedded React SPA in `ui/`
- The MCP server and REST API exposed by `docsiq serve`
- Build, release, and CI workflows under `.github/`

Out of scope:

- Third-party LLM providers (Azure OpenAI, OpenAI, Ollama) — report
  upstream
- Vulnerabilities that require a compromised local shell or filesystem
  access

## Supported Versions

docsiq is pre-1.0. Only the latest `v0.0.0-beta.N` prerelease receives
security patches.

## Disclosure

We follow coordinated disclosure. Once a fix ships in a release, we
publish a [GitHub Security Advisory](https://github.com/RandomCodeSpace/docsiq/security/advisories)
crediting the reporter unless they request anonymity.

## Report archive

Non-sensitive bug reports and their full discussion history are archived
publicly as [GitHub Issues](https://github.com/RandomCodeSpace/docsiq/issues).
Security reports are archived as
[GitHub Security Advisories](https://github.com/RandomCodeSpace/docsiq/security/advisories)
after coordinated disclosure.
