import { describe, it, expect, beforeEach } from "vitest";
import { act } from "@testing-library/react";
import { useUIStore } from "../ui";

describe("useUIStore", () => {
  beforeEach(() => {
    localStorage.clear();
    useUIStore.setState({
      sidebarCollapsed: false,
      theme: "system",
      treeDrawerPinned: false,
      linkDrawerPinned: false,
    });
  });

  it("toggles sidebar", () => {
    expect(useUIStore.getState().sidebarCollapsed).toBe(false);
    act(() => useUIStore.getState().toggleSidebar());
    expect(useUIStore.getState().sidebarCollapsed).toBe(true);
    act(() => useUIStore.getState().toggleSidebar());
    expect(useUIStore.getState().sidebarCollapsed).toBe(false);
  });

  it("sets theme", () => {
    act(() => useUIStore.getState().setTheme("dark"));
    expect(useUIStore.getState().theme).toBe("dark");
  });

  it("persists theme to localStorage", () => {
    act(() => useUIStore.getState().setTheme("light"));
    const persisted = JSON.parse(localStorage.getItem("docsiq-ui")!);
    expect(persisted.state.theme).toBe("light");
  });
});
