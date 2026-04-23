import { test, expect } from "./fixtures";

// The Shell renders the primary landmark as `<main id="main">`; selecting by id
// avoids strict-mode ambiguity with any other <main> rendered by shadcn sidebar.
const main = (page: import("@playwright/test").Page) => page.locator("main#main");

test.describe("smoke", () => {
  test("home renders shell and main landmark", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await expect(main(page)).toBeVisible();
    await expect(page).toHaveTitle(/docsiq/i);
  });

  test("command palette opens with Ctrl+K and closes with Escape", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await main(page).waitFor();
    await page.keyboard.press("ControlOrMeta+k");
    const palette = page.getByPlaceholder(/search notes, docs, entities/i);
    await expect(palette).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(palette).not.toBeVisible();
  });

  test("command palette navigates to Documents", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await main(page).waitFor();
    await page.keyboard.press("ControlOrMeta+k");
    await page.getByPlaceholder(/search notes, docs, entities/i).waitFor();
    await page.getByRole("option", { name: /^documents$/i }).click();
    await expect(page).toHaveURL(/\/docs$/);
  });

  test("chord hotkey g,g navigates to Graph", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await main(page).waitFor();
    await page.keyboard.press("g");
    await page.keyboard.press("g");
    await expect(page).toHaveURL(/\/graph$/);
  });

  test("theme toggle switches to light and persists on reload", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await main(page).waitFor();
    await page.getByRole("button", { name: /change theme/i }).click();
    await page.getByRole("menuitem", { name: /^light$/i }).click();
    await expect(page.locator("html")).not.toHaveClass(/dark/);
    await page.reload();
    await expect(page.locator("html")).not.toHaveClass(/dark/);
  });
});
