import { useState, useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { apiFetch } from "@/lib/api-client";
import { qk } from "@/hooks/api/keys";
import { useProjectStore } from "@/stores/project";
import type { NoteHit } from "@/types/api";

export default function NotesSearch() {
  const project = useProjectStore((s) => s.slug);
  const [q, setQ] = useState("");
  const [debounced, setDebounced] = useState("");

  useEffect(() => {
    const t = setTimeout(() => setDebounced(q.trim()), 300);
    return () => clearTimeout(t);
  }, [q]);

  const { data, isFetching } = useQuery({
    queryKey: qk.notesSearch(project, debounced),
    enabled: debounced.length > 0,
    queryFn: () =>
      apiFetch<{ hits: NoteHit[] }>(
        `/api/projects/${encodeURIComponent(project)}/search?q=${encodeURIComponent(debounced)}`,
      ),
  });

  return (
    <div className="p-8 max-w-[720px] mx-auto">
      <input
        autoFocus
        value={q}
        onChange={(e) => setQ(e.target.value)}
        placeholder="Search notes…"
        className="w-full px-4 py-3 bg-[var(--color-surface-1)] border border-[var(--color-border-strong)] rounded-md text-sm"
        aria-label="Search notes"
      />
      {isFetching && <p className="text-xs text-[var(--color-text-muted)] mt-2">searching…</p>}
      <ul className="mt-6 space-y-1.5">
        {data?.hits.map((h) => (
          <li key={h.key}>
            <Link
              to={`/notes/${encodeURIComponent(h.key)}`}
              className="block p-3 border border-[var(--color-border)] rounded-md hover:bg-[var(--color-surface-2)]"
            >
              <div className="text-sm font-mono">{h.key}</div>
              <div
                className="text-xs text-[var(--color-text-muted)] mt-1"
                dangerouslySetInnerHTML={{ __html: h.snippet }}
              />
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
