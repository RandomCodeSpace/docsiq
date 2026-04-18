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
    return <div className="p-8 text-muted-foreground text-sm">Loading…</div>;
  }
  if (error || !note) {
    return (
      <div className="p-8 max-w-[620px] mx-auto">
        <h1 className="text-xl font-semibold">Note not found</h1>
        <p className="text-sm text-muted-foreground mt-2 font-mono">{key}</p>
      </div>
    );
  }

  return (
    <article className="doc-view">
      <header>
        <h1 className="doc-view-title">{note.key.split("/").pop()}</h1>
        <div className="doc-view-meta">
          {note.key} · updated {formatRelativeTime(new Date(note.updated_at).getTime())}
          {note.author && ` · by ${note.author}`}
        </div>
      </header>
      <div className="prose-body">
        <MarkdownView source={note.content} />
      </div>
    </article>
  );
}
