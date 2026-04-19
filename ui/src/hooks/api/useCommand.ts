import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import type { NoteHit, SearchHit } from "@/types/api";

export function useCommandSearch(project: string, query: string) {
  return useQuery({
    queryKey: ["command-search", project, query],
    enabled: query.trim().length > 0,
    queryFn: async () => {
      const [notes, docs] = await Promise.all([
        apiFetch<{ hits: NoteHit[] }>(
          `/api/projects/${encodeURIComponent(project)}/search?q=${encodeURIComponent(query)}`,
        ).catch(() => ({ hits: [] as NoteHit[] })),
        apiFetch<{ hits: SearchHit[] }>(
          `/api/search?project=${encodeURIComponent(project)}&q=${encodeURIComponent(query)}&mode=local&top_k=5`,
        ).catch(() => ({ hits: [] as SearchHit[] })),
      ]);
      return { notes: notes.hits, docs: docs.hits };
    },
    staleTime: 10_000,
  });
}
