import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { server } from "@/test/msw";
import NoteView from "../NoteView";

describe("NoteView", () => {
  it("renders note content + metadata", async () => {
    server.use(
      http.get("/api/projects/_default/notes/jwt", () =>
        HttpResponse.json({
          key: "jwt", content: "# JWT rotation\n\nbody", author: "claude",
          tags: [], created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
        }),
      ),
    );
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={["/notes/jwt"]}>
          <Routes><Route path="/notes/:key" element={<NoteView />} /></Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );
    await screen.findByRole("heading", { name: /jwt rotation/i });
    expect(screen.getByText(/by claude/i)).toBeInTheDocument();
  });
});
