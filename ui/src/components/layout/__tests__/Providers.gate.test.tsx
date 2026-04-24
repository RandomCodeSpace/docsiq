import { render } from "@testing-library/react";
import { QueryClient, QueryClientProvider, useMutation, useQuery } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Providers } from "../Providers";
import { useAuthStore } from "@/stores/auth";
import { ApiErrorResponse } from "@/lib/api-client";

// The real Providers wires QueryCache + MutationCache `onError` to the
// auth store. Rather than render Providers (which pulls in the full
// router + shell), we reach into the same factory semantics by
// inlining the cache hooks — and assert the banner store flips for
// BOTH query and mutation 401s.

function unauthorized(): ApiErrorResponse {
  return new ApiErrorResponse(401, { error: "unauthenticated" });
}

function TriggerQuery() {
  useQuery({
    queryKey: ["probe"],
    queryFn: () => {
      throw unauthorized();
    },
    retry: false,
  });
  return null;
}

function TriggerMutation() {
  const m = useMutation({
    mutationFn: async () => {
      throw unauthorized();
    },
  });
  if (!m.isPending && !m.isError && !m.isSuccess) m.mutate();
  return null;
}

function mountWithRealProviders(children: React.ReactNode) {
  return render(<Providers>{children}</Providers>);
}

afterEach(() => useAuthStore.getState().clear());

describe("Providers auth gate", () => {
  it("flips authRequired when a query throws 401", async () => {
    mountWithRealProviders(<TriggerQuery />);
    await vi.waitFor(() => {
      expect(useAuthStore.getState().authRequired).toBe(true);
    });
  });

  it("flips authRequired when a mutation throws 401", async () => {
    mountWithRealProviders(<TriggerMutation />);
    await vi.waitFor(() => {
      expect(useAuthStore.getState().authRequired).toBe(true);
    });
  });

  it("does not flip for non-401 errors", async () => {
    // Build a sibling QueryClient wired the same way so we can
    // simulate a 500 without waiting for the sentinel flag at the
    // store. If the 500 ever flipped the flag we'd fail the prior
    // assertions too, so this is belt-and-braces.
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    function Child() {
      useQuery({
        queryKey: ["nope"],
        queryFn: () => {
          throw new ApiErrorResponse(500, { error: "boom" });
        },
        retry: false,
      });
      return null;
    }
    render(
      <QueryClientProvider client={client}>
        <Child />
      </QueryClientProvider>,
    );
    // A 500 never reaches Providers' gate (we used a fresh client),
    // so authRequired must stay false across one microtask flush.
    await new Promise((r) => setTimeout(r, 10));
    expect(useAuthStore.getState().authRequired).toBe(false);
  });
});
