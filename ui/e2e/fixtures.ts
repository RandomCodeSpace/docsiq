import { test as base, expect, type Page } from "@playwright/test";

// Match only backend paths — anchored to pathname start so Vite's dev-server
// module requests under src/hooks/api/** are not stubbed.
const API_PATH = /^\/api\//;
const MCP_PATH = /^\/mcp\//;

async function stubApi(page: Page) {
  await page.route(
    (url) => API_PATH.test(url.pathname),
    async (route) => {
      const path = new URL(route.request().url()).pathname;

      if (/\/notes(\?|$)/.test(path) || /\/documents(\?|$)/.test(path) || /\/activity(\?|$)/.test(path)) {
        return route.fulfill({ status: 200, contentType: "application/json", body: "[]" });
      }
      if (/\/stats$/.test(path)) {
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ notes: 0, documents: 0, entities: 0, relationships: 0 }),
        });
      }
      if (/\/search/.test(path)) {
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ notes: [], docs: [] }),
        });
      }
      return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
    },
  );
  await page.route(
    (url) => MCP_PATH.test(url.pathname),
    (route) => route.fulfill({ status: 200, contentType: "application/json", body: "{}" }),
  );
}

export const test = base.extend<{ stubbedPage: Page }>({
  stubbedPage: async ({ page }, use) => {
    await stubApi(page);
    await use(page);
  },
});

export { expect };
