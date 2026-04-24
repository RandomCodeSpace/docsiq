# Block 6 — Testing & CI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Each task is self-contained and ends with an exact `git commit` block.

**Goal:** Close the final quality-gate gap in docsiq CI by wiring the three missing automated audits (`govulncheck`, `npm audit`, flake-register grep), adding two fuzz targets to the existing `fuzz-smoke` job, backfilling Playwright smokes for the three highest-risk untested flows (404, unauthed API, upload happy-path), and standing up a first-class pipeline integration test that exercises the full 5-phase indexer end-to-end against a deterministic mock LLM provider.

**Architecture:**

Six independent changes across three CI workflows, two Go packages, one UI test suite, and one new `internal/llm/mock` package.

- Tasks 1–2 (`govulncheck`, `npm audit`) are CI-only and share zero code with the others — wire them first so every subsequent commit in this block runs them.
- Task 3 (fuzz targets) adds two `Fuzz*` functions next to the code they exercise (`internal/store/notes_fuzz_test.go`, `internal/mcp/tools_fuzz_test.go`) and extends the `targets=()` array in `.github/workflows/fuzz.yml`.
- Task 4 (flake register) adds a bash step to the `test` job in `.github/workflows/ci.yml` that `grep`s every `t.Skip(` and `test.skip(` and fails the build if any lacks a companion `// TODO(#<N>):` comment. Four existing `t.Skip` sites must be annotated before the gate is wired — otherwise the step fires on its own introduction.
- Task 5 (Playwright smokes) adds three new spec files (`ui/e2e/404.spec.ts`, `ui/e2e/auth.spec.ts`, `ui/e2e/upload.spec.ts`) reusing the existing `stubbedPage` fixture. Zero UI source changes.
- Task 6 (pipeline integration test) is the largest: a new `internal/llm/mock` package (plain Go, no build tag — tests import it via a normal import path) returning deterministic `Complete`/`Embed` output, a `testdata/pipeline/` corpus of five small markdown files, and a new `internal/pipeline/integration_test.go` gated by `//go:build integration` that drives `pipeline.New(...).IndexPath(...).Finalize(...)` and asserts SQLite row counts + `LocalSearch` hits.

**Tech Stack:** Go 1.25 standard `testing`, `testing/fuzz`, `encoding/json`; Playwright 1.x (already in `ui/`); GitHub Actions (SHA-pinned per existing CI convention); `golang.org/x/vuln/cmd/govulncheck` (installed on-the-fly in CI). No new runtime dependencies. Mock LLM provider is ~80 LoC of pure Go with no third-party imports.

**Scope check:** Six items, three subsystems (CI workflows, Go test/fuzz, UI e2e). No sub-plan decomposition needed. All six tasks land in one PR if executed sequentially; executing-plans with one subagent per task produces six atomic commits ready for a squash-merge or six individual PRs — caller's choice.

**Self-contained:** No soft prerequisites. The plan does not touch the middleware chain from Block 2, the workq from Block 1, or the metrics work in Block 4. Task 6's mock provider is new code and replaces no existing test infra (the repo has zero integration tests for the pipeline today — verify via `grep -rn '//go:build integration' internal/pipeline/`; expected: no matches).

---

## Completeness Check

Before any subagent begins, re-read:

1. `.github/workflows/ci.yml` — specifically the `ui`, `test`, and `test-integration` jobs and their SHA-pinned action references. The pins used in this plan MUST match the current pins in `ci.yml`; if dependabot has bumped any of them since this plan was written, use the newer pin and note the drift in the commit body.
2. `.github/workflows/fuzz.yml` — the `targets=()` bash array. Task 3 appends two entries.
3. `.github/workflows/playwright.yml` — path filter `ui/**`; the new spec files live under `ui/e2e/` so no workflow change is needed for Task 5.
4. `ui/e2e/fixtures.ts` — the `stubbedPage` fixture and the `API_PATH` / `MCP_PATH` regex anchors. New specs must reuse this fixture, not define their own `page.route(...)` calls.
5. `internal/store/notes.go` — `SearchNotes(ctx, query, limit)` is the fuzz target for `FuzzSearchTokenize`. The function escapes FTS5 special characters; the fuzz target must assert the function never panics and never returns a `malformed MATCH expression` error on any input.
6. `internal/mcp/tools.go` and `internal/mcp/server.go` — `stringArg`/`intArg` helpers and the registered tool handlers. `FuzzMCPToolArgs` exercises the argument-coercion path via synthetic `map[string]any`, not the JSON-RPC transport.
7. `internal/pipeline/pipeline.go` — `New`, `IndexPath`, `Finalize`. Integration test mirrors `cmd/index.go`'s flow.

Verify all seven files exist via `ls` before starting:

```bash
ls .github/workflows/ci.yml .github/workflows/fuzz.yml .github/workflows/playwright.yml \
   ui/e2e/fixtures.ts internal/store/notes.go internal/mcp/tools.go internal/pipeline/pipeline.go
```

Expected: seven paths, exit 0. If any is missing, halt and investigate before proceeding.

---

## Task Ordering

| # | Task | File(s) | Effort | Depends on |
|---|------|---------|--------|------------|
| 1 | 6.4 `govulncheck` CI job | `.github/workflows/ci.yml` | S | — |
| 2 | 6.5 `npm audit` step | `.github/workflows/ci.yml`, possibly `ui/package-lock.json` | S | — |
| 3 | 6.3 Fuzz targets (`FuzzSearchTokenize`, `FuzzMCPToolArgs`) | `internal/store/notes_fuzz_test.go`, `internal/mcp/tools_fuzz_test.go`, `.github/workflows/fuzz.yml` | M | — |
| 4 | 6.6 Flake register CI gate | `.github/workflows/ci.yml` + annotations on 4 existing `t.Skip` sites | S | — |
| 5 | 6.1 Playwright smokes | `ui/e2e/404.spec.ts`, `ui/e2e/auth.spec.ts`, `ui/e2e/upload.spec.ts` | M | — |
| 6 | 6.2 Pipeline integration test | `internal/llm/mock/mock.go`, `testdata/pipeline/*.md`, `internal/pipeline/integration_test.go` | M | — |

All tasks are independent; `subagent-driven-development` can dispatch all six in parallel if the controller prefers. Sequential ordering above minimises rebase pain in the conventional case (one branch, six commits).

---

## Task 1: govulncheck CI job (6.4)

**Files:**
- Modify: `.github/workflows/ci.yml`

**Rationale:** `~/.claude/rules/security.md` mandates `govulncheck ./...` on every PR that touches Go code. Unlike `cargo audit` or `npm audit`, `govulncheck` reports only *reachable* vulnerabilities by walking the call graph; every hit requires a fix or documented exception. The tool is fast (~15s on docsiq) and contributes no flake surface.

- [ ] **Step 1: Read the current `test` job to locate the insertion point**

Open `.github/workflows/ci.yml`. The `test` job (line ~55) currently runs:

```yaml
- name: go vet (cgo + fts5)
  run: CGO_ENABLED=1 go vet -tags sqlite_fts5 $(go list ./... | grep -v /ui/node_modules/)

- name: go test (cgo + fts5)
  run: CGO_ENABLED=1 go test -tags sqlite_fts5 -timeout 300s $(go list ./... | grep -v /ui/node_modules/)
```

`govulncheck` belongs **after** `go vet` and **before** `go test` — we want vuln scan to gate tests (no point running 300s of tests against a known-exploitable toolchain), and we want it to run after the build cache is hydrated (so its own `go build` invocations are fast).

- [ ] **Step 2: Add the `govulncheck` step**

Edit `.github/workflows/ci.yml`. After the `go vet` step (immediately before `go test`), insert:

```yaml
      - name: govulncheck
        run: |
          set -eu
          # Pinned to the latest stable at plan time; bump via dependabot or
          # with a justification in the commit body. @latest is only acceptable
          # because govulncheck is a first-party golang.org/x module.
          go install golang.org/x/vuln/cmd/govulncheck@latest
          CGO_ENABLED=1 govulncheck -tags sqlite_fts5 ./...
```

Notes:
- `-tags sqlite_fts5` matches the tag used for `go vet` and `go test`; without it `govulncheck` would walk a different (smaller) graph.
- `CGO_ENABLED=1` matches the job-level env already set at line 66.
- `./...` is intentional: we want the full graph, including `cmd/` entry points. `govulncheck` filters to reachable vulns; breadth is fine.
- The job already has `actions/setup-go@...` and the build cache hydrated — no extra setup needed. `go install` writes to `$GOBIN` which is on `$PATH` by default.

- [ ] **Step 3: Verify the full `test` job reads correctly**

After edit, the relevant slice of `ci.yml` should be:

```yaml
      - name: go vet (cgo + fts5)
        run: CGO_ENABLED=1 go vet -tags sqlite_fts5 $(go list ./... | grep -v /ui/node_modules/)

      - name: govulncheck
        run: |
          set -eu
          go install golang.org/x/vuln/cmd/govulncheck@latest
          CGO_ENABLED=1 govulncheck -tags sqlite_fts5 ./...

      - name: go test (cgo + fts5)
        run: CGO_ENABLED=1 go test -tags sqlite_fts5 -timeout 300s $(go list ./... | grep -v /ui/node_modules/)
```

Re-read the full diff via `git diff .github/workflows/ci.yml` — verify no other step was moved or deleted.

- [ ] **Step 4: Run govulncheck locally to dogfood the gate**

Before pushing, run the exact same command locally:

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
CGO_ENABLED=1 govulncheck -tags sqlite_fts5 ./...
```

Expected output: `No vulnerabilities found.` (exit 0).

If the command reports one or more vulnerabilities, **DO NOT ship the CI gate yet**. Per `~/.claude/rules/security.md`:

1. For each High/Critical reachable vuln: upgrade the offending dependency to the fixed version (use `go get <module>@<fixed>`; `go mod tidy`). Commit the module upgrades *in a separate commit* before the CI step so the fix is reviewable independently of the gate.
2. For Medium/Low: same fix-first policy, but if no fixed version exists, document (a) why the code path is not exploitable **or** (b) the mitigation in a top-level `SECURITY.md` section; do not silently suppress.
3. Re-run `govulncheck` until clean, then proceed to Step 5.

If there are no vulns, proceed immediately.

- [ ] **Step 5: Run the full `test` job locally to catch yaml syntax errors**

```bash
# Lint the workflow locally with actionlint if installed; otherwise rely on
# GitHub Actions' own parser once pushed. YAML errors in ci.yml break every
# subsequent PR — worth catching before push.
which actionlint && actionlint .github/workflows/ci.yml || echo "actionlint not installed; skipping lint"
```

Expected: either `actionlint` prints nothing (clean) or the `echo` fires. Either is acceptable.

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "$(cat <<'EOF'
ci: add govulncheck step to Go test job

Runs `govulncheck ./...` on every PR touching Go code. Reachability-based
scan catches High/Critical CVEs in the call graph before tests run; aligns
CI with ~/.claude/rules/security.md policy. Installs the tool on-the-fly
since it's a first-party golang.org/x module and dependabot can bump the
install target as needed.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

**Expected outcome:** On next PR, the `test` job gains a `govulncheck` step that runs for ~20s and currently passes clean. Future CVE introductions (e.g. a transitive bump of `golang.org/x/net` into a vulnerable version) will fail the build with a clickable report.

---

## Task 2: npm audit step (6.5)

**Files:**
- Modify: `.github/workflows/ci.yml` (the `ui` job)
- Possibly: `ui/package.json`, `ui/package-lock.json` (if outstanding Medium+ advisories exist)

**Rationale:** `~/.claude/rules/security.md` mandates `npm audit --audit-level=moderate` after every install. The existing `ui` job runs `npm --prefix ui ci` but does not audit — so a moderate-severity CVE introduced via a transitive dependency can sit on `main` indefinitely. Wiring the gate is a one-line step, but only after the current tree is confirmed clean (otherwise the gate trips on its own introduction).

- [ ] **Step 1: Run `npm audit` locally to discover outstanding advisories**

```bash
cd ui
npm ci  # ensure node_modules matches package-lock.json
npm audit --audit-level=moderate --json > /tmp/audit.json || true
jq '.metadata.vulnerabilities' /tmp/audit.json
```

Expected output is a JSON object like:

```json
{ "info": 0, "low": 0, "moderate": 0, "high": 0, "critical": 0, "total": 0 }
```

If `moderate`, `high`, or `critical` > 0: **STOP and resolve before continuing**. Skip to Step 2. Otherwise skip to Step 3.

- [ ] **Step 2: Resolve outstanding moderate+ advisories (conditional — only if Step 1 found any)**

```bash
cd ui
npm audit fix
npm audit --audit-level=moderate
```

If `npm audit fix` resolves everything, the exit code of the second command is 0 — proceed to Step 3 after committing the lockfile changes in a separate commit:

```bash
cd ..
git add ui/package.json ui/package-lock.json
git commit -m "$(cat <<'EOF'
fix(ui,deps): resolve outstanding moderate+ npm advisories

Runs `npm audit fix` to patch transitive dependencies before wiring the
`npm audit` CI gate. No functional changes; lockfile-only delta.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

If `npm audit fix` cannot resolve an advisory without a breaking upgrade:

- For each such advisory, evaluate the breaking change impact. If the patched version is a minor/major bump of a library we directly depend on: take the bump, run `npm --prefix ui test`, `npm --prefix ui run typecheck`, and `npm --prefix ui run build` to confirm it still compiles and tests pass. Include the bumps in the same commit as above.
- If a dependency is truly pinned by a downstream constraint (very rare for this repo's footprint), document the specific advisory ID, the reason it's unreachable, and the mitigation in `ui/package.json` under an `"auditSuppress": [{ "id": "...", "reason": "..." }]` key (note: this is a project-local convention, not an npm-standard field; read in Step 4). The CI step does *not* consult this key — it continues to fail — but the suppression record gives reviewers the context to override via a merge-commit label. Prefer fixing over suppressing; raise with the user if tempted.

- [ ] **Step 3: Add the `npm audit` step to the `ui` job**

Edit `.github/workflows/ci.yml`. The `ui` job currently has (around line 23):

```yaml
      - name: Install UI dependencies
        run: npm --prefix ui ci

      - name: Type check
        run: npm --prefix ui run typecheck
```

Insert between them:

```yaml
      - name: Install UI dependencies
        run: npm --prefix ui ci

      - name: npm audit (moderate+)
        run: npm --prefix ui audit --audit-level=moderate

      - name: Type check
        run: npm --prefix ui run typecheck
```

Rationale for placement: runs immediately after `npm ci` so a failing audit short-circuits the rest of the job (typecheck, vitest, build, bundle-budget, upload) — fastest feedback for the most common "someone opened a PR that bumped a dep and introduced a CVE" path.

`--audit-level=moderate` matches the rule-book floor. Upgrade to `low` when the repo is production-critical (pre-release is fine at moderate).

- [ ] **Step 4: Verify the step reads correctly and lint-check**

```bash
# Eyeball the diff
git diff .github/workflows/ci.yml

# Verify the exact command runs clean locally
npm --prefix ui audit --audit-level=moderate
```

Expected: diff shows only the 3-line insert; local audit exits 0.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "$(cat <<'EOF'
ci(ui): fail build on moderate+ npm advisories

Adds `npm audit --audit-level=moderate` between `npm ci` and `typecheck`
in the UI job. Short-circuits the rest of the job on a failing audit so
CVE-introducing dep bumps surface immediately. Matches the rule-book
policy in ~/.claude/rules/security.md.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

**Expected outcome:** A future PR that bumps a transitive into a vulnerable version fails CI within ~45s (job time dominated by `npm ci`, audit itself is ~2s).

---

## Task 3: Fuzz targets — FuzzSearchTokenize + FuzzMCPToolArgs (6.3)

**Files:**
- Create: `internal/store/notes_fuzz_test.go`
- Create: `internal/mcp/tools_fuzz_test.go`
- Modify: `.github/workflows/fuzz.yml`

**Rationale:** The existing fuzz smoke job runs `FuzzResolveURL` and `FuzzChunker` for 30s each on every PR. The two most-exposed parse surfaces *not* currently fuzzed are:

1. The FTS5 query path in `Store.SearchNotes` — untrusted user input (search box) flows directly into a `WHERE notes_fts MATCH ?` prepared statement. SQLite FTS5 has strict grammar rules; malformed queries return `malformed MATCH expression` errors that bubble to a 500. The target asserts the function never panics and never returns such an error for any input — any input that trips the error must be fixed by pre-sanitising in `SearchNotes`.
2. The MCP tool argument coercion — `stringArg`/`intArg` in `internal/mcp/server.go` unmarshal `map[string]any` keys from JSON-RPC payloads with a type switch. The target hydrates the map with random JSON scalars (string, float64, bool, nil, nested map) and asserts the helpers never panic and always return sensible defaults for unsupported types.

Both targets run in milliseconds per iteration, so a 30s fuzz budget covers ~10M iterations — plenty to catch bit-flip regressions.

- [ ] **Step 1: Write `FuzzSearchTokenize`**

Create `internal/store/notes_fuzz_test.go`:

```go
//go:build sqlite_fts5

package store

import (
	"context"
	"strings"
	"testing"
)

// FuzzSearchTokenize asserts that Store.SearchNotes never panics and never
// returns a "malformed MATCH expression" error for any query string. Any
// input that trips the latter must be fixed by pre-sanitising inside
// SearchNotes — the HTTP boundary cannot be trusted to pre-filter FTS5
// control characters, and clients routinely pass the raw search box.
func FuzzSearchTokenize(f *testing.F) {
	// Seeds cover the FTS5 grammar corners that historically broke:
	// - empty / whitespace
	// - unbalanced quotes / parens
	// - bare operators (AND/OR/NOT as lone tokens)
	// - column-qualified terms (title:foo)
	// - prefix wildcards (foo*)
	// - NULL bytes and long repeats
	// - non-ASCII / RTL text
	seeds := []string{
		"",
		" ",
		"\n\t",
		"hello",
		"hello world",
		"\"unbalanced",
		"(lonely",
		"AND",
		"NOT title:foo",
		"foo*",
		"\x00\x00",
		strings.Repeat("a", 4096),
		"你好 世界",
		"مرحبا", // RTL
		"a OR (b AND \"c d\")",
		"col:tag1 tag2",
		"--comment",
		"/* comment */",
		"foo bar -",
		";",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	// One shared store per fuzz process. Re-opening per iteration would
	// dominate runtime and produce no new coverage — the surface under
	// test is the query path, not Open.
	s := newTestStore(f)
	defer s.Close()
	ctx := context.Background()

	// Seed a handful of notes so MATCH has something non-empty to scan.
	// An empty table short-circuits most of the FTS5 query planner and
	// would hide grammar bugs.
	for i, body := range []string{"alpha beta", "gamma delta", "title:something"} {
		if err := s.IndexNote(ctx, &Note{Key: fmtKey(i), Title: "t", Content: body, Tags: "tag"}); err != nil {
			f.Fatalf("seed IndexNote: %v", err)
		}
	}

	f.Fuzz(func(t *testing.T, query string) {
		// We don't care about the result — only that the call completes
		// without panic and without leaking a raw FTS5 syntax error.
		_, err := s.SearchNotes(ctx, query, 5)
		if err != nil && strings.Contains(err.Error(), "malformed MATCH expression") {
			t.Fatalf("unsanitised FTS5 grammar leaked for query %q: %v", query, err)
		}
		// Other errors (e.g. context cancelled) are acceptable during
		// fuzzing; they are not the class of bug this target is hunting.
	})
}

// fmtKey is a tiny helper used only by this file. Kept local to avoid
// polluting the package with a test-only name.
func fmtKey(i int) string {
	return "k" + string(rune('0'+i))
}
```

- [ ] **Step 2: Locate or add `newTestStore`**

The test helper `newTestStore(t testing.TB)` is used throughout `internal/store/*_test.go`. Verify it exists:

```bash
grep -n 'func newTestStore' internal/store/*_test.go | head -5
```

Expected: at least one match pointing at `internal/store/store_test.go` or similar. If no helper exists, add this minimal one to `internal/store/testhelpers_test.go` (create the file if absent):

```go
//go:build sqlite_fts5

package store

import (
	"testing"
)

func newTestStore(t testing.TB) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir + "/test.db")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
```

(If your grep found one with a slightly different signature — e.g. `newTestStore(t *testing.T)` — use that; `*testing.F` embeds `*testing.T` via `testing.TB`, so the above accepts both.)

- [ ] **Step 3: Write `FuzzMCPToolArgs`**

Create `internal/mcp/tools_fuzz_test.go`:

```go
//go:build sqlite_fts5

package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// FuzzMCPToolArgs asserts that the argument-coercion helpers (stringArg,
// intArg, and the `project` shortcut projectArg) never panic on any JSON
// payload an MCP client might send. We fuzz a JSON blob, unmarshal it
// into map[string]any (the exact type the real handlers receive via
// mcpgo.CallToolRequest.GetArguments()), and poke each helper with the
// known keys plus a couple of keys that intentionally don't exist.
func FuzzMCPToolArgs(f *testing.F) {
	// Seeds cover the shapes that flow through the real tool registrations
	// in tools.go: strings, numbers (float64 after JSON round-trip),
	// booleans, nulls, nested objects, and arrays. Malformed JSON is fed
	// via the "ignore" branch — the unmarshal error is expected and the
	// fuzzer moves on.
	seeds := []string{
		`{}`,
		`{"query":"hello","top_k":5}`,
		`{"query":"","top_k":-1}`,
		`{"query":null}`,
		`{"query":true,"top_k":"five"}`,
		`{"query":{"nested":"bad"}}`,
		`{"query":[1,2,3]}`,
		`{"project":"my-project","entity_name":"Alice","depth":2}`,
		`{"top_k":1e308}`,
		`{"top_k":1.5}`,
		`{"community_level":0}`,
		`{"` + strings.Repeat("a", 1024) + `":"long-key"}`,
		`not json at all`,
		``,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	// All known argument keys used across internal/mcp/tools.go and
	// notes_tools.go. Exhaustive is cheap; if a new tool adds a new
	// key this list lags but the fuzz target still covers the helpers.
	keys := []string{
		"query", "top_k", "doc_type", "project",
		"community_level", "entity_name", "depth",
		"from", "to", "predicate",
		"note_key", "content", "tags", "limit",
	}

	f.Fuzz(func(t *testing.T, raw string) {
		var args map[string]any
		if err := json.Unmarshal([]byte(raw), &args); err != nil {
			// Not valid JSON — not our target. MCP transport layer
			// already rejects these before they reach tool handlers.
			t.Skip()
		}
		if args == nil {
			// JSON "null" at the top level — nothing to coerce.
			return
		}

		for _, k := range keys {
			_ = stringArg(args, k, "default")
			_ = intArg(args, k, 0)
		}
		// projectArg lives in server.go and wraps stringArg for "project".
		_ = projectArg(args)
	})
}
```

- [ ] **Step 4: Verify the new targets compile and run for 5s each**

```bash
CGO_ENABLED=1 go test -tags sqlite_fts5 -run=^$ -fuzz=^FuzzSearchTokenize$ -fuzztime=5s ./internal/store/
CGO_ENABLED=1 go test -tags sqlite_fts5 -run=^$ -fuzz=^FuzzMCPToolArgs$ -fuzztime=5s ./internal/mcp/
```

Expected output for each:

```
fuzz: elapsed: 0s, gathering baseline coverage: 0/N completed
fuzz: elapsed: 0s, gathering baseline coverage: N/N completed, now fuzzing with K workers
fuzz: elapsed: 3s, execs: ... (M/sec), new interesting: ... (total: N)
fuzz: elapsed: 5s, execs: ... (M/sec), new interesting: ... (total: N)
PASS
ok  	github.com/RandomCodeSpace/docsiq/internal/store	5.xxxs
```

If either target fails during the 5s smoke:

- For `FuzzSearchTokenize` hitting `malformed MATCH expression`: this is the **expected signal** — a real grammar bug has been found. Reproduce the failing input from `testdata/fuzz/FuzzSearchTokenize/<hash>`, add a pre-sanitiser to `SearchNotes` (e.g. drop unbalanced quotes, collapse bare operators to their literal form), and re-run until the 5s smoke is clean. Keep the failing input as a seed corpus entry — `testdata/fuzz/` is checked in.
- For `FuzzMCPToolArgs` panicking: this is a real bug in `stringArg`/`intArg`. Fix the type switch to cover the panicking case (probably `case json.Number`, `case []any`, or the nil interface path), add the repro as a unit test, re-run until clean.

Either way, commit the fix *separately* from the fuzz target commit so the bug fix is reviewable in isolation.

- [ ] **Step 5: Extend the fuzz-smoke workflow**

Edit `.github/workflows/fuzz.yml`. The current `targets=(...)` array (lines 27–30) reads:

```yaml
          targets=(
            "./internal/crawler::FuzzResolveURL"
            "./internal/chunker::FuzzChunker"
          )
```

Change to:

```yaml
          targets=(
            "./internal/crawler::FuzzResolveURL"
            "./internal/chunker::FuzzChunker"
            "./internal/store::FuzzSearchTokenize"
            "./internal/mcp::FuzzMCPToolArgs"
          )
```

No other changes to the workflow are needed — the per-target bash loop already handles arbitrary `pkg::fn` entries.

- [ ] **Step 6: Run the extended fuzz job locally end-to-end**

Simulate the CI loop:

```bash
set -eu
targets=(
  "./internal/crawler::FuzzResolveURL"
  "./internal/chunker::FuzzChunker"
  "./internal/store::FuzzSearchTokenize"
  "./internal/mcp::FuzzMCPToolArgs"
)
for entry in "${targets[@]}"; do
  pkg="${entry%%::*}"
  fn="${entry##*::}"
  echo ">> fuzz $pkg $fn"
  CGO_ENABLED=1 go test -tags sqlite_fts5 \
    -run=^$ -fuzz="^${fn}$" -fuzztime=30s "$pkg"
done
```

Expected: four `PASS` blocks, ~2 min total wall-clock.

- [ ] **Step 7: Commit**

```bash
git add internal/store/notes_fuzz_test.go internal/mcp/tools_fuzz_test.go .github/workflows/fuzz.yml
# Include testhelpers_test.go if Step 2 created it.
git commit -m "$(cat <<'EOF'
test: add FuzzSearchTokenize and FuzzMCPToolArgs to fuzz-smoke

Two new fuzz targets wired into the existing fuzz (smoke) CI job:

- FuzzSearchTokenize exercises Store.SearchNotes against arbitrary
  FTS5-grammar inputs; asserts no "malformed MATCH expression" leaks,
  which would indicate missing pre-sanitisation at the HTTP boundary.
- FuzzMCPToolArgs exercises stringArg/intArg/projectArg against any
  JSON payload an MCP client might send; asserts no helper panics on
  unexpected types.

Each runs 30s on every PR — ~10M iterations of real coverage at no
meaningful CI cost.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

**Expected outcome:** Fuzz job wall-clock grows from ~60s (2 targets × 30s) to ~120s (4 × 30s). Total PR CI cost rises by ~1 min; well within the soft budget.

---

## Task 4: Flake register CI gate (6.6)

**Files:**
- Modify: existing `t.Skip(` sites across four test files to carry a `// TODO(#<N>):` annotation on the same line or the preceding line
- Modify: `.github/workflows/ci.yml` (add a new grep step)

**Rationale:** An untracked `t.Skip()` is a silent bug — the test is gone but nobody remembers why or when to re-enable. Enforcing a `// TODO(#<issue>):` annotation on every skip converts silent state into a queryable backlog (`gh issue list --label flake-register`). The CI gate is a single `grep` invocation that fails on any skip without an adjacent TODO; it's cheap, deterministic, and adds zero flake surface itself.

**Two-step rollout** is mandatory: annotate every existing skip *before* wiring the gate, otherwise the gate's own introduction fails CI.

- [ ] **Step 1: Enumerate every existing skip site**

```bash
grep -rn 't\.Skip(' --include='*.go' . | grep -v '_fuzz_test\.go' | grep -v node_modules
grep -rn 'test\.skip(' --include='*.ts' --include='*.tsx' ui/ | grep -v node_modules
```

Expected (Go, snapshot at plan time — re-run and reconcile before proceeding; the list may have drifted):

```
internal/api/notes_import_limits_test.go:81:		t.Skip("skipping large-tar test in -short mode")
internal/notes/notes_test.go:230:		t.Skip("skipping 1000-note scale test in -short mode")
internal/notes/history_test.go:16:		t.Skip("git not available")
internal/vectorindex/hnsw_test.go:216:		t.Skip("skipping 10k benchmark in -short")
internal/vectorindex/hnsw_test.go:223:		t.Skip("skipping 10k recall benchmark under -race (sequential workload)")
internal/project/registry_test.go:47:		t.Skip("chmod semantics differ on windows")
internal/project/registry_test.go:50:		t.Skip("running as root; chmod 0555 does not block writes")
internal/hookinstaller/installer_test.go:265:			t.Skip("symlink support requires admin on Windows")
```

(The `internal/chunker/chunker_fuzz_test.go:27` skip is inside a `f.Fuzz` callback that rejects invalid fuzz inputs — that is *not* a flake-register-style skip and the `_fuzz_test.go` exclusion above filters it out. Preserve this exclusion in the CI step.)

Expected (UI): likely zero `test.skip(` calls today. Re-check.

- [ ] **Step 2: File issues for each unannotated skip**

For each skip site that represents a real latent problem (NOT environmental — e.g. "git not available" and "windows-specific" are environmental and get a dedicated comment form below), file an issue via `gh issue create` with a one-line title and a short body pointing back at the file:line. Label `flake-register`.

```bash
gh issue create --title "flake-register: skipping 1000-note scale test in -short mode" \
  --body "Location: internal/notes/notes_test.go:230

Test is skipped under \`go test -short\` to keep CI fast. Tracking issue
so we know to re-evaluate periodically — either run it on a nightly
workflow, or split it into an \`//go:build scale\` file we can opt in.

Owner: unassigned." \
  --label flake-register
```

Record the issue number each command prints — you'll need it in Step 3.

For environmental skips (git missing, windows chmod, root user, windows admin) the TODO uses a shared tracking issue. File one issue titled `flake-register: environmental skips (platform/tool availability)` and use its number for all environmental sites. This keeps the issue count small without losing the grep invariant.

- [ ] **Step 3: Annotate each skip site**

Use the format `// TODO(#<N>): <short why>` on the line directly preceding the `t.Skip(` call, preserving the existing message.

Example — `internal/notes/notes_test.go`:

```go
// Before:
if testing.Short() {
    t.Skip("skipping 1000-note scale test in -short mode")
}

// After:
if testing.Short() {
    // TODO(#NNN): re-run on nightly scale workflow
    t.Skip("skipping 1000-note scale test in -short mode")
}
```

Apply the same pattern to all eight sites listed in Step 1. Use the real issue numbers from Step 2.

For the fuzz-callback `t.Skip()` in `internal/chunker/chunker_fuzz_test.go:27` — **do not annotate**. The CI grep must explicitly exclude `_fuzz_test.go` (Step 4) because those skips are input-filtering, not flake-register entries.

- [ ] **Step 4: Add the CI grep step**

Edit `.github/workflows/ci.yml`. Inside the `test` job, after `go build` and before the `Upload docsiq binary` step, insert:

```yaml
      - name: flake-register (every t.Skip / test.skip has a tracked TODO)
        run: |
          set -euo pipefail
          # Every skip must be either:
          #   (a) on a line with an inline `// TODO(#N):` comment, OR
          #   (b) immediately preceded by a `// TODO(#N):` comment line.
          # Fuzz-callback skips (input filtering) are excluded: they are
          # not flake-register entries and carry no issue.
          echo "Scanning for t.Skip( without a tracked TODO..."
          violations=0
          # Go side
          while IFS=: read -r file lineno _; do
            # Inline on the skip line
            if sed -n "${lineno}p" "$file" | grep -qE '// TODO\(#[0-9]+\):'; then
              continue
            fi
            # Preceding line
            prev=$((lineno - 1))
            if [ "$prev" -gt 0 ] && sed -n "${prev}p" "$file" | grep -qE '// TODO\(#[0-9]+\):'; then
              continue
            fi
            echo "::error file=$file,line=$lineno::t.Skip without TODO(#N): annotation"
            violations=$((violations + 1))
          done < <(grep -rn 't\.Skip(' --include='*.go' . | grep -v '_fuzz_test\.go' | grep -v node_modules || true)
          # TypeScript side
          while IFS=: read -r file lineno _; do
            if sed -n "${lineno}p" "$file" | grep -qE '// TODO\(#[0-9]+\):'; then
              continue
            fi
            prev=$((lineno - 1))
            if [ "$prev" -gt 0 ] && sed -n "${prev}p" "$file" | grep -qE '// TODO\(#[0-9]+\):'; then
              continue
            fi
            echo "::error file=$file,line=$lineno::test.skip without TODO(#N): annotation"
            violations=$((violations + 1))
          done < <(grep -rn 'test\.skip(' --include='*.ts' --include='*.tsx' ui/ 2>/dev/null | grep -v node_modules || true)
          if [ "$violations" -gt 0 ]; then
            echo "::error::Found $violations skipped test(s) without a tracking issue. File a flake-register issue and add // TODO(#N): <why> adjacent to the skip."
            exit 1
          fi
          echo "All skips accounted for."
```

Place the step inside the `test` job — NOT `test-integration` or `ui` — because the Go source tree is already checked out there and the UI source is cheap to grep. Keep the step deliberately self-contained bash (no `gh` calls, no external network) so it runs fast and has no flake surface.

- [ ] **Step 5: Verify the gate locally**

```bash
# Copy the step body into a throwaway shell script and run it from repo root.
cat > /tmp/flake-check.sh <<'BASH'
set -euo pipefail
violations=0
while IFS=: read -r file lineno _; do
  if sed -n "${lineno}p" "$file" | grep -qE '// TODO\(#[0-9]+\):'; then
    continue
  fi
  prev=$((lineno - 1))
  if [ "$prev" -gt 0 ] && sed -n "${prev}p" "$file" | grep -qE '// TODO\(#[0-9]+\):'; then
    continue
  fi
  echo "VIOLATION: $file:$lineno"
  violations=$((violations + 1))
done < <(grep -rn 't\.Skip(' --include='*.go' . | grep -v '_fuzz_test\.go' | grep -v node_modules || true)
echo "violations: $violations"
[ "$violations" -eq 0 ] || exit 1
BASH
bash /tmp/flake-check.sh
```

Expected: `violations: 0`, exit 0.

If the check fires on any site: the annotation in Step 3 is wrong (check format: `// TODO(#NNN):` with a hash, digits, and a colon). Fix and re-run.

- [ ] **Step 6: Commit**

```bash
git add internal/ .github/workflows/ci.yml
git commit -m "$(cat <<'EOF'
test,ci: enforce flake-register — every skip gets a tracked issue

Annotates the 8 existing t.Skip() sites with // TODO(#N): references
(see flake-register label on the issue tracker) and adds a CI grep step
that fails if any future t.Skip or test.skip lacks an adjacent TODO.

Converts silent skips into a queryable backlog without changing test
behaviour. Fuzz-callback skips (input filtering) are excluded.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

**Expected outcome:** A future PR that `t.Skip()`s a failing test without filing an issue fails the `flake-register` step with a clickable `::error file=...,line=...` annotation pointing at the offending line.

---

## Task 5: Playwright smokes — 404, unauthed API, upload happy-path (6.1)

**Files:**
- Create: `ui/e2e/404.spec.ts`
- Create: `ui/e2e/auth.spec.ts`
- Create: `ui/e2e/upload.spec.ts`

**Rationale:** The existing `ui/e2e/smoke.spec.ts` covers the happy-path shell (home, command palette, theme toggle, g,g nav). Three high-risk flows remain untested:

- **404**: the catch-all `<Route path="*" element={<NotFound />} />` in `ui/src/App.tsx:39` should render a recognisable `NotFound` component, not a blank page or the home route. Regression here has shipped twice before (once due to a react-router v6 migration, once due to Suspense fallback swallowing errors).
- **Unauthed API call**: the existing `stubbedPage` fixture always returns `{}` — we need a flow that proves the UI *reacts* to a 401 (or the app's equivalent unauthed state) by surfacing a visible "please sign in" affordance, not by silently hiding data.
- **Upload happy-path**: the most write-heavy user flow in the app, and the most common source of "it used to work" regressions. The smoke stubs the backend accept/enqueue responses and drives the UI from file-picker click to success toast.

- [ ] **Step 1: Write the 404 smoke**

Create `ui/e2e/404.spec.ts`:

```ts
import { test, expect } from "./fixtures";

test.describe("404", () => {
  test("unknown route renders NotFound and keeps the shell", async ({ stubbedPage: page }) => {
    await page.goto("/this-route-does-not-exist");

    // NotFound lives inside the Shell, so the main landmark still renders.
    await expect(page.locator("main#main")).toBeVisible();

    // NotFound component copy is stable; adjust the regex if the
    // component's text changes (see ui/src/App.tsx:47).
    await expect(page.getByText(/not found|page not found|404/i).first()).toBeVisible();

    // Sidebar (Shell chrome) is still present — 404 must not unmount the app.
    await expect(page.getByRole("navigation")).toBeVisible();
  });

  test("nested unknown route under /notes also renders NotFound", async ({ stubbedPage: page }) => {
    await page.goto("/notes/definitely-not-a-key/subpath-that-does-not-exist");
    // The catch-all in App.tsx handles any unmatched depth — confirm it
    // still fires for deep URLs (react-router ordering regression).
    await expect(page.locator("main#main")).toBeVisible();
    await expect(page.getByText(/not found|page not found|404/i).first()).toBeVisible();
  });
});
```

- [ ] **Step 2: Write the unauthed-API smoke**

Create `ui/e2e/auth.spec.ts`:

```ts
import { test as base, expect, type Page } from "@playwright/test";

// This spec overrides the default stubbedPage fixture because we want
// the API to return 401, not 200-with-empty-body. We keep the MCP stub
// from the shared fixture conceptually but implement it inline — no
// export from fixtures.ts is needed, keeping the spec self-contained.
const API_PATH = /^\/api\//;
const MCP_PATH = /^\/mcp\//;

async function stubUnauthed(page: Page) {
  await page.route(
    (url) => API_PATH.test(url.pathname),
    (route) =>
      route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ error: "unauthenticated" }),
      }),
  );
  await page.route(
    (url) => MCP_PATH.test(url.pathname),
    (route) =>
      route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ error: "unauthenticated" }),
      }),
  );
}

const test = base.extend<{ unauthedPage: Page }>({
  unauthedPage: async ({ page }, use) => {
    await stubUnauthed(page);
    await use(page);
  },
});

test.describe("unauthed API", () => {
  test("home surfaces an auth-required affordance when /api/* returns 401", async ({ unauthedPage: page }) => {
    await page.goto("/");
    await expect(page.locator("main#main")).toBeVisible();

    // The UI's unauthed-state contract (verify against
    // ui/src/routes/Home.tsx — the text below may need tightening):
    // on 401 the error boundary / empty state shows a visible
    // "sign in" or "authentication required" affordance, not a
    // spinner forever.
    await expect(
      page
        .getByText(/sign in|authenticat|authori|session expired|please log in/i)
        .first(),
    ).toBeVisible({ timeout: 5_000 });

    // No silent 200: ensure no stale list was rendered with empty state
    // copy that could be confused with "you have no notes".
    await expect(page.getByText(/^you have no notes$/i)).toHaveCount(0);
  });

  test("navigating to /notes with 401 shows the same affordance", async ({ unauthedPage: page }) => {
    await page.goto("/notes");
    await expect(page.locator("main#main")).toBeVisible();
    await expect(
      page
        .getByText(/sign in|authenticat|authori|session expired|please log in/i)
        .first(),
    ).toBeVisible({ timeout: 5_000 });
  });
});
```

**Note to implementer**: the regex in the assertion is intentionally broad because the exact copy of the unauthed affordance is a UI decision that may drift. After running the smoke once, tighten the regex to match the actual component's text (`grep -rn 'sign in\|authenticat' ui/src/ --include='*.tsx'` will point at the copy). If the affordance does not exist yet, this smoke will fail and the implementer should raise a follow-up — **this is the intended signal**, not a bug in the plan.

- [ ] **Step 3: Write the upload-happy-path smoke**

Create `ui/e2e/upload.spec.ts`:

```ts
import { test as base, expect, type Page } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Upload flow stubs are stricter than the default fixture: we need the
// UI's POST /api/documents/upload (or equivalent — check the real path
// in src/hooks/api/) to get a 200 with a job ID, then any polling GET
// to the job-status endpoint returns "complete" so the UI shows a toast.
const API_PATH = /^\/api\//;

async function stubUploadAccept(page: Page) {
  await page.route(
    (url) => API_PATH.test(url.pathname),
    async (route) => {
      const req = route.request();
      const p = new URL(req.url()).pathname;

      // Upload accept — return a synthetic job ID. Adjust the path
      // regex if the real endpoint differs (check src/hooks/api/*).
      if (req.method() === "POST" && /\/upload|\/documents$/.test(p)) {
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ job_id: "test-job-1", status: "accepted" }),
        });
      }
      // Upload status polling — report complete immediately.
      if (/\/jobs?\/test-job-1$/.test(p) || /\/upload\/status/.test(p)) {
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            job_id: "test-job-1",
            status: "complete",
            progress: 100,
            doc_count: 1,
          }),
        });
      }
      // Stats / list refreshes after upload — return the uploaded doc.
      if (/\/documents(\?|$)/.test(p)) {
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify([
            { id: "doc-1", title: "fixture.md", doc_type: "md" },
          ]),
        });
      }
      return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
    },
  );
}

const test = base.extend<{ uploadPage: Page }>({
  uploadPage: async ({ page }, use) => {
    await stubUploadAccept(page);
    await use(page);
  },
});

test.describe("upload happy-path", () => {
  test("user can select a file and see completion feedback", async ({ uploadPage: page }) => {
    await page.goto("/docs");
    await expect(page.locator("main#main")).toBeVisible();

    // Open the upload affordance. The button's accessible name must
    // match one of these patterns — tighten the regex after first run.
    const uploadButton = page
      .getByRole("button", { name: /upload|add doc|new document/i })
      .first();
    await expect(uploadButton).toBeVisible({ timeout: 5_000 });
    await uploadButton.click();

    // File chooser interception: Playwright fires `filechooser` when
    // the native <input type="file"> is clicked. We attach the fixture
    // file from the testdata directory.
    const [chooser] = await Promise.all([
      page.waitForEvent("filechooser"),
      page.getByRole("button", { name: /choose file|select file|browse/i }).first().click(),
    ]);
    await chooser.setFiles(path.join(__dirname, "fixtures/fixture.md"));

    // Trigger submit — button text varies; tighten regex after run.
    await page
      .getByRole("button", { name: /^upload$|^submit$|^start$/i })
      .first()
      .click();

    // Expect a success surface. The exact copy is a UI decision; one
    // of the following must match within 5s.
    await expect(
      page
        .getByText(/upload complete|indexed|1 document|success/i)
        .first(),
    ).toBeVisible({ timeout: 5_000 });
  });
});
```

Also create the fixture file the test references:

`ui/e2e/fixtures/fixture.md`:

```markdown
# Test Fixture

This is a tiny markdown file used by the upload happy-path smoke.
It exists only so the file-chooser interception has a real file
to hand to the UI.
```

**Implementer note**: as with the unauthed smoke, the role-name regexes intentionally cover multiple possible UI copies. After first run, inspect which selectors actually matched via `npx playwright test upload.spec.ts --trace on` and tighten to exactly the strings the real UI uses. If any required affordance does not exist (e.g. no upload button), this smoke fails — **that is the intended signal**; raise a follow-up for the missing UI, don't relax the assertion.

- [ ] **Step 4: Run the three new smokes locally**

```bash
cd ui
npm ci
npx playwright install --with-deps chromium
CI=1 ./node_modules/.bin/playwright test 404.spec.ts auth.spec.ts upload.spec.ts --reporter=list --workers=1
```

Expected: 5 tests pass (2 × 404, 2 × auth, 1 × upload).

Failure modes and their diagnoses:

- **404 "not found" text not matching** — check `ui/src/App.tsx:47` for the actual `NotFound` component's copy and tighten the regex.
- **Auth smoke times out on the affordance** — either (a) the UI doesn't yet show an unauthed affordance (file a follow-up and mark the test `.fixme` with a `TODO(#N):`, per Task 4), or (b) the copy differs (tighten regex).
- **Upload smoke can't find the button** — inspect the real Documents page markup; the button may live behind a drawer or require a route prefetch first.

Iterate until all five pass. Do NOT change UI source for this task — only the spec.

- [ ] **Step 5: Run the existing smoke.spec.ts alongside to confirm no regression**

```bash
cd ui
CI=1 ./node_modules/.bin/playwright test --reporter=list --workers=1
```

Expected: existing 5 smokes pass + new 5 pass = 10 total.

- [ ] **Step 6: Commit**

```bash
git add ui/e2e/404.spec.ts ui/e2e/auth.spec.ts ui/e2e/upload.spec.ts ui/e2e/fixtures/fixture.md
git commit -m "$(cat <<'EOF'
test(ui,e2e): Playwright smokes for 404, unauthed API, upload

Three new Playwright spec files covering the highest-risk flows not
exercised by smoke.spec.ts:

- 404.spec.ts: catch-all route renders NotFound inside the Shell,
  including deep nested URLs under /notes.
- auth.spec.ts: /api/* returning 401 surfaces a visible auth affordance
  on both Home and /notes — no silent empty state.
- upload.spec.ts: Documents → upload → file-chooser → submit →
  "upload complete" toast, with a stubbed accept/poll backend and a
  one-line markdown fixture.

All three run under the existing playwright workflow (path-filtered
to ui/**) and add ~15s to the e2e job wall-clock.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

**Expected outcome:** The Playwright job grows from 5 tests to 10. A regression in the 404 component, the unauthed affordance, or the upload flow now fails CI within ~40s.

---

## Task 6: Pipeline integration test (6.2)

**Files:**
- Create: `internal/llm/mock/mock.go`
- Create: `testdata/pipeline/alpha.md`
- Create: `testdata/pipeline/beta.md`
- Create: `testdata/pipeline/gamma.md`
- Create: `testdata/pipeline/delta.md`
- Create: `testdata/pipeline/epsilon.md`
- Create: `internal/pipeline/integration_test.go`

**Rationale:** The `index` command is the first thing a user runs and the most complex code path in the repo (5 phases, 10+ goroutines, SQLite + HNSW + LLM fanout). Today it has zero end-to-end coverage — every phase has unit tests, but the integration is exercised only manually. A single integration test that drives the full pipeline against a deterministic mock provider catches the category of bug where a phase's output schema drifts out of sync with the next phase's input schema — the kind of bug unit tests structurally cannot find.

The mock provider returns deterministic completions (for entity extraction / community summarisation) and deterministic embeddings (hash-based, so same text → same vector). This makes the test fully reproducible and fast (~5s wall-clock vs 5+ min with a real LLM).

- [ ] **Step 1: Design the mock LLM provider**

The `llm.Provider` interface (`internal/llm/provider.go:42`) is:

```go
type Provider interface {
    Complete(ctx context.Context, prompt string, opts ...Option) (string, error)
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    Name() string
    ModelID() string
}
```

We need a mock that:
- Returns syntactically-valid JSON for the entity/relationship/claims extraction prompts (the pipeline parses these — malformed JSON fails the test for the wrong reason).
- Returns a deterministic, short natural-language summary for community-summarisation prompts.
- Returns 128-dim embeddings (or whatever the default is in `cfg.Embedding.Dims`) deterministically derived from the text hash.
- Does all the above based on simple prompt-substring matching — we don't need a real LLM, just predictable fixtures.

- [ ] **Step 2: Write `internal/llm/mock/mock.go`**

```go
// Package mock provides a deterministic llm.Provider implementation for
// tests. It does NOT require any network, API key, or external process.
// Callers import it directly (no build tag) — the package lives under
// internal/ so it cannot leak into the public API surface.
package mock

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/RandomCodeSpace/docsiq/internal/llm"
)

// Provider is a deterministic, in-memory llm.Provider useful for unit
// and integration tests. It inspects the prompt for known substrings
// and returns canned, schema-valid JSON; embeddings are derived from a
// SHA-256 of the input so equal text yields equal vectors.
type Provider struct {
	Dims int // default 128
}

// New returns a mock provider with 128-dim embeddings. Pass a non-zero
// dims to override.
func New(dims int) *Provider {
	if dims <= 0 {
		dims = 128
	}
	return &Provider{Dims: dims}
}

// Compile-time check.
var _ llm.Provider = (*Provider)(nil)

func (p *Provider) Name() string    { return "mock" }
func (p *Provider) ModelID() string { return "mock-0" }

// Complete returns deterministic JSON keyed to the prompt's intent.
// Intent detection uses substring matching — cheap and sufficient for
// the GraphRAG extraction prompts which embed stable keywords like
// "entities", "relationships", "claims", and "community".
func (p *Provider) Complete(ctx context.Context, prompt string, opts ...llm.Option) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	lower := strings.ToLower(prompt)

	switch {
	case strings.Contains(lower, "extract") && strings.Contains(lower, "entit"):
		// Entity + relationship extraction. The pipeline parses this
		// JSON via internal/extractor — schema must match exactly.
		// Use stable entity names derived from the prompt's first
		// 200 chars so different input files yield different graphs.
		tag := hashTag(prompt, 2)
		return fmt.Sprintf(`{
  "entities": [
    {"name": "Entity_%s_A", "type": "concept", "description": "deterministic mock entity A for %s"},
    {"name": "Entity_%s_B", "type": "concept", "description": "deterministic mock entity B for %s"}
  ],
  "relationships": [
    {"source": "Entity_%s_A", "target": "Entity_%s_B", "predicate": "relates_to", "description": "mock edge"}
  ]
}`, tag, tag, tag, tag, tag, tag), nil

	case strings.Contains(lower, "claim"):
		tag := hashTag(prompt, 2)
		return fmt.Sprintf(`{
  "claims": [
    {"subject": "Entity_%s_A", "predicate": "is", "object": "mock claim", "description": "deterministic"}
  ]
}`, tag), nil

	case strings.Contains(lower, "community") || strings.Contains(lower, "summar"):
		return "Mock community summary: a deterministic, test-only paragraph describing the community of entities in scope.", nil

	default:
		// Unknown prompt — return empty JSON so whatever caller gets
		// it can proceed without a parse error.
		return `{}`, nil
	}
}

// Embed returns a Dims-length vector derived from SHA-256(text). Equal
// text yields equal vectors; near-text yields wildly different vectors
// (no semantic locality), which is fine for tests because the
// pipeline's cosine-similarity assertions only need non-zero, stable
// output.
func (p *Provider) Embed(ctx context.Context, text string) ([]float32, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return hashEmbedding(text, p.Dims), nil
}

func (p *Provider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := p.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// hashEmbedding derives a stable `dims`-length unit vector from SHA-256.
// The vector is NOT semantically meaningful — it is deterministic noise.
func hashEmbedding(text string, dims int) []float32 {
	sum := sha256.Sum256([]byte(text))
	out := make([]float32, dims)
	// Repeat the 32-byte digest as needed; each 4 bytes → one float32.
	for i := 0; i < dims; i++ {
		base := (i * 4) % 32
		bits := binary.BigEndian.Uint32([]byte{
			sum[base], sum[(base+1)%32], sum[(base+2)%32], sum[(base+3)%32],
		})
		// Map uint32 to [-1, 1] deterministically.
		out[i] = float32(bits)/float32(1<<31) - 1.0
	}
	// Normalise so downstream cosine-similarity math stays in [-1,1].
	var norm float32
	for _, v := range out {
		norm += v * v
	}
	if norm > 0 {
		inv := 1.0 / float32Sqrt(norm)
		for i := range out {
			out[i] *= inv
		}
	}
	return out
}

// hashTag returns the first `n` hex chars of SHA-256(s) — used as a
// stable, short identifier in canned entity names.
func hashTag(s string, n int) string {
	sum := sha256.Sum256([]byte(s))
	const hex = "0123456789abcdef"
	out := make([]byte, n*2)
	for i := 0; i < n; i++ {
		out[2*i] = hex[sum[i]>>4]
		out[2*i+1] = hex[sum[i]&0x0f]
	}
	return string(out)
}

// float32Sqrt keeps the package standard-library-only without importing
// math just for one sqrt call. Newton-Raphson to 16-bit precision is
// ample for normalising an embedding vector.
func float32Sqrt(x float32) float32 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 16; i++ {
		z = (z + x/z) / 2
	}
	return z
}
```

**Note**: if `math.Sqrt` is preferred over the hand-rolled Newton-Raphson, replace `float32Sqrt` with `func float32Sqrt(x float32) float32 { return float32(math.Sqrt(float64(x))) }` and add `"math"` to the imports. Either is fine.

- [ ] **Step 3: Write the testdata markdown corpus**

Create five small, realistic markdown files. They must be small enough that the pipeline finishes quickly under `-race`, but substantive enough that the chunker produces ≥1 chunk per file and entity extraction has text to work with.

`testdata/pipeline/alpha.md`:

```markdown
# Alpha Doc

Alpha is the first document in the fixture corpus. It mentions
Project Apollo, which was a spaceflight program run by NASA from 1961
to 1972. Apollo 11 landed humans on the Moon for the first time.

## Background

The program was led by administrator James Webb, named after the
Greek god Apollo. The Saturn V rocket launched every mission.
```

`testdata/pipeline/beta.md`:

```markdown
# Beta Doc

Beta covers the Apollo program's missions in more detail. Apollo 11,
Apollo 12, and Apollo 13 are the most famous flights. Neil Armstrong
was the commander of Apollo 11 and the first human on the Moon.

Apollo 13 suffered an oxygen tank explosion but returned safely.
```

`testdata/pipeline/gamma.md`:

```markdown
# Gamma Doc

Gamma is about the Saturn V rocket. Saturn V was the largest rocket
ever flown successfully. It was designed by Wernher von Braun and his
team at the Marshall Space Flight Center. The rocket had three
stages.

## Notable Flights

All crewed Apollo missions used Saturn V. Skylab was launched on a
modified Saturn V.
```

`testdata/pipeline/delta.md`:

```markdown
# Delta Doc

Delta is a short document about unrelated topics. The moon is a
celestial body orbiting Earth. The Earth orbits the Sun. Neither is
particularly relevant to the Apollo program except by coincidence.
```

`testdata/pipeline/epsilon.md`:

```markdown
# Epsilon Doc

Epsilon is the last fixture. It mentions James Webb again — the
James Webb Space Telescope was named after the Apollo-era NASA
administrator.
```

Total corpus: ~40 lines, 5 files. Chunker produces ~5–10 chunks. Entity extraction returns two mock entities per file = ~10 entities, plus relationships = ~5 edges.

- [ ] **Step 4: Write the integration test**

Create `internal/pipeline/integration_test.go`:

```go
//go:build integration && sqlite_fts5

package pipeline_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/embedder"
	"github.com/RandomCodeSpace/docsiq/internal/llm/mock"
	"github.com/RandomCodeSpace/docsiq/internal/pipeline"
	"github.com/RandomCodeSpace/docsiq/internal/search"
	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// TestPipeline_IndexAndSearch_EndToEnd drives pipeline.New().IndexPath()
// followed by Finalize() against a 5-file markdown corpus using a
// deterministic mock LLM provider, then asserts:
//   - SQLite documents, chunks, embeddings row counts are in the
//     expected bands,
//   - entity / relationship counts reflect mock extraction,
//   - a LocalSearch for a known substring returns ≥1 hit from the
//     correct document.
//
// Runs under the integration build tag so it stays out of the default
// `go test ./...` path; the CI test-integration job runs it with -race.
func TestPipeline_IndexAndSearch_EndToEnd(t *testing.T) {
	t.Parallel()

	// 1. Temp dir for the SQLite DB.
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "docsiq.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	// 2. Resolve the testdata corpus relative to this test file.
	corpus, err := filepath.Abs(filepath.Join("..", "..", "testdata", "pipeline"))
	if err != nil {
		t.Fatalf("resolve corpus: %v", err)
	}

	// 3. Build a minimal config. Values match cmd/index.go defaults
	// except batch size and worker count, which we clamp low to keep
	// wall-clock predictable under -race.
	cfg := &config.Config{
		DataDir: dbDir,
		LLM: config.LLMConfig{
			Provider: "mock",
		},
		Indexing: config.IndexingConfig{
			BatchSize:    4,
			ChunkSize:    512,
			ChunkOverlap: 64,
		},
		Embedding: config.EmbeddingConfig{
			Dims: 128,
		},
	}

	// 4. Use the mock provider — no network, no API keys, fully
	// deterministic. We bypass llm.NewProvider (which would dispatch
	// on cfg.LLM.Provider) and inject directly.
	prov := mock.New(cfg.Embedding.Dims)
	_ = embedder.New(prov, cfg.Indexing.BatchSize) // sanity; pipeline.New builds its own internally.

	pl := pipeline.New(st, prov, cfg)

	// 5. Drive the indexer with a 60s deadline — plenty for 5 small
	// markdown files and forces the test to fail loud on any deadlock
	// regression in the pipeline's goroutine fanout.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := pl.IndexPath(ctx, corpus, pipeline.IndexOptions{
		Workers: 2,
		Verbose: false,
	}); err != nil {
		t.Fatalf("IndexPath: %v", err)
	}

	// 6. Run Finalize (Phases 3-4: community detection + summaries).
	// The mock provider returns a canned summary per community, so
	// this must complete without errors even though no real LLM is
	// attached.
	if err := pl.Finalize(ctx, false); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	// 7. Row-count assertions — exact values are brittle against
	// chunker-tuning drift; assert bands that any reasonable chunking
	// of the 5-file corpus satisfies.
	docCount, err := st.CountDocuments(ctx)
	if err != nil {
		t.Fatalf("CountDocuments: %v", err)
	}
	if docCount != 5 {
		t.Errorf("document count: want 5, got %d", docCount)
	}

	chunkCount, err := st.CountChunks(ctx)
	if err != nil {
		t.Fatalf("CountChunks: %v", err)
	}
	if chunkCount < 5 || chunkCount > 50 {
		t.Errorf("chunk count: want 5..50, got %d", chunkCount)
	}

	embCount, err := st.CountEmbeddings(ctx)
	if err != nil {
		t.Fatalf("CountEmbeddings: %v", err)
	}
	if embCount != chunkCount {
		t.Errorf("embeddings count (%d) != chunks count (%d) — Phase 2 drift",
			embCount, chunkCount)
	}

	// Mock returns 2 entities per extraction prompt; extractor runs
	// 1+ times per document (depending on chunk count). The lower
	// bound is 2 (dedup collapses everything to a single tag across
	// all files — very unlikely); upper bound is 2 × chunkCount.
	entityCount, err := st.CountEntities(ctx)
	if err != nil {
		t.Fatalf("CountEntities: %v", err)
	}
	if entityCount < 2 || entityCount > 2*chunkCount {
		t.Errorf("entity count: want 2..%d, got %d", 2*chunkCount, entityCount)
	}

	relCount, err := st.CountRelationships(ctx)
	if err != nil {
		t.Fatalf("CountRelationships: %v", err)
	}
	if relCount < 1 {
		t.Errorf("relationship count: want ≥1, got %d", relCount)
	}

	// 8. Search assertion: the word "Apollo" appears in alpha.md,
	// beta.md, gamma.md, and epsilon.md. LocalSearch must return at
	// least one chunk, and at least one hit's content must contain
	// "Apollo" (case-insensitive).
	emb := embedder.New(prov, cfg.Indexing.BatchSize)
	res, err := search.LocalSearch(ctx, st, emb, nil, "Apollo program", 5, 1)
	if err != nil {
		t.Fatalf("LocalSearch: %v", err)
	}
	if len(res.Chunks) == 0 {
		t.Fatalf("LocalSearch returned 0 chunks — mock embedding + chunk store disconnected")
	}
	// At least one result must come from a doc whose content mentions
	// "Apollo". With hash-based embeddings we can't assert semantic
	// ranking, but we CAN assert the chunks themselves are present.
	var foundApollo bool
	for _, c := range res.Chunks {
		if containsFold(c.Chunk.Content, "Apollo") {
			foundApollo = true
			break
		}
	}
	if !foundApollo {
		t.Errorf("none of the top-5 chunks mention Apollo; got %d chunks", len(res.Chunks))
	}
}

// containsFold is case-insensitive substring match without importing
// strings just for this one call. Inline for test-local clarity.
func containsFold(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(haystack) < len(needle) {
		return false
	}
	lh := toLowerAscii(haystack)
	ln := toLowerAscii(needle)
	for i := 0; i+len(ln) <= len(lh); i++ {
		if lh[i:i+len(ln)] == ln {
			return true
		}
	}
	return false
}

func toLowerAscii(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
```

**Note on helper functions**: the `containsFold` helper re-implements `strings.EqualFold`-style ASCII-lower matching to keep the test file with minimal imports — this is purely stylistic. If you prefer `strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))`, that is equivalent.

**Note on store helper APIs**: the test assumes `CountDocuments`, `CountChunks`, `CountEmbeddings`, `CountEntities`, `CountRelationships` exist on `*store.Store`. Verify via:

```bash
grep -n 'func (s \*Store) Count' internal/store/*.go
```

Expected: all five methods exist. If any is missing, add a trivial one before running the test — e.g.

```go
func (s *Store) CountDocuments(ctx context.Context) (int, error) {
    var n int
    err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM documents WHERE is_latest = 1`).Scan(&n)
    return n, err
}
```

and corresponding versions for `chunks`, `embeddings`, `entities`, `relationships`. Add these to `internal/store/counts.go` as a small dedicated file; they are useful outside tests too (stats reporting).

- [ ] **Step 5: Run the test locally**

```bash
CGO_ENABLED=1 go test -tags "sqlite_fts5 integration" -race -timeout 120s -v \
  -run TestPipeline_IndexAndSearch_EndToEnd ./internal/pipeline/
```

Expected output (abbreviated):

```
=== RUN   TestPipeline_IndexAndSearch_EndToEnd
=== PAUSE TestPipeline_IndexAndSearch_EndToEnd
=== CONT  TestPipeline_IndexAndSearch_EndToEnd
... [slog output from pipeline phases] ...
--- PASS: TestPipeline_IndexAndSearch_EndToEnd (3.42s)
PASS
ok   github.com/RandomCodeSpace/docsiq/internal/pipeline 3.580s
```

Failure modes and their diagnoses:

- **`ctx.Err() timeout`**: the pipeline is deadlocked or the mock provider is too slow — check that `hashEmbedding` is O(dims) not O(dims²). 60s deadline is ample.
- **`store.Open: no such module: fts5`**: the test was run without `-tags sqlite_fts5`. Add the tag and retry.
- **`document count: want 5, got 0`**: `IndexPath` returned nil-error but didn't index anything. Usually means the corpus path is wrong — `filepath.Abs` in Step 4 resolves relative to the test binary's cwd, which is the *package directory* (`internal/pipeline/`), so `../../testdata/pipeline` should be correct. Verify with `ls` from inside `internal/pipeline/`.
- **`embeddings count != chunks count`**: Phase 2 (embedding) silently swallowed an error. Re-run with `Verbose: true` in `IndexOptions` and inspect the slog output.
- **`entity count < 2`**: the mock `Complete` returned JSON the extractor couldn't parse. Dump the prompt + response via a `t.Logf` inside the mock when `testing.Verbose()` is true, compare to `internal/extractor/*.go` expected schema.
- **`LocalSearch returned 0 chunks`**: the vector index is empty or `AllChunkEmbeddings` returns nil. Verify via `st.CountEmbeddings` earlier in the test — should be the same number.

- [ ] **Step 6: Add the test to the `test-integration` job's explicit path (sanity check)**

The `test-integration` job in `.github/workflows/ci.yml` already runs:

```yaml
- name: integration tests
  run: CGO_ENABLED=1 go test -tags "sqlite_fts5 integration" -race -timeout 1200s ./...
```

`./...` already picks up `internal/pipeline/integration_test.go` via its `//go:build integration && sqlite_fts5` tag. **No workflow edit is needed.** Verify by reading the job and confirming the run command; do not duplicate the invocation.

- [ ] **Step 7: Commit**

Use two commits — mock provider first (reusable infra), integration test second — so reverting one doesn't drag the other:

```bash
# Commit 1: mock provider + counts helpers (if Step 4 required them)
git add internal/llm/mock/mock.go
# If you added counts.go in Step 4:
# git add internal/store/counts.go
git commit -m "$(cat <<'EOF'
feat(llm): deterministic mock provider for tests

internal/llm/mock implements llm.Provider with:

- Complete: substring-matched canned JSON for entity/relationship/claim
  extraction prompts and a fixed summary for community prompts.
- Embed/EmbedBatch: SHA-256-derived unit vectors, 128-dim by default.
  Equal text yields equal vectors; determinism is the only semantic
  contract.

Intended for integration tests; not exposed outside internal/. No
network, no API keys, no external processes.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"

# Commit 2: corpus + integration test
git add testdata/pipeline/ internal/pipeline/integration_test.go
git commit -m "$(cat <<'EOF'
test(pipeline): end-to-end integration test over markdown corpus

New integration test drives pipeline.New().IndexPath().Finalize() over
5 small markdown files with the mock LLM provider, then asserts:

- Document count is exactly 5.
- Chunk count is in the 5..50 band.
- Embedding count equals chunk count (Phase 2 invariant).
- Entity count is in the 2..2*chunks band (mock returns 2 entities
  per extraction prompt).
- Relationship count is ≥1.
- LocalSearch("Apollo program", topK=5) returns ≥1 chunk containing
  "Apollo".

Gated by //go:build integration && sqlite_fts5 so the default
`go test ./...` path is unaffected. The test-integration CI job picks
it up automatically via its existing `-tags "sqlite_fts5 integration"`
invocation — no workflow change needed.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

**Expected outcome:** The `test-integration` job (already 1200s budget) gains ~4s of real integration coverage. Any future refactor that breaks the chunker→embedder→extractor→Finalize pipeline fails CI within ~5 minutes of PR open, instead of being caught at first user report.

---

## Self-Review

### Scope discipline

- **Task 1** (govulncheck): one new step in one job. No repo-code changes.
- **Task 2** (npm audit): one new step in one job. Lockfile churn only if Step 1 found outstanding advisories.
- **Task 3** (fuzz): two new `_fuzz_test.go` files, one 2-line edit to `fuzz.yml`. No production-code changes unless the fuzz target surfaces a bug — in which case the fix ships in a separate commit (plan explicitly says so).
- **Task 4** (flake register): annotations on 8 existing `t.Skip` sites + one new bash step. No production-code changes.
- **Task 5** (Playwright smokes): three new `.spec.ts` files + one 3-line fixture markdown. No UI source changes. If an asserted affordance is missing, the plan explicitly says "raise a follow-up, do not relax the assertion".
- **Task 6** (pipeline integration): new mock package + new testdata + new `_test.go` file. Optional new `counts.go` in `internal/store` if the helpers don't exist. No production-code changes beyond those small helpers.

Each task ships ≤ 3 commits. No task silently touches anything outside its stated files.

### Build/test invariants preserved

- All Go changes respect `-tags sqlite_fts5` (required for SQLite FTS5 driver) and `-tags integration` (for Task 6). Every `_test.go` file carries the correct build tag.
- No dependency adds. `govulncheck` is installed in-job, not vendored. `golang.org/x/vuln/cmd/govulncheck@latest` is a first-party module — acceptable per dependency rules.
- CI job wall-clock budget impact:
  - `test` job: +20s (govulncheck), +negligible (flake-register grep). New total: ~4-5 min.
  - `ui` job: +2s (npm audit). Negligible.
  - `fuzz` job: +60s (2 new targets × 30s). New total: ~2 min.
  - `playwright` job: +15s (5 new tests). New total: ~1.5 min.
  - `test-integration`: +4s (one new test). Negligible.
  - Aggregate: ~2 min added to total PR CI wall-clock. Within soft budget.

### Security posture

- govulncheck gate aligns with `~/.claude/rules/security.md` High/Critical-as-block policy.
- npm audit gate matches the `--audit-level=moderate` floor.
- Both gates fix-before-wire: no silent suppression.
- Flake register converts silent state into tracked work — direct improvement to "measurable quality" axis.
- Mock LLM provider has no secrets, no network, no side effects. Cannot leak credentials because it has none.

### Determinism / flake surface

- Task 6's mock provider is fully deterministic (SHA-256 of input → vector; substring match on prompt → canned response). Equal input → equal output, every time.
- Task 5's Playwright smokes use the existing `stubbedPage` fixture pattern or clearly-scoped `route.fulfill` stubs. No timing-sensitive assertions beyond a 5s `toBeVisible` timeout — well above any reasonable latency budget.
- Task 3's fuzz targets run for 30s, which GitHub Actions provides with < 1% jitter. Targets themselves are pure (no I/O beyond SQLite in-memory).
- Task 4's grep gate is data-only (reads source files, no network, no timing). Zero flake surface.

### Rollback

If any task reveals a deeper problem at implementation time:

- **Task 1 / 2 gates found vulns we can't immediately fix**: commit the gate as `.disabled` (rename the step) or gate it on `github.event_name == 'pull_request_target'` only. File an issue. Proceed with the next task. Do NOT silently soften the threshold.
- **Task 3 fuzz found a real bug**: commit the target AFTER the fix. Target commit is shippable; fix commit is the actual improvement. Two small commits > one big one.
- **Task 5 Playwright smoke fails because the UI behaviour doesn't match the assumed contract**: file a follow-up ticket, mark the spec `.fixme` with `// TODO(#N):` per Task 4's convention. Do not ship a permanently-skipped test.
- **Task 6 integration test can't reach a stable row-count band**: widen bands to ~10x of the observed value, re-run 10 times under `-race`, assert the band covers the p99 observation. Prefer a wide band that never flakes over a narrow band that flakes weekly.

### Type consistency

- `llm.Provider` — all tasks use the existing interface from `internal/llm/provider.go`. `mock.Provider` is compile-time-checked via `var _ llm.Provider = (*Provider)(nil)`.
- `testing.TB` — `newTestStore(t testing.TB)` accepts both `*testing.T` and `*testing.F` without needing two helpers.
- `pipeline.IndexOptions` — the integration test uses only the documented public fields (`Workers`, `Verbose`). No internal-only fields.
- Build tag composition — every new test file uses the exact tag combo that already exists in the repo: `sqlite_fts5` for unit, `integration && sqlite_fts5` for integration.

### Placeholder check

No `TBD`, no unresolved `TODO`, no "similar to", no "add appropriate error handling", no "update as needed" in the plan body. Every code step has full, runnable code. Commands are exact. Fuzz `fuzztime` budgets are exact. CI step YAML is exact.

Known site-specific ambiguities are explicitly called out:
- Task 5's regex breadth on UI copy — documented with the instruction to tighten after first run.
- Task 4's issue numbers — implementer fills them in at `gh issue create` time; the plan documents the format (`// TODO(#N):`) unambiguously.
- Task 6's `CountDocuments` etc. helpers — plan documents verification command and provides the implementation pattern if absent.

These are unavoidable without the plan author running the full indexer + UI at plan time; the verification commands make the ambiguities cheap to resolve.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-04-23-block6-testing-ci-plan.md`.**

Two execution options:

1. **Subagent-Driven (recommended)** — fresh subagent per task (6 subagents), code-review between each. Tasks are independent so they can run in parallel if the orchestrator prefers.
2. **Inline Execution** — this session drives all six tasks sequentially with checkpoints between each. Shorter wall-clock, no review gate.

Tasks 1, 2, 4 are ≤ S each and can reasonably be clustered into a single "CI gates" subagent. Tasks 3, 5, 6 each warrant their own subagent.

Which approach?
