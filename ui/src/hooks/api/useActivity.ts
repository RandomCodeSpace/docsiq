import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { qk } from "./keys";
import type { Note, Document } from "@/types/api";

export type ActivityEventKind = "note_added" | "note_updated" | "doc_indexed" | "doc_error";

export interface ActivityEvent {
  id: string;
  kind: ActivityEventKind;
  title: string;
  detail?: string;
  timestamp: number;
  href: string;
}

export function useActivity(project: string) {
  return useQuery({
    queryKey: qk.activity(project),
    queryFn: async () => {
      const [notesRaw, docsRaw] = await Promise.all([
        apiFetch<{ keys?: string[] } | Note[] | null>(
          `/api/projects/${encodeURIComponent(project)}/notes`,
        ).catch(() => null),
        apiFetch<Document[] | null>(
          `/api/documents?project=${encodeURIComponent(project)}`,
        ).catch(() => null),
      ]);
      const notes: Note[] = Array.isArray(notesRaw)
        ? notesRaw
        : (notesRaw?.keys ?? []).map((key) => ({
            key,
            content: "",
            tags: [] as string[],
            created_at: "",
            updated_at: "",
          }));
      const docs: Document[] = Array.isArray(docsRaw) ? docsRaw : [];
      const events: ActivityEvent[] = [];
      for (const n of notes) {
        const ts = new Date(n.updated_at).getTime();
        if (!Number.isFinite(ts)) continue;
        const isNew = ts === new Date(n.created_at).getTime();
        events.push({
          id: `note-${n.key}-${ts}`,
          kind: isNew ? "note_added" : "note_updated",
          title: n.key,
          timestamp: ts,
          href: `/notes/${n.key}`,
        });
      }
      for (const d of docs) {
        const ts = Number(d.updated_at) * 1000;
        if (!Number.isFinite(ts)) continue;
        events.push({
          id: `doc-${d.id}-${d.updated_at}`,
          kind: "doc_indexed",
          title: d.title || d.path,
          detail: d.doc_type,
          timestamp: ts,
          href: `/docs/${d.id}`,
        });
      }
      events.sort((a, b) => b.timestamp - a.timestamp);
      return events.slice(0, 20);
    },
    refetchInterval: 10_000,
  });
}
