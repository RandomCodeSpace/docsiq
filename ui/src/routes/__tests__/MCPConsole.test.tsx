import { afterEach, describe, expect, it } from "vitest";
import { act } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { http, HttpResponse } from "msw";
import { server } from "@/test/msw";
import MCPConsole from "@/routes/MCPConsole";
import { AuthRequiredBanner } from "@/components/layout/AuthRequiredBanner";
import { useAuthStore } from "@/stores/auth";

afterEach(() => {
  // Auth store is module-level — without a reset between tests the 401
  // banner from a prior assertion leaks into the next render.
  act(() => useAuthStore.getState().clear());
});

describe("MCPConsole route", () => {
  it("renders the shared AuthRequiredBanner when /mcp returns 401", async () => {
    server.use(
      http.post("/mcp", () =>
        HttpResponse.json(
          { error: "session expired" },
          { status: 401, headers: { "Content-Type": "application/json" } },
        ),
      ),
    );

    render(
      <MemoryRouter initialEntries={["/mcp"]}>
        <AuthRequiredBanner />
        <MCPConsole />
      </MemoryRouter>,
    );

    // The hook fires `initialize` on mount; the 401 must flip the auth
    // store via mcpRequest so the shared banner renders on this route.
    const banner = await screen.findByTestId("auth-required-banner");
    expect(banner).toBeInTheDocument();
    expect(screen.getByText(/sign in required/i)).toBeInTheDocument();

    await waitFor(() => expect(useAuthStore.getState().authRequired).toBe(true));
  });
});
