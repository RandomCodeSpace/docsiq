import { useMemo } from "react";
import { useParams } from "react-router-dom";
import MarkdownIt from "markdown-it";
import { useDoc, useDocChunks } from "@/hooks/api/useDocs";
import { useProjectStore } from "@/stores/project";
import { EmptyState, ErrorState, LoadingSkeleton } from "@/components/empty";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";

const md = new MarkdownIt({ html: false, linkify: true, breaks: false });

export default function DocumentView() {
  const { id } = useParams();
  const project = useProjectStore((s) => s.slug);
  const { data, isLoading, error, refetch } = useDoc(project, id);
  const { data: chunks, isLoading: chunksLoading } = useDocChunks(project, id);
  const err = error as Error | null | undefined;

  const docLabel = data?.title || data?.path;
  useDocumentTitle(docLabel ? [docLabel, "Documents"] : undefined);

  const orderedChunks = useMemo(
    () => (chunks ? [...chunks].sort((a, b) => a.chunk_index - b.chunk_index) : []),
    [chunks],
  );

  const renderedHTML = useMemo(() => {
    if (orderedChunks.length === 0) return "";
    const text = orderedChunks.map((c) => c.content).join("\n\n");
    const isMarkdown = data?.doc_type === "md" || data?.doc_type === "markdown";
    return isMarkdown ? md.render(text) : "";
  }, [orderedChunks, data?.doc_type]);

  if (isLoading) {
    return (
      <div className="p-8 max-w-[760px] mx-auto">
        <LoadingSkeleton label="Loading document" rows={5} />
      </div>
    );
  }
  if (err) {
    return (
      <div className="p-8 max-w-[760px] mx-auto">
        <ErrorState
          title="Document failed to load"
          message={err.message || "Unknown error"}
          onRetry={() => refetch()}
        />
      </div>
    );
  }
  if (!data) {
    return (
      <div className="p-8 max-w-[760px] mx-auto">
        <EmptyState
          title="Document not found"
          description="The document may have been deleted."
        />
      </div>
    );
  }

  return (
    <article className="doc-view p-8 max-w-[760px] mx-auto">
      <header className="doc-view-header mb-6">
        <h1 className="doc-view-title text-2xl font-semibold">{data.title || data.path}</h1>
        <div className="doc-view-meta text-sm opacity-70 mt-1">
          {data.doc_type} · v{data.version}
          {orderedChunks.length > 0 && ` · ${orderedChunks.length} chunk${orderedChunks.length === 1 ? "" : "s"}`}
        </div>
      </header>

      {chunksLoading ? (
        <LoadingSkeleton label="Loading content" rows={6} />
      ) : orderedChunks.length === 0 ? (
        <EmptyState
          title="No content available"
          description="This document has no indexed chunks yet — try re-running `docsiq index` for this path."
        />
      ) : renderedHTML ? (
        <div
          className="doc-view-body prose dark:prose-invert max-w-none"
          dangerouslySetInnerHTML={{ __html: renderedHTML }}
        />
      ) : (
        <pre className="doc-view-body whitespace-pre-wrap text-sm leading-relaxed">
          {orderedChunks.map((c) => c.content).join("\n\n")}
        </pre>
      )}
    </article>
  );
}
