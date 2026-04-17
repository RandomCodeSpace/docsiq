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
        "src/components/notes/**",
        "src/components/nav/**",
        "src/components/shared/UnifiedSearchPanel.tsx",
        "src/hooks/useNotes.ts",
        "src/hooks/useProjects.ts",
        "src/hooks/useNotesTree.ts",
        "src/hooks/useNotesSearch.ts",
        "src/hooks/useNotesGraph.ts",
      ],
      thresholds: {
        statements: 70,
        branches: 60,
      },
    },
  },
});
