import { renderHook } from "@testing-library/react";
import { describe, it, expect, beforeEach } from "vitest";
import { useLastVisit } from "../useLastVisit";

describe("useLastVisit reference stability (Block 5.5 regression)", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("returns a stable `touch` function across renders", () => {
    const { result, rerender } = renderHook(() => useLastVisit());
    const first = result.current.touch;
    rerender();
    rerender();
    rerender();
    expect(result.current.touch).toBe(first);
  });

  it("touch() updates lastVisit but keeps `touch` identity stable", () => {
    const { result, rerender } = renderHook(() => useLastVisit());
    const firstTouch = result.current.touch;
    result.current.touch();
    rerender();
    expect(result.current.touch).toBe(firstTouch);
    expect(result.current.lastVisit).toBeGreaterThan(0);
  });
});
