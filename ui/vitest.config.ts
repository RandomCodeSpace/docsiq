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
