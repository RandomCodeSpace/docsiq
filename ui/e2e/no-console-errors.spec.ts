import { test, expect } from "./fixtures";

// All Block 5 primary routes. `/mcp` is navigated via client-side routing
// from `/` because the Vite dev-server has a proxy rule for `/mcp` that
// would otherwise intercept the document request.
const ROUTES = ["/", "/notes", "/docs", "/graph"];

test.describe("no update-depth warnings on any route", () => {
  for (const url of ROUTES) {
    test(`no update-depth warning on ${url}`, async ({ stubbedPage: page }) => {
      const errors: string[] = [];
      const warnings: string[] = [];
      page.on("console", (msg) => {
        const text = msg.text();
        if (msg.type() === "error") errors.push(text);
        if (msg.type() === "warning") warnings.push(text);
      });
      page.on("pageerror", (err) => errors.push(err.message));

      await page.goto(url);
      await page.locator("main#main").waitFor();
      // Give effects a tick to run — if there's an infinite loop, it fires
      // within a few microtasks and the warning lands here.
      await page.waitForTimeout(500);

      const offending = [...errors, ...warnings].filter((t) =>
        /maximum update depth|too many re-renders/i.test(t),
      );
      expect(offending, `${url} emitted: ${offending.join(" | ")}`).toEqual([]);
    });
  }

  test("no update-depth warning on /mcp (client-side nav)", async ({ stubbedPage: page }) => {
    const errors: string[] = [];
    const warnings: string[] = [];
    page.on("console", (msg) => {
      const text = msg.text();
      if (msg.type() === "error") errors.push(text);
      if (msg.type() === "warning") warnings.push(text);
    });
    page.on("pageerror", (err) => errors.push(err.message));

    // Start at Home, then navigate to /mcp via SPA history — this avoids the
    // Vite dev-server /mcp proxy rule that otherwise serves a 404 from the
    // backend port when a document request is issued directly.
    await page.goto("/");
    await page.locator("main#main").waitFor();
    await page.evaluate(() => window.history.pushState({}, "", "/mcp"));
    await page.evaluate(() => window.dispatchEvent(new PopStateEvent("popstate")));
    await page.waitForTimeout(500);

    const offending = [...errors, ...warnings].filter((t) =>
      /maximum update depth|too many re-renders/i.test(t),
    );
    expect(offending, `/mcp emitted: ${offending.join(" | ")}`).toEqual([]);
  });
});
