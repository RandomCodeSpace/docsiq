import { test as fixtureTest, expect } from "./fixtures";

const mobileTest = fixtureTest.extend({});
mobileTest.use({ viewport: { width: 375, height: 812 } });

mobileTest.describe("mobile 375px viewport", () => {
  mobileTest("sidebar collapses at 375px", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await page.locator("main#main").waitFor();
    // The shadcn sidebar hides on mobile by default; assert it is not part of
    // the initial visible tab-order.
    const collapsedOrHidden = await page.evaluate(() => {
      const sb = document.querySelector("[data-slot='sidebar'], [data-sidebar='sidebar']");
      if (!sb) return true; // treated as collapsed if not rendered
      const r = sb.getBoundingClientRect();
      return r.width < 60 || r.left < -10 || getComputedStyle(sb).display === "none";
    });
    expect(collapsedOrHidden).toBe(true);
  });

  mobileTest("header buttons meet 44x44 tap-target minimum", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await page.locator("main#main").waitFor();
    const buttons = page.locator(".site-header button, .site-header a[role='button']");
    const count = await buttons.count();
    for (let i = 0; i < count; i++) {
      const box = await buttons.nth(i).boundingBox();
      if (!box) continue; // button hidden on this viewport
      expect.soft(box.width, `button ${i} too narrow`).toBeGreaterThanOrEqual(44);
      expect.soft(box.height, `button ${i} too short`).toBeGreaterThanOrEqual(44);
    }
  });

  mobileTest("command palette fills the viewport", async ({ stubbedPage: page }) => {
    await page.goto("/");
    await page.locator("main#main").waitFor();
    await page.keyboard.press("ControlOrMeta+k");
    const dialog = page.locator("[role='dialog']").first();
    await dialog.waitFor();
    const box = await dialog.boundingBox();
    expect(box?.width ?? 0).toBeGreaterThanOrEqual(320);
  });

  mobileTest("documents list does not overflow horizontally", async ({ stubbedPage: page }) => {
    await page.goto("/docs");
    await page.locator("main#main").waitFor();
    // Detect horizontal scroll on <body> — tables should scroll inside their
    // wrapper, not push the body.
    const bodyOverflow = await page.evaluate(() => {
      const d = document.documentElement;
      return d.scrollWidth - d.clientWidth;
    });
    expect(bodyOverflow).toBeLessThanOrEqual(1);
  });
});
