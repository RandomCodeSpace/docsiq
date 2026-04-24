import {
  MutationCache,
  QueryCache,
  QueryClient,
  QueryClientProvider,
} from "@tanstack/react-query";
import { useEffect, useState, type ReactNode } from "react";
import { BrowserRouter } from "react-router-dom";
import { useUIStore } from "@/stores/ui";
import { useAuthStore } from "@/stores/auth";

// Global 401 gate: any /api/* fetch that throws an ApiErrorResponse
// with status === 401 flips the auth store so AuthRequiredBanner can
// render a visible "Sign in required" affordance. Wired to BOTH
// QueryCache and MutationCache — a 401 on a write action (note
// create/update/delete) must surface the banner just the same as a
// read-path failure.
function gateUnauthorized(error: unknown) {
  const status = (error as { status?: number })?.status ?? 0;
  if (status === 401) {
    useAuthStore.getState().signalUnauthorized();
  }
}

export function Providers({ children }: { children: ReactNode }) {
  const [client] = useState(
    () =>
      new QueryClient({
        queryCache: new QueryCache({ onError: gateUnauthorized }),
        mutationCache: new MutationCache({ onError: gateUnauthorized }),
        defaultOptions: {
          queries: {
            staleTime: 30_000,
            retry: (failureCount, error: unknown) => {
              const status = (error as { status?: number })?.status ?? 0;
              if (status >= 400 && status < 500) return false;
              return failureCount < 3;
            },
            refetchOnWindowFocus: false,
          },
        },
      }),
  );

  const theme = useUIStore((s) => s.theme);
  useEffect(() => {
    const root = document.documentElement;
    const apply = () => {
      const systemDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
      const effective = theme === "system" ? (systemDark ? "dark" : "light") : theme;
      root.dataset.theme = effective;
      root.classList.toggle("dark", effective === "dark");
    };
    apply();
    if (theme === "system") {
      const mq = window.matchMedia("(prefers-color-scheme: dark)");
      mq.addEventListener("change", apply);
      return () => mq.removeEventListener("change", apply);
    }
  }, [theme]);

  return (
    <QueryClientProvider client={client}>
      <BrowserRouter>{children}</BrowserRouter>
    </QueryClientProvider>
  );
}
