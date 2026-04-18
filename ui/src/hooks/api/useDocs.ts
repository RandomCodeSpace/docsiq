import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { qk } from "./keys";
import type { Document } from "@/types/api";

export function useDocs(project: string) {
  return useQuery({
    queryKey: qk.docs(project),
    queryFn: () => apiFetch<Document[]>(`/api/documents?project=${encodeURIComponent(project)}`),
  });
}

export function useDoc(project: string, id: string | undefined) {
  return useQuery({
    queryKey: qk.doc(project, id ?? ""),
    enabled: !!id,
    queryFn: () => apiFetch<Document>(`/api/documents/${encodeURIComponent(id!)}?project=${encodeURIComponent(project)}`),
  });
}
