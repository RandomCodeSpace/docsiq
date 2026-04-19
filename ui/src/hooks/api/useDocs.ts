import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { qk } from "./keys";
import type { Document } from "@/types/api";

export function useDocs(project: string) {
  return useQuery({
    queryKey: qk.docs(project),
    queryFn: async () => {
      const res = await apiFetch<Document[] | null>(
        `/api/documents?project=${encodeURIComponent(project)}`,
      );
      return Array.isArray(res) ? res : [];
    },
  });
}

export function useDoc(project: string, id: string | undefined) {
  return useQuery({
    queryKey: qk.doc(project, id ?? ""),
    enabled: !!id,
    queryFn: () => apiFetch<Document>(`/api/documents/${encodeURIComponent(id!)}?project=${encodeURIComponent(project)}`),
  });
}
