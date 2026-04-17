# docsiq Quality Sweep — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close all correctness, test-coverage, and documentation gaps from the kgraph→docsiq port without expanding feature scope. Result: a product that is correct and defensible, not larger.

**Architecture:** Six-wave execution — four parallel (review, frontend tests, integration tests, hooks+MCP+docs) followed by two serial (review-remediation, lint sweep) and final verification. File-isolation contracts prevent merge conflicts; severity rubric (P0/P1/P2) bounds remediation scope.

**Tech Stack:** Go 1.22 (CGO + `sqlite_fts5`), `mattn/go-sqlite3`, `modernc`-free; React 19 + Vite 7 + Vitest + @testing-library/react; `httptest` + `go.uber.org/goleak` for Go integration.

**Spec:** `docs/superpowers/specs/2026-04-17-quality-sweep-design.md`

---

## File Structure

**Created:**
- `REVIEW.md` (root) — Wave A output, consumed by Wave E
- `ui/vitest.config.ts`, `ui/src/setupTests.ts` — Wave B
- `ui/src/**/__tests__/*.test.tsx` — Wave B (~12 files)
- `internal/api/itest/harness.go`, `internal/api/itest/doubles.go` — Wave C
- `internal/**/*_integration_test.go` — Wave C (10 files)
- `internal/hookinstaller/fixtures/**/{before,after}.json` — Wave D1
- `docs/README.md`, `docs/getting-started.md`, `docs/cli-reference.md`, `docs/mcp-tools.md`, `docs/rest-api.md`, `docs/config.md`, `docs/hooks.md`, `docs/architecture.md` — Wave D4
- `/home/dev/projects/docsiq/kgraph/ARCHIVED.md` — Wave D3 (sibling repo)

**Modified:**
- `ui/package.json` — Wave B (add vitest + RTL devDeps, test scripts)
- `Makefile` — Wave C (add `test-integration` target), Wave B (add `test-ui` target)
- `.github/workflows/ci.yml` — Wave B (add vitest job), Wave C (add integration-test job)
- `internal/hookinstaller/{claude,cursor,copilot,codex}.go` — Wave D1
- `internal/mcp/tools.go` — Wave D2 (global_search per-project LLM)
- `CLAUDE.md` — Wave D3 (rebrand)
- `/home/dev/projects/docsiq/UNIFICATION-PLAN.md` — Wave D3 (rebrand + status banner)
- `README.md` — Wave D1 (add hook support matrix)

---

## Wave A — Code Review (single task)

### Task A1: Dispatch code reviewer

**Files:**
- Create: `/home/dev/projects/docsiq/docscontext/REVIEW.md`

- [ ] **Step 1: Dispatch `feature-dev:code-reviewer` agent**

Use this exact prompt:

```
You are doing a comprehensive code review of the docsiq repo at
/home/dev/projects/docsiq/docscontext/ — 7 feature commits since
the port of kgraph into Go (commits 3d2d2ce..790810f on main,
~12,000 LoC).

Do NOT fix anything. Write findings to REVIEW.md at repo root.

Review axes (mandatory checklist):
1. Correctness — races (projectStores cache, VectorIndexes map,
   note auto-commit mutexes), silent error-drop, FTS5 snippet/rank
   off-by-one, goroutine lifecycle on shutdown
2. Security — auth bypass, path traversal (note keys, tar import,
   hook installer paths), token logging, CSRF on REST
3. Concurrency — cache races, concurrent note writers, shutdown
   ordering
4. Resource leaks — unclosed sql.DB/sql.Rows/readers, tmpdir
   cleanup, goroutine leaks in upload-progress
5. API contract — HTTP status codes, JSON shapes, MCP tool arg
   validation
6. Test coverage gaps — paths not exercised by 424 existing subtests
7. Consistency — style drift, duplicated logic, naming

Severity rubric:
- P0 — bugs, data-loss, auth bypass, races → must fix
- P1 — correctness gaps, wrong error handling, misleading docs
- P2 — style, refactor opportunities, defer-OK

Output REVIEW.md with this structure:

  # Code Review — docsiq quality sweep
  ## Summary
  - N findings total: X P0 / Y P1 / Z P2
  - Packages audited: [list]
  - Lines audited: ~12k
  ## P0 — must fix
  ### [P0-1] <title> — <file:line>
  **What:** one-sentence description
  **Impact:** concrete consequence
  **Evidence:** code excerpt or test scenario
  **Recommended fix:** approach (not code)
  ## P1 — should fix ... same shape ...
  ## P2 — nice to have / defer ... same shape ...
  ## What looks good
  <intentional non-findings to confirm reviewer saw something and
   decided it was OK>

Key files to audit in depth:
- internal/api/stores.go (per-project store cache — race hotspot)
- internal/api/vector_indexes.go (per-project HNSW cache)
- internal/notes/notes.go + history.go (per-project mutex + git exec)
- internal/notes/graph.go (walks disk every call)
- internal/hookinstaller/*.go (cross-client JSON merge atomicity)
- internal/mcp/tools.go + notes_tools.go (arg validation)
- internal/api/auth.go (scheme parsing, constant-time compare)
- internal/api/router.go (middleware ordering)
- cmd/serve.go (shutdown ordering, registry+stores lifecycle)
- internal/store/store.go (DSN pragmas, schema migrations)
- internal/vectorindex/hnsw.go (concurrent Add/Search safety)
- internal/sqlitevec/load.go (extension loading error handling)

Do NOT modify any *.go files. Only write REVIEW.md.
```

- [ ] **Step 2: Verify REVIEW.md exists and has findings**

Run: `ls -la REVIEW.md && head -20 REVIEW.md`
Expected: file exists, "# Code Review — docsiq quality sweep" header present, Summary section populated.

- [ ] **Step 3: Commit**

```bash
git add REVIEW.md
git commit -m "docs: add Wave A code review findings"
```

---

## Wave B — Frontend Tests

### Task B1: Install vitest + testing-library stack

**Files:**
- Modify: `ui/package.json` (devDependencies + scripts)
- Create: `ui/vitest.config.ts`
- Create: `ui/src/setupTests.ts`

- [ ] **Step 1: Add devDeps**

```bash
cd ui && npm install --save-dev \
  vitest@^2.1 \
  @vitest/coverage-v8@^2.1 \
  @testing-library/react@^16 \
  @testing-library/user-event@^14 \
  @testing-library/jest-dom@^6 \
  jsdom@^25
```

- [ ] **Step 2: Create `ui/vitest.config.ts`**

```ts
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/setupTests.ts"],
    coverage: {
      reporter: ["text", "html"],
      include: [
        "src/components/notes/**",
        "src/components/nav/**",
        "src/components/shared/**",
        "src/hooks/**",
      ],
      thresholds: {
        statements: 70,
        branches: 60,
      },
    },
  },
});
```

- [ ] **Step 3: Create `ui/src/setupTests.ts`**

```ts
import "@testing-library/jest-dom";
```

- [ ] **Step 4: Add scripts to `ui/package.json`**

Add to the "scripts" object:
```json
"test": "vitest run",
"test:watch": "vitest",
"test:coverage": "vitest run --coverage"
```

- [ ] **Step 5: Smoke-test the pipeline**

```bash
cd ui && npm test -- --run src/setupTests.ts 2>&1 | head -10
```
Expected: "no test files" is fine; what matters is `vitest` launches without config errors. If error, fix before proceeding.

- [ ] **Step 6: Commit**

```bash
git add ui/package.json ui/package-lock.json ui/vitest.config.ts ui/src/setupTests.ts
git commit -m "test: add vitest + testing-library stack for ui"
```

---

### Task B2: FolderTree component tests

**Files:**
- Create: `ui/src/components/notes/__tests__/FolderTree.test.tsx`

- [ ] **Step 1: Write the failing test file**

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FolderTree } from "../FolderTree";

const sampleTree = {
  name: "",
  children: [
    { name: "architecture", children: [{ name: "auth.md", key: "architecture/auth" }] },
    { name: "intro.md", key: "intro" },
  ],
};

describe("FolderTree", () => {
  it("renders folder and file nodes", () => {
    render(<FolderTree tree={sampleTree} onSelect={() => {}} onCreate={() => {}} />);
    expect(screen.getByText("architecture")).toBeInTheDocument();
    expect(screen.getByText("intro.md")).toBeInTheDocument();
  });

  it("click on a file calls onSelect with key", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<FolderTree tree={sampleTree} onSelect={onSelect} onCreate={() => {}} />);
    await user.click(screen.getByText("intro.md"));
    expect(onSelect).toHaveBeenCalledWith("intro");
  });

  it("+ button opens the create-note modal", async () => {
    const user = userEvent.setup();
    render(<FolderTree tree={sampleTree} onSelect={() => {}} onCreate={() => {}} />);
    await user.click(screen.getByRole("button", { name: /new note/i }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("rejects invalid keys inline", async () => {
    const user = userEvent.setup();
    const onCreate = vi.fn();
    render(<FolderTree tree={sampleTree} onSelect={() => {}} onCreate={onCreate} />);
    await user.click(screen.getByRole("button", { name: /new note/i }));
    await user.type(screen.getByLabelText(/key/i), "../escape");
    await user.click(screen.getByRole("button", { name: /create/i }));
    expect(screen.getByText(/invalid/i)).toBeInTheDocument();
    expect(onCreate).not.toHaveBeenCalled();
  });

  it("Escape closes the modal without creating", async () => {
    const user = userEvent.setup();
    const onCreate = vi.fn();
    render(<FolderTree tree={sampleTree} onSelect={() => {}} onCreate={onCreate} />);
    await user.click(screen.getByRole("button", { name: /new note/i }));
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(onCreate).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run tests; adapt to actual component props if they differ**

```bash
cd ui && npm test -- --run src/components/notes/__tests__/FolderTree.test.tsx
```
Expected: 5 tests run. If any fails because the component's prop names / ARIA labels differ from the test's assumptions, inspect `src/components/notes/FolderTree.tsx` and adjust the test queries. Do NOT change component source in this task.

- [ ] **Step 3: Commit**

```bash
git add ui/src/components/notes/__tests__/FolderTree.test.tsx
git commit -m "test(ui): FolderTree component tests"
```

---

### Task B3: NoteView markdown renderer tests

**Files:**
- Create: `ui/src/components/notes/__tests__/NoteView.test.tsx`

- [ ] **Step 1: Write the test file**

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NoteView } from "../NoteView";

describe("NoteView markdown", () => {
  it("renders headings", () => {
    render(<NoteView note={{ key: "k", content: "# Hello\n\n## World" }} onNavigate={() => {}} />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Hello");
    expect(screen.getByRole("heading", { level: 2 })).toHaveTextContent("World");
  });

  it("renders bold, italic, code", () => {
    render(<NoteView note={{ key: "k", content: "**bold** *italic* `code`" }} onNavigate={() => {}} />);
    expect(screen.getByText("bold").tagName).toBe("STRONG");
    expect(screen.getByText("italic").tagName).toBe("EM");
    expect(screen.getByText("code").tagName).toBe("CODE");
  });

  it("renders wikilinks clickable", async () => {
    const user = userEvent.setup();
    const onNavigate = vi.fn();
    render(<NoteView note={{ key: "k", content: "See [[target]]." }} onNavigate={onNavigate} />);
    const link = screen.getByRole("link", { name: "target" });
    await user.click(link);
    expect(onNavigate).toHaveBeenCalledWith("target");
  });

  it("renders external markdown links with noopener", () => {
    render(<NoteView note={{ key: "k", content: "[docsiq](https://example.com)" }} onNavigate={() => {}} />);
    const link = screen.getByRole("link", { name: "docsiq" }) as HTMLAnchorElement;
    expect(link.href).toBe("https://example.com/");
    expect(link.target).toBe("_blank");
    expect(link.rel).toMatch(/noopener/);
  });

  it("renders images with lazy loading", () => {
    const { container } = render(<NoteView note={{ key: "k", content: "![alt](/pic.png)" }} onNavigate={() => {}} />);
    const img = container.querySelector("img");
    expect(img?.loading).toBe("lazy");
    expect(img?.alt).toBe("alt");
  });

  it("renders blockquotes", () => {
    const { container } = render(<NoteView note={{ key: "k", content: "> quoted line\n> more" }} onNavigate={() => {}} />);
    expect(container.querySelector("blockquote")).toBeInTheDocument();
  });

  it("renders GitHub tables", () => {
    const md = "| a | b |\n|---|---|\n| 1 | 2 |";
    const { container } = render(<NoteView note={{ key: "k", content: md }} onNavigate={() => {}} />);
    expect(container.querySelector("table")).toBeInTheDocument();
    expect(container.querySelectorAll("td")).toHaveLength(2);
  });

  it("renders horizontal rule", () => {
    const { container } = render(<NoteView note={{ key: "k", content: "a\n\n---\n\nb" }} onNavigate={() => {}} />);
    expect(container.querySelector("hr")).toBeInTheDocument();
  });

  it("strips frontmatter from display", () => {
    const content = "---\ntitle: Hidden\n---\n\nVisible body";
    render(<NoteView note={{ key: "k", content }} onNavigate={() => {}} />);
    expect(screen.queryByText("title: Hidden")).not.toBeInTheDocument();
    expect(screen.getByText("Visible body")).toBeInTheDocument();
  });

  it("handles empty body", () => {
    const { container } = render(<NoteView note={{ key: "k", content: "" }} onNavigate={() => {}} />);
    expect(container.firstChild).toBeTruthy();
  });
});
```

- [ ] **Step 2: Run tests**

```bash
cd ui && npm test -- --run src/components/notes/__tests__/NoteView.test.tsx
```
Expected: 10 tests. Any unexpected failures → inspect NoteView source, adjust test queries to match actual DOM. Record unexpected renderer behaviour as a potential P1 finding (note in commit message if so).

- [ ] **Step 3: Commit**

```bash
git add ui/src/components/notes/__tests__/NoteView.test.tsx
git commit -m "test(ui): NoteView markdown rendering"
```

---

### Task B4: NoteEditor, LinkPanel, NotesGraphView, NotesSearchPanel, UnifiedSearchPanel, TopNav, App tests

Each gets its own `__tests__/*.test.tsx` file with the test cases from spec §4 table. For each component:

**Files per component:**
- Create: `ui/src/components/<path>/__tests__/<Component>.test.tsx`

Pattern (apply per component):

- [ ] **Step 1: Read the component source**
  ```bash
  cat ui/src/components/<path>/<Component>.tsx
  ```
  Note: actual prop names and ARIA labels to query by.

- [ ] **Step 2: Write the test file** covering every bullet in the spec §4 row for this component. Include at minimum: basic render, interaction that calls a callback, empty / loading / error states where applicable.

- [ ] **Step 3: Run tests**
  ```bash
  cd ui && npm test -- --run src/components/<path>/__tests__/<Component>.test.tsx
  ```

- [ ] **Step 4: Commit**
  ```bash
  git add ui/src/components/<path>/__tests__/<Component>.test.tsx
  git commit -m "test(ui): <Component> component tests"
  ```

Components to cover in this task (one commit per component):
- [ ] `NoteEditor` — body update, tag input parse, save → writeNote, dirty-flag warn
- [ ] `LinkPanel` — inbound/outbound lists, empty state, click navigates
- [ ] `NotesGraphView` — N nodes render, empty state, note-accent class applied
- [ ] `NotesSearchPanel` — input debounce, results render, snippet highlight, click navigates, count+ms shown
- [ ] `shared/UnifiedSearchPanel` — fires both endpoints in parallel, merges labels, empty results state
- [ ] `nav/TopNav` — tabs + project selector, URL sync on change
- [ ] `App` (top-level) — initial tab from URL, tab switch updates URL, project switch reloads hooks

---

### Task B5: Hook tests (useNotes, useProjects, useNotesSearch, useNotesGraph, useNotesTree)

**Files:**
- Create: `ui/src/hooks/__tests__/useNotes.test.tsx` and siblings

- [ ] **Step 1: Set up fetch mocking**

Add to top of each hook test file:
```tsx
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";

beforeEach(() => {
  global.fetch = vi.fn();
});
afterEach(() => {
  vi.restoreAllMocks();
});

function mockFetch(status: number, body: unknown) {
  (global.fetch as any).mockResolvedValueOnce({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
    text: async () => JSON.stringify(body),
  });
}
```

- [ ] **Step 2: `useNotes.test.tsx`**

```tsx
import { useNotes, writeNote, deleteNote } from "../useNotes";

describe("useNotes", () => {
  it("calls /api/projects/:p/notes and returns list", async () => {
    mockFetch(200, [{ key: "a" }, { key: "b" }]);
    const { result } = renderHook(() => useNotes("_default"));
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/projects/_default/notes"),
      expect.any(Object),
    );
    expect(result.current.notes).toHaveLength(2);
  });

  it("exposes error on 500", async () => {
    mockFetch(500, { error: "boom" });
    const { result } = renderHook(() => useNotes("_default"));
    await waitFor(() => expect(result.current.error).toBeTruthy());
  });
});

describe("writeNote", () => {
  it("PUTs content to /api/projects/:p/notes/:key", async () => {
    mockFetch(200, { key: "x" });
    await writeNote("_default", "x", "body", "me", ["t"]);
    const call = (global.fetch as any).mock.calls[0];
    expect(call[0]).toMatch(/\/api\/projects\/_default\/notes\/x/);
    expect(call[1].method).toBe("PUT");
    expect(JSON.parse(call[1].body)).toMatchObject({
      content: "body", author: "me", tags: ["t"],
    });
  });

  it("rejects empty content locally (no PUT)", async () => {
    await expect(writeNote("_default", "x", "", undefined, [])).rejects.toThrow();
    expect(global.fetch).not.toHaveBeenCalled();
  });
});

describe("deleteNote", () => {
  it("DELETEs the note", async () => {
    mockFetch(204, null);
    await deleteNote("_default", "x");
    expect((global.fetch as any).mock.calls[0][1].method).toBe("DELETE");
  });
});
```

- [ ] **Step 3: Run** — `cd ui && npm test -- --run src/hooks/__tests__/useNotes.test.tsx`. All pass.

- [ ] **Step 4: Commit.**

- [ ] Repeat Steps 2-4 for each hook: `useProjects`, `useNotesSearch`, `useNotesGraph`, `useNotesTree`.

---

### Task B6: Verify coverage floor

- [ ] **Step 1: Run coverage**

```bash
cd ui && npm run test:coverage
```
Expected: statements ≥ 70% on `components/{notes,nav,shared}/**` and `hooks/**`; branches ≥ 60%. If below threshold, identify uncovered files, add targeted tests.

- [ ] **Step 2: Wire into Makefile**

Edit `Makefile`, add target:
```
test-ui:
	cd ui && npm test -- --run

test-ui-coverage:
	cd ui && npm run test:coverage
```
Add `test-ui` to the `check` target's prerequisites.

- [ ] **Step 3: Wire into ci.yml**

Edit `.github/workflows/ci.yml`, add inside the `ui-freshness` job (after the `npm run build` step):
```yaml
      - name: vitest
        run: npm --prefix ui test -- --run --coverage
```

- [ ] **Step 4: Commit**

```bash
git add Makefile .github/workflows/ci.yml
git commit -m "ci: wire vitest into Makefile + CI workflow"
```

---

## Wave C — Integration Tests

### Task C1: Integration harness

**Files:**
- Create: `internal/api/itest/harness.go`
- Create: `internal/api/itest/doubles.go`

- [ ] **Step 1: Add `go.uber.org/goleak`**

```bash
CGO_ENABLED=1 go get go.uber.org/goleak@latest
CGO_ENABLED=1 go mod tidy
```

- [ ] **Step 2: Write `internal/api/itest/harness.go`**

```go
//go:build integration

package itest

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/api"
	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/embedder"
	"github.com/RandomCodeSpace/docsiq/internal/llm"
	"github.com/RandomCodeSpace/docsiq/internal/project"
)

type Env struct {
	Server   *httptest.Server
	DataDir  string
	Registry *project.Registry
	Stores   *api.ProjectStores
	APIKey   string
	Client   *http.Client
}

func New(t *testing.T) *Env {
	t.Helper()
	dir := t.TempDir()
	reg, err := project.OpenRegistry(dir)
	if err != nil {
		t.Fatalf("OpenRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	stores := api.NewProjectStores(dir)
	t.Cleanup(func() { stores.CloseAll() })

	keyBytes := make([]byte, 16)
	_, _ = rand.Read(keyBytes)
	apiKey := hex.EncodeToString(keyBytes)

	cfg := &config.Config{DataDir: dir, DefaultProject: "_default"}
	cfg.Server.APIKey = apiKey

	prov := &FakeProvider{}
	emb := embedder.New(prov, 4)

	handler := api.NewRouter(prov, emb, cfg, reg,
		api.WithProjectStores(stores),
	)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return &Env{
		Server:   srv,
		DataDir:  dir,
		Registry: reg,
		Stores:   stores,
		APIKey:   apiKey,
		Client:   srv.Client(),
	}
}

func (e *Env) authReq(method, path string, body []byte) *http.Request {
	req, _ := http.NewRequest(method, e.Server.URL+path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+e.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func (e *Env) PUTNote(t *testing.T, project, key, content string, tags []string) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"content": content, "author": "tester", "tags": tags,
	})
	req := e.authReq("PUT", "/api/projects/"+project+"/notes/"+key+"?project="+project, body)
	resp, err := e.Client.Do(req)
	if err != nil {
		t.Fatalf("PUTNote: %v", err)
	}
	return resp
}

func (e *Env) GET(t *testing.T, path string) (*http.Response, []byte) {
	t.Helper()
	req := e.authReq("GET", path, nil)
	resp, err := e.Client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, b
}

func (e *Env) DB() string {
	return filepath.Join(e.DataDir, "projects", "_default", "docsiq.db")
}
```

- [ ] **Step 3: Write `internal/api/itest/doubles.go`**

```go
//go:build integration

package itest

import (
	"context"
	"crypto/sha256"
	"encoding/binary"

	"github.com/RandomCodeSpace/docsiq/internal/llm"
)

// FakeProvider is a deterministic test double.
type FakeProvider struct{ CallCount int }

func (p *FakeProvider) Name() string    { return "fake" }
func (p *FakeProvider) ModelID() string { return "fake-model-v1" }

func (p *FakeProvider) Chat(ctx context.Context, prompt string, opts ...llm.ChatOption) (string, error) {
	p.CallCount++
	return "fake-chat-response", nil
}

func (p *FakeProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	p.CallCount++
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = deterministic(t, 384)
	}
	return out, nil
}

func deterministic(text string, dim int) []float32 {
	h := sha256.Sum256([]byte(text))
	vec := make([]float32, dim)
	for i := 0; i < dim; i++ {
		off := (i * 4) % len(h)
		bits := binary.LittleEndian.Uint32(h[off : off+4 : off+4])
		vec[i] = float32(bits) / float32(1<<32)
	}
	return vec
}
```

- [ ] **Step 4: Add `test-integration` to Makefile**

```
test-integration:
	CGO_ENABLED=1 go test -tags "sqlite_fts5 integration" -timeout 600s $(GO_PKGS)
```

- [ ] **Step 5: Smoke-test the harness builds (no tests yet)**

```bash
CGO_ENABLED=1 go build -tags "sqlite_fts5 integration" ./internal/api/itest/...
```
Expected: clean build. If signature mismatches on api.NewRouter or similar, inspect current signatures and adapt. If FakeProvider doesn't satisfy llm.Provider, read `internal/llm/provider.go` and add the missing methods.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/api/itest/*.go Makefile
git commit -m "test: integration harness + FakeProvider"
```

---

### Task C2: auth_integration_test.go

**Files:**
- Create: `internal/api/auth_integration_test.go`

- [ ] **Step 1: Write the file**

```go
//go:build integration

package api_test

import (
	"net/http"
	"sync"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/api/itest"
)

func TestAuth_BearerRequiredOnAPI(t *testing.T) {
	e := itest.New(t)

	resp, err := e.Server.Client().Get(e.Server.URL + "/api/stats?project=_default")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no-auth GET /api/stats status=%d want 401", resp.StatusCode)
	}
}

func TestAuth_BearerNotRequiredOnHealth(t *testing.T) {
	e := itest.New(t)
	resp, err := e.Server.Client().Get(e.Server.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("no-auth /health status=%d want 200", resp.StatusCode)
	}
}

func TestAuth_OptionsBypasses(t *testing.T) {
	e := itest.New(t)
	req, _ := http.NewRequest(http.MethodOptions, e.Server.URL+"/api/stats", nil)
	resp, err := e.Server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		t.Errorf("OPTIONS got 401; should bypass auth")
	}
}

func TestAuth_ConcurrentFailuresNoRace(t *testing.T) {
	e := itest.New(t)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, _ := e.Server.Client().Get(e.Server.URL + "/api/stats?project=_default")
			if resp != nil {
				resp.Body.Close()
			}
		}()
	}
	wg.Wait()
	// Pass = no data race detected by -race flag
}
```

- [ ] **Step 2: Run**

```bash
CGO_ENABLED=1 go test -tags "sqlite_fts5 integration" -race -run 'TestAuth' ./internal/api/...
```
Expected: 4 tests PASS. Race detector clean.

- [ ] **Step 3: Commit**

```bash
git add internal/api/auth_integration_test.go
git commit -m "test: auth integration suite"
```

---

### Task C3: project_integration_test.go, notes_integration_test.go, docs_integration_test.go

Pattern (per file):

- [ ] **Step 1: Write test file** covering suite scope from spec §5 table.

For `project_integration_test.go`, the required cases:
- `?project=` scopes isolation end-to-end: write note A into `foo`, read as `bar` → 404
- `_default` auto-registers on first request
- Unknown project with valid slug → 404 on read

For `notes_integration_test.go`:
- PUT→GET→DELETE round-trip (status 200 / 200 / 204)
- Wikilink graph updates on write: PUT note with `[[target]]`, GET graph shows edge
- FTS5 search finds new note by token in body
- Tar export/import round-trip preserves file tree + content

For `docs_integration_test.go`:
- upload → index → search with FakeProvider
- doc uploaded to project A not visible in project B's search

- [ ] **Step 2: Run each suite with `-race`**, 0 FAIL expected.

- [ ] **Step 3: Commit each file separately** with message `test: <suite name> integration suite`.

---

### Task C4: mcp_integration_test.go

**Files:**
- Create: `internal/mcp/mcp_integration_test.go`

- [ ] **Step 1: Write MCP round-trip tests**

```go
//go:build integration

package mcp_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/api/itest"
)

type rpcRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func callTool(t *testing.T, e *itest.Env, tool string, args map[string]any) rpcResponse {
	t.Helper()
	payload := rpcRequest{
		JSONRPC: "2.0", ID: 1,
		Method: "tools/call",
		Params: map[string]any{"name": tool, "arguments": args},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", e.Server.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+e.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.Client.Do(req)
	if err != nil {
		t.Fatalf("MCP %s: %v", tool, err)
	}
	defer resp.Body.Close()
	var r rpcResponse
	raw, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("parse MCP response: %v body=%s", err, raw)
	}
	if r.Error != nil {
		t.Fatalf("MCP %s error: %s", tool, r.Error.Message)
	}
	return r
}

func TestMCP_WriteAndSearchNoteRoundTrip(t *testing.T) {
	e := itest.New(t)
	// Auto-register _default
	e.GET(t, "/api/projects")

	_ = callTool(t, e, "write_note", map[string]any{
		"project": "_default",
		"key":     "mcp-smoke",
		"content": "# hello\n\nMCP can [[write]] notes.",
		"tags":    []string{"mcp"},
	})

	hits := callTool(t, e, "search_notes", map[string]any{
		"project": "_default",
		"query":   "hello",
	})
	if len(hits.Result) == 0 {
		t.Fatalf("search_notes empty result")
	}
}

func TestMCP_ListProjectsReturnsDefault(t *testing.T) {
	e := itest.New(t)
	e.GET(t, "/api/projects")
	r := callTool(t, e, "list_projects", map[string]any{})
	if !bytes.Contains(r.Result, []byte("_default")) {
		t.Errorf("list_projects result missing _default: %s", r.Result)
	}
}

func TestMCP_StatsToolWorks(t *testing.T) {
	e := itest.New(t)
	e.GET(t, "/api/projects")
	r := callTool(t, e, "stats", map[string]any{"project": "_default"})
	if len(r.Result) == 0 {
		t.Fatal("stats empty")
	}
}
```

- [ ] **Step 2: Run**

```bash
CGO_ENABLED=1 go test -tags "sqlite_fts5 integration" -race -run 'TestMCP' ./internal/mcp/...
```
Expected: 3 tests PASS. If the MCP tools/call JSON shape differs in practice, adjust `rpcRequest` / result parsing.

- [ ] **Step 3: Commit**

```bash
git add internal/mcp/mcp_integration_test.go
git commit -m "test: MCP JSON-RPC integration suite"
```

---

### Task C5: concurrency_integration_test.go

**Files:**
- Create: `internal/api/concurrency_integration_test.go`

- [ ] **Step 1: Write**

```go
//go:build integration

package api_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/api/itest"
)

func TestConcurrency_100ParallelNotePUTsSameProject(t *testing.T) {
	e := itest.New(t)
	e.GET(t, "/api/projects") // auto-register _default

	const N = 100
	var wg sync.WaitGroup
	errs := make(chan error, N)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp := e.PUTNote(t, "_default", fmt.Sprintf("concurrent/n-%03d", i),
				fmt.Sprintf("# Note %d\n\nbody\n", i), []string{"c"})
			if resp.StatusCode != 200 {
				errs <- fmt.Errorf("PUT %d: status %d", i, resp.StatusCode)
			}
			resp.Body.Close()
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestConcurrency_50ReadsDuringWrites(t *testing.T) {
	e := itest.New(t)
	e.GET(t, "/api/projects")
	e.PUTNote(t, "_default", "hot", "# hot\n", nil).Body.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			e.PUTNote(t, "_default", "hot", fmt.Sprintf("# hot v%d\n", i), nil).Body.Close()
		}(i)
		go func() {
			defer wg.Done()
			resp, _ := e.GET(t, "/api/projects/_default/notes/hot")
			_ = resp
		}()
	}
	wg.Wait()
}
```

- [ ] **Step 2: Run with -race**
```bash
CGO_ENABLED=1 go test -tags "sqlite_fts5 integration" -race -run 'TestConcurrency' ./internal/api/...
```
Expected: PASS. Any race detector output = P0 finding.

- [ ] **Step 3: Commit**

---

### Task C6: history_integration_test.go, hooks_integration_test.go, shutdown_integration_test.go, metrics_integration_test.go

Each suite follows the same structure as C2 (auth): `package <pkg>_test`, `//go:build integration` header, one top-level `Test<Suite>_<Case>` per required case, uses `itest.New(t)` from the harness, assertions as described below.

For each file:
- [ ] **Step 1: Write suite per spec §5 scope** using the template pattern from C2/C4/C5 (build-tag header, package name, harness import, per-case `Test*` functions). Each required case below corresponds to one `Test*` function.
- [ ] **Step 2: Run with `-race -tags "sqlite_fts5 integration"`**
- [ ] **Step 3: Commit**

`history_integration_test.go`:
- write same key twice → history endpoint returns 2 entries in reverse-chrono order
- delete creates a commit with "remove:" message
- set PATH="" before request → write still 200 (graceful git-missing fallback)

`hooks_integration_test.go`:
- POST /api/hook/SessionStart with a registered remote → 200 + `{project, additionalContext}`
- POST with unknown remote → 204
- POST with malformed JSON → 400

`shutdown_integration_test.go`:
- `defer goleak.VerifyNone(t)` at test start
- fire 10 requests, call `srv.Close()`, assert no goroutines leaked
- import `go.uber.org/goleak` — guard against known stdlib leaks with `goleak.IgnoreCurrent()`

`metrics_integration_test.go`:
- GET /metrics → 200
- body matches `^docsiq_\w+\s+\d` on at least 3 lines (at least 3 metrics emitted)
- fire N requests to /health, scrape /metrics, verify `docsiq_requests_total{path="/health"}` incremented

---

### Task C7: Integration job in ci.yml

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Add integration-test job**

In `ci.yml`, add to the `test` job's matrix OR add a new `test-integration` job:

```yaml
  test-integration:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - name: cache go build
        uses: actions/cache@v4
        with:
          path: |
            ~/.cache/go-build
            ~/go/pkg/mod
          key: go-integ-${{ hashFiles('go.sum') }}
      - name: integration tests
        run: CGO_ENABLED=1 go test -tags "sqlite_fts5 integration" -race -timeout 600s ./...
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: run integration tests with -race on Linux"
```

---

## Wave D — Hooks + MCP + Docs

### Task D1: Hook schema verification

**Files:**
- Modify: `internal/hookinstaller/{claude,cursor,copilot,codex}.go`
- Create: `internal/hookinstaller/fixtures/<client>/{before,after}.json` per client
- Modify: `README.md` (add support matrix)
- Create: `internal/hookinstaller/<client>_fixture_test.go` per client

- [ ] **Step 1: Read current installer code**

```bash
cat internal/hookinstaller/claude.go
cat internal/hookinstaller/cursor.go
cat internal/hookinstaller/copilot.go
cat internal/hookinstaller/codex.go
```
Record the JSON shape each installer produces.

- [ ] **Step 2: Fetch authoritative docs for each client**

Use `ctx_fetch_and_index` or `WebFetch` against:
- Claude Code: `https://docs.claude.com/en/docs/claude-code/hooks`
- Cursor: `https://docs.cursor.com/` (search "hooks" / "mcp" / "context")
- GitHub Copilot CLI: `https://docs.github.com/en/copilot/` + `https://github.com/github/gh-copilot`
- OpenAI Codex CLI: `https://github.com/openai/codex` (README + config docs)

For each, record the verified hook schema (or note "no documented API").

- [ ] **Step 3: Update installers**

For each client:
- If the docs confirm the current schema → no source change, just add fixture.
- If the docs show a different schema → update the Go code and add a `// schema source: <URL> fetched 2026-04-17` comment above the merge logic.
- If the client has no documented hook API:
  - Add header comment: `// UNVERIFIED — <client> does not publicly document a SessionStart hook API as of 2026-04-17.`
  - In `Install()`, emit `slog.Warn("⚠️ installing unverified hook for <client>", "client", "<client>")`.

- [ ] **Step 4: Add fixtures**

For each client, create `internal/hookinstaller/fixtures/<client>/before.json` (a realistic pre-existing config with unrelated entries) and `after.json` (what it should look like after Install() with docsiq hook = `/path/to/hook.sh`).

Example (`claude/before.json`):
```json
{
  "hooks": {
    "PreToolUse": [{"type": "command", "command": "/other/script.sh"}]
  },
  "theme": "dark"
}
```

Example (`claude/after.json`):
```json
{
  "hooks": {
    "PreToolUse": [{"type": "command", "command": "/other/script.sh"}],
    "SessionStart": [{"type": "command", "command": "/path/to/hook.sh"}]
  },
  "theme": "dark"
}
```

- [ ] **Step 5: Write fixture tests**

`internal/hookinstaller/claude_fixture_test.go`:
```go
package hookinstaller

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestClaude_FixtureTransform(t *testing.T) {
	tmp := t.TempDir()
	before, err := os.ReadFile(filepath.Join("fixtures", "claude", "before.json"))
	if err != nil { t.Fatal(err) }
	cfg := filepath.Join(tmp, "settings.json")
	if err := os.WriteFile(cfg, before, 0o644); err != nil { t.Fatal(err) }

	inst := &ClaudeInstaller{configPath: cfg}
	if err := inst.Install("/path/to/hook.sh"); err != nil { t.Fatal(err) }

	got, _ := os.ReadFile(cfg)
	want, _ := os.ReadFile(filepath.Join("fixtures", "claude", "after.json"))

	var gotMap, wantMap map[string]any
	_ = json.Unmarshal(got, &gotMap)
	_ = json.Unmarshal(want, &wantMap)
	if !jsonEqual(gotMap, wantMap) {
		t.Errorf("mismatch:\n got=%s\nwant=%s", got, want)
	}
}

func jsonEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}
```

Repeat for each client.

- [ ] **Step 6: Add support matrix to README.md**

In `README.md`, add a section:
```markdown
## Hook support matrix

| Client | Config path | Schema source | Status |
|---|---|---|---|
| Claude Code | `~/.claude/settings.json` | [docs.claude.com/en/docs/claude-code/hooks](https://docs.claude.com/en/docs/claude-code/hooks) | ✅ verified |
| Cursor | `~/.cursor/<file>` | <URL or "none"> | ✅ verified / ⚠ unverified |
| Copilot CLI | `~/.config/github-copilot/<file>` | <URL or "none"> | ✅ / ⚠ |
| Codex CLI | `~/.codex/<file>` | <URL or "none"> | ✅ / ⚠ |
```

Fill in the actual URLs/status per the research.

- [ ] **Step 7: Run**

```bash
CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/hookinstaller/...
```
Expected: all PASS.

- [ ] **Step 8: Commit per client**

```bash
git add internal/hookinstaller/claude.go internal/hookinstaller/fixtures/claude/ internal/hookinstaller/claude_fixture_test.go
git commit -m "feat(hooks): verified Claude Code schema + fixture tests"
```
Repeat for cursor, copilot, codex. Then final commit for README matrix.

---

### Task D2: MCP global_search per-project LLM

**Files:**
- Modify: `internal/mcp/tools.go` (the `global_search` handler)

- [ ] **Step 1: Write failing test**

Add to `internal/mcp/mcp_integration_test.go`:
```go
func TestMCP_GlobalSearchUsesPerProjectProvider(t *testing.T) {
	// Two projects, different provider overrides, assert search uses the
	// project-specific one.
	// Setup requires config with LLMOverrides map populated.
	t.Skip("pending llm.ProviderForProject wiring — see Task D2")
}
```
(Swap to real assertions after Step 2.)

- [ ] **Step 2: Thread project arg**

In `internal/mcp/tools.go`, find the `global_search` registration. Update the handler:
```go
func (s *Server) globalSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    args := req.GetArguments()
    slug := projectSlugArg(args)  // existing helper; or add one
    query := stringArg(args, "query", "")
    topK := intArg(args, "top_k", 5)

    st, err := s.stores.ForProject(slug)
    if err != nil {
        return toolError(err), nil
    }
    prov := llm.ProviderForProject(s.cfg, slug)
    if prov == nil {
        prov = s.provider
    }
    hits, err := search.GlobalSearch(ctx, st, prov, s.embedder, query, topK)
    // ... rest unchanged
}
```

- [ ] **Step 3: Un-skip test, make it assert the provider name differs**

- [ ] **Step 4: Run**
```bash
CGO_ENABLED=1 go test -tags "sqlite_fts5 integration" -run 'TestMCP_GlobalSearchUsesPerProjectProvider' ./internal/mcp/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

---

### Task D3: Content cleanup

**Files:**
- Modify: `CLAUDE.md`
- Modify: `/home/dev/projects/docsiq/UNIFICATION-PLAN.md` (parent dir)
- Create: `/home/dev/projects/docsiq/kgraph/ARCHIVED.md`

- [ ] **Step 1: Rebrand CLAUDE.md**

```bash
sed -i \
  -e 's|DocsContext|docsiq|g' \
  -e 's|docscontext|docsiq|g' \
  -e 's|~/\.docscontext/|~/.docsiq/|g' \
  -e 's|DOCSCONTEXT_|DOCSIQ_|g' \
  CLAUDE.md
```
Then manually review the file; drop the "Recent Changes (already committed)" section entirely (it's superseded).

- [ ] **Step 2: Rebrand UNIFICATION-PLAN.md**

Apply the same sed. Add at the very top (after the H1 heading):
```markdown
> **STATUS: Implemented.** This plan was executed 2026-04-17 across commits `3d2d2ce..790810f` (and subsequent quality-sweep work). Retained for historical reference.
```

- [ ] **Step 3: Create kgraph/ARCHIVED.md**

`/home/dev/projects/docsiq/kgraph/ARCHIVED.md`:
```markdown
# Archived

This TypeScript codebase was ported into Go as **docsiq** on 2026-04-17.

- New home: `github.com/RandomCodeSpace/docsiq` (née `docscontext`)
- Port commits: `3d2d2ce..790810f` on `main` of the docsiq repo
- Spec: `docs/superpowers/specs/2026-04-17-quality-sweep-design.md` in docsiq

This repo is retained for historical reference only. No further
development happens here.
```

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: rebrand CLAUDE.md to docsiq; drop superseded sections"
```
The UNIFICATION-PLAN.md and kgraph/ARCHIVED.md live outside this repo — do NOT `git add` them here. Commit them separately in their own contexts (or skip if those dirs are untracked).

---

### Task D4: User-facing docs directory

**Files:**
- Create 8 files under `docs/`

- [ ] **Step 1: Create `docs/README.md`**

```markdown
# docsiq Documentation

- [Getting Started](./getting-started.md)
- [CLI Reference](./cli-reference.md)
- [MCP Tools](./mcp-tools.md)
- [REST API](./rest-api.md)
- [Configuration](./config.md)
- [Hooks](./hooks.md)
- [Architecture](./architecture.md)
```

- [ ] **Step 2: Create `docs/getting-started.md`** with sections: Prerequisites (Go 1.22+, gcc/Xcode CLT), Install (`go install github.com/RandomCodeSpace/docsiq@latest`), First project (`docsiq init` in a git repo), Start server (`docsiq serve`), Check it works (`curl localhost:8080/health`).

- [ ] **Step 3: Create `docs/cli-reference.md`** — one section per command (`init`, `serve`, `index`, `stats`, `projects`, `hooks`, `vec`, `version`) with flags + examples. Each command's section generated by running `./docsiq <cmd> --help` and reformatting.

- [ ] **Step 4: Create `docs/mcp-tools.md`** — all 19 MCP tools. Use this template per tool:
```markdown
### `tool_name`

<one-sentence description>

**Arguments:**
| name | type | required | description |

**Returns:** <JSON shape>

**Example:** `<MCP call example>`
```
Scrape from `internal/mcp/tools.go` + `internal/mcp/notes_tools.go`.

- [ ] **Step 5: Create `docs/rest-api.md`** — every REST endpoint. Template:
```markdown
### `METHOD /path`

<description>

**Auth:** Bearer | public

**Query params:** ...

**Request body:** (JSON shape)

**Response:** (status → body shape)
```
Scrape from `internal/api/router.go` + handler implementations.

- [ ] **Step 6: Create `docs/config.md`** — every config field (from `internal/config/config.go` struct) with env var name, default, type, description.

- [ ] **Step 7: Create `docs/hooks.md`** — duplicate + expand the support matrix from README.md; add troubleshooting section ("My hook doesn't fire" → check install status, server reachable, etc.).

- [ ] **Step 8: Create `docs/architecture.md`** — one diagram (ASCII or Mermaid), 2-page explanation of per-project layout, store + registry + HNSW flow.

- [ ] **Step 9: Commit**

```bash
git add docs/
git commit -m "docs: user-facing guides (CLI, MCP, REST, config, hooks, arch)"
```

---

## Wave E — Review Remediation

### Task E1: Read REVIEW.md, categorize findings

- [ ] **Step 1: Count findings**

```bash
grep -c '^### \[P0-' REVIEW.md
grep -c '^### \[P1-' REVIEW.md
grep -c '^### \[P2-' REVIEW.md
```

- [ ] **Step 2: If P0+P1 > 20 findings, PAUSE and escalate to user** (per spec §10 Risks). Post summary + recommended path (break into sub-plan?).

- [ ] **Step 3: If P0+P1 ≤ 20, proceed to E2.**

### Task E2: Fix each P0 finding (one commit per finding)

Pattern per P0:

- [ ] **Step 1: Read the finding** in REVIEW.md (`### [P0-<N>]`).
- [ ] **Step 2: Write the failing regression test** at the file:line indicated.
- [ ] **Step 3: Run the test; verify it fails.**
- [ ] **Step 4: Implement the recommended fix** (or alternative with equal correctness).
- [ ] **Step 5: Run the test; verify it passes.**
- [ ] **Step 6: Run full test suite** — `make test && make test-integration`. All green.
- [ ] **Step 7: Update REVIEW.md** — append under the finding: `**Status:** fixed in <commit-sha-short>`.
- [ ] **Step 8: Commit**
  ```bash
  git commit -m "fix: <short description> (P0-<N> from REVIEW)"
  ```

### Task E3: Fix P1 findings (group thematically, one commit per theme)

Same pattern as E2, grouped by theme (e.g., "fix all auth middleware edge cases in one commit").

### Task E4: Comment P2 findings

- [ ] **Step 1: For each P2 finding**, add a `// TODO(docsiq): P2-<N> <short summary>` comment at the file:line.
- [ ] **Step 2: Update REVIEW.md**: `**Status:** deferred (P2), TODO planted at <file>:<line>`.
- [ ] **Step 3: Commit all P2 TODOs together**
  ```bash
  git commit -m "docs: plant TODO markers for deferred P2 review findings"
  ```

### Task E5: Assert final REVIEW.md state

- [ ] **Step 1: Verify no unresolved P0/P1**
  ```bash
  awk '/^## P[01] —/,/^## /' REVIEW.md | grep -c '\*\*Status:\*\* fixed'
  awk '/^## P[01] —/,/^## /' REVIEW.md | grep -c '^### \[P[01]-'
  ```
  The two counts must match (every P0 and P1 has a Status: fixed or disputed line).

- [ ] **Step 2: Update summary counts at top of REVIEW.md** to reflect resolved vs. outstanding.

- [ ] **Step 3: Commit**
  ```bash
  git add REVIEW.md
  git commit -m "docs: REVIEW.md — all P0/P1 findings resolved"
  ```

---

## Wave F — Lint Modernization Sweep

### Task F1: Sweep Go 1.24 style hints

**Files:** any Go file flagged by `go vet` with modernization hints.

- [ ] **Step 1: Run `gopls` modernize check**

```bash
go install golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest 2>/dev/null || true
modernize -test ./... 2>&1 | head -50
```
If modernize binary isn't available, skip it and rely on the diagnostics already in the LSP output (the many `rangeint`, `stringsseq`, `mapsloop`, `b.Loop()`, `any`, `minmax` hints we've seen throughout).

- [ ] **Step 2: Apply modernizations, one transformation per commit**

Example commit shapes:
```bash
# Apply rangeint across all test files
git commit -m "style: modernize for-loops to range-over-int"
```

Transformations to apply:
- `for i := 0; i < n; i++` → `for i := range n`
- `for _, x := range strings.Split(s, sep)` → `for _, x := range strings.SplitSeq(s, sep)`
- manual copy loop `for k,v := range src { dst[k] = v }` → `maps.Copy(dst, src)`
- `for i := 0; i < b.N; i++` → `for b.Loop()`
- `interface{}` → `any`
- Delete unused `func min(...)`/`func max(...)` in favor of builtins

- [ ] **Step 3: After each transformation, run `make vet && make test`**

Expected: exit 0 after each. Any regression = revert and diagnose.

### Task F2: False-positive `unusedfunc` suppressions

- [ ] **Step 1: Find unused-by-linter functions that are actually used via composition**

Likely culprits:
- `bearerAuthMiddleware` (referenced in `NewRouter` return chain)
- `h.health` (registered by method reference in `mux.HandleFunc`)
- `registerHookRoutes` (called in `NewRouter`)
- Various `h.foo` methods referenced via `mux.HandleFunc`

- [ ] **Step 2: Add `//nolint:unusedfunc` directive** above each genuinely-used-but-flagged function.

- [ ] **Step 3: Commit**

```bash
git commit -m "style: silence unusedfunc false positives for method dispatch"
```

---

## Final Verification

### Task V1: Local gates

- [ ] **Step 1: Build**
  ```bash
  CGO_ENABLED=1 go build -tags sqlite_fts5 -o docsiq ./
  ```
  Expected: exit 0, `./docsiq` exists.

- [ ] **Step 2: Vet**
  ```bash
  make vet
  ```
  Expected: exit 0.

- [ ] **Step 3: Unit tests**
  ```bash
  make test
  ```
  Expected: all packages ok.

- [ ] **Step 4: Integration tests**
  ```bash
  make test-integration
  ```
  Expected: all green with `-race`.

- [ ] **Step 5: Frontend tests + coverage**
  ```bash
  cd ui && npm run test:coverage
  ```
  Expected: all green; coverage ≥ 70% statements, ≥ 60% branches on notes/nav/shared/hooks.

- [ ] **Step 6: UI build**
  ```bash
  npm --prefix ui run build
  git diff --exit-code -- ui/dist/
  ```
  Expected: clean (or commit fresh dist).

- [ ] **Step 7: Grep audits**
  ```bash
  grep -r 'docscontext' internal/ cmd/ ; echo "should be 0 hits"
  grep -r 'TODO(docsiq): P[01]' internal/ cmd/ ; echo "should be 0 hits"
  ```
  Expected: both 0 hits.

### Task V2: End-to-end smoke

- [ ] **Step 1: Isolated serve + full flow**

```bash
tmp=$(mktemp -d)
DOCSIQ_DATA_DIR="$tmp/data" \
DOCSIQ_SERVER_PORT=18888 \
DOCSIQ_DEFAULT_PROJECT=_default \
./docsiq serve >"$tmp/server.log" 2>&1 &
pid=$!
sleep 2

curl -sf http://127.0.0.1:18888/health
curl -sf http://127.0.0.1:18888/metrics | head -5
curl -sf "http://127.0.0.1:18888/api/projects" -H "Authorization: Bearer ${DOCSIQ_API_KEY:-}"

kill $pid
tail -5 "$tmp/server.log"
```
Expected: health=200, metrics=prom-format, projects=JSON with `_default`, no goroutine leak warnings in log tail.

### Task V3: Push

- [ ] **Step 1: Verify branch clean**
  ```bash
  git status --short
  ```
  Expected: clean.

- [ ] **Step 2: Push**
  ```bash
  git push origin main
  ```

- [ ] **Step 3: Watch CI**
  Monitor the GitHub Actions run from the push. If any job fails, file as follow-up in REVIEW.md (post-sweep) and fix — do NOT merge fails.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-17-quality-sweep.md`. Two execution options:

**1. Subagent-Driven (recommended)** — Dispatch fresh subagent per task, review between tasks, fast iteration. Uses `superpowers:subagent-driven-development`.

**2. Inline Execution** — Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints for review.

Which approach?
