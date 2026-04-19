# docsiq UI Redesign — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Greenfield rewrite of the docsiq web UI per `docs/superpowers/specs/2026-04-18-ui-redesign-design.md`. Ship a keyboard-first, responsive, WCAG-AA, reduced-motion-aware SPA that replaces the current hand-rolled React+CSS UI without any backend regression.

**Architecture:** React 19 + Vite 6 + Tailwind 4 + shadcn/ui primitives, TanStack Query for server state, Zustand for client state, React Router 6 for routing, markdown-it + shiki for note rendering, d3-force for graph layout, Framer Motion gated by `prefers-reduced-motion`. Fonts (Geist Sans + Geist Mono) self-hosted. One shared bearer token injected into `<meta>` tag by the Go SPA handler.

**Tech Stack:** React 19, Vite 6, TypeScript 5.7, Tailwind 4, shadcn/ui, Radix, lucide-react, TanStack Query v5, Zustand v5, React Router 6, React Hook Form 7, Zod 3, Framer Motion 11, markdown-it, shiki, d3-force, CodeMirror 6, cmdk, Vitest 3, @testing-library/react 16, MSW 2.

**Spec:** `docs/superpowers/specs/2026-04-18-ui-redesign-design.md`

---

## File Structure (final state)

```
ui/
├── src/
│   ├── main.tsx
│   ├── App.tsx
│   ├── routes/
│   │   ├── Home.tsx
│   │   ├── notes/{NotesLayout,NoteView,NoteEditor,NotesSearch}.tsx
│   │   ├── documents/{DocumentsList,DocumentView,UploadModal}.tsx
│   │   ├── Graph.tsx
│   │   └── MCPConsole.tsx
│   ├── components/
│   │   ├── ui/               — shadcn primitives (Button, Dialog, Input, etc.)
│   │   ├── layout/{Shell,Sidebar,TopBar,StatsStrip,SkipLink}.tsx
│   │   ├── command/{CommandPalette,ResultRow}.tsx
│   │   ├── activity/{ActivityFeed,EventRow,EventBadge}.tsx
│   │   ├── graph/{GraphCanvas,GlanceView,GraphFilters}.tsx
│   │   ├── notes/{MarkdownView,WikiLink,LinkPanel,TreeDrawer}.tsx
│   │   ├── docs/{DocTable,DocOutline}.tsx
│   │   └── common/{Toast,Skeleton,EmptyState,ErrorBoundary,Kbd}.tsx
│   ├── hooks/
│   │   ├── api/{useStats,useActivity,useNotes,useDocs,useGraph,useMCP,useCommand}.ts
│   │   ├── useHotkey.ts
│   │   ├── useLastVisit.ts
│   │   └── useReducedMotion.ts
│   ├── stores/{ui,project,toast}.ts
│   ├── lib/{api-client,markdown,graph-layout,utils,format,i18n}.ts
│   ├── i18n/en.ts
│   ├── styles/globals.css
│   └── types/api.ts
├── public/fonts/{Geist-400,Geist-500,Geist-600,GeistMono-400,GeistMono-500}.woff2
├── tailwind.config.ts
├── vitest.config.ts
├── tsconfig.{json,app.json,node.json}
├── vite.config.ts
├── index.html
├── embed.go                 — unchanged
└── package.json             — name: docsiq-ui

internal/api/router.go       — +8 lines (meta-tag injection, Phase 10)
```

---

## Wave 0 — Foundation

### Task 0.1: Create feature branch

- [ ] **Step 1: Branch from main**
  ```bash
  cd /home/dev/projects/docsiq
  git checkout -b ui-redesign
  git push -u origin ui-redesign
  ```

- [ ] **Step 2: Confirm clean starting state**
  ```bash
  git status --short; echo "---"; make test 2>&1 | grep -E '^(ok|FAIL)' | head -5
  ```
  Expected: zero lines in `git status`, all packages `ok`.

### Task 0.2: Wipe old ui/src and ui-root cruft

**Files:**
- Delete: `ui/src/**`, `ui/app.js`, `ui/graph.js`, `ui/style.css`, `ui/vendor/`

- [ ] **Step 1: Delete the old UI source**
  ```bash
  cd /home/dev/projects/docsiq/ui
  rm -rf src app.js graph.js style.css vendor
  ```

- [ ] **Step 2: Verify only config + embed remain**
  ```bash
  ls -la
  ```
  Expected dirs: `public/`, `dist/`, `node_modules/` (if present). Expected files: `embed.go`, `index.html`, `package.json`, `package-lock.json`, `tsconfig*.json`, `vite.config.ts`, `vitest.config.ts`.

- [ ] **Step 3: Commit the clean slate**
  ```bash
  cd /home/dev/projects/docsiq
  git add -A
  git commit -m "chore(ui): wipe pre-redesign src and legacy root cruft"
  ```

### Task 0.3: Placeholder `ui/dist/` so backend builds stay green

**Files:**
- Create: `ui/dist/index.html`

- [ ] **Step 1: Write placeholder**
  ```bash
  cat > ui/dist/index.html <<'EOF'
  <!doctype html>
  <html lang="en">
    <head>
      <meta charset="utf-8" />
      <meta name="viewport" content="width=device-width,initial-scale=1" />
      <title>docsiq — UI in progress</title>
      <style>body{font-family:ui-monospace,monospace;background:#0f1115;color:#e4e6ec;display:grid;place-items:center;min-height:100vh;margin:0}p{opacity:.6}</style>
    </head>
    <body>
      <div>
        <h1>docsiq</h1>
        <p>UI redesign in progress. API + MCP paths unchanged.</p>
      </div>
    </body>
  </html>
  EOF
  ```

- [ ] **Step 2: Verify Go build still succeeds**
  ```bash
  CGO_ENABLED=1 go build -tags sqlite_fts5 -o docsiq ./
  ```
  Expected: exit 0.

- [ ] **Step 3: Commit**
  ```bash
  git add ui/dist/index.html
  git commit -m "chore(ui): placeholder dist/index.html during rewrite"
  ```

### Task 0.4: Rewrite `ui/package.json`

**Files:**
- Modify: `ui/package.json` (full rewrite)

- [ ] **Step 1: Replace `ui/package.json` contents**
  ```json
  {
    "name": "docsiq-ui",
    "private": true,
    "version": "0.1.0",
    "type": "module",
    "scripts": {
      "dev": "vite",
      "build": "tsc -b --noEmit && vite build",
      "preview": "vite preview",
      "test": "vitest run",
      "test:watch": "vitest",
      "test:coverage": "vitest run --coverage",
      "typecheck": "tsc -b --noEmit",
      "lint": "eslint src"
    },
    "dependencies": {
      "@radix-ui/react-dialog": "^1.1.4",
      "@radix-ui/react-dropdown-menu": "^2.1.4",
      "@radix-ui/react-popover": "^1.1.4",
      "@radix-ui/react-scroll-area": "^1.2.2",
      "@radix-ui/react-slot": "^1.1.1",
      "@radix-ui/react-tabs": "^1.1.2",
      "@radix-ui/react-tooltip": "^1.1.4",
      "@tanstack/react-query": "^5.62.0",
      "class-variance-authority": "^0.7.1",
      "clsx": "^2.1.1",
      "cmdk": "^1.0.4",
      "codemirror": "^6.0.1",
      "@codemirror/lang-markdown": "^6.3.1",
      "@codemirror/view": "^6.35.0",
      "@codemirror/state": "^6.5.0",
      "@codemirror/commands": "^6.7.0",
      "d3-force": "^3.0.0",
      "framer-motion": "^11.15.0",
      "lucide-react": "^0.469.0",
      "markdown-it": "^14.1.0",
      "react": "^19.0.0",
      "react-dom": "^19.0.0",
      "react-hook-form": "^7.54.0",
      "react-router-dom": "^6.28.0",
      "shiki": "^1.24.0",
      "tailwind-merge": "^2.5.5",
      "zod": "^3.24.0",
      "zustand": "^5.0.2"
    },
    "devDependencies": {
      "@tailwindcss/vite": "^4.0.0",
      "@testing-library/jest-dom": "^6.9.1",
      "@testing-library/react": "^16.3.2",
      "@testing-library/user-event": "^14.6.1",
      "@types/d3-force": "^3.0.10",
      "@types/markdown-it": "^14.1.2",
      "@types/node": "^22.10.0",
      "@types/react": "^19.0.10",
      "@types/react-dom": "^19.0.4",
      "@vitejs/plugin-react": "^4.3.4",
      "@vitest/coverage-v8": "^3.2.4",
      "axe-core": "^4.10.2",
      "eslint": "^9.17.0",
      "jsdom": "^25.0.1",
      "msw": "^2.7.0",
      "tailwindcss": "^4.0.0",
      "typescript": "~5.7.2",
      "typescript-eslint": "^8.18.0",
      "vite": "^6.0.0",
      "vitest": "^3.2.4"
    }
  }
  ```

- [ ] **Step 2: Install**
  ```bash
  cd ui && rm -f package-lock.json && npm install
  ```

- [ ] **Step 3: Security audit**
  ```bash
  npm audit --audit-level=moderate
  ```
  Expected: `found 0 vulnerabilities` (or only transitive that can't be patched — document if any). If High/Critical: STOP and escalate.

- [ ] **Step 4: Commit**
  ```bash
  cd ..
  git add ui/package.json ui/package-lock.json
  git commit -m "feat(ui): new stack — React 19 + Vite 6 + Tailwind 4 + shadcn/ui"
  ```

### Task 0.5: Vite + Tailwind 4 config

**Files:**
- Modify: `ui/vite.config.ts`
- Modify: `ui/tsconfig.json`, `ui/tsconfig.app.json`, `ui/tsconfig.node.json`
- Create: `ui/src/styles/globals.css`

- [ ] **Step 1: Replace `ui/vite.config.ts`**
  ```ts
  import { defineConfig } from "vite";
  import react from "@vitejs/plugin-react";
  import tailwind from "@tailwindcss/vite";
  import { fileURLToPath, URL } from "node:url";

  export default defineConfig({
    plugins: [react(), tailwind()],
    resolve: {
      alias: {
        "@": fileURLToPath(new URL("./src", import.meta.url)),
      },
    },
    build: {
      outDir: "dist",
      emptyOutDir: true,
      sourcemap: false,
      rollupOptions: {
        output: {
          manualChunks: {
            "markdown": ["markdown-it", "shiki"],
            "graph": ["d3-force"],
            "editor": ["codemirror", "@codemirror/view", "@codemirror/state", "@codemirror/commands", "@codemirror/lang-markdown"],
          },
        },
      },
    },
    server: {
      proxy: {
        "/api": "http://localhost:8080",
        "/mcp": "http://localhost:8080",
        "/health": "http://localhost:8080",
        "/metrics": "http://localhost:8080",
      },
    },
  });
  ```

- [ ] **Step 2: Replace `ui/tsconfig.json`**
  ```json
  {
    "files": [],
    "references": [
      { "path": "./tsconfig.app.json" },
      { "path": "./tsconfig.node.json" }
    ],
    "compilerOptions": {
      "baseUrl": ".",
      "paths": { "@/*": ["src/*"] }
    }
  }
  ```

- [ ] **Step 3: Replace `ui/tsconfig.app.json`**
  ```json
  {
    "compilerOptions": {
      "target": "ES2022",
      "useDefineForClassFields": true,
      "lib": ["ES2022", "DOM", "DOM.Iterable"],
      "module": "ESNext",
      "skipLibCheck": true,
      "moduleResolution": "bundler",
      "allowImportingTsExtensions": true,
      "resolveJsonModule": true,
      "isolatedModules": true,
      "moduleDetection": "force",
      "noEmit": true,
      "jsx": "react-jsx",
      "strict": true,
      "noUnusedLocals": true,
      "noUnusedParameters": true,
      "noFallthroughCasesInSwitch": true,
      "noUncheckedSideEffectImports": true,
      "baseUrl": ".",
      "paths": { "@/*": ["src/*"] }
    },
    "include": ["src"]
  }
  ```

- [ ] **Step 4: Replace `ui/tsconfig.node.json`**
  ```json
  {
    "compilerOptions": {
      "target": "ES2022",
      "lib": ["ES2023"],
      "module": "ESNext",
      "skipLibCheck": true,
      "moduleResolution": "bundler",
      "allowImportingTsExtensions": true,
      "isolatedModules": true,
      "moduleDetection": "force",
      "noEmit": true,
      "strict": true,
      "noUnusedLocals": true,
      "noUnusedParameters": true,
      "noFallthroughCasesInSwitch": true
    },
    "include": ["vite.config.ts", "vitest.config.ts", "tailwind.config.ts"]
  }
  ```

- [ ] **Step 5: Create `ui/src/styles/globals.css`** (tokens + Tailwind 4 theme layer)
  ```css
  @import "tailwindcss";

  @theme {
    --color-base: #0f1115;
    --color-surface-1: #14171d;
    --color-surface-2: #1b1f26;
    --color-border: #1e2128;
    --color-border-strong: #2a2f38;
    --color-text: #e4e6ec;
    --color-text-muted: #6f7482;
    --color-text-faint: #4a4f59;

    --color-accent: #3ecf8e;
    --color-accent-hover: #4ad89a;
    --color-accent-contrast: #0f1115;

    --color-semantic-new: #3ecf8e;
    --color-semantic-index: #6ba6ff;
    --color-semantic-graph: #b08fe8;
    --color-semantic-error: #e06060;
    --color-semantic-warn: #f3b54a;

    --font-sans: "Geist", ui-sans-serif, system-ui, sans-serif;
    --font-mono: "Geist Mono", ui-monospace, monospace;

    --radius-sm: 4px;
    --radius: 6px;
    --radius-lg: 10px;
    --radius-pill: 999px;

    --ease-out: cubic-bezier(0.3, 0, 0, 1);
    --ease-in: cubic-bezier(0.7, 0, 1, 0.3);
  }

  @media (prefers-color-scheme: light) {
    @theme {
      --color-base: #f8f9fb;
      --color-surface-1: #ffffff;
      --color-surface-2: #f1f3f6;
      --color-border: #e5e8ec;
      --color-border-strong: #c8ced6;
      --color-text: #0f1115;
      --color-text-muted: #5e6672;
      --color-text-faint: #8c93a0;
      --color-accent: #1faa69;
      --color-accent-hover: #1a9a5f;
      --color-accent-contrast: #ffffff;
      --color-semantic-new: #1faa69;
      --color-semantic-index: #2968d4;
      --color-semantic-graph: #7246c2;
      --color-semantic-error: #c03030;
      --color-semantic-warn: #b8801e;
    }
  }

  @font-face {
    font-family: "Geist";
    src: url("/fonts/Geist-400.woff2") format("woff2");
    font-weight: 400;
    font-display: swap;
  }
  @font-face {
    font-family: "Geist";
    src: url("/fonts/Geist-500.woff2") format("woff2");
    font-weight: 500;
    font-display: swap;
  }
  @font-face {
    font-family: "Geist";
    src: url("/fonts/Geist-600.woff2") format("woff2");
    font-weight: 600;
    font-display: swap;
  }
  @font-face {
    font-family: "Geist Mono";
    src: url("/fonts/GeistMono-400.woff2") format("woff2");
    font-weight: 400;
    font-display: swap;
  }
  @font-face {
    font-family: "Geist Mono";
    src: url("/fonts/GeistMono-500.woff2") format("woff2");
    font-weight: 500;
    font-display: swap;
  }

  html, body, #root {
    height: 100%;
    margin: 0;
    background: var(--color-base);
    color: var(--color-text);
    font-family: var(--font-sans);
    font-feature-settings: "cv11", "ss01", "ss03";
    -webkit-font-smoothing: antialiased;
  }

  *:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
    border-radius: var(--radius-sm);
  }

  @media (prefers-reduced-motion: reduce) {
    *, *::before, *::after {
      animation-duration: 0.001ms !important;
      animation-iteration-count: 1 !important;
      transition-duration: 0.001ms !important;
      scroll-behavior: auto !important;
    }
  }
  ```

- [ ] **Step 6: Replace `ui/index.html`**
  ```html
  <!doctype html>
  <html lang="en" dir="ltr">
    <head>
      <meta charset="utf-8" />
      <meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover" />
      <meta name="color-scheme" content="dark light" />
      <title>docsiq</title>
    </head>
    <body>
      <div id="root"></div>
      <script type="module" src="/src/main.tsx"></script>
    </body>
  </html>
  ```

- [ ] **Step 7: Smoke test typecheck**
  ```bash
  cd ui && npm run typecheck
  ```
  Expected: exit 0 (no code yet — passes vacuously).

- [ ] **Step 8: Commit**
  ```bash
  cd ..
  git add ui/vite.config.ts ui/tsconfig.json ui/tsconfig.app.json ui/tsconfig.node.json ui/src/styles/globals.css ui/index.html
  git commit -m "feat(ui): vite + tailwind4 + tokens + font-face scaffold"
  ```

### Task 0.6: Vendor Geist fonts

**Files:**
- Create: `ui/public/fonts/{Geist-400,Geist-500,Geist-600,GeistMono-400,GeistMono-500}.woff2`

- [ ] **Step 1: Fetch Geist fonts locally**
  ```bash
  mkdir -p ui/public/fonts
  cd ui/public/fonts
  # Geist is Apache 2.0, hosted by Vercel. Download woff2 subsets.
  curl -L -o Geist-400.woff2 https://github.com/vercel/geist-font/raw/main/packages/next/dist/fonts/geist-sans/Geist-Regular.woff2
  curl -L -o Geist-500.woff2 https://github.com/vercel/geist-font/raw/main/packages/next/dist/fonts/geist-sans/Geist-Medium.woff2
  curl -L -o Geist-600.woff2 https://github.com/vercel/geist-font/raw/main/packages/next/dist/fonts/geist-sans/Geist-SemiBold.woff2
  curl -L -o GeistMono-400.woff2 https://github.com/vercel/geist-font/raw/main/packages/next/dist/fonts/geist-mono/GeistMono-Regular.woff2
  curl -L -o GeistMono-500.woff2 https://github.com/vercel/geist-font/raw/main/packages/next/dist/fonts/geist-mono/GeistMono-Medium.woff2
  ls -la
  ```
  Expected: 5 .woff2 files each 20-40 KB.

  If curl fails (air-gapped / CDN blocked): ask user to drop the 5 woff2 files into `ui/public/fonts/` manually.

- [ ] **Step 2: Commit**
  ```bash
  cd /home/dev/projects/docsiq
  git add ui/public/fonts/
  git commit -m "feat(ui): vendor Geist Sans + Mono fonts (Apache 2.0)"
  ```

### Task 0.7: Vitest config + setup file

**Files:**
- Modify: `ui/vitest.config.ts`
- Create: `ui/src/setupTests.ts`
- Create: `ui/src/test/msw.ts`
- Create: `ui/src/test/handlers.ts`

- [ ] **Step 1: Replace `ui/vitest.config.ts`**
  ```ts
  import { defineConfig } from "vitest/config";
  import react from "@vitejs/plugin-react";
  import { fileURLToPath, URL } from "node:url";

  export default defineConfig({
    plugins: [react()],
    resolve: {
      alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
    },
    test: {
      environment: "jsdom",
      globals: true,
      setupFiles: ["./src/setupTests.ts"],
      coverage: {
        reporter: ["text", "html"],
        include: ["src/components/**", "src/hooks/**", "src/lib/**", "src/routes/**"],
        exclude: ["src/test/**", "**/*.d.ts"],
        thresholds: { statements: 70, branches: 60 },
      },
    },
  });
  ```

- [ ] **Step 2: Create `ui/src/setupTests.ts`**
  ```ts
  import "@testing-library/jest-dom";
  import { server } from "@/test/msw";
  import { beforeAll, afterEach, afterAll } from "vitest";

  beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
  afterEach(() => server.resetHandlers());
  afterAll(() => server.close());
  ```

- [ ] **Step 3: Create `ui/src/test/handlers.ts`**
  ```ts
  import { http, HttpResponse } from "msw";

  export const handlers = [
    http.get("/api/stats", () =>
      HttpResponse.json({
        documents: 42,
        chunks: 512,
        entities: 380,
        relationships: 820,
        communities: 8,
        notes: 17,
        last_indexed: new Date().toISOString(),
      }),
    ),
    http.get("/api/projects", () => HttpResponse.json([{ slug: "_default", name: "_default" }])),
  ];
  ```

- [ ] **Step 4: Create `ui/src/test/msw.ts`**
  ```ts
  import { setupServer } from "msw/node";
  import { handlers } from "./handlers";

  export const server = setupServer(...handlers);
  ```

- [ ] **Step 5: Smoke test**
  ```bash
  cd ui && npm test 2>&1 | tail -6
  ```
  Expected: `No test files found` — vitest launches cleanly.

- [ ] **Step 6: Commit**
  ```bash
  cd ..
  git add ui/vitest.config.ts ui/src/setupTests.ts ui/src/test/
  git commit -m "test(ui): vitest + MSW 2 scaffold"
  ```

### Task 0.8: shadcn/ui initial primitives

**Files:**
- Create: `ui/src/components/ui/{button,dialog,dropdown-menu,input,popover,scroll-area,separator,tooltip,command}.tsx`
- Create: `ui/src/lib/utils.ts`
- Create: `ui/components.json` (shadcn manifest)

- [ ] **Step 1: Create `ui/components.json`**
  ```json
  {
    "$schema": "https://ui.shadcn.com/schema.json",
    "style": "new-york",
    "rsc": false,
    "tsx": true,
    "tailwind": {
      "config": "",
      "css": "src/styles/globals.css",
      "baseColor": "neutral",
      "cssVariables": true
    },
    "aliases": {
      "components": "@/components",
      "utils": "@/lib/utils",
      "ui": "@/components/ui"
    }
  }
  ```

- [ ] **Step 2: Create `ui/src/lib/utils.ts`**
  ```ts
  import { clsx, type ClassValue } from "clsx";
  import { twMerge } from "tailwind-merge";

  export function cn(...inputs: ClassValue[]) {
    return twMerge(clsx(inputs));
  }
  ```

- [ ] **Step 3: Run shadcn add for the initial primitive set**
  ```bash
  cd ui && npx shadcn@latest add button dialog dropdown-menu input popover scroll-area separator tooltip command --yes
  ```
  Expected: 9 files created under `src/components/ui/`. If the CLI prompts about Tailwind v4 compatibility, accept the new-york style.

- [ ] **Step 4: Commit**
  ```bash
  cd ..
  git add ui/components.json ui/src/lib/utils.ts ui/src/components/ui/
  git commit -m "feat(ui): shadcn/ui initial primitives"
  ```

### Task 0.9: React entry + placeholder App

**Files:**
- Create: `ui/src/main.tsx`
- Create: `ui/src/App.tsx`

- [ ] **Step 1: Create `ui/src/main.tsx`**
  ```tsx
  import { StrictMode } from "react";
  import { createRoot } from "react-dom/client";
  import "./styles/globals.css";
  import App from "./App";

  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
  ```

- [ ] **Step 2: Create `ui/src/App.tsx` (placeholder — replaced in Wave 1)**
  ```tsx
  export default function App() {
    return (
      <div className="grid min-h-screen place-items-center text-[var(--color-text)]">
        <div className="text-center">
          <h1 className="font-sans text-2xl font-semibold">docsiq</h1>
          <p className="text-[var(--color-text-muted)] font-mono text-sm mt-2">wave-0 scaffold complete</p>
        </div>
      </div>
    );
  }
  ```

- [ ] **Step 3: Run a real build**
  ```bash
  cd ui && npm run build 2>&1 | tail -6
  ```
  Expected: exit 0; dist/ contains index.html + assets/*.js + assets/*.css.

- [ ] **Step 4: Verify go:embed still picks it up**
  ```bash
  cd /home/dev/projects/docsiq && CGO_ENABLED=1 go build -tags sqlite_fts5 -o docsiq ./ && ls -la docsiq
  ```
  Expected: binary built, size ~25-27 MB.

- [ ] **Step 5: Commit**
  ```bash
  git add ui/src/main.tsx ui/src/App.tsx ui/dist/
  git commit -m "feat(ui): React entry + placeholder App"
  ```

---

## Wave 1 — Layout shell + providers

### Task 1.1: Zustand stores

**Files:**
- Create: `ui/src/stores/ui.ts`, `ui/src/stores/project.ts`, `ui/src/stores/toast.ts`
- Test: `ui/src/stores/__tests__/ui.test.ts`

- [ ] **Step 1: Create `ui/src/stores/ui.ts`**
  ```ts
  import { create } from "zustand";
  import { persist } from "zustand/middleware";

  type Theme = "light" | "dark" | "system";

  interface UIState {
    sidebarCollapsed: boolean;
    theme: Theme;
    treeDrawerPinned: boolean;
    linkDrawerPinned: boolean;
    setSidebarCollapsed: (v: boolean) => void;
    toggleSidebar: () => void;
    setTheme: (t: Theme) => void;
    setTreeDrawerPinned: (v: boolean) => void;
    setLinkDrawerPinned: (v: boolean) => void;
  }

  export const useUIStore = create<UIState>()(
    persist(
      (set) => ({
        sidebarCollapsed: false,
        theme: "system",
        treeDrawerPinned: false,
        linkDrawerPinned: false,
        setSidebarCollapsed: (sidebarCollapsed) => set({ sidebarCollapsed }),
        toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
        setTheme: (theme) => set({ theme }),
        setTreeDrawerPinned: (treeDrawerPinned) => set({ treeDrawerPinned }),
        setLinkDrawerPinned: (linkDrawerPinned) => set({ linkDrawerPinned }),
      }),
      { name: "docsiq-ui" },
    ),
  );
  ```

- [ ] **Step 2: Create `ui/src/stores/project.ts`**
  ```ts
  import { create } from "zustand";

  interface ProjectState {
    slug: string;
    setSlug: (s: string) => void;
  }

  export const useProjectStore = create<ProjectState>((set) => ({
    slug: "_default",
    setSlug: (slug) => set({ slug }),
  }));
  ```

- [ ] **Step 3: Create `ui/src/stores/toast.ts`**
  ```ts
  import { create } from "zustand";

  export type ToastKind = "info" | "success" | "error";

  export interface Toast {
    id: string;
    kind: ToastKind;
    message: string;
    createdAt: number;
  }

  interface ToastState {
    toasts: Toast[];
    push: (kind: ToastKind, message: string) => void;
    dismiss: (id: string) => void;
  }

  export const useToastStore = create<ToastState>((set) => ({
    toasts: [],
    push: (kind, message) =>
      set((s) => ({
        toasts: [
          ...s.toasts,
          { id: crypto.randomUUID(), kind, message, createdAt: Date.now() },
        ],
      })),
    dismiss: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
  }));
  ```

- [ ] **Step 4: Write `ui/src/stores/__tests__/ui.test.ts`**
  ```ts
  import { describe, it, expect, beforeEach } from "vitest";
  import { act } from "@testing-library/react";
  import { useUIStore } from "../ui";

  describe("useUIStore", () => {
    beforeEach(() => {
      localStorage.clear();
      useUIStore.setState({ sidebarCollapsed: false, theme: "system", treeDrawerPinned: false, linkDrawerPinned: false });
    });

    it("toggles sidebar", () => {
      expect(useUIStore.getState().sidebarCollapsed).toBe(false);
      act(() => useUIStore.getState().toggleSidebar());
      expect(useUIStore.getState().sidebarCollapsed).toBe(true);
      act(() => useUIStore.getState().toggleSidebar());
      expect(useUIStore.getState().sidebarCollapsed).toBe(false);
    });

    it("sets theme", () => {
      act(() => useUIStore.getState().setTheme("dark"));
      expect(useUIStore.getState().theme).toBe("dark");
    });

    it("persists theme to localStorage", () => {
      act(() => useUIStore.getState().setTheme("light"));
      const persisted = JSON.parse(localStorage.getItem("docsiq-ui")!);
      expect(persisted.state.theme).toBe("light");
    });
  });
  ```

- [ ] **Step 5: Run tests**
  ```bash
  cd ui && npm test -- stores/
  ```
  Expected: 3 pass.

- [ ] **Step 6: Commit**
  ```bash
  cd ..
  git add ui/src/stores/
  git commit -m "feat(ui): zustand stores (ui, project, toast)"
  ```

### Task 1.2: Typed API client + query key registry

**Files:**
- Create: `ui/src/lib/api-client.ts`
- Create: `ui/src/types/api.ts`
- Create: `ui/src/hooks/api/keys.ts`
- Test: `ui/src/lib/__tests__/api-client.test.ts`

- [ ] **Step 1: Create `ui/src/types/api.ts`** (shared response shapes with backend — hand-mirrored from `internal/api/handlers.go` + `notes_handlers.go`)
  ```ts
  export interface Stats {
    documents: number;
    chunks: number;
    entities: number;
    relationships: number;
    communities: number;
    notes: number;
    last_indexed: string | null;
  }

  export interface Project { slug: string; name: string; }

  export interface Note {
    key: string;
    content: string;
    author?: string;
    tags: string[];
    created_at: string;
    updated_at: string;
  }

  export interface NoteHit {
    key: string;
    title: string;
    snippet: string;
    tags: string[];
    rank: number;
  }

  export interface Document {
    id: string;
    path: string;
    title: string;
    doc_type: string;
    version: number;
    is_latest: boolean;
    created_at: number;
    updated_at: number;
  }

  export interface SearchHit {
    chunk_id: string;
    doc_id: string;
    doc_title: string;
    content: string;
    score: number;
  }

  export interface ApiError { error: string; request_id?: string; }
  ```

- [ ] **Step 2: Create `ui/src/lib/api-client.ts`**
  ```ts
  import type { ApiError } from "@/types/api";

  let bearer: string | null = null;

  function readBearerFromMeta(): string | null {
    if (typeof document === "undefined") return null;
    const m = document.querySelector('meta[name="docsiq-api-key"]');
    const v = m?.getAttribute("content");
    return v && v.length > 0 ? v : null;
  }

  export function initAuth() {
    bearer = readBearerFromMeta();
  }

  export class ApiErrorResponse extends Error {
    status: number;
    requestId?: string;
    constructor(status: number, body: ApiError) {
      super(body.error);
      this.status = status;
      this.requestId = body.request_id;
    }
  }

  export async function apiFetch<T>(
    path: string,
    init: RequestInit = {},
  ): Promise<T> {
    const headers = new Headers(init.headers);
    if (bearer) headers.set("Authorization", `Bearer ${bearer}`);
    if (init.body && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
    const res = await fetch(path, { ...init, headers });
    if (!res.ok) {
      let body: ApiError = { error: `HTTP ${res.status}` };
      try { body = await res.json(); } catch { /* non-json body */ }
      throw new ApiErrorResponse(res.status, body);
    }
    if (res.status === 204) return undefined as T;
    return res.json() as Promise<T>;
  }
  ```

- [ ] **Step 3: Create `ui/src/hooks/api/keys.ts`**
  ```ts
  export const qk = {
    stats: (project: string) => ["stats", project] as const,
    projects: () => ["projects"] as const,
    notes: (project: string) => ["notes", project] as const,
    note: (project: string, key: string) => ["note", project, key] as const,
    notesTree: (project: string) => ["notes-tree", project] as const,
    notesGraph: (project: string) => ["notes-graph", project] as const,
    notesSearch: (project: string, q: string) => ["notes-search", project, q] as const,
    docs: (project: string) => ["docs", project] as const,
    doc: (project: string, id: string) => ["doc", project, id] as const,
    search: (project: string, q: string, mode: string) => ["search", project, q, mode] as const,
    entities: (project: string) => ["entities", project] as const,
    communities: (project: string) => ["communities", project] as const,
    activity: (project: string) => ["activity", project] as const,
  };
  ```

- [ ] **Step 4: Write `ui/src/lib/__tests__/api-client.test.ts`**
  ```ts
  import { describe, it, expect } from "vitest";
  import { http, HttpResponse } from "msw";
  import { server } from "@/test/msw";
  import { apiFetch, ApiErrorResponse } from "../api-client";

  describe("apiFetch", () => {
    it("returns parsed json on 200", async () => {
      server.use(http.get("/api/ok", () => HttpResponse.json({ hello: "world" })));
      const body = await apiFetch<{ hello: string }>("/api/ok");
      expect(body.hello).toBe("world");
    });

    it("throws ApiErrorResponse on 4xx with error + request_id", async () => {
      server.use(
        http.get("/api/bad", () =>
          HttpResponse.json({ error: "nope", request_id: "req-123" }, { status: 400 }),
        ),
      );
      try {
        await apiFetch("/api/bad");
        throw new Error("should not reach");
      } catch (e) {
        expect(e).toBeInstanceOf(ApiErrorResponse);
        expect((e as ApiErrorResponse).status).toBe(400);
        expect((e as ApiErrorResponse).requestId).toBe("req-123");
        expect((e as ApiErrorResponse).message).toBe("nope");
      }
    });

    it("handles 204 no-content", async () => {
      server.use(http.delete("/api/x", () => new HttpResponse(null, { status: 204 })));
      const r = await apiFetch("/api/x", { method: "DELETE" });
      expect(r).toBeUndefined();
    });
  });
  ```

- [ ] **Step 5: Run tests**
  ```bash
  cd ui && npm test -- lib/
  ```
  Expected: 3 pass.

- [ ] **Step 6: Commit**
  ```bash
  cd ..
  git add ui/src/types/ ui/src/lib/api-client.ts ui/src/hooks/api/keys.ts ui/src/lib/__tests__/
  git commit -m "feat(ui): typed api client + query key registry"
  ```

### Task 1.3: i18n scaffold + formatting helpers

**Files:**
- Create: `ui/src/i18n/en.ts`, `ui/src/i18n/index.ts`
- Create: `ui/src/lib/format.ts`
- Test: `ui/src/lib/__tests__/format.test.ts`

- [ ] **Step 1: Create `ui/src/i18n/en.ts`**
  ```ts
  export const en = {
    common: {
      loading: "Loading…",
      error: "Something went wrong.",
      retry: "Retry",
      cancel: "Cancel",
      save: "Save",
      delete: "Delete",
      close: "Close",
    },
    nav: {
      home: "Home",
      notes: "Notes",
      documents: "Documents",
      graph: "Graph",
      mcp: "MCP console",
      search: "Search or jump to…",
      searchShort: "Search",
      skipToMain: "Skip to main content",
    },
    home: {
      sinceLastVisit: "Since your last visit",
      nothingNew: "Nothing new since your last visit.",
      viewFullActivity: "View full activity",
      pinnedNotes: "Pinned notes",
      graphGlance: "Graph glance",
      stats: {
        notes: "Notes",
        docs: "Docs",
        entities: "Entities",
        communities: "Communities",
        updated: "Updated",
      },
    },
    notes: {
      writtenBy: "Written by",
      linksIn: "in",
      linksOut: "out",
      noContent: "This note has no content.",
      invalidKey: "Invalid key — use letters, digits, /, -, _",
    },
  } as const;

  export type Messages = typeof en;
  ```

- [ ] **Step 2: Create `ui/src/i18n/index.ts`**
  ```ts
  import { en } from "./en";

  type PathsOf<T, P extends string = ""> = T extends string
    ? P
    : T extends object
    ? {
        [K in keyof T & string]: PathsOf<T[K], P extends "" ? K : `${P}.${K}`>;
      }[keyof T & string]
    : never;

  export type MessageKey = PathsOf<typeof en>;

  export function t(key: MessageKey): string {
    const parts = (key as string).split(".");
    let cur: unknown = en;
    for (const p of parts) {
      if (typeof cur !== "object" || cur === null || !(p in cur)) return key as string;
      cur = (cur as Record<string, unknown>)[p];
    }
    return typeof cur === "string" ? cur : (key as string);
  }
  ```

- [ ] **Step 3: Create `ui/src/lib/format.ts`**
  ```ts
  const rtf = new Intl.RelativeTimeFormat("en", { numeric: "auto" });

  export function formatRelativeTime(fromMs: number, now: number = Date.now()): string {
    const diffMs = fromMs - now;
    const abs = Math.abs(diffMs);
    const min = 60_000;
    const hr = 60 * min;
    const day = 24 * hr;
    if (abs < min) return rtf.format(Math.round(diffMs / 1000), "second");
    if (abs < hr) return rtf.format(Math.round(diffMs / min), "minute");
    if (abs < day) return rtf.format(Math.round(diffMs / hr), "hour");
    return rtf.format(Math.round(diffMs / day), "day");
  }

  export function formatCount(n: number): string {
    if (n < 1000) return String(n);
    if (n < 1_000_000) return (n / 1000).toFixed(n < 10_000 ? 1 : 0) + "k";
    return (n / 1_000_000).toFixed(1) + "m";
  }
  ```

- [ ] **Step 4: Write `ui/src/lib/__tests__/format.test.ts`**
  ```ts
  import { describe, it, expect } from "vitest";
  import { formatRelativeTime, formatCount } from "../format";

  describe("formatRelativeTime", () => {
    const now = new Date("2026-04-18T12:00:00Z").getTime();
    it("formats minutes-ago", () => {
      expect(formatRelativeTime(now - 3 * 60_000, now)).toMatch(/3 minutes? ago/);
    });
    it("formats hours-ago", () => {
      expect(formatRelativeTime(now - 2 * 3600_000, now)).toMatch(/2 hours? ago/);
    });
    it("formats days-ago", () => {
      expect(formatRelativeTime(now - 3 * 86400_000, now)).toMatch(/3 days? ago/);
    });
  });

  describe("formatCount", () => {
    it("raw for < 1k", () => expect(formatCount(42)).toBe("42"));
    it("k for thousands", () => expect(formatCount(1234)).toBe("1.2k"));
    it("k rounded for >= 10k", () => expect(formatCount(12_345)).toBe("12k"));
    it("m for millions", () => expect(formatCount(1_234_567)).toBe("1.2m"));
  });
  ```

- [ ] **Step 5: Run**
  ```bash
  cd ui && npm test -- lib/
  ```
  Expected: previous 3 + 7 new = 10 pass.

- [ ] **Step 6: Commit**
  ```bash
  cd ..
  git add ui/src/i18n/ ui/src/lib/format.ts ui/src/lib/__tests__/format.test.ts
  git commit -m "feat(ui): i18n scaffold + formatRelativeTime / formatCount"
  ```

### Task 1.4: Providers root (QueryClient + Theme + BrowserRouter)

**Files:**
- Create: `ui/src/components/layout/Providers.tsx`
- Modify: `ui/src/App.tsx`

- [ ] **Step 1: Create `ui/src/components/layout/Providers.tsx`**
  ```tsx
  import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
  import { useEffect, useState, type ReactNode } from "react";
  import { BrowserRouter } from "react-router-dom";
  import { useUIStore } from "@/stores/ui";

  export function Providers({ children }: { children: ReactNode }) {
    const [client] = useState(
      () =>
        new QueryClient({
          defaultOptions: {
            queries: {
              staleTime: 30_000,
              retry: (failureCount, error: unknown) => {
                const status = (error as { status?: number })?.status ?? 0;
                if (status >= 400 && status < 500) return false;
                return failureCount < 3;
              },
              refetchOnWindowFocus: false,
            },
          },
        }),
    );

    const theme = useUIStore((s) => s.theme);
    useEffect(() => {
      const root = document.documentElement;
      const systemDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
      const effective = theme === "system" ? (systemDark ? "dark" : "light") : theme;
      root.dataset.theme = effective;
    }, [theme]);

    return (
      <QueryClientProvider client={client}>
        <BrowserRouter>{children}</BrowserRouter>
      </QueryClientProvider>
    );
  }
  ```

- [ ] **Step 2: Replace `ui/src/App.tsx`**
  ```tsx
  import { useEffect } from "react";
  import { Providers } from "@/components/layout/Providers";
  import { initAuth } from "@/lib/api-client";

  export default function App() {
    useEffect(() => {
      initAuth();
    }, []);

    return (
      <Providers>
        <div className="grid min-h-screen place-items-center">
          <div className="text-center">
            <h1 className="font-sans text-2xl font-semibold">docsiq</h1>
            <p className="text-[var(--color-text-muted)] font-mono text-sm mt-2">
              wave-1 scaffold
            </p>
          </div>
        </div>
      </Providers>
    );
  }
  ```

- [ ] **Step 3: Typecheck**
  ```bash
  cd ui && npm run typecheck
  ```
  Expected: exit 0.

- [ ] **Step 4: Commit**
  ```bash
  cd ..
  git add ui/src/components/layout/Providers.tsx ui/src/App.tsx
  git commit -m "feat(ui): Providers (QueryClient + Router + theme effect)"
  ```

### Task 1.5: Layout shell — Sidebar + TopBar + Shell

**Files:**
- Create: `ui/src/components/layout/SkipLink.tsx`
- Create: `ui/src/components/layout/Sidebar.tsx`
- Create: `ui/src/components/layout/TopBar.tsx`
- Create: `ui/src/components/layout/Shell.tsx`
- Test: `ui/src/components/layout/__tests__/Shell.test.tsx`

- [ ] **Step 1: Create `ui/src/components/layout/SkipLink.tsx`**
  ```tsx
  export function SkipLink() {
    return (
      <a
        href="#main"
        className="fixed top-2 left-2 z-50 -translate-y-20 focus-visible:translate-y-0 transition-transform bg-[var(--color-accent)] text-[var(--color-accent-contrast)] px-3 py-2 rounded-md text-sm font-medium"
      >
        Skip to main content
      </a>
    );
  }
  ```

- [ ] **Step 2: Create `ui/src/components/layout/Sidebar.tsx`**
  ```tsx
  import { NavLink } from "react-router-dom";
  import { Home as HomeIcon, FileText, BookOpen, Network, Terminal } from "lucide-react";
  import { cn } from "@/lib/utils";
  import { useUIStore } from "@/stores/ui";
  import { t } from "@/i18n";

  interface NavItem { to: string; label: string; icon: React.ComponentType<{ size?: number }>; chord: string; }

  const ITEMS: NavItem[] = [
    { to: "/", label: t("nav.home"), icon: HomeIcon, chord: "G H" },
    { to: "/notes", label: t("nav.notes"), icon: FileText, chord: "G N" },
    { to: "/docs", label: t("nav.documents"), icon: BookOpen, chord: "G D" },
    { to: "/graph", label: t("nav.graph"), icon: Network, chord: "G G" },
    { to: "/mcp", label: t("nav.mcp"), icon: Terminal, chord: "G M" },
  ];

  export function Sidebar() {
    const collapsed = useUIStore((s) => s.sidebarCollapsed);

    return (
      <aside
        role="navigation"
        aria-label="Primary"
        className={cn(
          "border-r border-[var(--color-border)] bg-[var(--color-surface-1)] flex flex-col",
          collapsed ? "w-[56px]" : "w-[220px]",
        )}
      >
        <nav className="p-2 flex flex-col gap-1 flex-1" aria-label="Main">
          {ITEMS.map(({ to, label, icon: Icon, chord }) => (
            <NavLink
              key={to}
              to={to}
              end={to === "/"}
              className={({ isActive }) =>
                cn(
                  "flex items-center gap-3 rounded-md px-3 py-1.5 text-sm",
                  "hover:bg-[var(--color-surface-2)] transition-colors",
                  isActive && "bg-[var(--color-surface-2)] text-[var(--color-text)]",
                  !isActive && "text-[var(--color-text-muted)]",
                )
              }
              title={collapsed ? label : undefined}
            >
              <Icon size={16} />
              {!collapsed && (
                <>
                  <span className="flex-1">{label}</span>
                  <span className="font-mono text-[10px] text-[var(--color-text-faint)]">
                    {chord}
                  </span>
                </>
              )}
            </NavLink>
          ))}
        </nav>
      </aside>
    );
  }
  ```

- [ ] **Step 3: Create `ui/src/components/layout/TopBar.tsx`**
  ```tsx
  import { useUIStore } from "@/stores/ui";
  import { t } from "@/i18n";
  import { PanelLeft } from "lucide-react";

  interface TopBarProps { onCommandOpen: () => void; }

  export function TopBar({ onCommandOpen }: TopBarProps) {
    const toggle = useUIStore((s) => s.toggleSidebar);

    return (
      <header className="flex items-center gap-4 h-11 px-3 border-b border-[var(--color-border)] bg-[var(--color-surface-1)]">
        <button
          onClick={toggle}
          aria-label="Toggle sidebar"
          className="p-1.5 rounded-md hover:bg-[var(--color-surface-2)] transition-colors"
        >
          <PanelLeft size={16} />
        </button>
        <span className="font-mono text-sm text-[var(--color-text)]">docsiq</span>
        <span className="text-[var(--color-text-faint)]">/</span>
        <span className="font-mono text-sm text-[var(--color-text-muted)]">_default</span>
        <button
          onClick={onCommandOpen}
          aria-label="Open command palette"
          className="ml-auto flex items-center gap-2 px-3 py-1.5 rounded-md border border-[var(--color-border-strong)] bg-[var(--color-base)] text-sm text-[var(--color-text-muted)] hover:bg-[var(--color-surface-2)] transition-colors"
        >
          <span>{t("nav.search")}</span>
          <kbd className="font-mono text-[10px] px-1.5 py-0.5 rounded bg-[var(--color-surface-2)] border border-[var(--color-border)]">⌘K</kbd>
        </button>
      </header>
    );
  }
  ```

- [ ] **Step 4: Create `ui/src/components/layout/Shell.tsx`**
  ```tsx
  import { type ReactNode, useState } from "react";
  import { Sidebar } from "./Sidebar";
  import { TopBar } from "./TopBar";
  import { SkipLink } from "./SkipLink";

  export function Shell({ children }: { children: ReactNode }) {
    const [cmdOpen, setCmdOpen] = useState(false);
    return (
      <div className="min-h-screen flex flex-col">
        <SkipLink />
        <TopBar onCommandOpen={() => setCmdOpen(true)} />
        <div className="flex flex-1 min-h-0">
          <Sidebar />
          <main id="main" role="main" tabIndex={-1} className="flex-1 min-w-0 overflow-auto">
            {children}
          </main>
        </div>
        {/* CommandPalette drop-in happens in Wave 3; until then cmdOpen is unused */}
        <span className="sr-only" aria-hidden>{cmdOpen ? "open" : "closed"}</span>
      </div>
    );
  }
  ```

- [ ] **Step 5: Write `ui/src/components/layout/__tests__/Shell.test.tsx`**
  ```tsx
  import { describe, it, expect } from "vitest";
  import { render, screen } from "@testing-library/react";
  import { MemoryRouter } from "react-router-dom";
  import { Shell } from "../Shell";

  describe("Shell", () => {
    it("renders sidebar, topbar, skip link, and main landmark", () => {
      render(
        <MemoryRouter>
          <Shell>
            <div>content</div>
          </Shell>
        </MemoryRouter>,
      );
      expect(screen.getByRole("navigation", { name: /primary/i })).toBeInTheDocument();
      expect(screen.getByRole("main")).toBeInTheDocument();
      expect(screen.getByText(/skip to main content/i)).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /open command palette/i })).toBeInTheDocument();
      expect(screen.getByText("content")).toBeInTheDocument();
    });
  });
  ```

- [ ] **Step 6: Run**
  ```bash
  cd ui && npm test -- layout/
  ```
  Expected: 1 pass.

- [ ] **Step 7: Commit**
  ```bash
  cd ..
  git add ui/src/components/layout/
  git commit -m "feat(ui): layout shell (Sidebar, TopBar, SkipLink)"
  ```

### Task 1.6: useHotkey hook + sidebar toggle wired up

**Files:**
- Create: `ui/src/hooks/useHotkey.ts`
- Test: `ui/src/hooks/__tests__/useHotkey.test.tsx`
- Modify: `ui/src/components/layout/Shell.tsx`

- [ ] **Step 1: Create `ui/src/hooks/useHotkey.ts`**
  ```ts
  import { useEffect, useRef } from "react";

  interface Options {
    enabled?: boolean;
    preventDefault?: boolean;
  }

  // combo: "mod+k" or "mod+\\" or "g,h" (chord — g then h within 1s)
  export function useHotkey(combo: string, handler: (e: KeyboardEvent) => void, opts: Options = {}) {
    const { enabled = true, preventDefault = true } = opts;
    const handlerRef = useRef(handler);
    handlerRef.current = handler;

    useEffect(() => {
      if (!enabled) return;
      const chord = combo.includes(",");
      let lastKey: string | null = null;
      let lastTime = 0;

      const onKeyDown = (e: KeyboardEvent) => {
        const key = e.key.toLowerCase();
        const mod = e.metaKey || e.ctrlKey;

        if (chord) {
          const [first, second] = combo.split(",");
          if (!lastKey) {
            if (key === first) { lastKey = key; lastTime = Date.now(); return; }
          } else {
            if (key === second && Date.now() - lastTime < 1000) {
              if (preventDefault) e.preventDefault();
              handlerRef.current(e);
            }
            lastKey = null;
          }
          return;
        }

        const parts = combo.split("+");
        const needsMod = parts.includes("mod");
        const target = parts[parts.length - 1];
        if (needsMod === mod && key === target) {
          if (preventDefault) e.preventDefault();
          handlerRef.current(e);
        }
      };

      window.addEventListener("keydown", onKeyDown);
      return () => window.removeEventListener("keydown", onKeyDown);
    }, [combo, enabled, preventDefault]);
  }
  ```

- [ ] **Step 2: Write `ui/src/hooks/__tests__/useHotkey.test.tsx`**
  ```tsx
  import { describe, it, expect, vi } from "vitest";
  import { renderHook } from "@testing-library/react";
  import { useHotkey } from "../useHotkey";

  function fireKey(key: string, mod = false) {
    const e = new KeyboardEvent("keydown", { key, metaKey: mod, ctrlKey: mod, cancelable: true });
    window.dispatchEvent(e);
  }

  describe("useHotkey", () => {
    it("fires on mod+k", () => {
      const fn = vi.fn();
      renderHook(() => useHotkey("mod+k", fn));
      fireKey("k", true);
      expect(fn).toHaveBeenCalledTimes(1);
    });

    it("does not fire on k without mod", () => {
      const fn = vi.fn();
      renderHook(() => useHotkey("mod+k", fn));
      fireKey("k", false);
      expect(fn).not.toHaveBeenCalled();
    });

    it("fires on G H chord", () => {
      const fn = vi.fn();
      renderHook(() => useHotkey("g,h", fn));
      fireKey("g");
      fireKey("h");
      expect(fn).toHaveBeenCalledTimes(1);
    });

    it("aborts chord if wrong second key", () => {
      const fn = vi.fn();
      renderHook(() => useHotkey("g,h", fn));
      fireKey("g");
      fireKey("x");
      expect(fn).not.toHaveBeenCalled();
    });
  });
  ```

- [ ] **Step 3: Run**
  ```bash
  cd ui && npm test -- hooks/
  ```
  Expected: 4 pass.

- [ ] **Step 4: Wire `⌘\` sidebar toggle + G-chord nav in Shell.tsx** — replace `ui/src/components/layout/Shell.tsx` contents:
  ```tsx
  import { type ReactNode, useState } from "react";
  import { useNavigate } from "react-router-dom";
  import { Sidebar } from "./Sidebar";
  import { TopBar } from "./TopBar";
  import { SkipLink } from "./SkipLink";
  import { useUIStore } from "@/stores/ui";
  import { useHotkey } from "@/hooks/useHotkey";

  export function Shell({ children }: { children: ReactNode }) {
    const [cmdOpen, setCmdOpen] = useState(false);
    const toggleSidebar = useUIStore((s) => s.toggleSidebar);
    const navigate = useNavigate();

    useHotkey("mod+\\", () => toggleSidebar());
    useHotkey("mod+k", () => setCmdOpen((v) => !v));
    useHotkey("g,h", () => navigate("/"));
    useHotkey("g,n", () => navigate("/notes"));
    useHotkey("g,d", () => navigate("/docs"));
    useHotkey("g,g", () => navigate("/graph"));
    useHotkey("g,m", () => navigate("/mcp"));

    return (
      <div className="min-h-screen flex flex-col">
        <SkipLink />
        <TopBar onCommandOpen={() => setCmdOpen(true)} />
        <div className="flex flex-1 min-h-0">
          <Sidebar />
          <main id="main" role="main" tabIndex={-1} className="flex-1 min-w-0 overflow-auto">
            {children}
          </main>
        </div>
        <span className="sr-only" aria-hidden>{cmdOpen ? "open" : "closed"}</span>
      </div>
    );
  }
  ```

- [ ] **Step 5: Typecheck + test**
  ```bash
  cd ui && npm run typecheck && npm test -- layout/
  ```
  Expected: typecheck exit 0; Shell test still passes.

- [ ] **Step 6: Commit**
  ```bash
  cd ..
  git add ui/src/hooks/ ui/src/components/layout/Shell.tsx
  git commit -m "feat(ui): useHotkey + wire ⌘\\, ⌘K, G-chord navigation"
  ```

---

## Wave 2 — Router + route stubs + reduced-motion

### Task 2.1: Route stubs for all 5 destinations

**Files:**
- Create: `ui/src/routes/Home.tsx`, `ui/src/routes/Graph.tsx`, `ui/src/routes/MCPConsole.tsx`
- Create: `ui/src/routes/notes/NotesLayout.tsx`, `ui/src/routes/notes/NoteView.tsx`
- Create: `ui/src/routes/documents/DocumentsList.tsx`, `ui/src/routes/documents/DocumentView.tsx`
- Modify: `ui/src/App.tsx` (wire router)

- [ ] **Step 1: Create 5 stub route files** with identical pattern (replace `<name>` per file):
  ```tsx
  export default function RouteNameStub() {
    return (
      <div className="p-6">
        <h1 className="text-xl font-semibold">Route name</h1>
        <p className="text-sm text-[var(--color-text-muted)] mt-2">
          Stub — implemented in later wave.
        </p>
      </div>
    );
  }
  ```
  Create the 7 files listed under "Files" above, each returning a stub identifying the route by name.

- [ ] **Step 2: Rewrite `ui/src/App.tsx`**
  ```tsx
  import { useEffect } from "react";
  import { Route, Routes } from "react-router-dom";
  import { Providers } from "@/components/layout/Providers";
  import { Shell } from "@/components/layout/Shell";
  import { initAuth } from "@/lib/api-client";

  import Home from "@/routes/Home";
  import NotesLayout from "@/routes/notes/NotesLayout";
  import NoteView from "@/routes/notes/NoteView";
  import DocumentsList from "@/routes/documents/DocumentsList";
  import DocumentView from "@/routes/documents/DocumentView";
  import Graph from "@/routes/Graph";
  import MCPConsole from "@/routes/MCPConsole";

  export default function App() {
    useEffect(() => { initAuth(); }, []);
    return (
      <Providers>
        <Shell>
          <Routes>
            <Route path="/" element={<Home />} />
            <Route path="/notes" element={<NotesLayout />}>
              <Route path=":key" element={<NoteView />} />
            </Route>
            <Route path="/docs" element={<DocumentsList />} />
            <Route path="/docs/:id" element={<DocumentView />} />
            <Route path="/graph" element={<Graph />} />
            <Route path="/mcp" element={<MCPConsole />} />
            <Route path="*" element={<NotFound />} />
          </Routes>
        </Shell>
      </Providers>
    );
  }

  function NotFound() {
    return (
      <div className="p-6 text-[var(--color-text-muted)]">
        <h1 className="text-xl font-semibold text-[var(--color-text)]">Not found</h1>
        <p className="text-sm mt-2">No such page.</p>
      </div>
    );
  }
  ```

- [ ] **Step 3: Build + run**
  ```bash
  cd ui && npm run build && ls dist/
  ```
  Expected: exit 0, `dist/index.html` + `dist/assets/*.js` + `dist/assets/*.css` present.

- [ ] **Step 4: Commit (including fresh ui/dist)**
  ```bash
  cd ..
  git add ui/src/routes/ ui/src/App.tsx ui/dist/
  git commit -m "feat(ui): router + 5 route stubs"
  ```

### Task 2.2: `useReducedMotion` hook + motion utilities

**Files:**
- Create: `ui/src/hooks/useReducedMotion.ts`
- Create: `ui/src/lib/motion.ts`
- Test: `ui/src/hooks/__tests__/useReducedMotion.test.tsx`

- [ ] **Step 1: Create `ui/src/hooks/useReducedMotion.ts`**
  ```ts
  import { useEffect, useState } from "react";

  export function useReducedMotion(): boolean {
    const [prefers, setPrefers] = useState(() =>
      typeof window !== "undefined" &&
      window.matchMedia?.("(prefers-reduced-motion: reduce)").matches,
    );

    useEffect(() => {
      if (typeof window === "undefined") return;
      const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
      const onChange = () => setPrefers(mq.matches);
      mq.addEventListener("change", onChange);
      return () => mq.removeEventListener("change", onChange);
    }, []);

    return prefers;
  }
  ```

- [ ] **Step 2: Create `ui/src/lib/motion.ts`**
  ```ts
  import type { Transition } from "framer-motion";

  export const enterTransition: Transition = {
    duration: 0.18,
    ease: [0.3, 0, 0, 1],
  };

  export const exitTransition: Transition = {
    duration: 0.12,
    ease: [0.7, 0, 1, 0.3],
  };

  export function reducedMotionTransition(): Transition {
    return { duration: 0 };
  }
  ```

- [ ] **Step 3: Write `ui/src/hooks/__tests__/useReducedMotion.test.tsx`**
  ```tsx
  import { describe, it, expect, vi } from "vitest";
  import { renderHook } from "@testing-library/react";
  import { useReducedMotion } from "../useReducedMotion";

  function mockMatchMedia(matches: boolean) {
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      configurable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches,
        media: query,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
  }

  describe("useReducedMotion", () => {
    it("returns true when reduce is set", () => {
      mockMatchMedia(true);
      const { result } = renderHook(() => useReducedMotion());
      expect(result.current).toBe(true);
    });
    it("returns false otherwise", () => {
      mockMatchMedia(false);
      const { result } = renderHook(() => useReducedMotion());
      expect(result.current).toBe(false);
    });
  });
  ```

- [ ] **Step 4: Run**
  ```bash
  cd ui && npm test -- hooks/
  ```
  Expected: all hook tests pass.

- [ ] **Step 5: Commit**
  ```bash
  cd ..
  git add ui/src/hooks/useReducedMotion.ts ui/src/lib/motion.ts ui/src/hooks/__tests__/useReducedMotion.test.tsx
  git commit -m "feat(ui): useReducedMotion + motion tokens"
  ```

---

## Wave 3 — Command palette (⌘K)

### Task 3.1: CommandPalette component

**Files:**
- Create: `ui/src/components/command/CommandPalette.tsx`
- Create: `ui/src/hooks/api/useCommand.ts`
- Modify: `ui/src/components/layout/Shell.tsx` (render palette)
- Test: `ui/src/components/command/__tests__/CommandPalette.test.tsx`

- [ ] **Step 1: Create `ui/src/hooks/api/useCommand.ts`**
  ```ts
  import { useQuery } from "@tanstack/react-query";
  import { apiFetch } from "@/lib/api-client";
  import { qk } from "./keys";
  import type { NoteHit, SearchHit } from "@/types/api";

  export function useCommandSearch(project: string, query: string) {
    return useQuery({
      queryKey: ["command-search", project, query],
      enabled: query.trim().length > 0,
      queryFn: async () => {
        const [notes, docs] = await Promise.all([
          apiFetch<{ hits: NoteHit[] }>(
            `/api/projects/${encodeURIComponent(project)}/search?q=${encodeURIComponent(query)}`,
          ).catch(() => ({ hits: [] as NoteHit[] })),
          apiFetch<{ hits: SearchHit[] }>(
            `/api/search?project=${encodeURIComponent(project)}&q=${encodeURIComponent(query)}&mode=local&top_k=5`,
          ).catch(() => ({ hits: [] as SearchHit[] })),
        ]);
        return { notes: notes.hits, docs: docs.hits };
      },
      staleTime: 10_000,
    });
    /* qk import kept for future merged key usage */
    void qk;
  }
  ```

- [ ] **Step 2: Create `ui/src/components/command/CommandPalette.tsx`**
  ```tsx
  import { useState } from "react";
  import { useNavigate } from "react-router-dom";
  import {
    Command,
    CommandDialog,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
  } from "@/components/ui/command";
  import { useProjectStore } from "@/stores/project";
  import { useCommandSearch } from "@/hooks/api/useCommand";

  interface Props { open: boolean; onOpenChange: (v: boolean) => void; }

  export function CommandPalette({ open, onOpenChange }: Props) {
    const [q, setQ] = useState("");
    const navigate = useNavigate();
    const project = useProjectStore((s) => s.slug);
    const { data } = useCommandSearch(project, q);

    const close = () => { onOpenChange(false); setQ(""); };

    return (
      <CommandDialog open={open} onOpenChange={onOpenChange}>
        <Command shouldFilter={false}>
          <CommandInput
            value={q}
            onValueChange={setQ}
            placeholder="Search notes, docs, entities…"
            autoFocus
          />
          <CommandList>
            <CommandEmpty>{q ? "No results." : "Type to search."}</CommandEmpty>

            <CommandGroup heading="Pages">
              <CommandItem onSelect={() => { navigate("/"); close(); }}>Home</CommandItem>
              <CommandItem onSelect={() => { navigate("/notes"); close(); }}>Notes</CommandItem>
              <CommandItem onSelect={() => { navigate("/docs"); close(); }}>Documents</CommandItem>
              <CommandItem onSelect={() => { navigate("/graph"); close(); }}>Graph</CommandItem>
              <CommandItem onSelect={() => { navigate("/mcp"); close(); }}>MCP console</CommandItem>
            </CommandGroup>

            {data && data.notes.length > 0 && (
              <CommandGroup heading="Notes">
                {data.notes.slice(0, 5).map((n) => (
                  <CommandItem
                    key={`note-${n.key}`}
                    onSelect={() => { navigate(`/notes/${n.key}`); close(); }}
                  >
                    <span className="font-mono text-[10px] px-1.5 mr-2 rounded bg-[var(--color-surface-2)] text-[var(--color-text-muted)]">NOTE</span>
                    {n.title || n.key}
                  </CommandItem>
                ))}
              </CommandGroup>
            )}

            {data && data.docs.length > 0 && (
              <CommandGroup heading="Documents">
                {data.docs.slice(0, 5).map((d) => (
                  <CommandItem
                    key={`doc-${d.chunk_id}`}
                    onSelect={() => { navigate(`/docs/${d.doc_id}`); close(); }}
                  >
                    <span className="font-mono text-[10px] px-1.5 mr-2 rounded bg-[var(--color-surface-2)] text-[var(--color-text-muted)]">DOC</span>
                    {d.doc_title}
                  </CommandItem>
                ))}
              </CommandGroup>
            )}
          </CommandList>
        </Command>
      </CommandDialog>
    );
  }
  ```

- [ ] **Step 3: Update `ui/src/components/layout/Shell.tsx`** — replace the `<span className="sr-only">` placeholder with `<CommandPalette open={cmdOpen} onOpenChange={setCmdOpen} />`. Add import at the top: `import { CommandPalette } from "@/components/command/CommandPalette";`.

- [ ] **Step 4: Write `ui/src/components/command/__tests__/CommandPalette.test.tsx`**
  ```tsx
  import { describe, it, expect } from "vitest";
  import { render, screen } from "@testing-library/react";
  import userEvent from "@testing-library/user-event";
  import { MemoryRouter } from "react-router-dom";
  import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
  import { CommandPalette } from "../CommandPalette";

  function wrap(ui: React.ReactNode) {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    return (
      <QueryClientProvider client={qc}>
        <MemoryRouter>{ui}</MemoryRouter>
      </QueryClientProvider>
    );
  }

  describe("CommandPalette", () => {
    it("renders input + Pages group when open", () => {
      render(wrap(<CommandPalette open={true} onOpenChange={() => {}} />));
      expect(screen.getByPlaceholderText(/search notes/i)).toBeInTheDocument();
      expect(screen.getByText(/home/i)).toBeInTheDocument();
      expect(screen.getByText(/notes/i)).toBeInTheDocument();
    });

    it("shows 'Type to search.' when empty", () => {
      render(wrap(<CommandPalette open={true} onOpenChange={() => {}} />));
      expect(screen.getByText(/type to search/i)).toBeInTheDocument();
    });

    it("filters input via typing", async () => {
      const user = userEvent.setup();
      render(wrap(<CommandPalette open={true} onOpenChange={() => {}} />));
      const input = screen.getByPlaceholderText(/search notes/i);
      await user.type(input, "hello");
      expect(input).toHaveValue("hello");
    });
  });
  ```

- [ ] **Step 5: Run**
  ```bash
  cd ui && npm test -- command/
  ```
  Expected: 3 pass.

- [ ] **Step 6: Commit**
  ```bash
  cd ..
  git add ui/src/components/command/ ui/src/hooks/api/useCommand.ts ui/src/components/layout/Shell.tsx
  git commit -m "feat(ui): ⌘K command palette (pages + notes + docs search)"
  ```

---

## Wave 4 — Home screen

### Task 4.1: API hooks for stats + activity + recent notes + graph glance

**Files:**
- Create: `ui/src/hooks/api/useStats.ts`, `ui/src/hooks/api/useActivity.ts`, `ui/src/hooks/api/useNotes.ts`, `ui/src/hooks/api/useGraph.ts`
- Test: `ui/src/hooks/api/__tests__/useStats.test.tsx`

- [ ] **Step 1: Create `ui/src/hooks/api/useStats.ts`**
  ```ts
  import { useQuery } from "@tanstack/react-query";
  import { apiFetch } from "@/lib/api-client";
  import { qk } from "./keys";
  import type { Stats } from "@/types/api";

  export function useStats(project: string) {
    return useQuery({
      queryKey: qk.stats(project),
      queryFn: () => apiFetch<Stats>(`/api/stats?project=${encodeURIComponent(project)}`),
    });
  }
  ```

- [ ] **Step 2: Create `ui/src/hooks/api/useActivity.ts`** — derived client-side from existing list endpoints (since there's no single `/api/activity` endpoint yet; compose one in code).
  ```ts
  import { useQuery } from "@tanstack/react-query";
  import { apiFetch } from "@/lib/api-client";
  import { qk } from "./keys";
  import type { Note, Document } from "@/types/api";

  export type ActivityEventKind = "note_added" | "note_updated" | "doc_indexed" | "doc_error";

  export interface ActivityEvent {
    id: string;
    kind: ActivityEventKind;
    title: string;
    detail?: string;
    timestamp: number; // ms since epoch
    href: string;
  }

  export function useActivity(project: string) {
    return useQuery({
      queryKey: qk.activity(project),
      queryFn: async () => {
        const [notes, docs] = await Promise.all([
          apiFetch<Note[]>(`/api/projects/${encodeURIComponent(project)}/notes`).catch(() => []),
          apiFetch<Document[]>(`/api/documents?project=${encodeURIComponent(project)}`).catch(() => []),
        ]);
        const events: ActivityEvent[] = [];
        for (const n of notes) {
          const ts = new Date(n.updated_at).getTime();
          const isNew = ts === new Date(n.created_at).getTime();
          events.push({
            id: `note-${n.key}-${ts}`,
            kind: isNew ? "note_added" : "note_updated",
            title: n.key,
            timestamp: ts,
            href: `/notes/${n.key}`,
          });
        }
        for (const d of docs) {
          events.push({
            id: `doc-${d.id}-${d.updated_at}`,
            kind: "doc_indexed",
            title: d.title || d.path,
            detail: d.doc_type,
            timestamp: d.updated_at * 1000,
            href: `/docs/${d.id}`,
          });
        }
        events.sort((a, b) => b.timestamp - a.timestamp);
        return events.slice(0, 20);
      },
      refetchInterval: 10_000,
    });
  }
  ```

- [ ] **Step 3: Create `ui/src/hooks/api/useNotes.ts`**
  ```ts
  import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
  import { apiFetch } from "@/lib/api-client";
  import { qk } from "./keys";
  import type { Note } from "@/types/api";

  export function useNotes(project: string) {
    return useQuery({
      queryKey: qk.notes(project),
      queryFn: () => apiFetch<Note[]>(`/api/projects/${encodeURIComponent(project)}/notes`),
    });
  }

  export function useNote(project: string, key: string | undefined) {
    return useQuery({
      queryKey: qk.note(project, key ?? ""),
      enabled: !!key,
      queryFn: () =>
        apiFetch<Note>(`/api/projects/${encodeURIComponent(project)}/notes/${encodeURIComponent(key!)}`),
    });
  }

  export function useWriteNote(project: string) {
    const qc = useQueryClient();
    return useMutation({
      mutationFn: (input: { key: string; content: string; author?: string; tags?: string[] }) =>
        apiFetch<Note>(
          `/api/projects/${encodeURIComponent(project)}/notes/${encodeURIComponent(input.key)}`,
          { method: "PUT", body: JSON.stringify(input) },
        ),
      onSuccess: (_, v) => {
        qc.invalidateQueries({ queryKey: qk.notes(project) });
        qc.invalidateQueries({ queryKey: qk.note(project, v.key) });
        qc.invalidateQueries({ queryKey: qk.notesGraph(project) });
        qc.invalidateQueries({ queryKey: qk.activity(project) });
      },
    });
  }

  export function useDeleteNote(project: string) {
    const qc = useQueryClient();
    return useMutation({
      mutationFn: (key: string) =>
        apiFetch(
          `/api/projects/${encodeURIComponent(project)}/notes/${encodeURIComponent(key)}`,
          { method: "DELETE" },
        ),
      onSuccess: () => {
        qc.invalidateQueries({ queryKey: qk.notes(project) });
        qc.invalidateQueries({ queryKey: qk.notesGraph(project) });
        qc.invalidateQueries({ queryKey: qk.activity(project) });
      },
    });
  }
  ```

- [ ] **Step 4: Create `ui/src/hooks/api/useGraph.ts`**
  ```ts
  import { useQuery } from "@tanstack/react-query";
  import { apiFetch } from "@/lib/api-client";
  import { qk } from "./keys";

  export interface GraphNode { id: string; label: string; kind: "entity" | "note" | "community"; }
  export interface GraphEdge { source: string; target: string; }
  export interface GraphData { nodes: GraphNode[]; edges: GraphEdge[]; }

  export function useNotesGraph(project: string) {
    return useQuery({
      queryKey: qk.notesGraph(project),
      queryFn: () =>
        apiFetch<GraphData>(`/api/projects/${encodeURIComponent(project)}/graph`),
    });
  }
  ```

- [ ] **Step 5: Write `ui/src/hooks/api/__tests__/useStats.test.tsx`**
  ```tsx
  import { describe, it, expect } from "vitest";
  import { renderHook, waitFor } from "@testing-library/react";
  import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
  import { useStats } from "../useStats";

  function wrap() {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    // eslint-disable-next-line react/display-name
    return ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
  }

  describe("useStats", () => {
    it("fetches and returns stats from MSW handler", async () => {
      const { result } = renderHook(() => useStats("_default"), { wrapper: wrap() });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data?.documents).toBe(42);
      expect(result.current.data?.notes).toBe(17);
    });
  });
  ```

- [ ] **Step 6: Run**
  ```bash
  cd ui && npm test -- hooks/api/
  ```
  Expected: 1 pass.

- [ ] **Step 7: Commit**
  ```bash
  cd ..
  git add ui/src/hooks/api/
  git commit -m "feat(ui): api hooks (stats, activity, notes, graph)"
  ```

### Task 4.2: StatsStrip component

**Files:**
- Create: `ui/src/components/layout/StatsStrip.tsx`
- Test: `ui/src/components/layout/__tests__/StatsStrip.test.tsx`

- [ ] **Step 1: Create component**
  ```tsx
  import { formatCount, formatRelativeTime } from "@/lib/format";
  import { t } from "@/i18n";
  import type { Stats } from "@/types/api";

  interface Props { stats: Stats | undefined; delta?: { notes?: number }; }

  const CARD = "flex-1 border border-[var(--color-border)] rounded-md p-3 font-mono";
  const LABEL = "text-[10px] uppercase tracking-wider text-[var(--color-text-muted)]";
  const VALUE = "text-xl text-[var(--color-text)] mt-1";
  const DELTA = "text-xs text-[var(--color-accent)]";

  export function StatsStrip({ stats, delta }: Props) {
    const tiles = [
      { label: t("home.stats.notes"), value: stats ? formatCount(stats.notes) : "—", delta: delta?.notes },
      { label: t("home.stats.docs"), value: stats ? formatCount(stats.documents) : "—" },
      { label: t("home.stats.entities"), value: stats ? formatCount(stats.entities) : "—" },
      { label: t("home.stats.communities"), value: stats ? formatCount(stats.communities) : "—" },
      { label: t("home.stats.updated"), value: stats?.last_indexed ? formatRelativeTime(new Date(stats.last_indexed).getTime()) : "—" },
    ];
    return (
      <div role="region" aria-label="Project statistics" className="flex gap-3 mb-5 flex-wrap">
        {tiles.map((tl) => (
          <div key={tl.label} className={CARD}>
            <div className={LABEL}>{tl.label}</div>
            <div className={VALUE}>
              {tl.value}
              {tl.delta !== undefined && tl.delta > 0 && (
                <span className={`${DELTA} ml-2`}>+{tl.delta}</span>
              )}
            </div>
          </div>
        ))}
      </div>
    );
  }
  ```

- [ ] **Step 2: Write test**
  ```tsx
  import { describe, it, expect } from "vitest";
  import { render, screen } from "@testing-library/react";
  import { StatsStrip } from "../StatsStrip";

  describe("StatsStrip", () => {
    it("renders placeholders on undefined stats", () => {
      render(<StatsStrip stats={undefined} />);
      expect(screen.getAllByText("—").length).toBeGreaterThan(0);
    });
    it("renders counts + delta", () => {
      render(
        <StatsStrip
          stats={{
            documents: 42, chunks: 512, entities: 380, relationships: 820,
            communities: 8, notes: 17,
            last_indexed: new Date().toISOString(),
          }}
          delta={{ notes: 2 }}
        />,
      );
      expect(screen.getByText("17")).toBeInTheDocument();
      expect(screen.getByText("+2")).toBeInTheDocument();
      expect(screen.getByText("42")).toBeInTheDocument();
    });
  });
  ```

- [ ] **Step 3: Run + commit**
  ```bash
  cd ui && npm test -- StatsStrip
  cd ..
  git add ui/src/components/layout/StatsStrip.tsx ui/src/components/layout/__tests__/StatsStrip.test.tsx
  git commit -m "feat(ui): StatsStrip with formatted counts + deltas"
  ```

### Task 4.3: ActivityFeed component

**Files:**
- Create: `ui/src/components/activity/EventBadge.tsx`, `ui/src/components/activity/EventRow.tsx`, `ui/src/components/activity/ActivityFeed.tsx`
- Create: `ui/src/hooks/useLastVisit.ts`
- Test: `ui/src/components/activity/__tests__/ActivityFeed.test.tsx`

- [ ] **Step 1: Create `ui/src/hooks/useLastVisit.ts`**
  ```ts
  import { useEffect, useState } from "react";

  const KEY = "docsiq-last-visit";

  export function useLastVisit() {
    const [last, setLast] = useState<number>(() => {
      const v = localStorage.getItem(KEY);
      return v ? Number(v) : 0;
    });

    function touch() {
      const now = Date.now();
      localStorage.setItem(KEY, String(now));
      setLast(now);
    }

    return { lastVisit: last, touch };
  }

  export function useTouchOnUnmount() {
    useEffect(() => () => { localStorage.setItem("docsiq-last-visit", String(Date.now())); }, []);
  }
  ```

- [ ] **Step 2: Create `ui/src/components/activity/EventBadge.tsx`**
  ```tsx
  import type { ActivityEventKind } from "@/hooks/api/useActivity";

  const STYLES: Record<ActivityEventKind, { label: string; color: string }> = {
    note_added: { label: "+ NOTE", color: "var(--color-semantic-new)" },
    note_updated: { label: "~ NOTE", color: "var(--color-semantic-new)" },
    doc_indexed: { label: "INDEX", color: "var(--color-semantic-index)" },
    doc_error: { label: "ERROR", color: "var(--color-semantic-error)" },
  };

  export function EventBadge({ kind }: { kind: ActivityEventKind }) {
    const { label, color } = STYLES[kind];
    return (
      <span
        className="font-mono text-[10px] px-1.5 py-0.5 rounded"
        style={{ color, borderColor: color, border: "1px solid" }}
      >
        {label}
      </span>
    );
  }
  ```

- [ ] **Step 3: Create `ui/src/components/activity/EventRow.tsx`**
  ```tsx
  import { Link } from "react-router-dom";
  import { EventBadge } from "./EventBadge";
  import { formatRelativeTime } from "@/lib/format";
  import type { ActivityEvent } from "@/hooks/api/useActivity";

  export function EventRow({ event, isNew }: { event: ActivityEvent; isNew: boolean }) {
    return (
      <Link
        to={event.href}
        className="flex items-center gap-3 px-3 py-2 rounded-md border border-[var(--color-border)] hover:bg-[var(--color-surface-2)] transition-colors"
        style={isNew ? { background: "var(--color-surface-2)" } : undefined}
      >
        <EventBadge kind={event.kind} />
        <span className="flex-1 truncate text-sm text-[var(--color-text)]">
          {event.title}
          {event.detail && <span className="ml-2 text-[var(--color-text-muted)]">· {event.detail}</span>}
        </span>
        <span className="font-mono text-xs text-[var(--color-text-muted)]">
          {formatRelativeTime(event.timestamp)}
        </span>
      </Link>
    );
  }
  ```

- [ ] **Step 4: Create `ui/src/components/activity/ActivityFeed.tsx`**
  ```tsx
  import { EventRow } from "./EventRow";
  import { t } from "@/i18n";
  import type { ActivityEvent } from "@/hooks/api/useActivity";

  interface Props { events: ActivityEvent[]; lastVisit: number; }

  export function ActivityFeed({ events, lastVisit }: Props) {
    if (events.length === 0) {
      return (
        <div className="border border-dashed border-[var(--color-border)] rounded-md p-6 text-center text-sm text-[var(--color-text-muted)]">
          {t("home.nothingNew")}
        </div>
      );
    }
    return (
      <div>
        <h2 className="text-xs uppercase tracking-wider text-[var(--color-text-muted)] mb-2.5">
          {t("home.sinceLastVisit")}
        </h2>
        <div className="flex flex-col gap-1.5">
          {events.map((e) => (
            <EventRow key={e.id} event={e} isNew={e.timestamp > lastVisit} />
          ))}
        </div>
      </div>
    );
  }
  ```

- [ ] **Step 5: Write test**
  ```tsx
  import { describe, it, expect } from "vitest";
  import { render, screen } from "@testing-library/react";
  import { MemoryRouter } from "react-router-dom";
  import { ActivityFeed } from "../ActivityFeed";

  describe("ActivityFeed", () => {
    it("shows empty state on no events", () => {
      render(<MemoryRouter><ActivityFeed events={[]} lastVisit={0} /></MemoryRouter>);
      expect(screen.getByText(/nothing new/i)).toBeInTheDocument();
    });
    it("renders events and highlights ones newer than lastVisit", () => {
      const now = Date.now();
      render(
        <MemoryRouter>
          <ActivityFeed
            events={[
              { id: "1", kind: "note_added", title: "jwt", timestamp: now, href: "/notes/jwt" },
              { id: "2", kind: "doc_indexed", title: "api.md", timestamp: now - 3600_000, href: "/docs/1" },
            ]}
            lastVisit={now - 1800_000}
          />
        </MemoryRouter>,
      );
      expect(screen.getByText("+ NOTE")).toBeInTheDocument();
      expect(screen.getByText("INDEX")).toBeInTheDocument();
      expect(screen.getByText("jwt")).toBeInTheDocument();
      expect(screen.getByText("api.md")).toBeInTheDocument();
    });
  });
  ```

- [ ] **Step 6: Run + commit**
  ```bash
  cd ui && npm test -- activity/
  cd ..
  git add ui/src/components/activity/ ui/src/hooks/useLastVisit.ts
  git commit -m "feat(ui): ActivityFeed + EventRow + EventBadge + useLastVisit"
  ```

### Task 4.4: GraphGlance component

**Files:**
- Create: `ui/src/components/graph/GlanceView.tsx`
- Test: `ui/src/components/graph/__tests__/GlanceView.test.tsx`

- [ ] **Step 1: Create component (pure SVG, no d3 yet)**
  ```tsx
  import type { GraphData } from "@/hooks/api/useGraph";
  import { useMemo } from "react";

  interface Props { data: GraphData | undefined; maxNodes?: number; }

  const COLOR: Record<string, string> = {
    entity: "var(--color-semantic-new)",
    note: "var(--color-semantic-graph)",
    community: "var(--color-semantic-index)",
  };

  export function GlanceView({ data, maxNodes = 30 }: Props) {
    const layout = useMemo(() => {
      if (!data) return null;
      const nodes = data.nodes.slice(0, maxNodes);
      const n = nodes.length;
      const radius = 60;
      const placed = nodes.map((node, i) => ({
        node,
        x: 110 + radius * Math.cos((2 * Math.PI * i) / n),
        y: 70 + radius * Math.sin((2 * Math.PI * i) / n),
      }));
      const idx: Record<string, { x: number; y: number }> = {};
      placed.forEach((p) => (idx[p.node.id] = { x: p.x, y: p.y }));
      const edges = data.edges.filter((e) => idx[e.source] && idx[e.target]).slice(0, 60);
      return { placed, edges, idx };
    }, [data, maxNodes]);

    if (!data || !layout) {
      return (
        <div className="h-[140px] grid place-items-center text-xs text-[var(--color-text-muted)] font-mono">
          loading…
        </div>
      );
    }

    return (
      <svg viewBox="0 0 220 140" className="w-full h-auto" aria-label="Graph preview">
        {layout.edges.map((e, i) => (
          <line
            key={i}
            x1={layout.idx[e.source].x}
            y1={layout.idx[e.source].y}
            x2={layout.idx[e.target].x}
            y2={layout.idx[e.target].y}
            stroke="var(--color-border-strong)"
            strokeWidth={0.6}
          />
        ))}
        {layout.placed.map((p) => (
          <circle
            key={p.node.id}
            cx={p.x}
            cy={p.y}
            r={4}
            fill={COLOR[p.node.kind] ?? COLOR.entity}
          />
        ))}
      </svg>
    );
  }
  ```

- [ ] **Step 2: Write test**
  ```tsx
  import { describe, it, expect } from "vitest";
  import { render } from "@testing-library/react";
  import { GlanceView } from "../GlanceView";

  describe("GlanceView", () => {
    it("shows loading state on undefined", () => {
      const { getByText } = render(<GlanceView data={undefined} />);
      expect(getByText(/loading/i)).toBeInTheDocument();
    });
    it("renders N circles for N nodes (capped by maxNodes)", () => {
      const { container } = render(
        <GlanceView
          data={{
            nodes: Array.from({ length: 5 }, (_, i) => ({
              id: String(i), label: "n", kind: "entity",
            })),
            edges: [{ source: "0", target: "1" }],
          }}
        />,
      );
      expect(container.querySelectorAll("circle").length).toBe(5);
      expect(container.querySelectorAll("line").length).toBe(1);
    });
  });
  ```

- [ ] **Step 3: Run + commit**
  ```bash
  cd ui && npm test -- graph/
  cd ..
  git add ui/src/components/graph/
  git commit -m "feat(ui): GlanceView SVG graph preview"
  ```

### Task 4.5: Wire Home route with all pieces

**Files:**
- Modify: `ui/src/routes/Home.tsx`
- Test: `ui/src/routes/__tests__/Home.test.tsx`

- [ ] **Step 1: Rewrite `ui/src/routes/Home.tsx`**
  ```tsx
  import { useEffect, useMemo } from "react";
  import { StatsStrip } from "@/components/layout/StatsStrip";
  import { ActivityFeed } from "@/components/activity/ActivityFeed";
  import { GlanceView } from "@/components/graph/GlanceView";
  import { useProjectStore } from "@/stores/project";
  import { useStats } from "@/hooks/api/useStats";
  import { useActivity } from "@/hooks/api/useActivity";
  import { useNotes } from "@/hooks/api/useNotes";
  import { useNotesGraph } from "@/hooks/api/useGraph";
  import { useLastVisit } from "@/hooks/useLastVisit";
  import { t } from "@/i18n";

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

    // Touch on unmount so we track "last time the user looked at Home"
    useEffect(() => () => { touch(); }, [touch]);

    const recentNotes = (notes.data ?? []).slice(0, 5);

    return (
      <div className="p-6 max-w-[1400px] mx-auto">
        <StatsStrip stats={stats.data} delta={{ notes: newCount }} />
        <div className="grid grid-cols-1 lg:grid-cols-[1fr_320px] gap-5">
          <ActivityFeed events={activity.data ?? []} lastVisit={lastVisit} />
          <aside className="flex flex-col gap-4">
            <section aria-label={t("home.graphGlance")} className="border border-[var(--color-border)] rounded-md p-3">
              <h2 className="text-[10px] uppercase tracking-wider text-[var(--color-text-muted)] mb-2.5">
                {t("home.graphGlance")}
              </h2>
              <GlanceView data={graph.data} />
            </section>
            <section aria-label={t("home.pinnedNotes")} className="border border-[var(--color-border)] rounded-md p-3">
              <h2 className="text-[10px] uppercase tracking-wider text-[var(--color-text-muted)] mb-2.5">
                {t("home.pinnedNotes")}
              </h2>
              <ul className="text-sm text-[var(--color-text)] font-mono space-y-1.5">
                {recentNotes.map((n) => (
                  <li key={n.key} className="truncate">{n.key}</li>
                ))}
                {recentNotes.length === 0 && (
                  <li className="text-[var(--color-text-muted)]">—</li>
                )}
              </ul>
            </section>
          </aside>
        </div>
      </div>
    );
  }
  ```

- [ ] **Step 2: Write integration test**
  ```tsx
  import { describe, it, expect } from "vitest";
  import { render, screen, waitFor } from "@testing-library/react";
  import { MemoryRouter } from "react-router-dom";
  import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
  import { http, HttpResponse } from "msw";
  import { server } from "@/test/msw";
  import Home from "@/routes/Home";

  function wrap() {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    return (node: React.ReactNode) => (
      <QueryClientProvider client={qc}>
        <MemoryRouter>{node}</MemoryRouter>
      </QueryClientProvider>
    );
  }

  describe("Home route", () => {
    it("renders stats + empty activity + empty glance state", async () => {
      server.use(
        http.get("/api/projects/_default/notes", () => HttpResponse.json([])),
        http.get("/api/documents", () => HttpResponse.json([])),
        http.get("/api/projects/_default/graph", () => HttpResponse.json({ nodes: [], edges: [] })),
      );
      render(wrap()(<Home />));
      await waitFor(() => expect(screen.getByText(/since your last visit|nothing new/i)).toBeInTheDocument());
      expect(screen.getByRole("region", { name: /project statistics/i })).toBeInTheDocument();
    });
  });
  ```

- [ ] **Step 3: Run + build + commit**
  ```bash
  cd ui && npm test -- routes/ && npm run build
  cd ..
  git add ui/src/routes/Home.tsx ui/src/routes/__tests__/ ui/dist/
  git commit -m "feat(ui): Home route with stats + activity + glance + pinned"
  ```

---

## Wave 5 — Notes workspace

### Task 5.1: Markdown renderer

**Files:**
- Create: `ui/src/lib/markdown.ts`
- Create: `ui/src/components/notes/MarkdownView.tsx`, `ui/src/components/notes/WikiLink.tsx`
- Test: `ui/src/lib/__tests__/markdown.test.ts`, `ui/src/components/notes/__tests__/MarkdownView.test.tsx`

- [ ] **Step 1: Create `ui/src/lib/markdown.ts`**
  ```ts
  import MarkdownIt from "markdown-it";

  export interface MarkdownPart {
    kind: "html" | "wikilink";
    content: string;        // html string OR wikilink target
    label?: string;         // for wikilinks with alias
  }

  const WIKILINK = /\[\[([^\]|]+?)(?:\|([^\]]+))?\]\]/g;

  // Configure markdown-it with safe defaults
  export function createMd() {
    const md = new MarkdownIt({
      html: false,
      linkify: true,
      breaks: false,
    });
    // Open links in new tab with noopener
    const defaultRender = md.renderer.rules.link_open ||
      ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options));
    md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
      const href = tokens[idx].attrGet("href") ?? "";
      if (/^https?:\/\//.test(href)) {
        tokens[idx].attrSet("target", "_blank");
        tokens[idx].attrSet("rel", "noopener noreferrer");
      }
      return defaultRender(tokens, idx, options, env, self);
    };
    // Image: loading lazy
    const defaultImg = md.renderer.rules.image ||
      ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options));
    md.renderer.rules.image = (tokens, idx, options, env, self) => {
      tokens[idx].attrSet("loading", "lazy");
      return defaultImg(tokens, idx, options, env, self);
    };
    return md;
  }

  const md = createMd();

  export function renderMarkdown(source: string): MarkdownPart[] {
    // Strip YAML frontmatter
    let body = source;
    if (body.startsWith("---\n")) {
      const end = body.indexOf("\n---", 4);
      if (end > 0) body = body.slice(end + 4).replace(/^\n/, "");
    }

    const parts: MarkdownPart[] = [];
    let lastIndex = 0;
    for (const m of body.matchAll(WIKILINK)) {
      const idx = m.index ?? 0;
      if (idx > lastIndex) {
        parts.push({ kind: "html", content: md.render(body.slice(lastIndex, idx)) });
      }
      parts.push({ kind: "wikilink", content: m[1].trim(), label: m[2]?.trim() });
      lastIndex = idx + m[0].length;
    }
    if (lastIndex < body.length) {
      parts.push({ kind: "html", content: md.render(body.slice(lastIndex)) });
    }
    return parts;
  }
  ```

- [ ] **Step 2: Create `ui/src/components/notes/WikiLink.tsx`**
  ```tsx
  import { Link } from "react-router-dom";

  interface Props { target: string; label?: string; }

  export function WikiLink({ target, label }: Props) {
    return (
      <Link to={`/notes/${encodeURIComponent(target)}`} className="text-[var(--color-accent)] underline decoration-dotted hover:decoration-solid">
        {label ?? target}
      </Link>
    );
  }
  ```

- [ ] **Step 3: Create `ui/src/components/notes/MarkdownView.tsx`**
  ```tsx
  import { renderMarkdown } from "@/lib/markdown";
  import { WikiLink } from "./WikiLink";

  export function MarkdownView({ source }: { source: string }) {
    const parts = renderMarkdown(source);
    return (
      <div className="prose-notes max-w-[620px]">
        {parts.map((p, i) =>
          p.kind === "html" ? (
            <div key={i} dangerouslySetInnerHTML={{ __html: p.content }} />
          ) : (
            <WikiLink key={i} target={p.content} label={p.label} />
          ),
        )}
      </div>
    );
  }
  ```

- [ ] **Step 4: Write `ui/src/lib/__tests__/markdown.test.ts`**
  ```ts
  import { describe, it, expect } from "vitest";
  import { renderMarkdown } from "../markdown";

  describe("renderMarkdown", () => {
    it("parses headings + paragraphs", () => {
      const parts = renderMarkdown("# Hello\n\nworld");
      expect(parts).toHaveLength(1);
      expect(parts[0].content).toMatch(/<h1/);
      expect(parts[0].content).toMatch(/<p>world/);
    });
    it("strips YAML frontmatter", () => {
      const parts = renderMarkdown("---\ntitle: hi\n---\n\nbody");
      expect(parts[0].content).toMatch(/<p>body/);
      expect(parts[0].content).not.toMatch(/title:/);
    });
    it("extracts plain wikilink", () => {
      const parts = renderMarkdown("see [[target]]!");
      const link = parts.find((p) => p.kind === "wikilink");
      expect(link?.content).toBe("target");
      expect(link?.label).toBeUndefined();
    });
    it("extracts aliased wikilink and renders alias", () => {
      const parts = renderMarkdown("see [[target|Alias]]!");
      const link = parts.find((p) => p.kind === "wikilink");
      expect(link?.content).toBe("target");
      expect(link?.label).toBe("Alias");
    });
    it("opens external links in new tab", () => {
      const parts = renderMarkdown("[g](https://example.com)");
      expect(parts[0].content).toMatch(/target="_blank"/);
      expect(parts[0].content).toMatch(/rel="noopener noreferrer"/);
    });
    it("adds loading=lazy to images", () => {
      const parts = renderMarkdown("![alt](/x.png)");
      expect(parts[0].content).toMatch(/loading="lazy"/);
    });
  });
  ```

- [ ] **Step 5: Write `ui/src/components/notes/__tests__/MarkdownView.test.tsx`**
  ```tsx
  import { describe, it, expect } from "vitest";
  import { render, screen } from "@testing-library/react";
  import { MemoryRouter } from "react-router-dom";
  import { MarkdownView } from "../MarkdownView";

  describe("MarkdownView", () => {
    it("renders wikilinks as clickable router links with alias", () => {
      render(<MemoryRouter><MarkdownView source="pre [[target|Alias]] post" /></MemoryRouter>);
      const link = screen.getByRole("link", { name: "Alias" }) as HTMLAnchorElement;
      expect(link.getAttribute("href")).toBe("/notes/target");
    });
  });
  ```

- [ ] **Step 6: Run + commit**
  ```bash
  cd ui && npm test -- markdown MarkdownView
  cd ..
  git add ui/src/lib/markdown.ts ui/src/lib/__tests__/markdown.test.ts ui/src/components/notes/MarkdownView.tsx ui/src/components/notes/WikiLink.tsx ui/src/components/notes/__tests__/MarkdownView.test.tsx
  git commit -m "feat(ui): markdown renderer with wikilinks + aliased labels"
  ```

### Task 5.2: Tree drawer + Link drawer

**Files:**
- Create: `ui/src/components/notes/TreeDrawer.tsx`, `ui/src/components/notes/LinkPanel.tsx`
- Test: `ui/src/components/notes/__tests__/TreeDrawer.test.tsx`

- [ ] **Step 1: Create `ui/src/components/notes/TreeDrawer.tsx`**
  ```tsx
  import { Link } from "react-router-dom";
  import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/dialog";
  import { useNotes } from "@/hooks/api/useNotes";
  import type { Note } from "@/types/api";

  // Sheet in shadcn ships as Dialog-side; for our tree drawer a simple Dialog works.
  // If shadcn-sheet primitive isn't available, this falls back to Dialog with side:"left" CSS.

  interface Props { project: string; open: boolean; onOpenChange: (v: boolean) => void; currentKey?: string; }

  function groupByFolder(notes: Note[]) {
    const tree: Record<string, Note[]> = {};
    for (const n of notes) {
      const parts = n.key.split("/");
      const folder = parts.length === 1 ? "" : parts.slice(0, -1).join("/");
      (tree[folder] ??= []).push(n);
    }
    return tree;
  }

  export function TreeDrawer({ project, open, onOpenChange, currentKey }: Props) {
    const { data = [] } = useNotes(project);
    const grouped = groupByFolder(data);
    const folders = Object.keys(grouped).sort();
    return (
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent side="left" className="w-[300px] p-0">
          <SheetHeader className="px-4 py-3 border-b border-[var(--color-border)]">
            <SheetTitle className="font-mono text-xs uppercase tracking-wider text-[var(--color-text-muted)]">
              Notes
            </SheetTitle>
          </SheetHeader>
          <div className="p-2 overflow-auto text-sm">
            {folders.map((folder) => (
              <div key={folder || "(root)"} className="mb-2">
                {folder && (
                  <div className="font-mono text-xs text-[var(--color-text-muted)] px-2 py-1">
                    {folder}/
                  </div>
                )}
                {grouped[folder]
                  .sort((a, b) => a.key.localeCompare(b.key))
                  .map((n) => (
                    <Link
                      key={n.key}
                      to={`/notes/${encodeURIComponent(n.key)}`}
                      className={
                        "block px-2 py-1 rounded text-sm " +
                        (currentKey === n.key
                          ? "bg-[var(--color-surface-2)] text-[var(--color-text)]"
                          : "text-[var(--color-text-muted)] hover:bg-[var(--color-surface-2)]")
                      }
                      onClick={() => onOpenChange(false)}
                    >
                      {n.key.split("/").pop()}
                    </Link>
                  ))}
              </div>
            ))}
            {folders.length === 0 && (
              <div className="p-2 text-xs text-[var(--color-text-muted)]">No notes yet.</div>
            )}
          </div>
        </SheetContent>
      </Sheet>
    );
  }
  ```

  **Implementation note:** if `@/components/ui/dialog` doesn't export `Sheet` / `SheetContent` as named exports by default (shadcn's Dialog does, but Sheet is a separate file), run:
  ```bash
  cd ui && npx shadcn@latest add sheet --yes
  ```
  Then import from `@/components/ui/sheet` in the file above.

- [ ] **Step 2: Create `ui/src/components/notes/LinkPanel.tsx`**
  ```tsx
  import { Link } from "react-router-dom";
  import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
  import { useNotesGraph } from "@/hooks/api/useGraph";
  import { useMemo } from "react";

  interface Props { project: string; open: boolean; onOpenChange: (v: boolean) => void; currentKey?: string; }

  export function LinkPanel({ project, open, onOpenChange, currentKey }: Props) {
    const { data } = useNotesGraph(project);
    const { inbound, outbound } = useMemo(() => {
      if (!data || !currentKey) return { inbound: [] as string[], outbound: [] as string[] };
      const inb: string[] = [];
      const out: string[] = [];
      for (const e of data.edges) {
        if (e.target === currentKey) inb.push(e.source);
        if (e.source === currentKey) out.push(e.target);
      }
      return { inbound: Array.from(new Set(inb)), outbound: Array.from(new Set(out)) };
    }, [data, currentKey]);

    return (
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent side="right" className="w-[280px]">
          <SheetHeader className="mb-4">
            <SheetTitle className="font-mono text-xs uppercase tracking-wider text-[var(--color-text-muted)]">
              Links
            </SheetTitle>
          </SheetHeader>
          <div className="space-y-4 text-sm">
            <section>
              <h3 className="text-xs uppercase text-[var(--color-text-muted)] mb-1.5">Inbound</h3>
              {inbound.length === 0 && <p className="text-xs text-[var(--color-text-muted)]">—</p>}
              {inbound.map((k) => (
                <Link
                  key={k}
                  to={`/notes/${encodeURIComponent(k)}`}
                  onClick={() => onOpenChange(false)}
                  className="block px-2 py-1 rounded hover:bg-[var(--color-surface-2)]"
                >
                  {k}
                </Link>
              ))}
            </section>
            <section>
              <h3 className="text-xs uppercase text-[var(--color-text-muted)] mb-1.5">Outbound</h3>
              {outbound.length === 0 && <p className="text-xs text-[var(--color-text-muted)]">—</p>}
              {outbound.map((k) => (
                <Link
                  key={k}
                  to={`/notes/${encodeURIComponent(k)}`}
                  onClick={() => onOpenChange(false)}
                  className="block px-2 py-1 rounded hover:bg-[var(--color-surface-2)]"
                >
                  {k}
                </Link>
              ))}
            </section>
          </div>
        </SheetContent>
      </Sheet>
    );
  }
  ```

- [ ] **Step 3: Test TreeDrawer**
  ```tsx
  // ui/src/components/notes/__tests__/TreeDrawer.test.tsx
  import { describe, it, expect } from "vitest";
  import { render, screen } from "@testing-library/react";
  import { MemoryRouter } from "react-router-dom";
  import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
  import { http, HttpResponse } from "msw";
  import { server } from "@/test/msw";
  import { TreeDrawer } from "../TreeDrawer";

  describe("TreeDrawer", () => {
    it("renders grouped notes from API", async () => {
      server.use(
        http.get("/api/projects/_default/notes", () =>
          HttpResponse.json([
            { key: "architecture/jwt", content: "", tags: [], created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
            { key: "decisions/drop-redis", content: "", tags: [], created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
            { key: "intro", content: "", tags: [], created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
          ]),
        ),
      );
      const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      render(
        <QueryClientProvider client={qc}>
          <MemoryRouter>
            <TreeDrawer project="_default" open={true} onOpenChange={() => {}} />
          </MemoryRouter>
        </QueryClientProvider>,
      );
      // Labels appear async
      await screen.findByText("jwt");
      expect(screen.getByText("architecture/")).toBeInTheDocument();
      expect(screen.getByText("decisions/")).toBeInTheDocument();
    });
  });
  ```

- [ ] **Step 4: Run + commit**
  ```bash
  cd ui && npm test -- notes/
  cd ..
  git add ui/src/components/notes/TreeDrawer.tsx ui/src/components/notes/LinkPanel.tsx ui/src/components/notes/__tests__/TreeDrawer.test.tsx
  git commit -m "feat(ui): TreeDrawer + LinkPanel sheets"
  ```

### Task 5.3: NoteView + NotesLayout wiring

**Files:**
- Modify: `ui/src/routes/notes/NotesLayout.tsx`, `ui/src/routes/notes/NoteView.tsx`
- Test: `ui/src/routes/notes/__tests__/NoteView.test.tsx`

- [ ] **Step 1: Rewrite `ui/src/routes/notes/NotesLayout.tsx`**
  ```tsx
  import { Outlet, useParams } from "react-router-dom";
  import { useState } from "react";
  import { TreeDrawer } from "@/components/notes/TreeDrawer";
  import { LinkPanel } from "@/components/notes/LinkPanel";
  import { useProjectStore } from "@/stores/project";
  import { useHotkey } from "@/hooks/useHotkey";

  export default function NotesLayout() {
    const project = useProjectStore((s) => s.slug);
    const { key } = useParams();
    const [treeOpen, setTreeOpen] = useState(false);
    const [linksOpen, setLinksOpen] = useState(false);

    useHotkey("mod+/", () => setTreeOpen((v) => !v));
    useHotkey("mod+l", () => setLinksOpen((v) => !v));

    return (
      <div className="relative">
        <TreeDrawer project={project} open={treeOpen} onOpenChange={setTreeOpen} currentKey={key} />
        <LinkPanel project={project} open={linksOpen} onOpenChange={setLinksOpen} currentKey={key} />
        <Outlet />
        {!key && (
          <div className="p-8 max-w-[620px] mx-auto text-[var(--color-text-muted)] text-sm">
            Open the tree (<kbd className="font-mono text-xs px-1.5 py-0.5 border border-[var(--color-border)] rounded">⌘/</kbd>) or search (<kbd className="font-mono text-xs px-1.5 py-0.5 border border-[var(--color-border)] rounded">⌘K</kbd>) to select a note.
          </div>
        )}
      </div>
    );
  }
  ```

- [ ] **Step 2: Rewrite `ui/src/routes/notes/NoteView.tsx`**
  ```tsx
  import { useParams } from "react-router-dom";
  import { MarkdownView } from "@/components/notes/MarkdownView";
  import { useNote } from "@/hooks/api/useNotes";
  import { useProjectStore } from "@/stores/project";
  import { formatRelativeTime } from "@/lib/format";

  export default function NoteView() {
    const { key } = useParams();
    const project = useProjectStore((s) => s.slug);
    const { data: note, isLoading, error } = useNote(project, key);

    if (isLoading) {
      return <div className="p-8 text-[var(--color-text-muted)] text-sm">Loading…</div>;
    }
    if (error || !note) {
      return (
        <div className="p-8 max-w-[620px] mx-auto">
          <h1 className="text-xl font-semibold">Note not found</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-2 font-mono">{key}</p>
        </div>
      );
    }

    return (
      <article className="p-8 max-w-[620px] mx-auto">
        <header className="mb-6">
          <h1 className="text-2xl font-semibold">{note.key.split("/").pop()}</h1>
          <div className="text-xs font-mono text-[var(--color-text-muted)] mt-1">
            {note.key} · updated {formatRelativeTime(new Date(note.updated_at).getTime())}
            {note.author && ` · by ${note.author}`}
          </div>
        </header>
        <MarkdownView source={note.content} />
      </article>
    );
  }
  ```

- [ ] **Step 3: Write test**
  ```tsx
  // ui/src/routes/notes/__tests__/NoteView.test.tsx
  import { describe, it, expect } from "vitest";
  import { render, screen } from "@testing-library/react";
  import { MemoryRouter, Route, Routes } from "react-router-dom";
  import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
  import { http, HttpResponse } from "msw";
  import { server } from "@/test/msw";
  import NoteView from "../NoteView";

  describe("NoteView", () => {
    it("renders note content + metadata", async () => {
      server.use(
        http.get("/api/projects/_default/notes/jwt", () =>
          HttpResponse.json({
            key: "jwt", content: "# JWT rotation\n\nbody", author: "claude",
            tags: [], created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
          }),
        ),
      );
      const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      render(
        <QueryClientProvider client={qc}>
          <MemoryRouter initialEntries={["/notes/jwt"]}>
            <Routes><Route path="/notes/:key" element={<NoteView />} /></Routes>
          </MemoryRouter>
        </QueryClientProvider>,
      );
      await screen.findByRole("heading", { name: /jwt rotation/i });
      expect(screen.getByText(/by claude/i)).toBeInTheDocument();
    });
  });
  ```

- [ ] **Step 4: Run + build + commit**
  ```bash
  cd ui && npm test -- notes && npm run build
  cd ..
  git add ui/src/routes/notes/ ui/dist/
  git commit -m "feat(ui): NotesLayout + NoteView with ⌘/ tree + ⌘L links"
  ```

### Task 5.4: NoteEditor + NotesSearch (out-of-scope reduction)

**Files:**
- Create: `ui/src/routes/notes/NoteEditor.tsx`, `ui/src/routes/notes/NotesSearch.tsx`

For v1, keep NoteEditor minimal: a `<textarea>` with save button via React Hook Form + Zod. Replace with CodeMirror post-v1 if desired.

- [ ] **Step 1: Create `ui/src/routes/notes/NoteEditor.tsx`**
  ```tsx
  import { useParams, useNavigate } from "react-router-dom";
  import { useForm } from "react-hook-form";
  import { zodResolver } from "@hookform/resolvers/zod";
  import { z } from "zod";
  import { useWriteNote, useNote } from "@/hooks/api/useNotes";
  import { useProjectStore } from "@/stores/project";
  import { useEffect } from "react";

  const schema = z.object({
    content: z.string().min(1, "Content cannot be empty"),
    author: z.string().optional(),
    tagsRaw: z.string().optional(),
  });
  type FormData = z.infer<typeof schema>;

  export default function NoteEditor() {
    const { key } = useParams();
    const project = useProjectStore((s) => s.slug);
    const nav = useNavigate();
    const { data: existing } = useNote(project, key);
    const write = useWriteNote(project);

    const { register, handleSubmit, formState, reset } = useForm<FormData>({
      resolver: zodResolver(schema),
      defaultValues: { content: "", author: "", tagsRaw: "" },
    });

    useEffect(() => {
      if (existing) {
        reset({
          content: existing.content,
          author: existing.author ?? "",
          tagsRaw: existing.tags.join(", "),
        });
      }
    }, [existing, reset]);

    const onSubmit = async (data: FormData) => {
      if (!key) return;
      const tags = (data.tagsRaw ?? "").split(",").map((t) => t.trim()).filter(Boolean);
      await write.mutateAsync({ key, content: data.content, author: data.author || undefined, tags });
      nav(`/notes/${encodeURIComponent(key)}`);
    };

    return (
      <form onSubmit={handleSubmit(onSubmit)} className="p-8 max-w-[620px] mx-auto space-y-4">
        <h1 className="text-xl font-semibold">Edit {key}</h1>
        <textarea
          {...register("content")}
          rows={20}
          className="w-full font-mono text-sm p-3 bg-[var(--color-surface-1)] border border-[var(--color-border)] rounded-md"
          aria-label="Note content"
        />
        {formState.errors.content && (
          <p className="text-xs text-[var(--color-semantic-error)]">
            {formState.errors.content.message}
          </p>
        )}
        <input
          {...register("author")}
          placeholder="Author (optional)"
          className="w-full px-3 py-2 bg-[var(--color-surface-1)] border border-[var(--color-border)] rounded-md text-sm"
        />
        <input
          {...register("tagsRaw")}
          placeholder="Tags, comma-separated"
          className="w-full px-3 py-2 bg-[var(--color-surface-1)] border border-[var(--color-border)] rounded-md text-sm"
        />
        <div className="flex gap-2">
          <button
            type="submit"
            disabled={formState.isSubmitting}
            className="px-3 py-1.5 bg-[var(--color-accent)] text-[var(--color-accent-contrast)] rounded-md text-sm"
          >
            Save
          </button>
          <button
            type="button"
            onClick={() => nav(-1)}
            className="px-3 py-1.5 border border-[var(--color-border-strong)] rounded-md text-sm"
          >
            Cancel
          </button>
        </div>
      </form>
    );
  }
  ```

- [ ] **Step 2: Install `@hookform/resolvers`**
  ```bash
  cd ui && npm install --save @hookform/resolvers@^3.9.0
  ```

- [ ] **Step 3: Create `ui/src/routes/notes/NotesSearch.tsx`** (debounced search)
  ```tsx
  import { useState, useEffect } from "react";
  import { useQuery } from "@tanstack/react-query";
  import { Link } from "react-router-dom";
  import { apiFetch } from "@/lib/api-client";
  import { qk } from "@/hooks/api/keys";
  import { useProjectStore } from "@/stores/project";
  import type { NoteHit } from "@/types/api";

  export default function NotesSearch() {
    const project = useProjectStore((s) => s.slug);
    const [q, setQ] = useState("");
    const [debounced, setDebounced] = useState("");

    useEffect(() => {
      const t = setTimeout(() => setDebounced(q.trim()), 300);
      return () => clearTimeout(t);
    }, [q]);

    const { data, isFetching } = useQuery({
      queryKey: qk.notesSearch(project, debounced),
      enabled: debounced.length > 0,
      queryFn: () =>
        apiFetch<{ hits: NoteHit[] }>(
          `/api/projects/${encodeURIComponent(project)}/search?q=${encodeURIComponent(debounced)}`,
        ),
    });

    return (
      <div className="p-8 max-w-[720px] mx-auto">
        <input
          autoFocus
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Search notes…"
          className="w-full px-4 py-3 bg-[var(--color-surface-1)] border border-[var(--color-border-strong)] rounded-md text-sm"
          aria-label="Search notes"
        />
        {isFetching && <p className="text-xs text-[var(--color-text-muted)] mt-2">searching…</p>}
        <ul className="mt-6 space-y-1.5">
          {data?.hits.map((h) => (
            <li key={h.key}>
              <Link
                to={`/notes/${encodeURIComponent(h.key)}`}
                className="block p-3 border border-[var(--color-border)] rounded-md hover:bg-[var(--color-surface-2)]"
              >
                <div className="text-sm font-mono">{h.key}</div>
                <div
                  className="text-xs text-[var(--color-text-muted)] mt-1"
                  dangerouslySetInnerHTML={{ __html: h.snippet }}
                />
              </Link>
            </li>
          ))}
        </ul>
      </div>
    );
  }
  ```

- [ ] **Step 4: Add edit + search routes in `ui/src/App.tsx`** — under the `<Route path="/notes" …>` block:
  ```tsx
  <Route path="/notes" element={<NotesLayout />}>
    <Route path="search" element={<NotesSearch />} />
    <Route path=":key" element={<NoteView />} />
    <Route path=":key/edit" element={<NoteEditor />} />
  </Route>
  ```
  Add the imports at the top for `NoteEditor` and `NotesSearch`.

- [ ] **Step 5: Typecheck + build + commit**
  ```bash
  cd ui && npm run typecheck && npm run build
  cd ..
  git add ui/package.json ui/package-lock.json ui/src/App.tsx ui/src/routes/notes/NoteEditor.tsx ui/src/routes/notes/NotesSearch.tsx ui/dist/
  git commit -m "feat(ui): NoteEditor + NotesSearch (debounced) routes"
  ```

---

## Wave 6 — Documents

### Task 6.1: Docs list + detail routes + upload

**Files:**
- Create: `ui/src/hooks/api/useDocs.ts`, `ui/src/hooks/api/useUpload.ts`
- Modify: `ui/src/routes/documents/DocumentsList.tsx`, `ui/src/routes/documents/DocumentView.tsx`
- Create: `ui/src/routes/documents/UploadModal.tsx`

- [ ] **Step 1: `ui/src/hooks/api/useDocs.ts`**
  ```ts
  import { useQuery } from "@tanstack/react-query";
  import { apiFetch } from "@/lib/api-client";
  import { qk } from "./keys";
  import type { Document } from "@/types/api";

  export function useDocs(project: string) {
    return useQuery({
      queryKey: qk.docs(project),
      queryFn: () => apiFetch<Document[]>(`/api/documents?project=${encodeURIComponent(project)}`),
    });
  }

  export function useDoc(project: string, id: string | undefined) {
    return useQuery({
      queryKey: qk.doc(project, id ?? ""),
      enabled: !!id,
      queryFn: () => apiFetch<Document>(`/api/documents/${encodeURIComponent(id!)}?project=${encodeURIComponent(project)}`),
    });
  }
  ```

- [ ] **Step 2: `ui/src/routes/documents/DocumentsList.tsx`**
  ```tsx
  import { Link } from "react-router-dom";
  import { useState } from "react";
  import { useDocs } from "@/hooks/api/useDocs";
  import { useProjectStore } from "@/stores/project";
  import { formatRelativeTime } from "@/lib/format";
  import { UploadModal } from "./UploadModal";

  export default function DocumentsList() {
    const project = useProjectStore((s) => s.slug);
    const { data = [] } = useDocs(project);
    const [uploadOpen, setUploadOpen] = useState(false);

    return (
      <div className="p-6 max-w-[1200px] mx-auto">
        <header className="flex items-center justify-between mb-4">
          <h1 className="text-xl font-semibold">Documents</h1>
          <button
            onClick={() => setUploadOpen(true)}
            className="px-3 py-1.5 bg-[var(--color-accent)] text-[var(--color-accent-contrast)] rounded-md text-sm"
          >
            Upload
          </button>
        </header>
        <UploadModal open={uploadOpen} onOpenChange={setUploadOpen} />
        <table className="w-full text-sm font-mono border-collapse">
          <thead>
            <tr className="text-left text-xs uppercase tracking-wider text-[var(--color-text-muted)]">
              <th className="p-2">Title</th>
              <th className="p-2">Type</th>
              <th className="p-2">Updated</th>
            </tr>
          </thead>
          <tbody>
            {data.map((d) => (
              <tr key={d.id} className="border-t border-[var(--color-border)] hover:bg-[var(--color-surface-2)]">
                <td className="p-2">
                  <Link to={`/docs/${d.id}`} className="text-[var(--color-text)] underline decoration-dotted">
                    {d.title || d.path}
                  </Link>
                </td>
                <td className="p-2 text-[var(--color-text-muted)]">{d.doc_type}</td>
                <td className="p-2 text-[var(--color-text-muted)]">
                  {formatRelativeTime(d.updated_at * 1000)}
                </td>
              </tr>
            ))}
            {data.length === 0 && (
              <tr><td colSpan={3} className="p-6 text-center text-sm text-[var(--color-text-muted)]">No documents indexed yet.</td></tr>
            )}
          </tbody>
        </table>
      </div>
    );
  }
  ```

- [ ] **Step 3: `ui/src/routes/documents/DocumentView.tsx`**
  ```tsx
  import { useParams } from "react-router-dom";
  import { useDoc } from "@/hooks/api/useDocs";
  import { useProjectStore } from "@/stores/project";

  export default function DocumentView() {
    const { id } = useParams();
    const project = useProjectStore((s) => s.slug);
    const { data, isLoading } = useDoc(project, id);
    if (isLoading) return <div className="p-8 text-sm text-[var(--color-text-muted)]">Loading…</div>;
    if (!data) return <div className="p-8 text-sm">Not found.</div>;
    return (
      <article className="p-8 max-w-[720px] mx-auto">
        <h1 className="text-xl font-semibold">{data.title || data.path}</h1>
        <div className="mt-2 font-mono text-xs text-[var(--color-text-muted)]">
          {data.doc_type} · v{data.version}
        </div>
      </article>
    );
  }
  ```

- [ ] **Step 4: `ui/src/routes/documents/UploadModal.tsx`** — simple file-input version using shadcn Dialog
  ```tsx
  import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
  import { useProjectStore } from "@/stores/project";
  import { apiFetch } from "@/lib/api-client";
  import { useState } from "react";
  import { useQueryClient } from "@tanstack/react-query";
  import { qk } from "@/hooks/api/keys";

  export function UploadModal({ open, onOpenChange }: { open: boolean; onOpenChange: (v: boolean) => void }) {
    const project = useProjectStore((s) => s.slug);
    const qc = useQueryClient();
    const [busy, setBusy] = useState(false);
    const [err, setErr] = useState<string | null>(null);

    async function onFiles(files: FileList | null) {
      if (!files || files.length === 0) return;
      setBusy(true); setErr(null);
      try {
        const fd = new FormData();
        for (const f of Array.from(files)) fd.append("files", f, f.name);
        await apiFetch(`/api/upload?project=${encodeURIComponent(project)}`, { method: "POST", body: fd });
        qc.invalidateQueries({ queryKey: qk.docs(project) });
        qc.invalidateQueries({ queryKey: qk.stats(project) });
        onOpenChange(false);
      } catch (e) {
        setErr((e as Error).message);
      } finally {
        setBusy(false);
      }
    }

    return (
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent>
          <DialogHeader><DialogTitle>Upload documents</DialogTitle></DialogHeader>
          <input
            type="file"
            multiple
            onChange={(e) => onFiles(e.currentTarget.files)}
            className="block w-full text-sm"
          />
          {busy && <p className="text-xs text-[var(--color-text-muted)]">Uploading…</p>}
          {err && <p className="text-xs text-[var(--color-semantic-error)]">{err}</p>}
        </DialogContent>
      </Dialog>
    );
  }
  ```

- [ ] **Step 5: Typecheck + build + commit**
  ```bash
  cd ui && npm run typecheck && npm run build
  cd ..
  git add ui/src/hooks/api/useDocs.ts ui/src/routes/documents/ ui/dist/
  git commit -m "feat(ui): Documents list + detail + upload modal"
  ```

---

## Wave 7 — Graph route

### Task 7.1: GraphCanvas with d3-force

**Files:**
- Create: `ui/src/lib/graph-layout.ts`
- Create: `ui/src/components/graph/GraphCanvas.tsx`
- Modify: `ui/src/routes/Graph.tsx`

- [ ] **Step 1: `ui/src/lib/graph-layout.ts`**
  ```ts
  import { forceSimulation, forceManyBody, forceLink, forceCenter, forceX, forceY, type Simulation } from "d3-force";
  import type { GraphData } from "@/hooks/api/useGraph";

  export interface LaidOutNode { id: string; label: string; kind: string; x: number; y: number; }
  export interface LaidOutEdge { source: string; target: string; }

  export function layoutGraph(
    data: GraphData,
    width: number,
    height: number,
    ticks = 200,
  ): { nodes: LaidOutNode[]; edges: LaidOutEdge[] } {
    const nodes = data.nodes.map((n) => ({ ...n, x: 0, y: 0 }));
    const links = data.edges.map((e) => ({ source: e.source, target: e.target }));
    const sim: Simulation<LaidOutNode, LaidOutEdge> = forceSimulation(nodes)
      .force("charge", forceManyBody().strength(-60))
      .force("center", forceCenter(width / 2, height / 2))
      .force("link", forceLink(links).id((d: any) => d.id).distance(40).strength(0.6))
      .force("x", forceX(width / 2).strength(0.02))
      .force("y", forceY(height / 2).strength(0.02))
      .stop();
    for (let i = 0; i < ticks; i++) sim.tick();
    return { nodes, edges: links.map((l) => ({ source: (l.source as any).id ?? l.source, target: (l.target as any).id ?? l.target })) };
  }
  ```

- [ ] **Step 2: `ui/src/components/graph/GraphCanvas.tsx`**
  ```tsx
  import { useMemo } from "react";
  import { layoutGraph } from "@/lib/graph-layout";
  import type { GraphData } from "@/hooks/api/useGraph";

  const COLOR: Record<string, string> = {
    entity: "var(--color-semantic-new)",
    note: "var(--color-semantic-graph)",
    community: "var(--color-semantic-index)",
  };

  export function GraphCanvas({ data, width = 1200, height = 700 }: { data: GraphData; width?: number; height?: number }) {
    const laid = useMemo(() => layoutGraph(data, width, height), [data, width, height]);
    const idx = useMemo(() => Object.fromEntries(laid.nodes.map((n) => [n.id, n])), [laid.nodes]);
    return (
      <svg viewBox={`0 0 ${width} ${height}`} className="w-full h-full" role="img" aria-label="Entity + notes graph">
        {laid.edges.map((e, i) => {
          const a = idx[e.source]; const b = idx[e.target];
          if (!a || !b) return null;
          return <line key={i} x1={a.x} y1={a.y} x2={b.x} y2={b.y} stroke="var(--color-border-strong)" strokeWidth={0.7} />;
        })}
        {laid.nodes.map((n) => (
          <g key={n.id}>
            <circle cx={n.x} cy={n.y} r={4.5} fill={COLOR[n.kind] ?? COLOR.entity} />
            <title>{n.label || n.id}</title>
          </g>
        ))}
      </svg>
    );
  }
  ```

- [ ] **Step 3: Rewrite `ui/src/routes/Graph.tsx`**
  ```tsx
  import { GraphCanvas } from "@/components/graph/GraphCanvas";
  import { useNotesGraph } from "@/hooks/api/useGraph";
  import { useProjectStore } from "@/stores/project";

  export default function Graph() {
    const project = useProjectStore((s) => s.slug);
    const { data, isLoading } = useNotesGraph(project);
    if (isLoading) return <div className="p-8 text-sm text-[var(--color-text-muted)]">Loading graph…</div>;
    if (!data) return <div className="p-8 text-sm">No graph data.</div>;
    return (
      <div className="h-[calc(100vh-44px)] p-4">
        <GraphCanvas data={data} />
      </div>
    );
  }
  ```

- [ ] **Step 4: Typecheck + build + commit**
  ```bash
  cd ui && npm run typecheck && npm run build
  cd ..
  git add ui/src/lib/graph-layout.ts ui/src/components/graph/GraphCanvas.tsx ui/src/routes/Graph.tsx ui/dist/
  git commit -m "feat(ui): Graph route with d3-force layout"
  ```

---

## Wave 8 — MCP Console

### Task 8.1: Minimal MCP console route

**Files:**
- Create: `ui/src/hooks/api/useMCP.ts`
- Modify: `ui/src/routes/MCPConsole.tsx`

- [ ] **Step 1: `ui/src/hooks/api/useMCP.ts`**
  ```ts
  import { useState, useCallback } from "react";
  import { apiFetch } from "@/lib/api-client";

  export interface MCPCallRecord {
    id: string;
    tool: string;
    args: unknown;
    result?: unknown;
    error?: string;
    tookMs: number;
    timestamp: number;
  }

  export function useMCP() {
    const [history, setHistory] = useState<MCPCallRecord[]>([]);

    const invoke = useCallback(async (tool: string, args: unknown) => {
      const started = performance.now();
      const rec: MCPCallRecord = { id: crypto.randomUUID(), tool, args, tookMs: 0, timestamp: Date.now() };
      try {
        const result = await apiFetch<unknown>("/mcp", {
          method: "POST",
          body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "tools/call", params: { name: tool, arguments: args } }),
        });
        rec.result = result;
      } catch (e) {
        rec.error = (e as Error).message;
      } finally {
        rec.tookMs = Math.round(performance.now() - started);
        setHistory((h) => [rec, ...h].slice(0, 50));
      }
    }, []);

    return { history, invoke };
  }
  ```

- [ ] **Step 2: Rewrite `ui/src/routes/MCPConsole.tsx`**
  ```tsx
  import { useMCP } from "@/hooks/api/useMCP";
  import { useState } from "react";

  const KNOWN_TOOLS = [
    "list_projects", "stats", "search_documents", "search_notes",
    "list_notes", "read_note", "write_note", "list_entities",
    "query_entity", "find_relationships", "get_graph_neighborhood",
    "get_entity_claims",
  ];

  export default function MCPConsole() {
    const { history, invoke } = useMCP();
    const [tool, setTool] = useState("stats");
    const [args, setArgs] = useState("{}");
    const [err, setErr] = useState<string | null>(null);

    async function onRun() {
      setErr(null);
      let parsed: unknown;
      try { parsed = JSON.parse(args); } catch { setErr("Invalid JSON"); return; }
      await invoke(tool, parsed);
    }

    return (
      <div className="p-6 max-w-[1000px] mx-auto">
        <h1 className="text-xl font-semibold mb-4">MCP Console</h1>
        <div className="flex gap-2 mb-3">
          <select value={tool} onChange={(e) => setTool(e.currentTarget.value)} className="px-3 py-2 bg-[var(--color-surface-1)] border border-[var(--color-border)] rounded-md text-sm font-mono">
            {KNOWN_TOOLS.map((t) => <option key={t}>{t}</option>)}
          </select>
          <input
            value={args}
            onChange={(e) => setArgs(e.currentTarget.value)}
            className="flex-1 px-3 py-2 bg-[var(--color-surface-1)] border border-[var(--color-border)] rounded-md text-sm font-mono"
            aria-label="Tool arguments (JSON)"
          />
          <button onClick={onRun} className="px-3 py-2 bg-[var(--color-accent)] text-[var(--color-accent-contrast)] rounded-md text-sm">
            Run
          </button>
        </div>
        {err && <p className="text-sm text-[var(--color-semantic-error)] mb-2">{err}</p>}
        <div className="space-y-2">
          {history.map((h) => (
            <details key={h.id} className="border border-[var(--color-border)] rounded-md">
              <summary className="cursor-pointer p-3 flex items-center gap-3">
                <span className="font-mono text-xs px-2 py-0.5 rounded bg-[var(--color-surface-2)]">{h.tool}</span>
                <span className="text-xs text-[var(--color-text-muted)]">{h.tookMs}ms</span>
                {h.error && <span className="text-xs text-[var(--color-semantic-error)] ml-auto">{h.error}</span>}
              </summary>
              <pre className="p-3 text-xs font-mono overflow-auto bg-[var(--color-surface-1)]">
  args: {JSON.stringify(h.args, null, 2)}
  result: {JSON.stringify(h.result, null, 2)}
              </pre>
            </details>
          ))}
          {history.length === 0 && (
            <p className="text-sm text-[var(--color-text-muted)]">No calls yet.</p>
          )}
        </div>
      </div>
    );
  }
  ```

- [ ] **Step 3: Typecheck + build + commit**
  ```bash
  cd ui && npm run typecheck && npm run build
  cd ..
  git add ui/src/hooks/api/useMCP.ts ui/src/routes/MCPConsole.tsx ui/dist/
  git commit -m "feat(ui): minimal MCP console"
  ```

---

## Wave 9 — Backend delta (meta-tag injection)

### Task 9.1: Inject bearer meta tag into SPA `index.html`

**Files:**
- Modify: `internal/api/router.go` (≤ 8 lines in `spaHandler`)
- Test: `internal/api/spa_meta_test.go` (new)

- [ ] **Step 1: Read existing `spaHandler` in `internal/api/router.go`** to locate the block that writes `index.html`.

- [ ] **Step 2: Modify the block that serves `index.html`** to inject the meta tag when `cfg.Server.APIKey != ""`:
  ```go
  // In spaHandler, replace the block that writes content:
  if cfg.Server.APIKey != "" {
      // Inject meta tag just before </head>
      replaced := bytes.Replace(
          content,
          []byte("</head>"),
          []byte(`<meta name="docsiq-api-key" content="`+html.EscapeString(cfg.Server.APIKey)+`"></head>`),
          1,
      )
      w.Header().Set("Content-Type", "text/html; charset=utf-8")
      w.WriteHeader(http.StatusOK)
      _, _ = w.Write(replaced)
      return
  }
  ```
  Ensure imports: `"bytes"`, `"html"`.

  **Note:** `spaHandler` currently does not take `cfg` — plumb it through. Signature becomes `spaHandler(assets fs.FS, cfg *config.Config)` and the `mux.Handle("/", …)` call site passes `cfg`.

- [ ] **Step 3: Write test `internal/api/spa_meta_test.go`** (no build tag; runs under `make test`)
  ```go
  package api

  import (
      "io"
      "net/http"
      "net/http/httptest"
      "strings"
      "testing"

      "github.com/RandomCodeSpace/docsiq/internal/config"
      "github.com/RandomCodeSpace/docsiq/ui"
  )

  func TestSPA_InjectsMetaWhenAPIKeySet(t *testing.T) {
      cfg := &config.Config{}
      cfg.Server.APIKey = "secret-key-abc"
      h := spaHandler(ui.Assets, cfg)
      srv := httptest.NewServer(h)
      defer srv.Close()
      resp, err := http.Get(srv.URL + "/")
      if err != nil { t.Fatal(err) }
      body, _ := io.ReadAll(resp.Body)
      if !strings.Contains(string(body), `name="docsiq-api-key"`) {
          t.Fatalf("expected meta tag, body:\n%s", body)
      }
      if !strings.Contains(string(body), `content="secret-key-abc"`) {
          t.Fatalf("expected API key in content attr, body:\n%s", body)
      }
  }

  func TestSPA_OmitsMetaWhenAPIKeyUnset(t *testing.T) {
      cfg := &config.Config{}
      cfg.Server.APIKey = ""
      h := spaHandler(ui.Assets, cfg)
      srv := httptest.NewServer(h)
      defer srv.Close()
      resp, err := http.Get(srv.URL + "/")
      if err != nil { t.Fatal(err) }
      body, _ := io.ReadAll(resp.Body)
      if strings.Contains(string(body), `name="docsiq-api-key"`) {
          t.Fatalf("meta tag should not be present when APIKey empty")
      }
  }
  ```

- [ ] **Step 4: Run tests**
  ```bash
  cd /home/dev/projects/docsiq && make test 2>&1 | grep -E '^(ok|FAIL)' | head -20
  ```
  Expected: all packages ok.

- [ ] **Step 5: Commit**
  ```bash
  git add internal/api/router.go internal/api/spa_meta_test.go
  git commit -m "feat(api): inject bearer meta tag into SPA index.html when auth enabled"
  ```

---

## Wave 10 — Polish, a11y audit, bundle check

### Task 10.0: Theme toggle in TopBar

**Files:**
- Create: `ui/src/components/layout/ThemeToggle.tsx`
- Modify: `ui/src/components/layout/TopBar.tsx` (mount it)
- Test: `ui/src/components/layout/__tests__/ThemeToggle.test.tsx`

- [ ] **Step 1: Create `ui/src/components/layout/ThemeToggle.tsx`**
  ```tsx
  import { Sun, Moon, Monitor } from "lucide-react";
  import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
  } from "@/components/ui/dropdown-menu";
  import { useUIStore } from "@/stores/ui";

  export function ThemeToggle() {
    const theme = useUIStore((s) => s.theme);
    const setTheme = useUIStore((s) => s.setTheme);
    const Icon = theme === "light" ? Sun : theme === "dark" ? Moon : Monitor;
    return (
      <DropdownMenu>
        <DropdownMenuTrigger
          aria-label="Change theme"
          className="p-1.5 rounded-md hover:bg-[var(--color-surface-2)] transition-colors"
        >
          <Icon size={16} />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={() => setTheme("light")}>
            <Sun size={14} className="mr-2" /> Light
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => setTheme("dark")}>
            <Moon size={14} className="mr-2" /> Dark
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => setTheme("system")}>
            <Monitor size={14} className="mr-2" /> System
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    );
  }
  ```

- [ ] **Step 2: Modify `ui/src/components/layout/TopBar.tsx`** — add `<ThemeToggle />` between the search button and the end. Add import at the top.

  Replace the last element of TopBar's outer `<header>` (the search button) with:
  ```tsx
        <button
          onClick={onCommandOpen}
          aria-label="Open command palette"
          className="ml-auto flex items-center gap-2 px-3 py-1.5 rounded-md border border-[var(--color-border-strong)] bg-[var(--color-base)] text-sm text-[var(--color-text-muted)] hover:bg-[var(--color-surface-2)] transition-colors"
        >
          <span>{t("nav.search")}</span>
          <kbd className="font-mono text-[10px] px-1.5 py-0.5 rounded bg-[var(--color-surface-2)] border border-[var(--color-border)]">⌘K</kbd>
        </button>
        <ThemeToggle />
  ```

- [ ] **Step 3: Write test**
  ```tsx
  // ui/src/components/layout/__tests__/ThemeToggle.test.tsx
  import { describe, it, expect } from "vitest";
  import { render, screen } from "@testing-library/react";
  import userEvent from "@testing-library/user-event";
  import { ThemeToggle } from "../ThemeToggle";
  import { useUIStore } from "@/stores/ui";

  describe("ThemeToggle", () => {
    it("opens menu and switches theme to light", async () => {
      useUIStore.setState({ theme: "system" });
      const user = userEvent.setup();
      render(<ThemeToggle />);
      await user.click(screen.getByRole("button", { name: /change theme/i }));
      await user.click(screen.getByText(/^light$/i));
      expect(useUIStore.getState().theme).toBe("light");
    });
  });
  ```

- [ ] **Step 4: Run + commit**
  ```bash
  cd ui && npm test -- ThemeToggle
  cd ..
  git add ui/src/components/layout/ThemeToggle.tsx ui/src/components/layout/TopBar.tsx ui/src/components/layout/__tests__/ThemeToggle.test.tsx
  git commit -m "feat(ui): ThemeToggle (light / dark / system) in TopBar"
  ```

### Task 10.1: Axe-core in dev + pa11y CI

**Files:**
- Modify: `ui/src/main.tsx` (dev-only axe)
- Create: `.github/workflows/ui-a11y.yml`

- [ ] **Step 1: Update `ui/src/main.tsx`**
  ```tsx
  import { StrictMode } from "react";
  import { createRoot } from "react-dom/client";
  import "./styles/globals.css";
  import App from "./App";

  if (import.meta.env.DEV) {
    import("axe-core").then((axe) => {
      axe.default.run().then((res) => {
        if (res.violations.length > 0) {
          // Console table is fine in dev
          console.warn("axe violations:", res.violations);
        }
      });
    });
  }

  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
  ```

- [ ] **Step 2: Skip pa11y CI for v1** (out-of-scope per spec). Instead, document the manual audit step in `CONTRIBUTING.md`.

  Add this paragraph to `CONTRIBUTING.md`:
  ```markdown

  ## Accessibility audit (manual, pre-release)

  Before merging a UI change:

  ```bash
  cd ui && npm run build
  cd /home/dev/projects/docsiq
  ./docsiq serve &
  # wait for http://localhost:8080 then
  npx pa11y http://localhost:8080 http://localhost:8080/notes http://localhost:8080/docs http://localhost:8080/graph http://localhost:8080/mcp
  # stop the server
  ```

  Zero violations required at `WCAG2AA` level.
  ```

- [ ] **Step 3: Commit**
  ```bash
  git add ui/src/main.tsx CONTRIBUTING.md
  git commit -m "feat(ui): dev-only axe-core audit + manual a11y runbook"
  ```

### Task 10.2: Bundle-size assertion in CI

**Files:**
- Modify: `.github/workflows/ci.yml` (add a step)

- [ ] **Step 1: Read the existing `ui-freshness` job** — extend it with a bundle-size gate.

- [ ] **Step 2: Add to the same job, after the build step**
  ```yaml
        - name: Assert bundle budget
          run: |
            set -eu
            js_bytes=$(stat -c %s ui/dist/assets/index-*.js 2>/dev/null || stat -f %z ui/dist/assets/index-*.js)
            css_bytes=$(stat -c %s ui/dist/assets/index-*.css 2>/dev/null || stat -f %z ui/dist/assets/index-*.css)
            total=$((js_bytes + css_bytes))
            echo "bundle size: $total bytes"
            if [ "$total" -gt 460000 ]; then
              echo "::error::Bundle exceeds 460 KB budget"
              exit 1
            fi
  ```

- [ ] **Step 3: Commit**
  ```bash
  git add .github/workflows/ci.yml
  git commit -m "ci: enforce 460 KB bundle budget on ui/dist"
  ```

### Task 10.3: Fresh `ui/dist/` commit + final smoke

- [ ] **Step 1: Full rebuild + all gates**
  ```bash
  cd ui && npm run typecheck && npm run build && npm test -- --run 2>&1 | tail -4
  cd .. && make vet && make test 2>&1 | grep -E '^(ok|FAIL)' | head -20
  ```
  Expected: typecheck exit 0; vitest 100% pass; go vet exit 0; all Go packages ok.

- [ ] **Step 2: End-to-end smoke via `docsiq serve`**
  ```bash
  tmp=$(mktemp -d)
  DOCSIQ_DATA_DIR="$tmp/data" DOCSIQ_SERVER_PORT=18888 ./docsiq serve >"$tmp/server.log" 2>&1 &
  pid=$!
  sleep 2
  curl -sf http://127.0.0.1:18888/health
  curl -sf http://127.0.0.1:18888/ | head -10
  kill $pid
  ```
  Expected: `/health` returns `{"status":"ok"}`; `/` returns HTML containing `<div id="root"></div>`.

- [ ] **Step 3: Commit final dist + push**
  ```bash
  git add ui/dist/
  git commit --allow-empty -m "chore(ui): final dist rebuild for v1 merge" || true
  git push origin ui-redesign
  ```

### Task 10.4: Open PR to main

- [ ] **Step 1: Create PR via gh**
  ```bash
  gh pr create --base main --head ui-redesign --title "UI redesign — v1" --body "$(cat <<'EOF'
  Greenfield rewrite of the docsiq web UI per
  docs/superpowers/specs/2026-04-18-ui-redesign-design.md and executed via
  docs/superpowers/plans/2026-04-18-ui-redesign.md.

  Highlights:
  - Linear-style labeled sidebar + Raycast-style ⌘K palette
  - Responsive: mobile drawer / tablet icon rail / desktop sidebar
  - Home: stats strip + since-last-visit activity feed + graph glance
  - Notes: focused column + ⌘/ tree drawer + ⌘L links drawer
  - Docs + Graph + MCP console reimplemented on new stack
  - Visual: Geist + #0f1115 + #3ecf8e (theme B)
  - Tech: Tailwind 4 + shadcn/ui + TanStack Query + Zustand + RR6 + RHF + Zod

  Backend delta:
  - 8 lines in internal/api/router.go SPA handler to inject bearer meta tag
    when DOCSIQ_API_KEY is set (Wave 9). New test: spa_meta_test.go.
  - No other backend changes; existing integration tests unchanged.
  EOF
  )"
  ```

- [ ] **Step 2: Watch CI, iterate on failures if any.**

- [ ] **Step 3: Merge once green and reviewed.**

---

## Out of scope (deferred post-v1)

- Visual regression (Playwright snapshots)
- Storybook / component gallery
- CodeMirror 6 markdown editor (v1 uses textarea)
- Note history UI
- Real-time SSE for activity feed (v1 polls every 10s)
- Additional LLM providers UI configuration
- Pinned notes backend extension
- Command palette: entity + community search (v1: pages + notes + docs)

---

## Acceptance — v1 ships when

All 12 acceptance criteria from spec §19 pass:

1. All 5 destinations render
2. `⌘K` palette works
3. Keyboard shortcuts (`G+letter`, `⌘\`, `⌘/`, `⌘L`) functional
4. Responsive across 375 / 768 / 1280 / 1920
5. Dark + light themes, `prefers-color-scheme` respected, manual toggle persists
6. `prefers-reduced-motion` reduces motion
7. Axe-core zero violations on each route in dev
8. Vitest coverage ≥ 70 % on scoped surface
9. `npm audit --audit-level=moderate` clean
10. Bundle ≤ 460 KB JS + CSS combined
11. `make test` + `make test-integration` green
12. End-to-end smoke (Task 10.3 Step 2) green

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-18-ui-redesign.md`. Two execution options:

**1. Subagent-Driven (recommended)** — Dispatch a fresh subagent per task, two-stage review per task (spec compliance + code quality), fast iteration. Uses `superpowers:subagent-driven-development`.

**2. Inline Execution** — Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints.

Which approach?
