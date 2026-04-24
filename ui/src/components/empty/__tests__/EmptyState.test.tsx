import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { EmptyState } from "../EmptyState";

describe("EmptyState", () => {
  it("renders title and description", () => {
    render(<EmptyState title="No notes yet" description="Create one to get started." />);
    expect(screen.getByRole("status")).toBeInTheDocument();
    expect(screen.getByText("No notes yet")).toBeInTheDocument();
    expect(screen.getByText("Create one to get started.")).toBeInTheDocument();
  });

  it("renders an action slot when provided", () => {
    render(
      <EmptyState
        title="No notes"
        description="Try ingesting one."
        action={<button type="button">Ingest</button>}
      />,
    );
    expect(screen.getByRole("button", { name: /ingest/i })).toBeInTheDocument();
  });

  it("exposes role=status so screen readers announce it", () => {
    render(<EmptyState title="x" description="y" />);
    expect(screen.getByRole("status")).toHaveAttribute("aria-live", "polite");
  });
});
