import { useState, useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { apiFetch } from "@/lib/api-client";
import { qk } from "@/hooks/api/keys";
import { useProjectStore } from "@/stores/project";
import type { NoteHit } from "@/types/api";
import { EmptyState, ErrorState, LoadingSkeleton } from "@/components/empty";

export default function NotesSearch() {
  const project = useProjectStore((s) => s.slug);
  const [q, setQ] = useState("");
  const [debounced, setDebounced] = useState("");

  useEffect(() => {
    const t = setTimeout(() => setDebounced(q.trim()), 300);
    return () => clearTimeout(t);
  }, [q]);

  const { data, isFetching, error, refetch } = useQuery({
    queryKey: qk.notesSearch(project, debounced),
    enabled: debounced.length > 0,
    queryFn: () =>
      apiFetch<{ hits: NoteHit[] }>(
        `/api/projects/${encodeURIComponent(project)}/search?q=${encodeURIComponent(debounced)}`,
      ),
  });
  const err = error as Error | null | undefined;

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
      <div className="mt-6">
        {debounced.length === 0 ? null : isFetching ? (
          <LoadingSkeleton label="Searching notes" rows={4} />
        ) : err ? (
          <ErrorState
            title="Search failed"
            message={err.message || "Unknown error"}
            onRetry={() => refetch()}
          />
        ) : (data?.hits.length ?? 0) === 0 ? (
          <EmptyState
            title="No results"
            description={`No notes match "${debounced}".`}
          />
        ) : (
          <ul className="space-y-1.5">
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
        )}
      </div>
    </div>
  );
}
