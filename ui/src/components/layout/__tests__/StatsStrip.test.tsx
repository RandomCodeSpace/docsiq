import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { StatsStrip } from "../StatsStrip";

describe("StatsStrip", () => {
  it("renders placeholders on undefined stats", () => {
    render(<StatsStrip stats={undefined} />);
    expect(screen.getAllByText("—").length).toBeGreaterThan(0);
  });
  it("renders counts + delta", () => {
    render(
      <StatsStrip
        stats={{
          documents: 42, chunks: 512, entities: 380, relationships: 820,
          communities: 8, notes: 17,
          last_indexed: new Date().toISOString(),
        }}
        delta={{ notes: 2 }}
      />,
    );
    expect(screen.getByText("17")).toBeInTheDocument();
    expect(screen.getByText("+2")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
  });
});
