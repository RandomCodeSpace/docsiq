# docsiq UI Redesign — Design Spec

**Date:** 2026-04-18
**Status:** Ready for user review
**Supersedes:** current hand-rolled React + CSS UI under `ui/src/`
**Follows rules:** `~/.claude/rules/{ui,build,testing,performance,dependencies,security,git}.md`

---

## 1. Goal

Greenfield redesign of docsiq's embedded web UI. One coherent, keyboard-first, responsive interface for an investigator-style workflow, replacing the hand-rolled React + CSS implementation that grew from the kgraph port.

Backend API and MCP surface remain unchanged (acceptance criterion). UI ships as static assets embedded via `//go:embed ui/dist/` in the Go binary — same distribution story as today.

---

## 2. User profile + dominant workflow

**Solo developer. UI-secondary.** Primary interface is Claude Code via MCP tools + hooks. UI is opened 5–20 min/day to:

- browse notes captured automatically during AI sessions
- search across notes and indexed docs
- investigate the entity graph
- debug MCP tool calls
- curate / audit the index

Design priorities follow from this profile: fast to parse when opened, keyboard-first, minimal chrome, information-dense where needed, never "dashboard for dashboard's sake".

---

## 3. Constraints (locked by `~/.claude/rules/`)

- **Air-gapped build** — all fonts, icons, and assets vendored locally. No Google Fonts, no CDN, no external analytics.
- **WCAG AA** — text contrast ≥ 4.5:1 (body) / 3:1 (large). Full keyboard nav. Visible `:focus-visible` rings. Correct ARIA.
- **Respect user prefs** — `prefers-color-scheme` sets theme default; `prefers-reduced-motion` disables non-essential motion; `prefers-contrast` honored.
- **Performance targets** — interaction response < 100 ms; TTI < 2.5 s on median device/network; 60 fps (< 16 ms frame).
- **Design system** — tokens only (color / space / radius / type / motion). Arbitrary values are a code smell.
- **Mobile-first responsive**, not desktop-down.
- **Dark mode default**, themeable.
- **i18n-ready** from day one: strings externalized, CSS logical properties only (no `margin-left`; use `margin-inline-start`), `dir="rtl"` support in root.

---

## 4. Architecture

### 4.1 Navigation — combined Linear + Raycast

**Default (desktop ≥ 1024 px):** labeled 220 px sidebar + top bar with prominent ⌘K.

**Sections (in order):** Home, Notes, Documents, Graph, MCP console. Project selector at sidebar bottom.

**Keyboard:** `G H`, `G N`, `G D`, `G G`, `G M` (Linear-style chords); `⌘K` command palette; `⌘\` toggle sidebar collapse.

**Sidebar collapse:** user toggle via `⌘\` → 48 px icon rail; state persisted to `localStorage`.

**Command palette (⌘K):** powered by shadcn `<Command>` (cmdk). Single search input hits in parallel: notes (FTS5), documents (hybrid), entities, projects, and page jumps. Result types distinguished by leading monospace badge.

### 4.2 Responsive breakdown

| Breakpoint | Nav treatment | Notes workspace | Graph view |
|---|---|---|---|
| `< 640 px` (mobile) | Hamburger + ⌕ icon button; drawer slide-out | Tabs: Note / Tree / Links | Pinch-zoom, reduced node density |
| `640–1023 px` (tablet) | 44 px icon rail persistent; ⌕ button in top bar | Tree + Note 2-col; Links inline strip | Desktop layout at smaller scale |
| `1024–1439 px` (desktop) | 220 px labeled sidebar (collapsible) | Focused column + drawers (⌘/, ⌘L) | Full-viewport SVG |
| `≥ 1440 px` (wide) | Same as desktop, max widths kept | Focused column still ~620 px | More nodes shown |

Touch targets ≥ 44 px. No hover-only interactions on touch. Keyboard shortcut hints hidden on touch devices.

### 4.3 Routing

- **React Router 6** — fixes the current `replaceState` hack. Browser back/forward works naturally.
- URL = source of truth for current view + project: `/` (Home), `/notes/:key?`, `/docs/:id?`, `/graph`, `/mcp`.
- Project selection as search param: `?project=<slug>`, default `_default`.
- `pushState` on navigation, `replaceState` only for query-param-only updates (filters, search text).

---

## 5. Home screen

**Layout:** 5-tile stats strip + "since your last visit" activity feed as main column + right rail (desktop only) with graph glance and pinned notes.

### Stats strip (5 tiles)
- Notes (with `+N` delta in accent color)
- Documents
- Entities
- Communities
- Last updated (relative time)

### Activity feed
- Chronological, newest first, typed event badges:
  - `+ NOTE` (green accent)
  - `INDEX` (blue semantic)
  - `GRAPH` (purple semantic)
  - `ERROR` (red semantic)
- "Since your last visit" section header.
- "Last visit" = `localStorage` timestamp updated on navigation away from Home.
- Paginated: 20 newest + "view full activity" link to a dedicated timeline route (deferred to post-v1).

### Right rail (desktop ≥ 1024 px only)
- **Graph glance** — small SVG preview of the current project's graph (~30 nodes max). Clickable → Graph view.
- **Pinned notes** — up to 5 note keys marked as pinned. Click → note view. Pin toggle is a backend extension (deferred); v1 shows the 5 most-recently-written notes.

On tablet: right rail content drops below the activity feed. On mobile: stacked in order (stats → feed → graph preview).

---

## 6. Notes workspace (`/notes/:key?`)

**Pattern:** focused reading column, ~620 px max. Tree and Links are drawer panels opened on demand.

### Reading mode (default)
- Frontmatter stripped from display, title rendered as H1.
- Metadata row under title: key path, last updated, inbound/outbound link counts.
- Body: markdown renderer (new, better than current — see §6.3).
- Wikilinks `[[target]]` are primary in-page navigation; click → navigate to target note.
- Aliased wikilinks `[[target|alias]]` **render the alias** (fixes current bug).

### Edit mode
- Click body or press `E` → inline editor (CodeMirror 6 or Monaco — see §9).
- Frontmatter editable as YAML.
- `⌘S` saves. Dirty state shown in tab title (`*`) and nav.
- Cancel (`Escape`) with confirm-discard if dirty.

### Drawers
- `⌘/` → tree drawer (left, 300 px). Folder-structured tree; click a key to navigate; `+` button to create.
- `⌘L` → links drawer (right, 280 px). Inbound + outbound lists, click to navigate.
- Both drawers pinned-open persistable to `localStorage` per user preference.

### Search sub-route
- `/notes/search?q=…` — dedicated search view with highlighted snippets, scored results.
- Debounced input (300 ms) — fixes current non-debounced bug.

### Responsive
- Tablet: tree always visible (240 px), focused column beside, links as inline strip.
- Mobile: three tabs at top: Note / Tree / Links.

### 6.3 Markdown rendering (scope v1)
**Supported:**
- Headings (ATX)
- Bold, italic, inline code
- Fenced code blocks with syntax highlighting (Shiki, vendored languages list: ts, tsx, go, bash, json, yaml, md, sql, py, rust)
- Unordered + ordered lists (1 level of nesting)
- Links (external → `target="_blank" rel="noopener noreferrer"`)
- Images (`loading="lazy"`)
- Blockquotes
- Horizontal rules
- GitHub-style tables (header + body only, no alignment flags)
- Inline math `\(...\)` rendered as styled code
- `[[wikilinks]]` + `[[target|alias]]`

**Explicitly not in v1:**
- Task list checkboxes
- Setext headings
- Reference-style links
- Nested blockquotes / nested tables
- Math block rendering (KaTeX / MathJax)
- Mermaid diagrams

Use **markdown-it** (MIT, actively maintained, ~80 KB gzip incl. plugins we need; smaller than micromark+remark full stack we'd otherwise need).

---

## 7. Other screens (lower-fidelity specs)

### 7.1 Documents (`/docs/:id?`)
- List view: table of indexed docs with columns (title, type, last indexed, chunks, entities). Click row → doc detail.
- Detail view: focused column with doc structure outline, entity list, chunk preview. Link to raw source.
- Upload modal (triggered from list view header): drag-drop + file picker + URL crawler tab. Progress via existing SSE (`/api/upload/progress?job_id=…`).

### 7.2 Graph (`/graph`)
- Full-viewport SVG, force layout (d3-force, already in scope).
- Filters (left drawer): entity types, communities, date range.
- Layer toggle: Entities / Notes / Both (note overlay uses `--accent-notes` hue).
- Node click → side panel with details + neighborhood.
- `⌘+` / `⌘-` zoom, `0` reset, `/` focus search.

### 7.3 MCP console (`/mcp`)
- Power-user / debugging screen. Kept from current implementation with visual refresh.
- List of recent tool calls with request + response JSON, latency, auth status.
- Filter by tool name; click → full request/response in modal.
- Deprioritized for visual polish; functional parity with current is acceptable for v1.

### 7.4 No dedicated Upload, Search, Stats, Communities, or Overview routes
- Upload = modal from Documents
- Unified search = `⌘K` palette
- Stats = stats strip on Home
- Communities = filter/overlay inside Graph
- Overview = Home

This collapses the current 8-tab TopNav to **5 destinations**.

---

## 8. Visual system — Terminal (choice B)

### 8.1 Palette — dark (default)
```
--color-base        #0f1115   /* app background */
--color-surface-1   #14171d   /* cards, sidebar */
--color-surface-2   #1b1f26   /* elevated / popover */
--color-border      #1e2128
--color-border-strong #2a2f38
--color-text        #e4e6ec
--color-text-muted  #6f7482
--color-text-faint  #4a4f59

--color-accent      #3ecf8e   /* primary */
--color-accent-hover #4ad89a
--color-accent-contrast #0f1115

/* Event semantics (theme-independent) */
--semantic-new      #3ecf8e
--semantic-index    #6ba6ff
--semantic-graph    #b08fe8
--semantic-error    #e06060
--semantic-warn     #f3b54a
```

### 8.2 Palette — light (auto from `prefers-color-scheme` + manual toggle)
```
--color-base        #f8f9fb
--color-surface-1   #ffffff
--color-surface-2   #f1f3f6
--color-border      #e5e8ec
--color-border-strong #c8ced6
--color-text        #0f1115
--color-text-muted  #5e6672
--color-text-faint  #8c93a0

--color-accent      #1faa69   /* darker for AA on white */
--color-accent-hover #1a9a5f
--color-accent-contrast #ffffff

/* Event semantics (darker variants for AA on light bg) */
--semantic-new      #1faa69
--semantic-index    #2968d4
--semantic-graph    #7246c2
--semantic-error    #c03030
--semantic-warn     #b8801e
```

### 8.3 Typography
- **Sans:** Geist (400, 500, 600). Body text, UI labels, headings.
- **Mono:** Geist Mono (400, 500). Keyboard shortcuts, keys, timestamps, event badges, code.
- **Font feature settings:** `"cv11", "ss01", "ss03"` (Geist's tabular numerals for aligned tables).
- **Vendored to** `ui/public/fonts/`, loaded via `@font-face` with `font-display: swap`.
- **Font sizes (tokens):** 11 (micro), 12 (caption), 13 (small), 14 (body), 16 (large body), 18 (h3), 22 (h2), 28 (h1), 36 (display). No other sizes allowed.
- **Line heights:** 1.4 tight (UI), 1.6 relaxed (reading body).

### 8.4 Spacing tokens
`4, 8, 12, 16, 20, 24, 32, 40, 48, 64, 96` — px. No arbitrary values in Tailwind classes.

### 8.5 Radius tokens
- `4` — inputs, small badges
- `6` — cards, buttons (default)
- `10` — modals, popovers
- `999` — pill (sparingly; only for status dots and small count chips)

### 8.6 Shadow / elevation
- `--shadow-sm` — `0 1px 2px rgba(0,0,0,.2)` (dark) / `0 1px 2px rgba(15,17,21,.04)` (light)
- `--shadow-md` — popovers, dropdowns
- `--shadow-lg` — modals, command palette
- Never use shadow as primary boundary; borders do that job.

---

## 9. Tech stack

```
React 19                   — kept
Vite 6                     — kept (static build for //go:embed)
TypeScript 5.7             — kept

shadcn/ui                  — component primitives, copy-pasted into src/components/ui/
Tailwind CSS 4             — utility-first styling, JIT
Radix primitives           — backing shadcn, already in stack
lucide-react               — icons (already vendored via npm)
cmdk                       — command palette (via shadcn <Command>)

TanStack Query v5          — server state, replaces manual fetch + loading flags
Zustand v5                 — client state (theme, sidebar-collapsed, project slug)
React Router 6             — routing
React Hook Form v7 + Zod v3 — forms + validation

Framer Motion v11          — motion, always gated by prefers-reduced-motion
markdown-it + shiki        — note markdown rendering + code highlighting
d3-force                   — graph layout (lightweight, tree-shakeable)
CodeMirror 6               — markdown editor (inline-edit mode)

Vitest 3 + Testing Library — unit + component tests
MSW v2                     — mock API for integration tests

Geist Sans + Geist Mono    — fonts, Apache 2.0, self-hosted
```

**Bundle budget:** ≤ 450 KB JS / ≤ 135 KB gzip. First CI run after each PR publishes the bundle size; regressions > 5% fail the check.

---

## 10. Motion philosophy

- **Purposeful motion only.** Each animation justifies itself against: does it show a relationship (drawer slides in from the side it belongs to), provides orientation (route transition fades the old view out), or confirms an action (pulse on copy)? If not — no animation.
- **Durations:** 120–180 ms for UI state changes (hover, focus, small toggles); 250–400 ms for content transitions (drawer, modal, route change).
- **Easing:** `cubic-bezier(0.3, 0, 0, 1)` for enter (ease-out); `cubic-bezier(0.7, 0, 1, 0.3)` for exit (ease-in). Tokenized as `--ease-out`, `--ease-in`.
- **Entry style:** opacity + translate (never scale — scale animations read as decorative at this fidelity).
- **`prefers-reduced-motion: reduce`** — every motion either becomes instant (opacity swap with no translate) or reduces to opacity-only fade. Framer Motion's `useReducedMotion()` hook respected throughout.
- **Framer Motion scope:** drawer slide-in/out, modal / command-palette enter/exit, route transition. Everything else is CSS transitions on discrete properties (`color`, `background-color`, `border-color`, `opacity`) — no `all`.

---

## 11. Accessibility requirements

- **WCAG AA** contrast on all text. Validated via automated check in CI (axe-core or pa11y).
- **Keyboard:** every interactive element reachable via Tab. Focus order follows reading order. `:focus-visible` ring uses `--color-accent` (2 px outline + 2 px offset).
- **ARIA:** `role="navigation"` on sidebar + top bar, `role="main"` on content, `role="complementary"` on right rail. `aria-label` on all icon-only buttons. `aria-live="polite"` region for toast notifications and upload progress.
- **Skip link:** "Skip to main content" as first tab-stop, visible on focus.
- **Screen reader:** landmark regions, descriptive page titles on route change, proper heading hierarchy (one `<h1>` per route).
- **Form labels:** every input has an associated `<label>` (explicit or `aria-labelledby`).
- **Error states:** `aria-invalid` + `aria-describedby` pointing at error text.

---

## 12. State management

### Server state → TanStack Query
One query key per endpoint. Retries on 5xx (max 3, exponential backoff). No retry on 4xx. Default stale-time 30 s, cache-time 5 min.

**Query key conventions:**
- `['stats', projectSlug]`
- `['notes', projectSlug, { filter }]`
- `['note', projectSlug, key]`
- `['activity', projectSlug, sinceMs]`

**Mutations:** `writeNote`, `deleteNote`, `uploadDoc`, `deleteDoc`. All invalidate affected query keys on success.

### Client state → Zustand
Three small stores:
- `useUIStore` — sidebar collapsed, current theme preference, pinned-state for drawers
- `useProjectStore` — current project slug (synced with URL)
- `useToastStore` — toast queue

No global app store, no Redux, no Context layer for server data.

### Forms → React Hook Form + Zod
Every form has a Zod schema. RHF's `resolver: zodResolver(schema)` wires them. Schemas are the source of truth for both client + runtime validation and shared TS types via `z.infer`.

---

## 13. File structure

```
ui/
├── src/
│   ├── main.tsx                      # React entry, router setup
│   ├── App.tsx                       # layout shell + providers
│   ├── routes/
│   │   ├── Home.tsx
│   │   ├── notes/
│   │   │   ├── NotesLayout.tsx       # focused column shell
│   │   │   ├── NoteView.tsx
│   │   │   ├── NoteEditor.tsx
│   │   │   └── NotesSearch.tsx
│   │   ├── documents/
│   │   │   ├── DocumentsList.tsx
│   │   │   ├── DocumentView.tsx
│   │   │   └── UploadModal.tsx
│   │   ├── Graph.tsx
│   │   └── MCPConsole.tsx
│   ├── components/
│   │   ├── ui/                       # shadcn primitives (Button, Dialog, Input, etc.)
│   │   ├── layout/                   # Shell, Sidebar, TopBar, StatsStrip
│   │   ├── command/                  # CommandPalette, result-type renderers
│   │   ├── activity/                 # ActivityFeed, EventRow, EventBadge
│   │   ├── graph/                    # GraphCanvas, GlanceView, Filters
│   │   ├── notes/                    # MarkdownView, WikiLink, LinkPanel, TreeDrawer
│   │   ├── docs/                     # DocTable, DocOutline
│   │   └── common/                   # Toast, Skeleton, EmptyState, ErrorBoundary
│   ├── hooks/
│   │   ├── api/                      # TanStack Query wrappers, one file per endpoint group
│   │   ├── useCommand.ts
│   │   ├── useHotkey.ts
│   │   ├── useLastVisit.ts
│   │   └── useReducedMotion.ts
│   ├── stores/
│   │   ├── ui.ts
│   │   ├── project.ts
│   │   └── toast.ts
│   ├── lib/
│   │   ├── api-client.ts             # typed fetch wrapper, bearer header, error normalization
│   │   ├── markdown.ts               # markdown-it config
│   │   ├── graph-layout.ts           # d3-force config
│   │   └── utils.ts                  # cn(), formatRelativeTime(), etc.
│   ├── i18n/
│   │   ├── en.ts                     # all strings as keys
│   │   └── index.ts                  # t() function
│   ├── styles/
│   │   └── globals.css               # Tailwind + tokens + resets
│   └── types/
│       └── api.ts                    # response shapes matching internal/api/
├── public/
│   └── fonts/
│       ├── Geist-400.woff2
│       ├── Geist-500.woff2
│       ├── Geist-600.woff2
│       ├── GeistMono-400.woff2
│       └── GeistMono-500.woff2
├── tailwind.config.ts
├── vitest.config.ts
├── tsconfig.json
├── index.html
├── embed.go
└── package.json                      # renamed to docsiq-ui
```

---

## 14. API contract preservation (acceptance criterion)

- Backend REST + MCP endpoints unchanged during the UI refresh.
- TypeScript response types (`src/types/api.ts`) hand-mirror `internal/api/handlers.go` shapes. A CI check greps for known field names to detect drift.
- Error shape consumed: `{error: string, request_id?: string}` per NF-P1-3 fix.
- Auth decision: the current UI is served by the backend as public static assets, so the `/` request itself is unauthenticated. `/api/*` and `/mcp` require the bearer. The redesigned UI preserves this topology but needs a way to obtain the bearer at runtime when `DOCSIQ_API_KEY` is set. Chosen mechanism: the Go SPA handler injects a `<meta name="docsiq-api-key" content="…">` tag into `index.html` server-side when serving the UI. The UI reads this via `document.querySelector` at bootstrap, stores it in memory (not localStorage, not sessionStorage), and attaches it to every fetch. If the env is unset, the meta tag is omitted and the UI makes unauthenticated calls (local dev path).
- **Backend delta:** ~8 lines in `internal/api/router.go`'s SPA handler to inject the meta tag when `cfg.Server.APIKey != ""`. This is the only backend change permitted by this spec; it is additive and gated by auth being enabled.

---

## 15. Testing strategy

### Unit + component tests (Vitest + Testing Library)
- Coverage target: ≥ 70 % statements on `src/components/**`, `src/hooks/**`, `src/lib/**`. Critical paths (auth / error handling / keyboard shortcuts) aim for near-100 %.
- Convention: colocated `__tests__/` folders as before.
- Fake data: MSW 2.x with handlers in `src/test/handlers.ts`.

### Integration tests (Vitest + MSW)
- Full router + providers rendered, MSW intercepts `/api/*`.
- One file per major flow: `test/flows/{home, notes, search, upload, graph}.test.tsx`.

### Accessibility
- `@axe-core/react` wired in dev mode; violations logged to console.
- CI: `pa11y-ci` against the built preview (one page per route).

### Backend tests unchanged
Go integration suites continue to run against the real API — this spec does not affect them.

### Visual regression
**Not in scope for v1.** Added later via Playwright + screenshots, tracked as a post-v1 task.

---

## 16. Migration — greenfield rewrite (option 1)

### Deletion scope
- All of `ui/src/` contents (keep `ui/src/main.tsx` replaced from scratch)
- `ui/app.js`, `ui/graph.js`, `ui/style.css`, `ui/vendor/vis-network.min.js` (pre-Vite cruft)

### Preserved
- `ui/embed.go` — Go embed wiring
- `ui/index.html` — Vite entry
- `ui/public/` — including new `fonts/` subdirectory
- `ui/package.json` — rewritten (name `docsiq-ui`, new deps); lockfile regenerated
- `ui/vite.config.ts` — rewritten for Tailwind 4 + aliases
- `ui/tsconfig.json`, `ui/tsconfig.app.json`, `ui/tsconfig.node.json` — updated
- `ui/vitest.config.ts` — updated

### Backend untouched (acceptance)
- No Go file outside `ui/embed.go` changes as part of this spec.
- Exception: the 5-line meta-tag server change in §14 if we pursue bearer-in-meta; otherwise no backend changes.

### Build / embed integrity
- `ui-freshness` CI check stays green: committed `ui/dist/` must match a fresh `npm run build`.
- `//go:embed ui/dist/` continues to serve the UI.
- Placeholder `ui/dist/index.html` ("docsiq UI — in progress") committed at branch head so backend builds keep passing during the rewrite period.

### Branch / commit discipline
- All work on `ui-redesign` feature branch.
- Atomic per-wave commits (`feat(ui):` prefix) — one subsystem per commit.
- PR to `main` when complete; squash or rebase-merge at user's preference.

---

## 17. Out of scope (deferred, tracked post-v1)

- Pinned notes backend extension (v1 uses "most recent" as proxy)
- Full activity-timeline page (v1 caps feed at 20; "view all" links to a placeholder route that says "coming soon")
- Note version history UI (git history exists backend-side; UI doesn't consume it in v1)
- Real-time updates via SSE (v1 uses polling every 10 s for Home activity feed when visible)
- Additional LLM providers in the config UI (Anthropic/Bedrock/Groq)
- Multi-user / RBAC UI
- Theme customization beyond dark/light (e.g., custom accent, density variants)
- Visual regression testing infrastructure
- Storybook / component catalogue
- PWA / offline support
- Mobile-specific capabilities (e.g., share sheet integration)

---

## 18. Risks

| # | Risk | Mitigation |
|---|---|---|
| 1 | Bundle budget (≤ 450 KB JS) creeps past ceiling | First drop = Framer Motion (save ~30 KB gzip). Second = replace markdown-it with a smaller custom renderer. |
| 2 | Tailwind 4 recency issues (released late 2024) | Pin to stable v4.x; fall back to v3.4 if issues. Both share the utility-first model; switch is mechanical. |
| 3 | shadcn/ui × React 19 compatibility | Verify before starting. Known-good version combinations documented in plan. |
| 4 | Greenfield = UI broken for ~1–2 weeks | Placeholder "in-progress" page in `ui/dist/index.html` keeps backend builds green. MCP + REST paths unaffected — users can still work. |
| 5 | CSS logical properties unfamiliar to maintainers | Tailwind 4 exposes them via class names (`ms-4`, `me-2`, `ps-3`). Minimal re-learning curve. |
| 6 | 10 new runtime deps increase supply-chain surface | Every add runs `npm audit --audit-level=moderate` per `rules/security.md`. Indirect-dep growth watched in PR review. |
| 7 | Accessibility audit (pa11y / axe-core) flags issues late | Wired from day-one of the new shell; each component ships with its own axe-clean assertion. |

---

## 19. Acceptance criteria (how we know v1 ships)

1. All 5 destinations (Home, Notes, Documents, Graph, MCP) implemented per §§5–7.
2. Command palette (⌘K) operational with notes + docs + entities + page-jump search.
3. Keyboard shortcuts per §4.1 all functional.
4. Responsive across the 4 breakpoints per §4.2; tested at 375, 768, 1280, 1920 widths.
5. Dark + light themes, auto-switch via `prefers-color-scheme`, manual toggle persisted.
6. `prefers-reduced-motion` reduces motion per §10 — validated by test.
7. WCAG AA contrast validated — zero axe-core violations on each route.
8. Vitest coverage ≥ 70 % on scoped surface; zero test failures under `npm test`.
9. `npm audit --audit-level=moderate` clean.
10. Bundle ≤ 450 KB JS / 135 KB gzip measured at `npm run build`.
11. `make test` + `make test-integration` on backend continue green (no backend regression).
12. `ui-freshness` CI check passes on the final `ui/dist/` commit.
13. Smoke test: `docsiq serve` in a fresh tempdir → all 5 destinations render → commands palette opens → write a note → read it back → graph renders → theme toggle works.

---

## 20. Implementation plan handoff

Next step: invoke `superpowers:writing-plans` skill to convert this spec into numbered per-task implementation steps with:
- file-path specifics
- bite-sized steps (~2–5 min each)
- TDD ladders where applicable
- commit shapes and messages
- acceptance per wave

The plan will decompose this spec into roughly: foundation (stack setup, tokens, shell) → navigation → Home → Notes → Documents → Graph → MCP → polish (reduced-motion, a11y audit, coverage) → smoke + deploy.
