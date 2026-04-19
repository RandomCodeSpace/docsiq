import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { qk } from "./keys";
import type { Stats } from "@/types/api";

export function useStats(project: string) {
  return useQuery({
    queryKey: qk.stats(project),
    queryFn: () => apiFetch<Stats>(`/api/stats?project=${encodeURIComponent(project)}`),
  });
}
