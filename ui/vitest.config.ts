import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import { resolve } from "path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": resolve(__dirname, "./src") },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/setupTests.ts"],
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
