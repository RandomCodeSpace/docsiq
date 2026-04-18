import { describe, it, expect } from "vitest";
import { formatRelativeTime, formatCount } from "../format";

describe("formatRelativeTime", () => {
  const now = new Date("2026-04-18T12:00:00Z").getTime();
  it("formats minutes-ago", () => {
    expect(formatRelativeTime(now - 3 * 60_000, now)).toMatch(/3 minutes? ago/);
  });
  it("formats hours-ago", () => {
    expect(formatRelativeTime(now - 2 * 3600_000, now)).toMatch(/2 hours? ago/);
  });
  it("formats days-ago", () => {
    expect(formatRelativeTime(now - 3 * 86400_000, now)).toMatch(/3 days? ago/);
  });
});

describe("formatCount", () => {
  it("raw for < 1k", () => expect(formatCount(42)).toBe("42"));
  it("k for thousands", () => expect(formatCount(1234)).toBe("1.2k"));
  it("k rounded for >= 10k", () => expect(formatCount(12_345)).toBe("12k"));
  it("m for millions", () => expect(formatCount(1_234_567)).toBe("1.2m"));
});
