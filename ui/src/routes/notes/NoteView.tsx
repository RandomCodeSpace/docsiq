import { useParams } from "react-router-dom";
import { MarkdownView } from "@/components/notes/MarkdownView";
import { useNote } from "@/hooks/api/useNotes";
import { useProjectStore } from "@/stores/project";
import { formatRelativeTime } from "@/lib/format";
import { EmptyState, ErrorState, LoadingSkeleton } from "@/components/empty";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";

export default function NoteView() {
  const { key } = useParams();
  const project = useProjectStore((s) => s.slug);
  const { data: note, isLoading, error, refetch } = useNote(project, key);
  const err = error as Error | null | undefined;

  const noteLabel =
    note?.key?.split("/").pop() ??
    (key ? decodeURIComponent(key).split("/").pop() : undefined);
  useDocumentTitle(noteLabel ? [noteLabel, "Notes"] : undefined);

  if (isLoading) {
    return (
      <div className="p-8 max-w-[620px] mx-auto">
        <LoadingSkeleton label="Loading note" rows={5} />
      </div>
    );
  }
  if (err) {
    return (
      <div className="p-8 max-w-[620px] mx-auto">
        <ErrorState
          title="Note failed to load"
          message={err.message || "Unknown error"}
          onRetry={() => refetch()}
        />
      </div>
    );
  }
  if (!note) {
    return (
      <div className="p-8 max-w-[620px] mx-auto">
        <EmptyState
          title="Note not found"
          description="The note may have been deleted or the link is stale."
        />
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
