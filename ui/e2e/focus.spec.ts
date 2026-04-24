import { test, expect } from "./fixtures";

test.describe("focus management", () => {
  test("skip-link is the first tab target and moves focus to main#main", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await page.locator("main#main").waitFor();
    await page.keyboard.press("Tab");
    const focusedIsSkipLink = await page.evaluate(
      () => document.activeElement?.textContent?.toLowerCase().includes("skip to main content") ?? false,
    );
    expect(focusedIsSkipLink).toBe(true);
    await page.keyboard.press("Enter");
    const mainFocused = await page.evaluate(() => document.activeElement?.id === "main");
    expect(mainFocused).toBe(true);
  });

  test("command palette returns focus to the invoking button on close", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await page.locator("main#main").waitFor();
    const searchBtn = page.locator(".site-header-search").first();
    await searchBtn.focus();
    await expect(searchBtn).toBeFocused();
    await page.keyboard.press("Enter");
    await page.getByPlaceholder(/search notes, docs, entities/i).waitFor();
    await page.keyboard.press("Escape");
    // After close, focus must return to the search button.
    await expect(searchBtn).toBeFocused();
  });

  test("dialog traps focus while open (radix)", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await page.locator("main#main").waitFor();
    await page.keyboard.press("ControlOrMeta+k");
    await page.getByPlaceholder(/search notes, docs, entities/i).waitFor();
    // Tab 20 times; focus must stay inside the palette dialog.
    for (let i = 0; i < 20; i++) {
      await page.keyboard.press("Tab");
      const inside = await page.evaluate(() =>
        Boolean(document.activeElement?.closest("[role=\"dialog\"]")),
      );
      expect(inside, `focus escaped the dialog on Tab ${i}`).toBe(true);
    }
  });
});
