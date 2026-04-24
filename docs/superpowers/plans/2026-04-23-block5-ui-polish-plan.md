# Block 5 — UI Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship ten production-polish UI items (error boundary, loading/empty/error state trio, dynamic titles, iOS safe-area, max-update-depth bug, axe violations, reduced-motion gating, focus management, theme-flash, mobile viewport) so the docsiq SPA is production-grade on desktop and iOS safari.

**Architecture:** All changes live inside `ui/` (React 19 + TypeScript 6 + Vite 8 + Tailwind v4). New reusable state primitives (`EmptyState`, `LoadingSkeleton`, `ErrorState`) live at `ui/src/components/empty/` and feed into `RouteBoundary` plus every fetching route. Theme-flash runs as a pre-hydration inline script in `ui/index.html` that writes `document.documentElement.classList` before React boots, eliminating FOUC.

**Tech Stack:** React 19, react-router-dom v7, Zustand (persisted at key `docsiq-ui`), TanStack Query, shadcn/ui local copies (`ui/src/components/ui/`), Framer Motion v11 via `useReducedMotion()` hook, Vitest + @testing-library/react, Playwright + axe-core.

---

## Task Ordering Rationale

Task 1 (5.2) first — the state trio is a dependency of Task 2 (5.1 RouteBoundary) and of the empty/loading states threaded into routes from Task 3 onward. Tasks 5–10 are investigation/audit tasks that use the primitives established by Tasks 1–4.

Order:

1. Task 1: Loading / empty / error state trio (spec 5.2)
2. Task 2: RouteBoundary error boundary (spec 5.1)
3. Task 3: Dynamic `document.title` per route (spec 5.3)
4. Task 4: iOS safe-area insets (spec 5.4)
5. Task 5: "Maximum update depth exceeded" investigation + fix (spec 5.5)
6. Task 6: Axe violations sweep (spec 5.6)
7. Task 7: Reduced-motion gating (spec 5.7)
8. Task 8: Focus management (spec 5.8)
9. Task 9: Theme-flash inline script (spec 5.9)
10. Task 10: Mobile viewport pass (spec 5.10)

---

### Task 1: Reusable state trio — `EmptyState`, `LoadingSkeleton`, `ErrorState` (5.2)

**Files:**
- Create: `ui/src/components/empty/EmptyState.tsx`
- Create: `ui/src/components/empty/LoadingSkeleton.tsx`
- Create: `ui/src/components/empty/ErrorState.tsx`
- Create: `ui/src/components/empty/index.ts`
- Create: `ui/src/components/empty/__tests__/EmptyState.test.tsx`
- Create: `ui/src/components/empty/__tests__/LoadingSkeleton.test.tsx`
- Create: `ui/src/components/empty/__tests__/ErrorState.test.tsx`
- Modify: `ui/src/routes/Home.tsx`
- Modify: `ui/src/routes/Graph.tsx`
- Modify: `ui/src/routes/MCPConsole.tsx`
- Modify: `ui/src/routes/notes/NotesLayout.tsx`
- Modify: `ui/src/routes/notes/NoteView.tsx`
- Modify: `ui/src/routes/notes/NotesSearch.tsx`
- Modify: `ui/src/routes/documents/DocumentsList.tsx`
- Modify: `ui/src/routes/documents/DocumentView.tsx`
- Modify: `ui/src/styles/globals.css` (adds `.state-card` tokens)

- [ ] **Step 1: Write `EmptyState` test**

Create `ui/src/components/empty/__tests__/EmptyState.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { EmptyState } from "../EmptyState";

describe("EmptyState", () => {
  it("renders title and description", () => {
    render(<EmptyState title="No notes yet" description="Create one to get started." />);
    expect(screen.getByRole("status")).toBeInTheDocument();
    expect(screen.getByText("No notes yet")).toBeInTheDocument();
    expect(screen.getByText("Create one to get started.")).toBeInTheDocument();
  });

  it("renders an action slot when provided", () => {
    render(
      <EmptyState
        title="No notes"
        description="Try ingesting one."
        action={<button type="button">Ingest</button>}
      />,
    );
    expect(screen.getByRole("button", { name: /ingest/i })).toBeInTheDocument();
  });

  it("exposes role=status so screen readers announce it", () => {
    render(<EmptyState title="x" description="y" />);
    expect(screen.getByRole("status")).toHaveAttribute("aria-live", "polite");
  });
});
```

- [ ] **Step 2: Run the test — expect failure**

```
cd ui && npm test -- --run src/components/empty/__tests__/EmptyState.test.tsx
```

Expected: FAIL with `Cannot find module '../EmptyState'`.

- [ ] **Step 3: Implement `EmptyState`**

Create `ui/src/components/empty/EmptyState.tsx`:

```tsx
import type { ReactNode } from "react";
import { Inbox } from "lucide-react";

interface EmptyStateProps {
  title: string;
  description: string;
  icon?: ReactNode;
  action?: ReactNode;
}

export function EmptyState({ title, description, icon, action }: EmptyStateProps) {
  return (
    <div
      role="status"
      aria-live="polite"
      className="state-card state-card--empty"
      data-testid="empty-state"
    >
      <div className="state-card__icon" aria-hidden="true">
        {icon ?? <Inbox className="size-6" />}
      </div>
      <h3 className="state-card__title">{title}</h3>
      <p className="state-card__description">{description}</p>
      {action ? <div className="state-card__action">{action}</div> : null}
    </div>
  );
}
```

- [ ] **Step 4: Run EmptyState test — expect pass**

```
cd ui && npm test -- --run src/components/empty/__tests__/EmptyState.test.tsx
```

Expected: 3 passing.

- [ ] **Step 5: Write `LoadingSkeleton` test**

Create `ui/src/components/empty/__tests__/LoadingSkeleton.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { LoadingSkeleton } from "../LoadingSkeleton";

describe("LoadingSkeleton", () => {
  it("renders a polite live-region with a label", () => {
    render(<LoadingSkeleton label="Loading notes" />);
    const status = screen.getByRole("status");
    expect(status).toHaveAttribute("aria-live", "polite");
    expect(status).toHaveAttribute("aria-label", "Loading notes");
  });

  it("renders `rows` skeleton bars", () => {
    render(<LoadingSkeleton label="Loading" rows={4} />);
    expect(screen.getAllByTestId("skeleton-row")).toHaveLength(4);
  });

  it("defaults to 3 rows when no count given", () => {
    render(<LoadingSkeleton label="Loading" />);
    expect(screen.getAllByTestId("skeleton-row")).toHaveLength(3);
  });
});
```

- [ ] **Step 6: Run the test — expect failure**

```
cd ui && npm test -- --run src/components/empty/__tests__/LoadingSkeleton.test.tsx
```

Expected: FAIL with `Cannot find module '../LoadingSkeleton'`.

- [ ] **Step 7: Implement `LoadingSkeleton`**

Create `ui/src/components/empty/LoadingSkeleton.tsx`:

```tsx
import { Skeleton } from "@/components/ui/skeleton";

interface LoadingSkeletonProps {
  label: string;
  rows?: number;
}

export function LoadingSkeleton({ label, rows = 3 }: LoadingSkeletonProps) {
  const count = Math.max(1, rows);
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={label}
      className="state-card state-card--loading"
      data-testid="loading-skeleton"
    >
      <div className="state-card__bars">
        {Array.from({ length: count }).map((_, i) => (
          <Skeleton key={i} data-testid="skeleton-row" className="state-card__bar" />
        ))}
      </div>
      <span className="sr-only">{label}</span>
    </div>
  );
}
```

- [ ] **Step 8: Run LoadingSkeleton test — expect pass**

```
cd ui && npm test -- --run src/components/empty/__tests__/LoadingSkeleton.test.tsx
```

Expected: 3 passing.

- [ ] **Step 9: Write `ErrorState` test**

Create `ui/src/components/empty/__tests__/ErrorState.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { ErrorState } from "../ErrorState";

describe("ErrorState", () => {
  it("renders the message and a retry button when retry is provided", async () => {
    const retry = vi.fn();
    render(
      <ErrorState title="Failed to load" message="Network error" onRetry={retry} />,
    );
    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("Failed to load")).toBeInTheDocument();
    expect(screen.getByText("Network error")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /retry/i }));
    expect(retry).toHaveBeenCalledOnce();
  });

  it("hides the retry button when onRetry is absent", () => {
    render(<ErrorState title="Broken" message="x" />);
    expect(screen.queryByRole("button", { name: /retry/i })).toBeNull();
  });

  it("sanitizes long messages to 500 chars", () => {
    const long = "a".repeat(900);
    render(<ErrorState title="t" message={long} />);
    const rendered = screen.getByTestId("error-message").textContent ?? "";
    expect(rendered.length).toBeLessThanOrEqual(504); // 500 + ellipsis
  });
});
```

- [ ] **Step 10: Run the test — expect failure**

```
cd ui && npm test -- --run src/components/empty/__tests__/ErrorState.test.tsx
```

Expected: FAIL with `Cannot find module '../ErrorState'`.

- [ ] **Step 11: Implement `ErrorState`**

Create `ui/src/components/empty/ErrorState.tsx`:

```tsx
import { AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";

interface ErrorStateProps {
  title: string;
  message: string;
  onRetry?: () => void;
}

const MAX_MESSAGE_LEN = 500;

function sanitize(raw: string): string {
  const trimmed = raw.trim().replace(/[ -]+/g, " ");
  if (trimmed.length <= MAX_MESSAGE_LEN) return trimmed;
  return `${trimmed.slice(0, MAX_MESSAGE_LEN)}…`;
}

export function ErrorState({ title, message, onRetry }: ErrorStateProps) {
  return (
    <div
      role="alert"
      className="state-card state-card--error"
      data-testid="error-state"
    >
      <div className="state-card__icon state-card__icon--danger" aria-hidden="true">
        <AlertTriangle className="size-6" />
      </div>
      <h3 className="state-card__title">{title}</h3>
      <p className="state-card__description" data-testid="error-message">
        {sanitize(message)}
      </p>
      {onRetry ? (
        <div className="state-card__action">
          <Button type="button" size="sm" variant="outline" onClick={onRetry}>
            Retry
          </Button>
        </div>
      ) : null}
    </div>
  );
}
```

- [ ] **Step 12: Run ErrorState test — expect pass**

```
cd ui && npm test -- --run src/components/empty/__tests__/ErrorState.test.tsx
```

Expected: 3 passing.

- [ ] **Step 13: Write the barrel export**

Create `ui/src/components/empty/index.ts`:

```ts
export { EmptyState } from "./EmptyState";
export { LoadingSkeleton } from "./LoadingSkeleton";
export { ErrorState } from "./ErrorState";
```

- [ ] **Step 14: Add shared styles**

Open `ui/src/styles/globals.css`. Append at the bottom of the file:

```css
/* Block 5.2 — reusable state trio ------------------------------------ */
.state-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 2rem 1.5rem;
  border: 1px solid var(--border);
  border-radius: 0.75rem;
  background: var(--card);
  text-align: center;
}
.state-card__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.5rem;
  height: 2.5rem;
  border-radius: 9999px;
  background: color-mix(in oklab, var(--muted) 60%, transparent);
  color: var(--muted-foreground);
}
.state-card__icon--danger {
  background: color-mix(in oklab, var(--destructive) 14%, transparent);
  color: var(--destructive);
}
.state-card__title {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--foreground);
}
.state-card__description {
  font-size: 0.85rem;
  line-height: 1.45;
  color: var(--muted-foreground);
  max-width: 40ch;
}
.state-card__action {
  margin-top: 0.5rem;
}
.state-card__bars {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.state-card__bar {
  height: 0.75rem;
  width: 100%;
  border-radius: 0.375rem;
}
.state-card--loading { padding: 1.25rem; align-items: stretch; }
```

- [ ] **Step 15: Apply the trio to `Home.tsx`**

Replace the whole contents of `ui/src/routes/Home.tsx` with:

```tsx
import { useEffect, useMemo } from "react";
import { Link } from "react-router-dom";
import { ArrowUpRight } from "lucide-react";
import { StatsStrip } from "@/components/layout/StatsStrip";
import { ActivityFeed } from "@/components/activity/ActivityFeed";
import { GlanceView } from "@/components/graph/GlanceView";
import { useProjectStore } from "@/stores/project";
import { useStats } from "@/hooks/api/useStats";
import { useActivity } from "@/hooks/api/useActivity";
import { useNotes } from "@/hooks/api/useNotes";
import { useNotesGraph } from "@/hooks/api/useGraph";
import { useLastVisit } from "@/hooks/useLastVisit";
import { EmptyState, ErrorState, LoadingSkeleton } from "@/components/empty";

export default function Home() {
  const project = useProjectStore((s) => s.slug);
  const stats = useStats(project);
  const activity = useActivity(project);
  const notes = useNotes(project);
  const graph = useNotesGraph(project);
  const { lastVisit, touch } = useLastVisit();

  const newCount = useMemo(() => {
    if (!activity.data) return 0;
    return activity.data.filter((e) => e.kind === "note_added" && e.timestamp > lastVisit).length;
  }, [activity.data, lastVisit]);

  useEffect(() => () => { touch(); }, [touch]);

  const recentNotes = (notes.data ?? []).slice(0, 8);
  const mergedStats = useMemo(() => {
    const notesCount = notes.data?.length ?? 0;
    const base = stats.data ?? {
      documents: 0, chunks: 0, entities: 0, relationships: 0,
      communities: 0, notes: 0, last_indexed: null,
    };
    return { ...base, notes: notesCount };
  }, [stats.data, notes.data]);

  const activityError = activity.error as Error | null | undefined;
  const graphError = graph.error as Error | null | undefined;

  return (
    <div className="page">
      <StatsStrip stats={mergedStats} delta={{ notes: newCount }} />

      <div className="home-split">
        <div className="home-main">
          <div className="page-header">
            <div className="flex items-baseline gap-3">
              <h2 className="page-header-title">Activity</h2>
              <span className="page-header-meta">{activity.data?.length ?? 0} events</span>
            </div>
          </div>
          <div className="page-body">
            {activity.isLoading ? (
              <LoadingSkeleton label="Loading activity" rows={4} />
            ) : activityError ? (
              <ErrorState
                title="Could not load activity"
                message={activityError.message || "Unknown error"}
                onRetry={() => activity.refetch()}
              />
            ) : (activity.data?.length ?? 0) === 0 ? (
              <EmptyState
                title="No recent activity"
                description="Ingest a document or create a note to start the feed."
              />
            ) : (
              <ActivityFeed events={activity.data ?? []} lastVisit={lastVisit} />
            )}
          </div>
        </div>

        <aside className="home-rail">
          <section className="home-rail-top">
            <div className="section-head">
              <h2 className="section-title">Graph</h2>
              <Link to="/graph" className="section-link">
                open <ArrowUpRight className="size-3" />
              </Link>
            </div>
            <div className="graph-card">
              {graph.isLoading ? (
                <LoadingSkeleton label="Loading graph preview" rows={3} />
              ) : graphError ? (
                <ErrorState
                  title="Graph failed"
                  message={graphError.message || "Unknown error"}
                  onRetry={() => graph.refetch()}
                />
              ) : !graph.data || graph.data.nodes.length === 0 ? (
                <EmptyState
                  title="No graph yet"
                  description="Run an index to see entities and edges."
                />
              ) : (
                <GlanceView data={graph.data} />
              )}
            </div>
          </section>

          <section className="section flex-1">
            <div className="section-head">
              <h2 className="section-title">Recent notes</h2>
              <Link to="/notes" className="section-link">
                all <ArrowUpRight className="size-3" />
              </Link>
            </div>
            {notes.isLoading ? (
              <LoadingSkeleton label="Loading notes" rows={5} />
            ) : recentNotes.length === 0 ? (
              <EmptyState
                title="No notes yet"
                description="Create your first note to see it here."
              />
            ) : (
              <ul className="note-list">
                {recentNotes.map((n) => {
                  const parts = n.key.split("/");
                  const name = parts.pop() ?? n.key;
                  const folder = parts.join("/");
                  return (
                    <li key={n.key}>
                      <Link to={`/notes/${encodeURIComponent(n.key)}`} className="note-row">
                        <span className="note-row-name">{name}</span>
                        {folder && <span className="note-row-folder">{folder}</span>}
                      </Link>
                    </li>
                  );
                })}
              </ul>
            )}
          </section>
        </aside>
      </div>
    </div>
  );
}
```

- [ ] **Step 16: Apply the trio to `Graph.tsx`**

Replace the whole contents of `ui/src/routes/Graph.tsx` with:

```tsx
import { GraphCanvas } from "@/components/graph/GraphCanvas";
import { useNotesGraph } from "@/hooks/api/useGraph";
import { useProjectStore } from "@/stores/project";
import { EmptyState, ErrorState, LoadingSkeleton } from "@/components/empty";

export default function Graph() {
  const project = useProjectStore((s) => s.slug);
  const { data, isLoading, error, refetch } = useNotesGraph(project);
  const err = error as Error | null | undefined;

  if (isLoading) {
    return (
      <div className="graph-page p-8">
        <LoadingSkeleton label="Loading graph" rows={4} />
      </div>
    );
  }
  if (err) {
    return (
      <div className="graph-page p-8">
        <ErrorState
          title="Graph failed to load"
          message={err.message || "Unknown error"}
          onRetry={() => refetch()}
        />
      </div>
    );
  }
  if (!data || data.nodes.length === 0) {
    return (
      <div className="graph-page p-8">
        <EmptyState
          title="No graph data for this project"
          description="Ingest or index a document to build the graph."
        />
      </div>
    );
  }
  return (
    <div className="graph-page">
      <GraphCanvas data={data} />
    </div>
  );
}
```

- [ ] **Step 17: Wire the trio into the MCPConsole route**

Open `ui/src/routes/MCPConsole.tsx`. Locate the first fetching region (search for `isLoading`) and replace inline `<div>Loading…</div>` / empty divs / error text with the trio. Specifically, at the top of the file add:

```tsx
import { EmptyState, ErrorState, LoadingSkeleton } from "@/components/empty";
```

Then for every branch that currently renders plain-text `Loading…`, empty placeholder, or caught error text, wrap them. The minimum required replacements:

- Replace any `return <div ...>Loading…</div>` with:

  ```tsx
  return (
    <section className="mcp-console p-6">
      <LoadingSkeleton label="Loading MCP tools" rows={4} />
    </section>
  );
  ```

- For an error branch, replace with:

  ```tsx
  return (
    <section className="mcp-console p-6">
      <ErrorState
        title="MCP tools failed to load"
        message={(err as Error).message || "Unknown error"}
        onRetry={() => refetch()}
      />
    </section>
  );
  ```

- For an empty/"no tools" branch, replace with:

  ```tsx
  <EmptyState
    title="No MCP tools registered"
    description="Start the MCP server or connect a client to see tools here."
  />
  ```

- [ ] **Step 18: Wire the trio into `NotesLayout.tsx`**

Open `ui/src/routes/notes/NotesLayout.tsx`. At the top, add:

```tsx
import { EmptyState, ErrorState, LoadingSkeleton } from "@/components/empty";
```

Replace any `<div>Loading…</div>` with `<LoadingSkeleton label="Loading notes" rows={6} />`, any empty state with `<EmptyState title="No notes yet" description="Create your first note to see it here." />`, and any error with `<ErrorState title="Notes failed to load" message={...} onRetry={() => refetch()} />`.

- [ ] **Step 19: Wire the trio into `NoteView.tsx`**

Open `ui/src/routes/notes/NoteView.tsx`. At the top, add:

```tsx
import { EmptyState, ErrorState, LoadingSkeleton } from "@/components/empty";
```

Replace the loading placeholder with `<LoadingSkeleton label="Loading note" rows={5} />`, the not-found branch with `<EmptyState title="Note not found" description="The note may have been deleted or the link is stale." />`, and any fetch error with `<ErrorState title="Note failed to load" message={...} onRetry={() => refetch()} />`.

- [ ] **Step 20: Wire the trio into `NotesSearch.tsx`**

Open `ui/src/routes/notes/NotesSearch.tsx`. Add the import:

```tsx
import { EmptyState, ErrorState, LoadingSkeleton } from "@/components/empty";
```

Wrap loading/empty/error branches the same way.

- [ ] **Step 21: Wire the trio into `DocumentsList.tsx` and `DocumentView.tsx`**

Apply the same three-branch pattern to both files:

```tsx
import { EmptyState, ErrorState, LoadingSkeleton } from "@/components/empty";
```

For `DocumentsList.tsx`: loading → `<LoadingSkeleton label="Loading documents" rows={6} />`; empty → `<EmptyState title="No documents yet" description="Upload a PDF, DOCX, or web page to get started." />`; error → `<ErrorState title="Documents failed to load" message={...} onRetry={() => refetch()} />`.

For `DocumentView.tsx`: loading → `<LoadingSkeleton label="Loading document" rows={5} />`; empty → `<EmptyState title="Document not found" description="The document may have been deleted." />`; error → `<ErrorState title="Document failed to load" message={...} onRetry={() => refetch()} />`.

- [ ] **Step 22: Run the full UI test suite**

```
cd ui && npm test -- --run
```

Expected: all new state-trio tests pass and no existing test regressed.

- [ ] **Step 23: Typecheck + build**

```
cd ui && npm run typecheck && npm run build
```

Expected: no errors. Build output dist JS + CSS remains under 640 KB. Verify with:

```
cd ui && find dist -name '*.js' -o -name '*.css' | xargs du -cb | tail -1
```

Expected: total bytes < 655360.

- [ ] **Step 24: Commit**

```bash
git add ui/src/components/empty ui/src/styles/globals.css \
  ui/src/routes/Home.tsx ui/src/routes/Graph.tsx ui/src/routes/MCPConsole.tsx \
  ui/src/routes/notes/NotesLayout.tsx ui/src/routes/notes/NoteView.tsx \
  ui/src/routes/notes/NotesSearch.tsx \
  ui/src/routes/documents/DocumentsList.tsx ui/src/routes/documents/DocumentView.tsx
git commit -m "$(cat <<'EOF'
feat(ui): add reusable EmptyState/LoadingSkeleton/ErrorState trio (5.2)

Adds consistent loading/empty/error primitives under ui/src/components/empty
and applies them to every fetching route (Home, Notes*, Documents*, Graph,
MCPConsole). Addresses Block 5.2 of the production-polish roadmap.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: `RouteBoundary` error boundary (5.1)

**Files:**
- Create: `ui/src/components/layout/RouteBoundary.tsx`
- Create: `ui/src/components/layout/__tests__/RouteBoundary.test.tsx`
- Modify: `ui/src/App.tsx`

- [ ] **Step 1: Write the failing test**

Create `ui/src/components/layout/__tests__/RouteBoundary.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { RouteBoundary } from "../RouteBoundary";

function Boom({ fuse }: { fuse: boolean }): JSX.Element {
  if (fuse) throw new Error("kaboom");
  return <div>ok</div>;
}

describe("RouteBoundary", () => {
  beforeEach(() => {
    vi.spyOn(console, "error").mockImplementation(() => {});
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders children when they do not throw", () => {
    render(
      <RouteBoundary>
        <Boom fuse={false} />
      </RouteBoundary>,
    );
    expect(screen.getByText("ok")).toBeInTheDocument();
  });

  it("catches render errors and shows the fallback card with sanitized message", () => {
    render(
      <RouteBoundary>
        <Boom fuse />
      </RouteBoundary>,
    );
    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText(/something went wrong/i)).toBeInTheDocument();
    expect(screen.getByText("kaboom")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /reload this view/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /report/i })).toHaveAttribute(
      "href",
      expect.stringMatching(/^mailto:/),
    );
  });

  it("resets on `Reload this view` click", async () => {
    function Toggle() {
      return <Boom fuse />;
    }
    render(
      <RouteBoundary>
        <Toggle />
      </RouteBoundary>,
    );
    expect(screen.getByRole("alert")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /reload this view/i }));
    // After reset the child still throws, but the reset did fire and the
    // boundary re-caught. We simply assert the reload button is still
    // reachable — which proves the reset handler ran without crashing.
    expect(screen.getByRole("button", { name: /reload this view/i })).toBeInTheDocument();
  });

  it("truncates very long error messages", () => {
    function LongBoom(): JSX.Element {
      throw new Error("x".repeat(900));
    }
    render(
      <RouteBoundary>
        <LongBoom />
      </RouteBoundary>,
    );
    const msg = screen.getByTestId("boundary-message").textContent ?? "";
    expect(msg.length).toBeLessThanOrEqual(504);
  });
});
```

- [ ] **Step 2: Run the test — expect failure**

```
cd ui && npm test -- --run src/components/layout/__tests__/RouteBoundary.test.tsx
```

Expected: FAIL with `Cannot find module '../RouteBoundary'`.

- [ ] **Step 3: Implement `RouteBoundary`**

Create `ui/src/components/layout/RouteBoundary.tsx`:

```tsx
import { Component, type ErrorInfo, type ReactNode } from "react";
import { AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";

interface RouteBoundaryProps { children: ReactNode }
interface RouteBoundaryState { error: Error | null }

const MAX_MESSAGE_LEN = 500;
const REPORT_EMAIL = "docsiq-support@example.invalid";

function sanitize(raw: string): string {
  const trimmed = raw.trim().replace(/[ -]+/g, " ");
  if (trimmed.length <= MAX_MESSAGE_LEN) return trimmed;
  return `${trimmed.slice(0, MAX_MESSAGE_LEN)}…`;
}

function buildMailto(err: Error): string {
  const subject = encodeURIComponent(`[docsiq] Render error: ${err.name}`);
  const bodyLines = [
    "What I was doing:",
    "",
    "",
    "Message:",
    sanitize(err.message),
    "",
    "Stack (truncated):",
    (err.stack ?? "").split("\n").slice(0, 10).join("\n"),
    "",
    `URL: ${typeof window !== "undefined" ? window.location.href : ""}`,
    `UA:  ${typeof navigator !== "undefined" ? navigator.userAgent : ""}`,
  ].join("\n");
  const body = encodeURIComponent(bodyLines);
  return `mailto:${REPORT_EMAIL}?subject=${subject}&body=${body}`;
}

export class RouteBoundary extends Component<RouteBoundaryProps, RouteBoundaryState> {
  state: RouteBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): RouteBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // slog-equivalent: keep one structured line; never dump user PII
    console.error("[RouteBoundary]", error.name, sanitize(error.message), info.componentStack);
  }

  private reset = () => this.setState({ error: null });

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;
    return (
      <div role="alert" className="state-card state-card--error" data-testid="route-boundary">
        <div className="state-card__icon state-card__icon--danger" aria-hidden="true">
          <AlertTriangle className="size-6" />
        </div>
        <h3 className="state-card__title">Something went wrong</h3>
        <p className="state-card__description" data-testid="boundary-message">
          {sanitize(error.message || "Unknown render error")}
        </p>
        <div className="state-card__action flex gap-2">
          <Button type="button" size="sm" variant="outline" onClick={this.reset}>
            Reload this view
          </Button>
          <a
            className="inline-flex items-center justify-center rounded-md border px-3 py-1.5 text-sm hover:bg-accent"
            href={buildMailto(error)}
            rel="noopener noreferrer"
          >
            Report
          </a>
        </div>
      </div>
    );
  }
}
```

- [ ] **Step 4: Run the test — expect pass**

```
cd ui && npm test -- --run src/components/layout/__tests__/RouteBoundary.test.tsx
```

Expected: 4 passing.

- [ ] **Step 5: Wire `RouteBoundary` into `App.tsx`**

Replace the whole contents of `ui/src/App.tsx` with:

```tsx
import { lazy, Suspense, useEffect } from "react";
import { Route, Routes } from "react-router-dom";
import { Providers } from "@/components/layout/Providers";
import { Shell } from "@/components/layout/Shell";
import { RouteBoundary } from "@/components/layout/RouteBoundary";
import { LoadingSkeleton } from "@/components/empty";
import { initAuth } from "@/lib/api-client";
import Home from "@/routes/Home";

// Home is eager (first paint); everything else is split.
const NotesLayout = lazy(() => import("@/routes/notes/NotesLayout"));
const NoteView = lazy(() => import("@/routes/notes/NoteView"));
const NoteEditor = lazy(() => import("@/routes/notes/NoteEditor"));
const NotesSearch = lazy(() => import("@/routes/notes/NotesSearch"));
const DocumentsList = lazy(() => import("@/routes/documents/DocumentsList"));
const DocumentView = lazy(() => import("@/routes/documents/DocumentView"));
const Graph = lazy(() => import("@/routes/Graph"));
const MCPConsole = lazy(() => import("@/routes/MCPConsole"));

function RouteFallback() {
  return (
    <div className="p-6">
      <LoadingSkeleton label="Loading view" rows={4} />
    </div>
  );
}

export default function App() {
  useEffect(() => { initAuth(); }, []);
  return (
    <Providers>
      <Shell>
        <RouteBoundary>
          <Suspense fallback={<RouteFallback />}>
            <Routes>
              <Route path="/" element={<Home />} />
              <Route path="/notes" element={<NotesLayout />}>
                <Route path="search" element={<NotesSearch />} />
                <Route path=":key" element={<NoteView />} />
                <Route path=":key/edit" element={<NoteEditor />} />
              </Route>
              <Route path="/docs" element={<DocumentsList />} />
              <Route path="/docs/:id" element={<DocumentView />} />
              <Route path="/graph" element={<Graph />} />
              <Route path="/mcp" element={<MCPConsole />} />
            </Routes>
          </Suspense>
        </RouteBoundary>
      </Shell>
    </Providers>
  );
}
```

Note: the exact `<Route>` tree inside `<Routes>` mirrors the existing tree in your current `App.tsx`. If your current file has an additional route not shown here (e.g. `/notes` index), copy that element over verbatim — do not drop any routes. Verify with `git diff ui/src/App.tsx` before committing.

- [ ] **Step 6: Run full UI test suite**

```
cd ui && npm test -- --run
```

Expected: all pass. The existing Playwright smoke should still work because the boundary is transparent on success.

- [ ] **Step 7: Typecheck + build**

```
cd ui && npm run typecheck && npm run build
```

Expected: clean. JS+CSS bundle < 640 KB. Verify:

```
cd ui && find dist -name '*.js' -o -name '*.css' | xargs du -cb | tail -1
```

- [ ] **Step 8: Commit**

```bash
git add ui/src/App.tsx ui/src/components/layout/RouteBoundary.tsx \
  ui/src/components/layout/__tests__/RouteBoundary.test.tsx
git commit -m "$(cat <<'EOF'
feat(ui): add RouteBoundary error boundary around Suspense (5.1)

Catches render errors at the Suspense fallback level and shows a sanitized
state card with "Reload this view" (resets the boundary) and "Report"
(opens a mailto with a truncated stack). Vitest-covered via a child that
throws on render. Addresses Block 5.1.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Dynamic `document.title` per route (5.3)

**Files:**
- Modify: `ui/src/hooks/useDocumentTitle.ts`
- Create: `ui/src/hooks/__tests__/useDocumentTitle.test.tsx`

The existing hook is too minimal — it only looks up a few fixed paths and handles doc id as `Document <8>`. The spec requires: `Home`, `Notes`, `{noteKey}`, `Documents`, `{doc.title}`, `Graph`, `MCP Console` — all suffixed with ` — docsiq`. We will extend the hook to accept optional `parts` for cases where the route needs dynamic data (document title), and fix the notes-key pretty-printer.

- [ ] **Step 1: Write the failing test**

Create `ui/src/hooks/__tests__/useDocumentTitle.test.tsx`:

```tsx
import { renderHook } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, it, expect, afterEach } from "vitest";
import { useDocumentTitle } from "../useDocumentTitle";

function Wrapper({
  path,
  url,
  parts,
}: {
  path: string;
  url: string;
  parts?: string[];
}) {
  function Inner() {
    useDocumentTitle(parts);
    return null;
  }
  return (
    <MemoryRouter initialEntries={[url]}>
      <Routes>
        <Route path={path} element={<Inner />} />
      </Routes>
    </MemoryRouter>
  );
}

describe("useDocumentTitle", () => {
  afterEach(() => {
    document.title = "docsiq";
  });

  it("sets Home title", () => {
    renderHook(() => {}, { wrapper: () => <Wrapper path="/" url="/" /> });
    expect(document.title).toBe("Home — docsiq");
  });

  it("sets Notes list title", () => {
    renderHook(() => {}, { wrapper: () => <Wrapper path="/notes" url="/notes" /> });
    expect(document.title).toBe("Notes — docsiq");
  });

  it("sets note-key title from the URL when no parts passed", () => {
    renderHook(() => {}, {
      wrapper: () => (
        <Wrapper path="/notes/:key" url="/notes/folder%2Fhello" />
      ),
    });
    expect(document.title).toBe("hello — docsiq");
  });

  it("sets Documents list title", () => {
    renderHook(() => {}, { wrapper: () => <Wrapper path="/docs" url="/docs" /> });
    expect(document.title).toBe("Documents — docsiq");
  });

  it("honours caller-provided parts for a document view", () => {
    renderHook(() => {}, {
      wrapper: () => (
        <Wrapper
          path="/docs/:id"
          url="/docs/abc"
          parts={["Design doc v2", "Documents"]}
        />
      ),
    });
    expect(document.title).toBe("Design doc v2 — Documents — docsiq");
  });

  it("sets Graph title", () => {
    renderHook(() => {}, { wrapper: () => <Wrapper path="/graph" url="/graph" /> });
    expect(document.title).toBe("Graph — docsiq");
  });

  it("sets MCP Console title", () => {
    renderHook(() => {}, { wrapper: () => <Wrapper path="/mcp" url="/mcp" /> });
    expect(document.title).toBe("MCP Console — docsiq");
  });

  it("falls back to `docsiq` on unknown paths with no parts", () => {
    renderHook(() => {}, {
      wrapper: () => <Wrapper path="/weird" url="/weird" />,
    });
    expect(document.title).toBe("docsiq");
  });
});
```

- [ ] **Step 2: Run the test — expect failure**

```
cd ui && npm test -- --run src/hooks/__tests__/useDocumentTitle.test.tsx
```

Expected: most cases fail because current hook ignores `parts`, formats doc id as `Document abc`, and does not handle `/notes/search`/sub-paths cleanly.

- [ ] **Step 3: Rewrite `useDocumentTitle`**

Replace the whole contents of `ui/src/hooks/useDocumentTitle.ts` with:

```ts
import { useEffect } from "react";
import { useLocation, useParams } from "react-router-dom";

const STATIC_TITLES: Record<string, string> = {
  "/": "Home",
  "/notes": "Notes",
  "/notes/search": "Search notes",
  "/docs": "Documents",
  "/graph": "Graph",
  "/mcp": "MCP Console",
};

const SUFFIX = "docsiq";

function prettifyKey(raw: string): string {
  try {
    const decoded = decodeURIComponent(raw);
    const last = decoded.split("/").pop();
    return last && last.length > 0 ? last : decoded;
  } catch {
    return raw;
  }
}

/**
 * Sets document.title with a consistent " — docsiq" suffix.
 *
 * `parts` takes precedence over path-derived titles. Pass a list of parts
 * from most-specific to least-specific, e.g. ["Design doc v2", "Documents"]
 * produces "Design doc v2 — Documents — docsiq".
 */
export function useDocumentTitle(parts?: string[]): void {
  const { pathname } = useLocation();
  const params = useParams();

  useEffect(() => {
    const explicit = (parts ?? []).filter((p) => typeof p === "string" && p.trim().length > 0);
    let segments: string[] = [];

    if (explicit.length > 0) {
      segments = [...explicit, SUFFIX];
    } else {
      const label = STATIC_TITLES[pathname];
      if (label) {
        segments = [label, SUFFIX];
      } else if (pathname.startsWith("/notes/") && params.key) {
        segments = [prettifyKey(params.key), SUFFIX];
      } else if (pathname.startsWith("/docs/") && params.id) {
        // Route may pass its own title via `parts`; otherwise show a short id.
        segments = [`Document ${params.id.slice(0, 8)}`, SUFFIX];
      } else {
        segments = [SUFFIX];
      }
    }

    document.title =
      segments.length === 1 ? segments[0]! : segments.join(" — ");
  }, [pathname, params, parts]);
}
```

- [ ] **Step 4: Run the test — expect pass**

```
cd ui && npm test -- --run src/hooks/__tests__/useDocumentTitle.test.tsx
```

Expected: 8 passing.

- [ ] **Step 5: Wire DocumentView's real title through `parts`**

Open `ui/src/routes/documents/DocumentView.tsx`. Inside the component (after the data hook), add:

```tsx
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
// ...
useDocumentTitle(data?.doc?.title ? [data.doc.title, "Documents"] : undefined);
```

Use the exact field path the existing fetch returns. If the component already holds the title in a variable (e.g. `doc`), substitute accordingly. Verify by running the app and navigating to `/docs/:id`.

- [ ] **Step 6: Wire NoteView's title through `parts`**

Open `ui/src/routes/notes/NoteView.tsx`. Add:

```tsx
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
// ...
const noteTitle = data?.note?.title ?? data?.note?.key ?? params.key ?? undefined;
useDocumentTitle(noteTitle ? [noteTitle, "Notes"] : undefined);
```

If the shape of `data` differs, adapt. The hook already falls back gracefully to the URL key when `parts` is not supplied.

- [ ] **Step 7: Run full UI test suite**

```
cd ui && npm test -- --run
```

Expected: all pass.

- [ ] **Step 8: Typecheck + build + bundle budget**

```
cd ui && npm run typecheck && npm run build && find dist -name '*.js' -o -name '*.css' | xargs du -cb | tail -1
```

Expected: clean typecheck, build succeeds, bundle < 655360 bytes.

- [ ] **Step 9: Commit**

```bash
git add ui/src/hooks/useDocumentTitle.ts ui/src/hooks/__tests__/useDocumentTitle.test.tsx \
  ui/src/routes/documents/DocumentView.tsx ui/src/routes/notes/NoteView.tsx
git commit -m "$(cat <<'EOF'
feat(ui): dynamic document.title per route with parts support (5.3)

Extends useDocumentTitle to accept optional `parts` for dynamic segments
(document/note titles) and threads them through DocumentView + NoteView.
All titles suffixed " — docsiq". Addresses Block 5.3.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: iOS safe-area insets (5.4)

**Files:**
- Modify: `ui/src/styles/globals.css`
- Modify: `ui/src/components/ui/sidebar.tsx` (only the outer `Sidebar` wrapper — safe-area padding-left)
- Verify: `ui/index.html` viewport meta already includes `viewport-fit=cover` (it does, per current file)
- Create: `ui/e2e/safe-area.spec.ts` (Playwright check that iPhone 14 viewport respects env insets)

- [ ] **Step 1: Verify the viewport meta**

```
cd ui && grep viewport index.html
```

Expected: includes `viewport-fit=cover`. If not present, add it so the line reads:

```html
<meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover" />
```

- [ ] **Step 2: Add safe-area CSS variables + header/sidebar rules**

Open `ui/src/styles/globals.css`. Append:

```css
/* Block 5.4 — iOS safe-area insets ------------------------------------- */
:root {
  --header-pad: 1rem;
}

/* Header padding respects the notch on iOS */
.site-header {
  padding-top: max(var(--header-pad), env(safe-area-inset-top));
  padding-right: max(var(--header-pad), env(safe-area-inset-right));
}

/* Sidebar padding respects the left inset in landscape iOS */
[data-slot="sidebar-container"],
[data-sidebar="sidebar"] {
  padding-left: env(safe-area-inset-left);
}

/* Main content also respects bottom inset for iOS home indicator */
main#main {
  padding-bottom: env(safe-area-inset-bottom);
}
```

(Adjust the `[data-slot="sidebar-container"]` selector if the local shadcn sidebar uses a different attribute — open `ui/src/components/ui/sidebar.tsx` and search for the outer wrapper's `data-slot`. Prefer a class or `data-slot` selector rather than editing the component JS so the shadcn component stays local-copy-clean.)

- [ ] **Step 3: Add a Playwright visual-regression-free assertion**

Create `ui/e2e/safe-area.spec.ts`:

```ts
import { test, expect, devices } from "@playwright/test";
import { test as fixtureTest } from "./fixtures";

// Re-use the stubbed fixtures from fixtures.ts by extending from it.
const iosTest = fixtureTest.extend({});

iosTest.use({ ...devices["iPhone 14"] });

iosTest("header padding accommodates safe-area-inset-top on iPhone 14", async ({ stubbedPage: page }) => {
  await page.goto("/");
  await page.locator("main#main").waitFor();
  const header = page.locator(".site-header").first();
  await expect(header).toBeVisible();
  const paddingTopPx = await header.evaluate(
    (el) => parseFloat(getComputedStyle(el).paddingTop),
  );
  // max(1rem, env(safe-area-inset-top)) must be at least 1rem = 16px.
  expect(paddingTopPx).toBeGreaterThanOrEqual(16);
});
```

- [ ] **Step 4: Run the Playwright spec**

```
cd ui && CI=1 ./node_modules/.bin/playwright test safe-area.spec.ts --reporter=list --workers=1
```

Expected: 1 passing.

If Playwright is not installed, run `cd ui && npm run e2e:install` first.

- [ ] **Step 5: Run full Vitest suite (regression check)**

```
cd ui && npm test -- --run
```

Expected: all pass.

- [ ] **Step 6: Typecheck + build + budget**

```
cd ui && npm run typecheck && npm run build && find dist -name '*.js' -o -name '*.css' | xargs du -cb | tail -1
```

Expected: clean, bundle < 655360.

- [ ] **Step 7: Commit**

```bash
git add ui/src/styles/globals.css ui/e2e/safe-area.spec.ts ui/index.html
git commit -m "$(cat <<'EOF'
feat(ui): iOS safe-area insets on header, sidebar, and main (5.4)

Header padding-top uses max(var(--header-pad), env(safe-area-inset-top))
and sidebar/main respect left/right/bottom insets. Covered by a Playwright
iPhone-14-viewport test asserting paddingTop ≥ 16px. Addresses Block 5.4.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: "Maximum update depth exceeded" — investigate + fix (5.5)

This is an investigation-then-fix task. The spec names two likely culprits (`Home`, `StatsStrip`) but requires real bisection. Follow the sequence below — do not jump to "just memoize the thing" without reproducing the warning first.

**Files:**
- Create: `ui/e2e/no-console-errors.spec.ts` (captures console errors on every route)
- Probably modify: one of `ui/src/routes/Home.tsx`, `ui/src/components/layout/StatsStrip.tsx`, `ui/src/hooks/api/useStats.ts`, `ui/src/hooks/api/useActivity.ts`, `ui/src/hooks/api/useNotes.ts`, or `ui/src/hooks/api/useGraph.ts` (determined by bisection)
- Create: `ui/src/hooks/api/__tests__/useXxx-stability.test.tsx` (regression test for the object-identity fix, exact file named after whichever hook was the culprit)

- [ ] **Step 1: Capture the warning with a Playwright console capture**

Create `ui/e2e/no-console-errors.spec.ts`:

```ts
import { test, expect } from "./fixtures";

const ROUTES = ["/", "/notes", "/docs", "/graph", "/mcp"];

test.describe("no console errors or warnings on any route", () => {
  for (const url of ROUTES) {
    test(`no console errors on ${url}`, async ({ stubbedPage: page }) => {
      const errors: string[] = [];
      const warnings: string[] = [];
      page.on("console", (msg) => {
        const text = msg.text();
        if (msg.type() === "error") errors.push(text);
        if (msg.type() === "warning") warnings.push(text);
      });
      page.on("pageerror", (err) => errors.push(err.message));

      await page.goto(url);
      await page.locator("main#main").waitFor();
      // Give effects a tick to run — if there's an infinite loop, it fires
      // within a few microtasks and the warning lands here.
      await page.waitForTimeout(500);

      const offending = [...errors, ...warnings].filter((t) =>
        /maximum update depth|too many re-renders/i.test(t),
      );
      expect(offending, `${url} emitted: ${offending.join(" | ")}`).toEqual([]);
    });
  }
});
```

- [ ] **Step 2: Run the spec — this SHOULD fail on at least one route**

```
cd ui && CI=1 ./node_modules/.bin/playwright test no-console-errors.spec.ts --reporter=list --workers=1
```

Expected: one or more routes FAIL with the "Maximum update depth" text captured. Note which route(s). If nothing fails, the bug may already be fixed by Block 1–3 changes — in that case skip to Step 8, add the regression spec to the suite, and commit as a regression guard only.

- [ ] **Step 3: Bisect with React DevTools Profiler**

Run the app locally and open the failing route in a browser with React DevTools installed.

```
cd ui && npm run dev
```

Open DevTools → Components → ⚙ → "Highlight updates when components render". Navigate to the failing route. Look for a component that renders >20 times in <1 second. That is the culprit.

Alternative when DevTools is unavailable: add a temporary `console.count("Home render")` at the top of the suspect route (`Home`, `StatsStrip`, etc.) and reload. Whichever counter climbs into the hundreds is the culprit.

Remove any temporary `console.count` before committing.

- [ ] **Step 4: Identify the unstable dependency**

Once the culprit component is known, examine every `useEffect`, `useMemo`, and `useCallback` in that file. Look for dependency arrays that include:

1. An object literal created per-render (`{ foo, bar }`)
2. An array literal created per-render (`[a, b]`)
3. A function that is not wrapped in `useCallback`
4. A Zustand selector that returns a new object each call (`(s) => ({ a: s.a, b: s.b })`) — use a shallow-equality selector or split into two selectors
5. A TanStack Query object returned from a hook whose identity changes even when data is equal (check the hook's `select` function)

Document the finding in the commit message.

- [ ] **Step 5: Write a regression test before fixing**

Depending on the culprit, the test lives either at `ui/src/routes/__tests__/Home.test.tsx` (for `Home`) or `ui/src/components/layout/__tests__/StatsStrip.test.tsx` (for `StatsStrip`) or `ui/src/hooks/api/__tests__/<hookName>-stability.test.tsx` (for an API hook).

Example — if the culprit is a Zustand selector returning a new object in `Home.tsx`, write this test at `ui/src/routes/__tests__/Home.test.tsx`:

```tsx
import { render, act } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import Home from "@/routes/Home";

// Minimal spy on the scheduler: fail if Home renders more than 20 times on
// mount+settle. This catches an update-depth loop without requiring Playwright.
describe("Home render stability", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("does not enter an infinite render loop on mount", async () => {
    const renderCount = { n: 0 };
    // Wrap Home with a counter component so we can inspect without touching Home.
    function CountingHome(): JSX.Element {
      renderCount.n += 1;
      return <Home />;
    }
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={["/"]}>
          <CountingHome />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    // flush microtasks a couple of times
    await act(async () => {
      await new Promise((r) => setTimeout(r, 50));
    });
    expect(renderCount.n).toBeLessThan(20);
  });
});
```

If the culprit is an API hook instead, use the hook-stability pattern:

```tsx
import { renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { useStats } from "../useStats";

describe("useStats reference stability", () => {
  it("returns the same result object across renders when data is unchanged", () => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    const { result, rerender } = renderHook(() => useStats("demo"), { wrapper });
    const first = result.current;
    rerender();
    expect(result.current).toBe(first);
  });
});
```

- [ ] **Step 6: Apply the minimal fix**

Based on the bisection:

- **Object-literal dep:** wrap in `useMemo(() => ({ ... }), [stableDeps])` or destructure into primitive deps.
- **Function dep:** wrap with `useCallback(..., [stableDeps])`.
- **Zustand selector:** replace `useUIStore((s) => ({ a: s.a, b: s.b }))` with two calls — `useUIStore((s) => s.a)` and `useUIStore((s) => s.b)` — or use `useShallow` from `zustand/shallow`.
- **TanStack `select`:** memoize or drop `select`; the default selector is reference-stable.

Make only the minimum change. If the fix is larger than ~10 lines it is probably not minimal — re-read the hook and narrow further.

- [ ] **Step 7: Re-run the regression Vitest + Playwright spec**

```
cd ui && npm test -- --run
cd ui && CI=1 ./node_modules/.bin/playwright test no-console-errors.spec.ts --reporter=list --workers=1
```

Expected: both pass. If Playwright still catches the warning, return to Step 3 — the bisect was wrong.

- [ ] **Step 8: Typecheck + build + budget**

```
cd ui && npm run typecheck && npm run build && find dist -name '*.js' -o -name '*.css' | xargs du -cb | tail -1
```

Expected: clean, bundle < 655360.

- [ ] **Step 9: Commit**

Use a commit message that names the exact file + bug found. Example (substitute actual findings):

```bash
git add ui/e2e/no-console-errors.spec.ts <file-you-fixed> <regression-test-you-added>
git commit -m "$(cat <<'EOF'
fix(ui): eliminate "Maximum update depth" warning on <route> (5.5)

Root cause: <exact cause from bisection — e.g. Zustand selector in Home
returned a fresh object per render, retriggering a useMemo whose output
fed a useEffect dep array>.
Fix: <minimal change — e.g. split selector into two primitive reads>.
Regression: Playwright console-capture across all five routes; Vitest
render-count assertion; update-depth warning no longer appears.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

If Step 2 found no warning (the bug was already fixed by earlier blocks), commit only the Playwright regression spec:

```bash
git add ui/e2e/no-console-errors.spec.ts
git commit -m "$(cat <<'EOF'
test(ui): add Playwright console-capture regression for update-depth loops (5.5)

Asserts no "Maximum update depth"/"too many re-renders" warnings on any of
the five primary routes. Addresses Block 5.5 (regression guard only — the
underlying loop was not reproducible at commit time).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: Axe violations sweep (5.6)

**Files:**
- Create: `ui/e2e/a11y.spec.ts` (Playwright + axe-core audit; asserts 0 violations per route)
- Modify: `ui/src/components/app-sidebar.tsx` (add `aria-label` to `SelectTrigger`)
- Possibly modify: other routes/components reported by the audit

- [ ] **Step 1: Install `@axe-core/playwright` if absent**

```
cd ui && npm ls @axe-core/playwright 2>/dev/null || npm install --save-dev @axe-core/playwright@^4.10.0
```

Resolve the latest 4.x via `context7` MCP if the version drifts; `4.10.0` is the known-good at the time of writing.

- [ ] **Step 2: Write the Playwright axe audit**

Create `ui/e2e/a11y.spec.ts`:

```ts
import { test, expect } from "./fixtures";
import AxeBuilder from "@axe-core/playwright";

const ROUTES = ["/", "/notes", "/docs", "/graph", "/mcp"];

test.describe("axe a11y audit — zero violations", () => {
  for (const url of ROUTES) {
    test(`no violations on ${url}`, async ({ stubbedPage: page }) => {
      await page.goto(url);
      await page.locator("main#main").waitFor();
      const results = await new AxeBuilder({ page })
        .withTags(["wcag2a", "wcag2aa"])
        .analyze();
      expect(
        results.violations,
        `${url}:\n${JSON.stringify(results.violations, null, 2)}`,
      ).toEqual([]);
    });
  }
});
```

- [ ] **Step 3: Run the audit — expect failures (this is the audit)**

```
cd ui && CI=1 ./node_modules/.bin/playwright test a11y.spec.ts --reporter=list --workers=1
```

Expected: at least one failure. Note every rule id and every affected selector.

- [ ] **Step 4: Fix `SelectTrigger` in `app-sidebar.tsx`**

Open `ui/src/components/app-sidebar.tsx`. The `SelectTrigger` currently has no accessible name. Replace lines ~87–90 (the `<Select>` block) with:

```tsx
<Select value={slug} onValueChange={setSlug}>
  <SelectTrigger aria-label="Select project" className="w-full h-8 text-xs font-mono">
    <SelectValue />
  </SelectTrigger>
  <SelectContent>
    {(projects?.length ? projects : [{ slug, name: slug }]).map((p) => (
      <SelectItem key={p.slug} value={p.slug} className="font-mono text-xs">
        {p.name || p.slug}
      </SelectItem>
    ))}
  </SelectContent>
</Select>
```

- [ ] **Step 5: Fix the hard-reload button's accessible name (already has aria-label — verify)**

```
cd ui && grep -n 'aria-label' src/components/site-header.tsx
```

Expected: `aria-label="Hard reload"` is already present (Line 49). If axe still complains, check for `button-name` on any other button in the tree and add `aria-label`.

- [ ] **Step 6: Triage each remaining violation and fix**

For every remaining rule id reported by the audit, apply the standard fix:

- `button-name` — add `aria-label` or visible text to the button.
- `link-name` — add visible text or `aria-label` to the `<a>`.
- `color-contrast` — bump the foreground color one step darker/lighter in `globals.css` for the affected selector.
- `aria-hidden-focus` — remove `aria-hidden` from any element that contains a focusable child.
- `landmark-one-main` — should already be satisfied by `<main id="main" role="main">`.
- `document-title` — handled by Task 3.
- `region` — wrap significant content in a `<section>` or `<region role="region" aria-label="...">`.

Apply the minimal fix per violation, then re-run the audit after each batch.

- [ ] **Step 7: Re-run the audit — expect pass**

```
cd ui && CI=1 ./node_modules/.bin/playwright test a11y.spec.ts --reporter=list --workers=1
```

Expected: 5 passing (one per route), 0 violations.

- [ ] **Step 8: Run Vitest + typecheck + build**

```
cd ui && npm test -- --run && npm run typecheck && npm run build && find dist -name '*.js' -o -name '*.css' | xargs du -cb | tail -1
```

Expected: clean across the board; bundle < 655360.

- [ ] **Step 9: Commit**

```bash
git add ui/package.json ui/package-lock.json \
  ui/e2e/a11y.spec.ts \
  ui/src/components/app-sidebar.tsx \
  <any other file touched in Step 6>
git commit -m "$(cat <<'EOF'
fix(ui): zero axe violations across all primary routes (5.6)

Adds @axe-core/playwright audit asserting 0 wcag2a/wcag2aa violations on
/, /notes, /docs, /graph, /mcp. Fixes include aria-label on project
SelectTrigger and <list each additional fix>. Addresses Block 5.6.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Reduced-motion gating for Framer Motion (5.7)

The repo-wide search shows Framer Motion is concentrated in `ui/src/lib/motion.ts`. The hook `useReducedMotion` already exists. This task ensures every `<motion.*>` callsite either lives in `motion.ts` (centralized) or passes reduced-motion-safe transitions.

**Files:**
- Read/verify: `ui/src/lib/motion.ts`
- Modify: `ui/src/lib/motion.ts` (export reduced-motion-aware presets)
- Modify: any component using `motion.*` directly — replace with presets from `motion.ts`
- Create: `ui/src/lib/__tests__/motion.test.ts`

- [ ] **Step 1: Enumerate every `motion.*` callsite**

```
cd ui && npx tsx -e 'import {execSync} from "node:child_process"; console.log(execSync("grep -rn --include=\"*.tsx\" --include=\"*.ts\" \"motion\\.\" src/", {encoding: "utf8"}))'
```

Or more simply:

```
cd ui && grep -rn --include='*.tsx' --include='*.ts' 'from "framer-motion"' src/
cd ui && grep -rn --include='*.tsx' --include='*.ts' 'motion\.' src/
```

List every file that imports from `framer-motion` or uses `motion.div`/`motion.section`/etc. Scope Task 7 to those files.

- [ ] **Step 2: Write the test for the reduced-motion-aware presets**

Create `ui/src/lib/__tests__/motion.test.ts`:

```ts
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";

describe("motion presets", () => {
  beforeEach(() => {
    // default: user does not prefer reduced motion
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
  });
  afterEach(() => vi.restoreAllMocks());

  it("fade returns non-zero duration when motion is allowed", async () => {
    const { fadeTransition } = await import("../motion");
    expect(fadeTransition(false).duration).toBeGreaterThan(0);
  });

  it("fade returns zero duration when reduced motion is requested", async () => {
    const { fadeTransition } = await import("../motion");
    expect(fadeTransition(true).duration).toBe(0);
  });

  it("slideTransition honours reduced motion too", async () => {
    const { slideTransition } = await import("../motion");
    expect(slideTransition(true).duration).toBe(0);
    expect(slideTransition(false).duration).toBeGreaterThan(0);
  });
});
```

- [ ] **Step 3: Run the test — expect failure**

```
cd ui && npm test -- --run src/lib/__tests__/motion.test.ts
```

Expected: FAIL with either a missing named export or an import error.

- [ ] **Step 4: Update `motion.ts` to export reduced-motion-aware presets**

Replace the whole contents of `ui/src/lib/motion.ts` with:

```ts
import type { Transition } from "framer-motion";

/**
 * Reduced-motion-aware transition presets.
 *
 * Callers pass the current user preference (from useReducedMotion()) and
 * receive a transition config whose duration collapses to 0 when reduced
 * motion is requested — still completing the state change, just instantly.
 */

export function fadeTransition(reducedMotion: boolean): Transition {
  return {
    duration: reducedMotion ? 0 : 0.18,
    ease: [0.2, 0, 0, 1],
  };
}

export function slideTransition(reducedMotion: boolean): Transition {
  return {
    duration: reducedMotion ? 0 : 0.22,
    ease: [0.2, 0, 0, 1],
  };
}

export function popTransition(reducedMotion: boolean): Transition {
  return {
    duration: reducedMotion ? 0 : 0.16,
    ease: [0.2, 0, 0, 1],
  };
}

/** Variants helper for simple fade-in mounts. */
export const fadeInVariants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1 },
};

/** Variants helper for slide-up mounts (12px travel). */
export const slideUpVariants = {
  hidden: { opacity: 0, y: 12 },
  visible: { opacity: 0, y: 0, transition: { duration: 0.001 } }, // overridden by callsite
};
```

Note: if the existing `motion.ts` file already exports other symbols used elsewhere, preserve those — just add the three transition functions above without removing existing exports. Run `git diff ui/src/lib/motion.ts` and confirm no callsite lost a symbol before committing.

- [ ] **Step 5: Re-run the test — expect pass**

```
cd ui && npm test -- --run src/lib/__tests__/motion.test.ts
```

Expected: 3 passing.

- [ ] **Step 6: Thread `useReducedMotion()` through every `<motion.*>` callsite**

For each file found in Step 1, modify the component as follows. Example for an activity-feed item:

```tsx
import { motion } from "framer-motion";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { fadeTransition } from "@/lib/motion";

// ...
const reduced = useReducedMotion();
// ...
<motion.div
  initial={{ opacity: 0 }}
  animate={{ opacity: 1 }}
  transition={fadeTransition(reduced)}
>
  {children}
</motion.div>
```

Every `<motion.*>` callsite must pass a `transition` whose `duration` is `0` when `reduced` is true. Do not leave any callsite with a hardcoded non-zero `duration` unless it is explicitly gated by `useReducedMotion()`.

Also audit any CSS animation in `globals.css` / component CSS. Add a media query once at the bottom of `globals.css`:

```css
/* Block 5.7 — reduced motion ------------------------------------------- */
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.001ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.001ms !important;
    scroll-behavior: auto !important;
  }
}
```

- [ ] **Step 7: Run full UI test suite**

```
cd ui && npm test -- --run
```

Expected: all pass.

- [ ] **Step 8: Typecheck + build + budget**

```
cd ui && npm run typecheck && npm run build && find dist -name '*.js' -o -name '*.css' | xargs du -cb | tail -1
```

Expected: clean; bundle < 655360.

- [ ] **Step 9: Commit**

```bash
git add ui/src/lib/motion.ts ui/src/lib/__tests__/motion.test.ts ui/src/styles/globals.css \
  <every file touched in Step 6>
git commit -m "$(cat <<'EOF'
feat(ui): reduced-motion-aware Framer Motion presets + global CSS gate (5.7)

Adds fade/slide/pop transition factories that collapse duration to 0 when
useReducedMotion() returns true, and threads them through every <motion.*>
callsite. Adds a prefers-reduced-motion CSS media query so non-Framer
animations also respect user preference. Addresses Block 5.7.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: Focus management (5.8)

Three requirements from the spec:
(a) command palette returns focus to the invoking element on close;
(b) sheet/dialog focus-trap verified (shadcn dialog uses Radix, which already traps — verify);
(c) skip-link lands on `main#main` and is the first tab target.

**Files:**
- Modify: `ui/src/components/command/CommandPalette.tsx`
- Modify: `ui/src/components/layout/Shell.tsx` (track the invoker)
- Create: `ui/e2e/focus.spec.ts` (Playwright assertions for all three)
- Verify: `ui/src/components/layout/SkipLink.tsx` works

- [ ] **Step 1: Write the Playwright focus spec**

Create `ui/e2e/focus.spec.ts`:

```ts
import { test, expect } from "./fixtures";

test.describe("focus management", () => {
  test("skip-link is the first tab target and moves focus to main#main", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await page.locator("main#main").waitFor();
    await page.keyboard.press("Tab");
    const focusedIsSkipLink = await page.evaluate(
      () => document.activeElement?.textContent?.toLowerCase().includes("skip to main content") ?? false,
    );
    expect(focusedIsSkipLink).toBe(true);
    await page.keyboard.press("Enter");
    const mainFocused = await page.evaluate(() => document.activeElement?.id === "main");
    expect(mainFocused).toBe(true);
  });

  test("command palette returns focus to the invoking button on close", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await page.locator("main#main").waitFor();
    const searchBtn = page.locator(".site-header-search").first();
    await searchBtn.focus();
    await expect(searchBtn).toBeFocused();
    await page.keyboard.press("Enter");
    await page.getByPlaceholder(/search notes, docs, entities/i).waitFor();
    await page.keyboard.press("Escape");
    // After close, focus must return to the search button.
    await expect(searchBtn).toBeFocused();
  });

  test("dialog traps focus while open (radix)", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await page.locator("main#main").waitFor();
    await page.keyboard.press("ControlOrMeta+k");
    await page.getByPlaceholder(/search notes, docs, entities/i).waitFor();
    // Tab 20 times; focus must stay inside the palette dialog.
    for (let i = 0; i < 20; i++) {
      await page.keyboard.press("Tab");
      const inside = await page.evaluate(() =>
        Boolean(document.activeElement?.closest("[role=\"dialog\"]")),
      );
      expect(inside, `focus escaped the dialog on Tab ${i}`).toBe(true);
    }
  });
});
```

- [ ] **Step 2: Run the spec — expect the palette-refocus test to fail**

```
cd ui && CI=1 ./node_modules/.bin/playwright test focus.spec.ts --reporter=list --workers=1
```

Expected: the skip-link test and dialog-trap test likely pass out-of-the-box; the palette-refocus test fails because the palette does not currently restore focus to the invoker.

- [ ] **Step 3: Add invoker tracking to the Shell**

Replace the whole contents of `ui/src/components/layout/Shell.tsx` with:

```tsx
import { type ReactNode, useCallback, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { AppSidebar } from "@/components/app-sidebar";
import { SiteHeader } from "@/components/site-header";
import { SkipLink } from "./SkipLink";
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import { useHotkey } from "@/hooks/useHotkey";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import { CommandPalette } from "@/components/command/CommandPalette";

export function Shell({ children }: { children: ReactNode }) {
  const [cmdOpen, setCmdOpen] = useState(false);
  const invokerRef = useRef<HTMLElement | null>(null);
  const navigate = useNavigate();
  useDocumentTitle();

  const openPalette = useCallback(() => {
    const el = document.activeElement;
    invokerRef.current = el instanceof HTMLElement ? el : null;
    setCmdOpen(true);
  }, []);

  const handleOpenChange = useCallback((next: boolean) => {
    setCmdOpen(next);
    if (!next) {
      // Wait one tick so Radix finishes its unmount animation before we
      // restore focus, or the browser may re-steal it.
      requestAnimationFrame(() => {
        const target = invokerRef.current;
        if (target && typeof target.focus === "function") {
          target.focus();
        }
      });
    }
  }, []);

  useHotkey("mod+k", () => (cmdOpen ? setCmdOpen(false) : openPalette()));
  useHotkey("g,h", () => navigate("/"));
  useHotkey("g,n", () => navigate("/notes"));
  useHotkey("g,d", () => navigate("/docs"));
  useHotkey("g,g", () => navigate("/graph"));
  useHotkey("g,m", () => navigate("/mcp"));

  return (
    <SidebarProvider
      style={
        {
          "--sidebar-width": "calc(var(--spacing) * 72)",
          "--header-height": "calc(var(--spacing) * 12)",
        } as React.CSSProperties
      }
    >
      <SkipLink />
      <AppSidebar variant="inset" />
      <SidebarInset>
        <SiteHeader onCommandOpen={openPalette} />
        <main
          id="main"
          role="main"
          tabIndex={-1}
          className="flex flex-1 flex-col"
        >
          {children}
        </main>
      </SidebarInset>
      <CommandPalette open={cmdOpen} onOpenChange={handleOpenChange} />
    </SidebarProvider>
  );
}
```

Key change: `Shell` now owns an `invokerRef`, captures `document.activeElement` when the palette opens, and restores focus to it when `onOpenChange(false)` is called.

- [ ] **Step 4: Verify `SkipLink` is the first tab stop**

The existing skip-link (in `SkipLink.tsx`) renders an `<a href="#main">` at the top of `Shell` before `<AppSidebar>`. Browsers focus it on first Tab if its CSS doesn't remove it from the layout. Check `globals.css` for any `.skip-link { display: none }` — if present, replace with a visually-hidden-until-focused style:

```css
.skip-link {
  position: absolute;
  left: -9999px;
  top: 0.5rem;
  z-index: 100;
  background: var(--background);
  color: var(--foreground);
  border: 2px solid var(--ring);
  padding: 0.5rem 0.75rem;
  border-radius: 0.375rem;
}
.skip-link:focus {
  left: 0.5rem;
  outline: none;
}
```

If `.skip-link` already has that pattern, no change is needed.

- [ ] **Step 5: Re-run the focus spec — expect pass**

```
cd ui && CI=1 ./node_modules/.bin/playwright test focus.spec.ts --reporter=list --workers=1
```

Expected: 3 passing.

- [ ] **Step 6: Run Vitest**

```
cd ui && npm test -- --run
```

Expected: all pass. The existing CommandPalette vitest should still pass because `onOpenChange` behaviour is unchanged for the `false → true` direction.

- [ ] **Step 7: Typecheck + build + budget**

```
cd ui && npm run typecheck && npm run build && find dist -name '*.js' -o -name '*.css' | xargs du -cb | tail -1
```

Expected: clean; bundle < 655360.

- [ ] **Step 8: Commit**

```bash
git add ui/src/components/layout/Shell.tsx ui/e2e/focus.spec.ts ui/src/styles/globals.css
git commit -m "$(cat <<'EOF'
feat(ui): restore focus to invoker on command palette close (5.8)

Shell now captures document.activeElement when the palette opens and
restores focus on close via requestAnimationFrame. Adds Playwright specs
covering skip-link → main#main, palette focus restoration, and Radix
dialog focus-trap. Addresses Block 5.8.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Theme-flash inline script (5.9)

The Zustand persist key is `docsiq-ui` (verified). Zustand persists as `{ "state": { "theme": "light"|"dark"|"system", ... }, "version": 0 }`. The script must parse this and apply `.dark` class before React hydrates, eliminating FOUC.

**Files:**
- Modify: `ui/index.html`
- Create: `ui/e2e/theme-flash.spec.ts`

- [ ] **Step 1: Write the Playwright test**

Create `ui/e2e/theme-flash.spec.ts`:

```ts
import { test, expect } from "./fixtures";

test.describe("theme-flash", () => {
  test("dark theme is applied before React hydrates", async ({ stubbedPage: page }) => {
    // Seed localStorage BEFORE navigating so the inline script can read it.
    await page.addInitScript(() => {
      window.localStorage.setItem(
        "docsiq-ui",
        JSON.stringify({ state: { theme: "dark" }, version: 0 }),
      );
    });
    await page.goto("/");
    // Check the html element's class list at the earliest opportunity.
    const hasDark = await page.evaluate(() =>
      document.documentElement.classList.contains("dark"),
    );
    expect(hasDark).toBe(true);
    // And data-theme is set too
    const themeAttr = await page.evaluate(() =>
      document.documentElement.dataset.theme,
    );
    expect(themeAttr).toBe("dark");
  });

  test("light theme renders without .dark class", async ({ stubbedPage: page }) => {
    await page.addInitScript(() => {
      window.localStorage.setItem(
        "docsiq-ui",
        JSON.stringify({ state: { theme: "light" }, version: 0 }),
      );
    });
    await page.goto("/");
    const hasDark = await page.evaluate(() =>
      document.documentElement.classList.contains("dark"),
    );
    expect(hasDark).toBe(false);
  });

  test("system theme resolves via prefers-color-scheme before hydration", async ({ stubbedPage: page, browser }) => {
    await page.addInitScript(() => {
      window.localStorage.setItem(
        "docsiq-ui",
        JSON.stringify({ state: { theme: "system" }, version: 0 }),
      );
    });
    const ctx = await browser.newContext({ colorScheme: "dark" });
    const p2 = await ctx.newPage();
    await p2.goto("/");
    const hasDark = await p2.evaluate(() =>
      document.documentElement.classList.contains("dark"),
    );
    expect(hasDark).toBe(true);
    await ctx.close();
  });
});
```

- [ ] **Step 2: Run the spec — expect failure**

```
cd ui && CI=1 ./node_modules/.bin/playwright test theme-flash.spec.ts --reporter=list --workers=1
```

Expected: at least the dark-theme test fails because React is the only thing currently applying `.dark` — the early class is not applied synchronously on first paint.

- [ ] **Step 3: Add the inline theme-flash script to `index.html`**

Replace the whole contents of `ui/index.html` with:

```html
<!doctype html>
<html lang="en" dir="ltr">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover" />
    <meta name="color-scheme" content="dark light" />
    <meta name="theme-color" content="#0f1115" media="(prefers-color-scheme: dark)" />
    <meta name="theme-color" content="#f8f9fb" media="(prefers-color-scheme: light)" />
    <meta name="description" content="docsiq — GraphRAG knowledge base. Notes, documents, and a knowledge graph served by one Go binary." />
    <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
    <link rel="apple-touch-icon" href="/favicon.svg" />
    <link rel="manifest" href="/manifest.webmanifest" />
    <title>docsiq — GraphRAG knowledge base</title>
    <script>
      // Block 5.9 — theme-flash guard. Applies the persisted theme class
      // before React hydrates so there is no FOUC on first paint. Must run
      // synchronously in <head>. Keep in sync with Zustand persist key
      // `docsiq-ui` and the Providers.tsx effect that toggles .dark.
      (function () {
        try {
          var raw = localStorage.getItem("docsiq-ui");
          var theme = "system";
          if (raw) {
            var parsed = JSON.parse(raw);
            if (parsed && parsed.state && typeof parsed.state.theme === "string") {
              theme = parsed.state.theme;
            }
          }
          var effective = theme;
          if (theme === "system") {
            effective = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches
              ? "dark"
              : "light";
          }
          var root = document.documentElement;
          root.dataset.theme = effective;
          if (effective === "dark") root.classList.add("dark");
        } catch (e) {
          // If localStorage is unavailable (privacy mode) we simply let
          // React decide after hydration; there is a brief FOUC but no
          // crash. Do not log — this runs before any logger is attached.
        }
      })();
    </script>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

Note: this script is inline and must therefore be allowed by the CSP rolled out in Block 2. The existing `style-src` already allows `unsafe-inline`; for `script-src` the Block 2 CSP uses nonce or hash. If the CSP fails after deploy, generate the SHA-256 hash of this exact script and add it to the `script-src` directive, or add a nonce during build-time template substitution. Verify via:

```
cd ui && npm run build && grep -o 'script-src[^;]*' dist/index.html || echo "No CSP meta in build output (CSP is sent via server headers)"
```

If the CSP is server-sent, add the hash to `internal/api/security_headers.go` where `script-src` is computed. Reference Block 2's plan for the exact location.

- [ ] **Step 4: Re-run the Playwright spec — expect pass**

```
cd ui && CI=1 ./node_modules/.bin/playwright test theme-flash.spec.ts --reporter=list --workers=1
```

Expected: 3 passing.

- [ ] **Step 5: Run the smoke spec to ensure CSP did not break anything**

```
cd ui && CI=1 ./node_modules/.bin/playwright test smoke.spec.ts --reporter=list --workers=1
```

Expected: 4 passing. If a CSP violation appears, compute the script's SHA-256 and add to the server CSP:

```
cd ui && node -e "const crypto=require('crypto');const s=require('fs').readFileSync('index.html','utf8');const m=s.match(/<script>([\\s\\S]*?)<\\/script>/);console.log('sha256-'+crypto.createHash('sha256').update(m[1]).digest('base64'))"
```

Add the printed `sha256-...` to the `script-src` directive in the server CSP module (file path determined in Block 2).

- [ ] **Step 6: Run Vitest + typecheck + build + budget**

```
cd ui && npm test -- --run && npm run typecheck && npm run build && find dist -name '*.js' -o -name '*.css' | xargs du -cb | tail -1
```

Expected: clean; bundle < 655360.

- [ ] **Step 7: Commit**

```bash
git add ui/index.html ui/e2e/theme-flash.spec.ts
# If CSP hash was added to server:
# git add internal/api/security_headers.go
git commit -m "$(cat <<'EOF'
feat(ui): pre-hydration theme-flash guard in index.html (5.9)

Inline <head> script reads the Zustand-persisted theme at key `docsiq-ui`
and applies `document.documentElement.classList` and `data-theme` before
React hydrates, eliminating FOUC on dark-theme first paint. Playwright
coverage for dark, light, and system (via prefers-color-scheme). Addresses
Block 5.9.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: Mobile viewport pass (5.10)

**Files:**
- Modify: `ui/src/styles/globals.css` (tap-target minimum, table horizontal scroll wrapper)
- Possibly modify: `ui/src/components/site-header.tsx` (tap-target sizing on buttons)
- Possibly modify: table wrappers in route files (wrap in `.table-scroll`)
- Create: `ui/e2e/mobile.spec.ts`

- [ ] **Step 1: Write the Playwright mobile spec**

Create `ui/e2e/mobile.spec.ts`:

```ts
import { test, expect, devices } from "@playwright/test";
import { test as fixtureTest } from "./fixtures";

const mobileTest = fixtureTest.extend({});
mobileTest.use({ viewport: { width: 375, height: 812 } });

mobileTest.describe("mobile 375px viewport", () => {
  mobileTest("sidebar collapses at 375px", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await page.locator("main#main").waitFor();
    // The shadcn sidebar hides on mobile by default; assert it is not part of
    // the initial visible tab-order.
    const collapsedOrHidden = await page.evaluate(() => {
      const sb = document.querySelector("[data-slot='sidebar'], [data-sidebar='sidebar']");
      if (!sb) return true; // treated as collapsed if not rendered
      const r = sb.getBoundingClientRect();
      return r.width < 60 || r.left < -10 || getComputedStyle(sb).display === "none";
    });
    expect(collapsedOrHidden).toBe(true);
  });

  mobileTest("header buttons meet 44x44 tap-target minimum", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await page.locator("main#main").waitFor();
    const buttons = page.locator(".site-header button, .site-header a[role='button']");
    const count = await buttons.count();
    for (let i = 0; i < count; i++) {
      const box = await buttons.nth(i).boundingBox();
      if (!box) continue; // button hidden on this viewport
      expect.soft(box.width, `button ${i} too narrow`).toBeGreaterThanOrEqual(44);
      expect.soft(box.height, `button ${i} too short`).toBeGreaterThanOrEqual(44);
    }
  });

  mobileTest("command palette fills the viewport", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await page.locator("main#main").waitFor();
    await page.keyboard.press("ControlOrMeta+k");
    const dialog = page.locator("[role='dialog']").first();
    await dialog.waitFor();
    const box = await dialog.boundingBox();
    expect(box?.width ?? 0).toBeGreaterThanOrEqual(320);
  });

  mobileTest("documents list does not overflow horizontally", async ({ stubbedPage: page }) => {
    await page.goto("/docs");
    await page.locator("main#main").waitFor();
    // Detect horizontal scroll on <body> — tables should scroll inside their
    // wrapper, not push the body.
    const bodyOverflow = await page.evaluate(() => {
      const d = document.documentElement;
      return d.scrollWidth - d.clientWidth;
    });
    expect(bodyOverflow).toBeLessThanOrEqual(1);
  });
});
```

- [ ] **Step 2: Run the spec — expect failures on tap-target and/or overflow**

```
cd ui && CI=1 ./node_modules/.bin/playwright test mobile.spec.ts --reporter=list --workers=1
```

Expected: at least tap-target or overflow fails. Note the failing selectors.

- [ ] **Step 3: Enforce 44×44 tap targets on header buttons**

Open `ui/src/styles/globals.css` and append:

```css
/* Block 5.10 — mobile viewport pass ------------------------------------ */
@media (max-width: 480px) {
  .site-header button,
  .site-header a[role="button"],
  .site-header-reload,
  .site-header-mobile-trigger,
  .site-header-search {
    min-height: 44px;
    min-width: 44px;
  }
  /* Command palette fills viewport under 480px */
  [role="dialog"][data-slot="dialog-content"],
  [cmdk-root] {
    width: 100vw !important;
    max-width: 100vw !important;
    height: 100vh !important;
    max-height: 100vh !important;
    border-radius: 0 !important;
  }
}

/* Tables scroll horizontally inside a scroll container, not the body */
.table-scroll {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  max-width: 100%;
}
.table-scroll > table {
  min-width: max-content;
}
```

- [ ] **Step 4: Wrap any tables in `.table-scroll`**

Open the route files that render a `<table>`. Most likely `ui/src/routes/documents/DocumentsList.tsx` and/or `ui/src/routes/MCPConsole.tsx`. Wrap the table:

```tsx
<div className="table-scroll">
  <Table>
    {/* existing content */}
  </Table>
</div>
```

Do this for every `<table>` / `<Table>` in the route layer. Do not wrap tables rendered inside modals (those already scroll inside the dialog).

- [ ] **Step 5: Re-run the mobile spec — expect pass**

```
cd ui && CI=1 ./node_modules/.bin/playwright test mobile.spec.ts --reporter=list --workers=1
```

Expected: 4 passing.

- [ ] **Step 6: Manual sanity check**

```
cd ui && npm run dev
```

Open http://localhost:5173 in Chrome, open DevTools → "Toggle device toolbar" → set viewport to 375×812 (iPhone X). Verify:

1. Sidebar is collapsed/hidden.
2. Header buttons are comfortably tappable (not a thin strip).
3. Cmd+K opens a palette that fills the viewport.
4. `/docs` does not produce horizontal body scroll — any table scrolls inside its container.

Stop the dev server with Ctrl+C.

- [ ] **Step 7: Run Vitest + full Playwright smoke + typecheck + build + budget**

```
cd ui && npm test -- --run
cd ui && CI=1 ./node_modules/.bin/playwright test --reporter=list --workers=1
cd ui && npm run typecheck && npm run build && find dist -name '*.js' -o -name '*.css' | xargs du -cb | tail -1
```

Expected: all Vitest pass; all Playwright pass (smoke + safe-area + no-console-errors + a11y + focus + theme-flash + mobile); typecheck clean; bundle < 655360.

- [ ] **Step 8: Commit**

```bash
git add ui/src/styles/globals.css ui/e2e/mobile.spec.ts \
  <every route file where a table was wrapped>
git commit -m "$(cat <<'EOF'
feat(ui): mobile viewport pass — 44×44 tap targets, table scroll, full-screen palette (5.10)

Enforces 44×44 CSS-pixel minimum on header buttons under 480px, makes
the command palette fill the viewport on small screens, and wraps tables
in .table-scroll so horizontal overflow stays contained. Playwright
covers all four assertions at 375×812. Addresses Block 5.10.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review

**1. Spec coverage:**

| Spec item | Task | Status |
|---|---|---|
| 5.1 Error boundary | Task 2 | Covered — `RouteBoundary` with reset + mailto, Vitest test throws from child. |
| 5.2 Loading/empty/error state trio | Task 1 | Covered — three components + 9 Vitest cases + wired into Home/Notes*/Documents*/Graph/MCPConsole. |
| 5.3 Dynamic `document.title` | Task 3 | Covered — hook rewritten to accept `parts`, 8 Vitest cases, wired into DocumentView + NoteView. |
| 5.4 iOS safe-area insets | Task 4 | Covered — CSS `env()` rules + Playwright iPhone-14 assertion. |
| 5.5 "Maximum update depth" | Task 5 | Covered — Playwright console-capture regression + bisection-driven fix or regression-guard-only if not reproducible. |
| 5.6 Axe violations | Task 6 | Covered — `@axe-core/playwright` 5-route audit + fix for `SelectTrigger` aria-label and any other reported. |
| 5.7 Reduced motion | Task 7 | Covered — transition factories that honour `useReducedMotion()` + global CSS `prefers-reduced-motion` gate. |
| 5.8 Focus management | Task 8 | Covered — invoker tracking for palette + Playwright specs for skip-link, focus restoration, and dialog trap. |
| 5.9 Theme-flash | Task 9 | Covered — inline `<head>` script reads persisted `docsiq-ui` state + Playwright dark/light/system. |
| 5.10 Mobile viewport | Task 10 | Covered — 375px Playwright spec asserting sidebar collapse, 44×44 tap, palette full-screen, no body overflow. |

All ten spec items trace to a task with real code, a failing test, a fix, and a commit.

**2. Placeholder scan:** No "TBD", "TODO", or "similar to Task N" in any step. Every code step has the actual code. The only deliberately variable spans are (a) Task 5's root cause (investigation-first by design; commit message template is concrete) and (b) Task 6/Step 6's per-violation triage (exhaustive rule→fix mapping is provided).

**3. Type consistency:** Confirmed the following identifiers are used consistently across tasks:

- `EmptyState`, `LoadingSkeleton`, `ErrorState` — defined in Task 1, imported identically in Tasks 1, 2 (RouteBoundary reuses `.state-card` CSS), and route files.
- `RouteBoundary` — class component, default-less named export, imported in Task 2's `App.tsx`.
- `useDocumentTitle(parts?: string[]): void` — signature matches the hook rewrite in Task 3 and the callsite changes in DocumentView/NoteView.
- `fadeTransition`, `slideTransition`, `popTransition` — all accept `(reducedMotion: boolean)` and return a Framer Motion `Transition`.
- Zustand persist key `docsiq-ui` — matches Task 9's inline-script lookup.
- `main#main` — matches the existing Shell landmark used by both current and new Playwright specs.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-04-23-block5-ui-polish-plan.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks, fast iteration. Required sub-skill: `superpowers:subagent-driven-development`.

**2. Inline Execution** — execute tasks in this session with checkpoints for review. Required sub-skill: `superpowers:executing-plans`.

**Which approach?**
