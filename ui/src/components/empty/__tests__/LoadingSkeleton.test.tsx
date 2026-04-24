import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { LoadingSkeleton } from "../LoadingSkeleton";

describe("LoadingSkeleton", () => {
  it("renders a polite live-region with a label", () => {
    render(<LoadingSkeleton label="Loading notes" />);
    const status = screen.getByRole("status");
    expect(status).toHaveAttribute("aria-live", "polite");
    expect(status).toHaveAttribute("aria-label", "Loading notes");
  });

  it("renders `rows` skeleton bars", () => {
    render(<LoadingSkeleton label="Loading" rows={4} />);
    expect(screen.getAllByTestId("skeleton-row")).toHaveLength(4);
  });

  it("defaults to 3 rows when no count given", () => {
    render(<LoadingSkeleton label="Loading" />);
    expect(screen.getAllByTestId("skeleton-row")).toHaveLength(3);
  });
});
