import { test as base, expect, type Page } from "@playwright/test";

// This spec overrides the default stubbedPage fixture because we want
// the API to return 401, not 200-with-empty-body. Kept self-contained
// so no export from fixtures.ts is required.
const API_PATH = /^\/api\//;
const MCP_PATH = /^\/mcp\//;

async function stubUnauthed(page: Page) {
  await page.route(
    (url) => API_PATH.test(url.pathname),
    (route) =>
      route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ error: "unauthenticated" }),
      }),
  );
  await page.route(
    (url) => MCP_PATH.test(url.pathname),
    (route) =>
      route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ error: "unauthenticated" }),
      }),
  );
}

const test = base.extend<{ unauthedPage: Page }>({
  unauthedPage: async ({ page }, use) => {
    await stubUnauthed(page);
    await use(page);
  },
});

// TODO(#66): re-enable these once the UI renders a visible "sign in" /
// "authentication required" affordance on 401. Today apiFetch throws an
// ApiErrorResponse that bubbles into React Query error states without a
// recognisable auth-copy surface. See flake-register issue #66.
test.describe("unauthed API", () => {
  test.fixme(
    "home surfaces an auth-required affordance when /api/* returns 401",
    async ({ unauthedPage: page }) => {
      await page.goto("/");
      await expect(page.locator("main#main")).toBeVisible();
      await expect(
        page
          .getByText(/sign in|authenticat|authori|session expired|please log in/i)
          .first(),
      ).toBeVisible({ timeout: 5_000 });
    },
  );

  test.fixme(
    "navigating to /notes with 401 shows the same affordance",
    async ({ unauthedPage: page }) => {
      await page.goto("/notes");
      await expect(page.locator("main#main")).toBeVisible();
      await expect(
        page
          .getByText(/sign in|authenticat|authori|session expired|please log in/i)
          .first(),
      ).toBeVisible({ timeout: 5_000 });
    },
  );
});
