import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { CommandPalette } from "../CommandPalette";

// cmdk relies on ResizeObserver which jsdom does not provide
if (typeof globalThis.ResizeObserver === "undefined") {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}
// jsdom does not implement scrollIntoView used by cmdk's list
if (!Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = vi.fn();
}

function wrap(ui: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return (
    <QueryClientProvider client={qc}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  );
}

describe("CommandPalette", () => {
  it("renders input + Pages group when open", () => {
    render(wrap(<CommandPalette open={true} onOpenChange={() => {}} />));
    expect(screen.getByPlaceholderText(/search notes/i)).toBeInTheDocument();
    expect(screen.getByText(/home/i)).toBeInTheDocument();
    expect(screen.getByText(/^notes$/i)).toBeInTheDocument();
  });

  it("shows 'Type to search.' when empty", () => {
    render(wrap(<CommandPalette open={true} onOpenChange={() => {}} />));
    expect(screen.getByText(/type to search/i)).toBeInTheDocument();
  });

  it("filters input via typing", async () => {
    const user = userEvent.setup();
    render(wrap(<CommandPalette open={true} onOpenChange={() => {}} />));
    const input = screen.getByPlaceholderText(/search notes/i);
    await user.type(input, "hello");
    expect(input).toHaveValue("hello");
  });
});
