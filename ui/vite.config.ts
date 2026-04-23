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
        manualChunks(id) {
          if (/node_modules[\\/](markdown-it|shiki)[\\/]/.test(id)) return "markdown";
          if (/node_modules[\\/]d3-force[\\/]/.test(id)) return "graph";
          if (/node_modules[\\/](codemirror|@codemirror[\\/])/.test(id)) return "editor";
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
