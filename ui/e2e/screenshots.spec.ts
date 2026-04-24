// Capture the five canonical docsiq screenshots used in the README and
// docs/screenshots/. Runs against a live server on $BASE_URL (default
// http://localhost:37778) seeded with the docs/samples/ fixture corpus.
//
// Usage:
//   DOCSIQ_DATA_DIR=/tmp/fixture ./docsiq serve --port 37778 &
//   BASE_URL=http://localhost:37778 \
//     npx playwright test ui/e2e/screenshots.spec.ts

import { test } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";

const BASE_URL = process.env.BASE_URL ?? "http://localhost:37778";
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const OUT_DIR = path.resolve(__dirname, "..", "..", "docs", "screenshots");

// Desktop viewport. Matches the typical reviewer's Retina display;
// the sharp script downscales as needed.
test.use({
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: 2,
  colorScheme: "dark", // dark theme is the default in the app
});

async function settle(page: import("@playwright/test").Page) {
  // Wait for network idle + a small buffer for any d3 transitions.
  await page.waitForLoadState("networkidle");
  await page.waitForTimeout(500);
}

test("home", async ({ page }) => {
  await page.goto(`${BASE_URL}/`);
  await settle(page);
  await page.screenshot({
    path: path.join(OUT_DIR, "home.png"),
    fullPage: true,
  });
});

test("notes", async ({ page }) => {
  await page.goto(`${BASE_URL}/notes`);
  await settle(page);
  await page.screenshot({
    path: path.join(OUT_DIR, "notes.png"),
    fullPage: true,
  });
});

test("documents", async ({ page }) => {
  await page.goto(`${BASE_URL}/docs`);
  await settle(page);
  await page.screenshot({
    path: path.join(OUT_DIR, "documents.png"),
    fullPage: true,
  });
});

test("graph", async ({ page }) => {
  await page.goto(`${BASE_URL}/graph`);
  await settle(page);
  // Graph has an SVG force simulation — give it a bit longer to settle.
  await page.waitForTimeout(2000);
  await page.screenshot({
    path: path.join(OUT_DIR, "graph.png"),
    fullPage: true,
  });
});

test("mcp", async ({ page }) => {
  // /mcp is claimed by the server (MCP over HTTP). Load the SPA shell first,
  // then navigate via the client-side router so we render the MCP Console UI
  // instead of hitting the server's 503/JSON handler directly.
  await page.goto(`${BASE_URL}/`);
  await settle(page);
  await page.evaluate(() => {
    window.history.pushState({}, "", "/mcp");
    window.dispatchEvent(new PopStateEvent("popstate"));
  });
  await settle(page);
  await page.screenshot({
    path: path.join(OUT_DIR, "mcp.png"),
    fullPage: true,
  });
});
