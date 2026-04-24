import { test, expect } from "./fixtures";

test.describe("404", () => {
  test("unknown route renders NotFound and keeps the shell", async ({ stubbedPage: page }) => {
    await page.goto("/this-route-does-not-exist");

    // NotFound lives inside the Shell, so the main landmark still renders.
    await expect(page.locator("main#main")).toBeVisible();

    // NotFound component copy is stable; see ui/src/App.tsx NotFound().
    await expect(
      page.getByRole("heading", { name: /^not found$/i }),
    ).toBeVisible();
    await expect(page.getByText(/no such page/i)).toBeVisible();
  });

  test("nested unknown route also renders NotFound (shell intact)", async ({ stubbedPage: page }) => {
    await page.goto("/definitely/not/a/real/path");
    // The catch-all in App.tsx handles any unmatched depth — confirm it
    // still fires for deep URLs (react-router ordering regression guard).
    await expect(page.locator("main#main")).toBeVisible();
    await expect(
      page.getByRole("heading", { name: /^not found$/i }),
    ).toBeVisible();
  });
});
