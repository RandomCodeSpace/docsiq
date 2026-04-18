import { useParams } from "react-router-dom";
import { MarkdownView } from "@/components/notes/MarkdownView";
import { useNote } from "@/hooks/api/useNotes";
import { useProjectStore } from "@/stores/project";
import { formatRelativeTime } from "@/lib/format";

export default function NoteView() {
  const { key } = useParams();
  const project = useProjectStore((s) => s.slug);
  const { data: note, isLoading, error } = useNote(project, key);

  if (isLoading) {
    return <div className="p-8 text-[var(--color-text-muted)] text-sm">Loading…</div>;
  }
  if (error || !note) {
    return (
      <div className="p-8 max-w-[620px] mx-auto">
        <h1 className="text-xl font-semibold">Note not found</h1>
        <p className="text-sm text-[var(--color-text-muted)] mt-2 font-mono">{key}</p>
      </div>
    );
  }

  return (
    <article className="p-8 max-w-[620px] mx-auto">
      <header className="mb-6">
        <h1 className="text-2xl font-semibold">{note.key.split("/").pop()}</h1>
        <div className="text-xs font-mono text-[var(--color-text-muted)] mt-1">
          {note.key} · updated {formatRelativeTime(new Date(note.updated_at).getTime())}
          {note.author && ` · by ${note.author}`}
        </div>
      </header>
      <MarkdownView source={note.content} />
    </article>
  );
}
