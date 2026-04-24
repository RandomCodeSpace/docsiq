import { defineConfig, devices } from "@playwright/test";

const PORT = Number(process.env.PLAYWRIGHT_PORT ?? 5173);
const BASE_URL = process.env.PLAYWRIGHT_BASE_URL ?? `http://localhost:${PORT}`;
const isCI = !!process.env.CI;

export default defineConfig({
  testDir: "./e2e",
  // Screenshot regeneration is a manual task run against a live docsiq
  // backend with seeded fixtures; it has no role in the CI smoke suite
  // and fails fast when no backend is reachable. Gate behind the
  // PLAYWRIGHT_SCREENSHOTS env var so the dedicated script opts in.
  testIgnore: process.env.PLAYWRIGHT_SCREENSHOTS ? undefined : ["**/screenshots.spec.ts"],
  fullyParallel: true,
  forbidOnly: isCI,
  retries: isCI ? 1 : 0,
  workers: isCI ? 2 : undefined,
  reporter: isCI ? [["github"], ["html", { open: "never" }]] : "list",
  timeout: 30_000,
  expect: { timeout: 5_000 },
  use: {
    baseURL: BASE_URL,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  ],
  webServer: process.env.PLAYWRIGHT_SKIP_WEBSERVER
    ? undefined
    : {
        command: `npm run dev -- --port ${PORT} --strictPort`,
        url: BASE_URL,
        reuseExistingServer: !isCI,
        timeout: 60_000,
        stdout: "ignore",
        stderr: "pipe",
      },
});
