import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { qk } from "./keys";
import type { Document } from "@/types/api";

export interface DocChunk {
  id: string;
  chunk_index: number;
  content: string;
  token_count: number;
}

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

export function useDocChunks(project: string, id: string | undefined) {
  return useQuery({
    queryKey: qk.docChunks(project, id ?? ""),
    enabled: !!id,
    queryFn: async () => {
      const res = await apiFetch<DocChunk[] | null>(
        `/api/documents/${encodeURIComponent(id!)}/chunks?project=${encodeURIComponent(project)}`,
      );
      return Array.isArray(res) ? res : [];
    },
  });
}

// useDeleteDoc returns a mutation that hard-deletes a document and
// cascades graph cleanup on the server. Invalidates the list, the
// per-doc cache, the stats strip, and the entity graph so every view
// that referenced the doc re-fetches.
export function useDeleteDoc(project: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(
        `/api/documents/${encodeURIComponent(id)}?project=${encodeURIComponent(project)}`,
        { method: "DELETE" },
      ),
    onSuccess: (_data, id) => {
      qc.invalidateQueries({ queryKey: qk.docs(project) });
      qc.invalidateQueries({ queryKey: qk.stats(project) });
      qc.invalidateQueries({ queryKey: qk.entityGraph(project) });
      qc.removeQueries({ queryKey: qk.doc(project, id) });
      qc.removeQueries({ queryKey: qk.docChunks(project, id) });
    },
  });
}
