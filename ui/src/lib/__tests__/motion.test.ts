import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";

function durationOf(t: unknown): number | undefined {
  return (t as { duration?: number }).duration;
}

describe("motion presets", () => {
  beforeEach(() => {
    // default: user does not prefer reduced motion
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
  });
  afterEach(() => vi.restoreAllMocks());

  it("fade returns non-zero duration when motion is allowed", async () => {
    const { fadeTransition } = await import("../motion");
    expect(durationOf(fadeTransition(false))).toBeGreaterThan(0);
  });

  it("fade returns zero duration when reduced motion is requested", async () => {
    const { fadeTransition } = await import("../motion");
    expect(durationOf(fadeTransition(true))).toBe(0);
  });

  it("slideTransition honours reduced motion too", async () => {
    const { slideTransition } = await import("../motion");
    expect(durationOf(slideTransition(true))).toBe(0);
    expect(durationOf(slideTransition(false))).toBeGreaterThan(0);
  });

  it("popTransition honours reduced motion too", async () => {
    const { popTransition } = await import("../motion");
    expect(durationOf(popTransition(true))).toBe(0);
    expect(durationOf(popTransition(false))).toBeGreaterThan(0);
  });
});
