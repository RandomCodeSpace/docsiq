import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { afterEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { server } from "@/test/msw";
import { DeleteDocumentDialog } from "../DeleteDocumentDialog";

// sonner has its own mounted Toaster portal in production. The unit
// test asserts the toast call surface, not its DOM rendering — that
// keeps the test fast (no portal, no animations) and the assertion
// targeted on the contract we care about: success / failure feedback.
const toastMock = {
  success: vi.fn(),
  error: vi.fn(),
};
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastMock.success(...args),
    error: (...args: unknown[]) => toastMock.error(...args),
  },
}));

function Wrapper({ children }: { children: React.ReactNode }) {
  // Fresh client per render so cache pollution can't leak between
  // tests. retry: false so a network error fails fast instead of
  // burning the test timeout on three retries.
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

afterEach(() => {
  toastMock.success.mockReset();
  toastMock.error.mockReset();
});

describe("DeleteDocumentDialog", () => {
  it("renders the document label and a Cancel + Delete button", () => {
    render(
      <Wrapper>
        <DeleteDocumentDialog
          open
          onOpenChange={() => {}}
          project="_default"
          docId="d1"
          docLabel="my-doc.md"
        />
      </Wrapper>,
    );
    expect(screen.getByRole("heading", { name: /delete this document/i })).toBeInTheDocument();
    expect(screen.getByText("my-doc.md")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^cancel$/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^delete$/i })).toBeInTheDocument();
  });

  it("calls DELETE and surfaces a success toast on confirm", async () => {
    const seen: string[] = [];
    server.use(
      http.delete("/api/documents/:id", ({ request, params }) => {
        seen.push(`${request.method} ${params.id as string}`);
        return new HttpResponse(null, { status: 204 });
      }),
    );

    const onOpenChange = vi.fn();
    const onDeleted = vi.fn();
    render(
      <Wrapper>
        <DeleteDocumentDialog
          open
          onOpenChange={onOpenChange}
          project="_default"
          docId="d1"
          docLabel="my-doc.md"
          onDeleted={onDeleted}
        />
      </Wrapper>,
    );

    await userEvent.click(screen.getByRole("button", { name: /^delete$/i }));

    await vi.waitFor(() => {
      expect(seen).toEqual(["DELETE d1"]);
    });
    await vi.waitFor(() => {
      expect(toastMock.success).toHaveBeenCalledTimes(1);
      expect(onOpenChange).toHaveBeenCalledWith(false);
      expect(onDeleted).toHaveBeenCalledTimes(1);
    });
    expect(toastMock.error).not.toHaveBeenCalled();
  });

  it("surfaces an error toast and keeps the dialog open on server failure", async () => {
    server.use(
      http.delete("/api/documents/:id", () =>
        HttpResponse.json({ error: "boom" }, { status: 500 }),
      ),
    );

    const onOpenChange = vi.fn();
    const onDeleted = vi.fn();
    render(
      <Wrapper>
        <DeleteDocumentDialog
          open
          onOpenChange={onOpenChange}
          project="_default"
          docId="d1"
          docLabel="my-doc.md"
          onDeleted={onDeleted}
        />
      </Wrapper>,
    );

    await userEvent.click(screen.getByRole("button", { name: /^delete$/i }));

    await vi.waitFor(() => {
      expect(toastMock.error).toHaveBeenCalledTimes(1);
    });
    expect(onDeleted).not.toHaveBeenCalled();
    // Dialog should remain open on error so the user can retry.
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });
});
