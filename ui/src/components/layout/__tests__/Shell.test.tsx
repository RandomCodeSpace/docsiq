import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Shell } from "../Shell";

describe("Shell", () => {
  it("renders sidebar, topbar, skip link, and main landmark", () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter>
          <Shell>
            <div>content</div>
          </Shell>
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(screen.getAllByRole("main").length).toBeGreaterThan(0);
    expect(screen.getByText(/skip to main content/i)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /docsiq/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /search/i })).toBeInTheDocument();
    expect(screen.getByText("content")).toBeInTheDocument();
  });
});
