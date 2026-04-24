import { afterEach, describe, expect, it } from "vitest";
import { useAuthStore } from "../auth";

afterEach(() => useAuthStore.getState().clear());

describe("useAuthStore", () => {
  it("starts clean", () => {
    expect(useAuthStore.getState().authRequired).toBe(false);
  });

  it("signalUnauthorized flips the flag; clear resets it", () => {
    useAuthStore.getState().signalUnauthorized();
    expect(useAuthStore.getState().authRequired).toBe(true);
    useAuthStore.getState().clear();
    expect(useAuthStore.getState().authRequired).toBe(false);
  });
});
