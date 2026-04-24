import { render, screen } from "@testing-library/react";
import { act } from "react";
import { afterEach, describe, expect, it } from "vitest";
import { AuthRequiredBanner } from "../AuthRequiredBanner";
import { useAuthStore } from "@/stores/auth";

afterEach(() => {
  // Store is module-level; reset between tests or flipped state leaks.
  act(() => useAuthStore.getState().clear());
});

describe("AuthRequiredBanner", () => {
  it("renders nothing while authRequired is false", () => {
    const { container } = render(<AuthRequiredBanner />);
    expect(container.firstChild).toBeNull();
  });

  it("renders a visible sign-in affordance once authRequired flips", () => {
    render(<AuthRequiredBanner />);
    act(() => useAuthStore.getState().signalUnauthorized());

    const banner = screen.getByTestId("auth-required-banner");
    expect(banner).toBeInTheDocument();
    expect(banner).toHaveAttribute("role", "alert");
    expect(banner).toHaveAttribute("aria-live", "assertive");
    expect(screen.getByText(/sign in required/i)).toBeInTheDocument();
    expect(
      screen.getByText(/session has expired|please sign in/i),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /reload/i })).toBeInTheDocument();
  });
});
