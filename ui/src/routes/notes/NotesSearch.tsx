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
    <div className="notes-search">
      <input
        autoFocus
        value={q}
        onChange={(e) => setQ(e.target.value)}
        placeholder="Search notes…"
        className="notes-search-input"
        aria-label="Search notes"
      />
      {isFetching && <p className="text-xs text-muted-foreground mt-2">searching…</p>}
      <ul className="mt-6 space-y-1.5">
        {data?.hits.map((h) => (
          <li key={h.key}>
            <Link
              to={`/notes/${encodeURIComponent(h.key)}`}
              className="notes-search-hit"
            >
              <div className="notes-search-hit-title">{h.key}</div>
              <div
                className="notes-search-hit-snippet"
                dangerouslySetInnerHTML={{ __html: h.snippet }}
              />
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
