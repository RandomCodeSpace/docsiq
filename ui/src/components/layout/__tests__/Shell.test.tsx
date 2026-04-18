import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { Shell } from "../Shell";

describe("Shell", () => {
  it("renders sidebar, topbar, skip link, and main landmark", () => {
    render(
      <MemoryRouter>
        <Shell>
          <div>content</div>
        </Shell>
      </MemoryRouter>,
    );
    expect(screen.getByRole("navigation", { name: /primary/i })).toBeInTheDocument();
    expect(screen.getByRole("main")).toBeInTheDocument();
    expect(screen.getByText(/skip to main content/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /open command palette/i })).toBeInTheDocument();
    expect(screen.getByText("content")).toBeInTheDocument();
  });
});
