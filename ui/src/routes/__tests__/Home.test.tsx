import { describe, it, expect } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { server } from "@/test/msw";
import Home from "@/routes/Home";

function wrap() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return (node: React.ReactNode) => (
    <QueryClientProvider client={qc}>
      <MemoryRouter>{node}</MemoryRouter>
    </QueryClientProvider>
  );
}

describe("Home route", () => {
  it("renders stats + empty activity + empty glance state", async () => {
    server.use(
      http.get("/api/projects/_default/notes", () => HttpResponse.json([])),
      http.get("/api/documents", () => HttpResponse.json([])),
      http.get("/api/projects/_default/graph", () => HttpResponse.json({ nodes: [], edges: [] })),
    );
    render(wrap()(<Home />));
    await waitFor(() => expect(screen.getByText(/since your last visit|nothing new/i)).toBeInTheDocument());
    expect(screen.getByRole("region", { name: /project statistics/i })).toBeInTheDocument();
  });
});
