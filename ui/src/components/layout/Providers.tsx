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
import { Toaster } from "@/components/ui/sonner";

// Defensive 401 gate. The HTTP boundary (apiFetch / mcpRequest in
// lib/api-client.ts) is the primary signal for AuthRequiredBanner — it
// already flips the auth store on every 401 it sees. This React Query
// hook is kept so any consumer that synthesises an ApiErrorResponse
// outside the fetch path (e.g. tests, future non-HTTP transports) still
// surfaces the banner. signalUnauthorized is idempotent, so the
// double-signal on real /api/* 401s is harmless.
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
      <Toaster richColors position="bottom-right" />
    </QueryClientProvider>
  );
}
