import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ThemeToggle } from "../ThemeToggle";
import { useUIStore } from "@/stores/ui";

// Polyfills for Radix dropdown in jsdom
if (!(globalThis as any).ResizeObserver) {
  (globalThis as any).ResizeObserver = class { observe() {} unobserve() {} disconnect() {} };
}
if (!(Element.prototype as any).scrollIntoView) {
  (Element.prototype as any).scrollIntoView = () => {};
}

describe("ThemeToggle", () => {
  it("opens menu and switches theme to light", async () => {
    useUIStore.setState({ theme: "system" });
    const user = userEvent.setup();
    render(<ThemeToggle />);
    await user.click(screen.getByRole("button", { name: /change theme/i }));
    await user.click(screen.getByText(/^light$/i));
    expect(useUIStore.getState().theme).toBe("light");
  });
});
