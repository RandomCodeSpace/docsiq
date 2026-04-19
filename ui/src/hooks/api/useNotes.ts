import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { qk } from "./keys";
import type { Note } from "@/types/api";

export function useNotes(project: string) {
  return useQuery({
    queryKey: qk.notes(project),
    queryFn: async (): Promise<Note[]> => {
      const res = await apiFetch<{ keys?: string[] } | Note[] | null>(
        `/api/projects/${encodeURIComponent(project)}/notes`,
      );
      if (Array.isArray(res)) return res;
      const keys = res?.keys ?? [];
      return keys.map((key) => ({
        key,
        content: "",
        tags: [],
        created_at: "",
        updated_at: "",
      }));
    },
  });
}

export function useNote(project: string, key: string | undefined) {
  return useQuery({
    queryKey: qk.note(project, key ?? ""),
    enabled: !!key,
    queryFn: async (): Promise<Note> => {
      const res = await apiFetch<Note | { note: Note }>(
        `/api/projects/${encodeURIComponent(project)}/notes/${encodeURIComponent(key!)}`,
      );
      return "note" in res && res.note ? (res as { note: Note }).note : (res as Note);
    },
  });
}

export function useWriteNote(project: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { key: string; content: string; author?: string; tags?: string[] }) =>
      apiFetch<Note>(
        `/api/projects/${encodeURIComponent(project)}/notes/${encodeURIComponent(input.key)}`,
        { method: "PUT", body: JSON.stringify(input) },
      ),
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: qk.notes(project) });
      qc.invalidateQueries({ queryKey: qk.note(project, v.key) });
      qc.invalidateQueries({ queryKey: qk.notesGraph(project) });
      qc.invalidateQueries({ queryKey: qk.activity(project) });
    },
  });
}

export function useDeleteNote(project: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (key: string) =>
      apiFetch(
        `/api/projects/${encodeURIComponent(project)}/notes/${encodeURIComponent(key)}`,
        { method: "DELETE" },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.notes(project) });
      qc.invalidateQueries({ queryKey: qk.notesGraph(project) });
      qc.invalidateQueries({ queryKey: qk.activity(project) });
    },
  });
}
