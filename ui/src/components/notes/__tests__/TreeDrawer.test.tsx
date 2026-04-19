import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { server } from "@/test/msw";
import { TreeDrawer } from "../TreeDrawer";

// Polyfills for jsdom that shadcn Sheet (Radix) may need:
if (!(global as any).ResizeObserver) {
  (global as any).ResizeObserver = class { observe() {} unobserve() {} disconnect() {} };
}
if (!(Element.prototype as any).scrollIntoView) {
  (Element.prototype as any).scrollIntoView = () => {};
}

describe("TreeDrawer", () => {
  it("renders grouped notes from API", async () => {
    server.use(
      http.get("/api/projects/_default/notes", () =>
        HttpResponse.json([
          { key: "architecture/jwt", content: "", tags: [], created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
          { key: "decisions/drop-redis", content: "", tags: [], created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
          { key: "intro", content: "", tags: [], created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
        ]),
      ),
    );
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter>
          <TreeDrawer project="_default" open={true} onOpenChange={() => {}} />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    await screen.findByText("jwt");
    expect(screen.getByText("architecture/")).toBeInTheDocument();
    expect(screen.getByText("decisions/")).toBeInTheDocument();
  });
});
