import { describe, it, expect, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useHotkey } from "../useHotkey";

function fireKey(key: string, mod = false) {
  const e = new KeyboardEvent("keydown", {
    key,
    metaKey: mod,
    ctrlKey: mod,
    cancelable: true,
  });
  window.dispatchEvent(e);
}

describe("useHotkey", () => {
  it("fires on mod+k", () => {
    const fn = vi.fn();
    renderHook(() => useHotkey("mod+k", fn));
    fireKey("k", true);
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it("does not fire on k without mod", () => {
    const fn = vi.fn();
    renderHook(() => useHotkey("mod+k", fn));
    fireKey("k", false);
    expect(fn).not.toHaveBeenCalled();
  });

  it("fires on G H chord", () => {
    const fn = vi.fn();
    renderHook(() => useHotkey("g,h", fn));
    fireKey("g");
    fireKey("h");
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it("aborts chord if wrong second key", () => {
    const fn = vi.fn();
    renderHook(() => useHotkey("g,h", fn));
    fireKey("g");
    fireKey("x");
    expect(fn).not.toHaveBeenCalled();
  });
});
