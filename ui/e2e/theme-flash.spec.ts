import { test, expect } from "./fixtures";

test.describe("theme-flash", () => {
  test("dark theme is applied before React hydrates", async ({ stubbedPage: page }) => {
    // Seed localStorage BEFORE navigating so the inline script can read it.
    await page.addInitScript(() => {
      window.localStorage.setItem(
        "docsiq-ui",
        JSON.stringify({ state: { theme: "dark" }, version: 0 }),
      );
    });
    await page.goto("/");
    // Check the html element's class list at the earliest opportunity.
    const hasDark = await page.evaluate(() =>
      document.documentElement.classList.contains("dark"),
    );
    expect(hasDark).toBe(true);
    // And data-theme is set too
    const themeAttr = await page.evaluate(() =>
      document.documentElement.dataset.theme,
    );
    expect(themeAttr).toBe("dark");
  });

  test("light theme renders without .dark class", async ({ stubbedPage: page }) => {
    await page.addInitScript(() => {
      window.localStorage.setItem(
        "docsiq-ui",
        JSON.stringify({ state: { theme: "light" }, version: 0 }),
      );
    });
    await page.goto("/");
    const hasDark = await page.evaluate(() =>
      document.documentElement.classList.contains("dark"),
    );
    expect(hasDark).toBe(false);
  });

  test("system theme resolves via prefers-color-scheme before hydration", async ({ browser }) => {
    const ctx = await browser.newContext({ colorScheme: "dark" });
    const p2 = await ctx.newPage();
    await p2.addInitScript(() => {
      window.localStorage.setItem(
        "docsiq-ui",
        JSON.stringify({ state: { theme: "system" }, version: 0 }),
      );
    });
    await p2.goto("/");
    const hasDark = await p2.evaluate(() =>
      document.documentElement.classList.contains("dark"),
    );
    expect(hasDark).toBe(true);
    await ctx.close();
  });
});
