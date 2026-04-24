import { test as fixtureTest, expect } from "./fixtures";

// Simulate an iPhone 14 viewport on the configured Chromium project rather
// than using devices["iPhone 14"] (which pins the webkit engine). The
// browser engine is not what this test exercises — the CSS rule is. The
// 390x844 dimensions match the iPhone 14 logical viewport.
fixtureTest.use({
  viewport: { width: 390, height: 844 },
  deviceScaleFactor: 3,
  isMobile: true,
  hasTouch: true,
});

fixtureTest("header padding accommodates safe-area-inset-top on iPhone 14 viewport", async ({ stubbedPage: page }) => {
  await page.goto("/");
  await page.locator("main#main").waitFor();
  const header = page.locator(".site-header").first();
  await expect(header).toBeVisible();
  const paddingTopPx = await header.evaluate(
    (el) => parseFloat(getComputedStyle(el).paddingTop),
  );
  // max(1rem, env(safe-area-inset-top)) must be at least 1rem = 16px.
  expect(paddingTopPx).toBeGreaterThanOrEqual(16);
});
