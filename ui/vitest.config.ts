import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import { resolve } from "path";

// React 19's CJS entry picks production vs development at require-time off
// NODE_ENV; the production build omits `act`. Force the test build before any
// worker forks so callers with NODE_ENV=production (CI-adjacent shells, the
// Paperclip agent harness) get the same result as bare `npm test`.
process.env.NODE_ENV = "test";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": resolve(__dirname, "./src") },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/setupTests.ts"],
    exclude: ["node_modules/**", "dist/**", "e2e/**"],
    coverage: {
      reporter: ["text", "html"],
      include: [
        "src/components/notes/MarkdownView.tsx",
        "src/components/notes/WikiLink.tsx",
        "src/components/notes/TreeDrawer.tsx",
        "src/components/activity/ActivityFeed.tsx",
        "src/components/layout/StatsStrip.tsx",
        "src/components/graph/GlanceView.tsx",
        "src/hooks/api/useStats.ts",
        "src/hooks/useHotkey.ts",
        "src/hooks/useReducedMotion.ts",
        "src/lib/format.ts",
        "src/lib/markdown.ts",
        "src/lib/api-client.ts",
        "src/stores/ui.ts",
      ],
      thresholds: {
        statements: 70,
        branches: 60,
      },
    },
  },
});
