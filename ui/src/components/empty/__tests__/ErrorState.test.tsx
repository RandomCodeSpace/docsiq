import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { ErrorState } from "../ErrorState";

describe("ErrorState", () => {
  it("renders the message and a retry button when retry is provided", async () => {
    const retry = vi.fn();
    render(
      <ErrorState title="Failed to load" message="Network error" onRetry={retry} />,
    );
    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("Failed to load")).toBeInTheDocument();
    expect(screen.getByText("Network error")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /retry/i }));
    expect(retry).toHaveBeenCalledOnce();
  });

  it("hides the retry button when onRetry is absent", () => {
    render(<ErrorState title="Broken" message="x" />);
    expect(screen.queryByRole("button", { name: /retry/i })).toBeNull();
  });

  it("sanitizes long messages to 500 chars", () => {
    const long = "a".repeat(900);
    render(<ErrorState title="t" message={long} />);
    const rendered = screen.getByTestId("error-message").textContent ?? "";
    expect(rendered.length).toBeLessThanOrEqual(504); // 500 + ellipsis
  });
});
