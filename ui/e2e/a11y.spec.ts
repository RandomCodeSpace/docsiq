import { test, expect } from "./fixtures";
import AxeBuilder from "@axe-core/playwright";

const ROUTES = ["/", "/notes", "/docs", "/graph"];

test.describe("axe a11y audit — zero violations", () => {
  for (const url of ROUTES) {
    test(`no violations on ${url}`, async ({ stubbedPage: page }) => {
      await page.goto(url);
      await page.locator("main#main").waitFor();
      const results = await new AxeBuilder({ page })
        .withTags(["wcag2a", "wcag2aa"])
        .analyze();
      expect(
        results.violations,
        `${url}:\n${JSON.stringify(results.violations, null, 2)}`,
      ).toEqual([]);
    });
  }

  test("no violations on /mcp (client-side nav)", async ({ stubbedPage: page }) => {
    // /mcp is proxied in dev-server, navigate via SPA history.
    await page.goto("/");
    await page.locator("main#main").waitFor();
    await page.evaluate(() => window.history.pushState({}, "", "/mcp"));
    await page.evaluate(() => window.dispatchEvent(new PopStateEvent("popstate")));
    await page.waitForTimeout(200);
    const results = await new AxeBuilder({ page })
      .withTags(["wcag2a", "wcag2aa"])
      .analyze();
    expect(
      results.violations,
      `/mcp:\n${JSON.stringify(results.violations, null, 2)}`,
    ).toEqual([]);
  });
});
