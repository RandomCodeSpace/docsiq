import { useParams } from "react-router-dom";
import { useDoc } from "@/hooks/api/useDocs";
import { useProjectStore } from "@/stores/project";
import { EmptyState, ErrorState, LoadingSkeleton } from "@/components/empty";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";

export default function DocumentView() {
  const { id } = useParams();
  const project = useProjectStore((s) => s.slug);
  const { data, isLoading, error, refetch } = useDoc(project, id);
  const err = error as Error | null | undefined;

  const docLabel = data?.title || data?.path;
  useDocumentTitle(docLabel ? [docLabel, "Documents"] : undefined);

  if (isLoading) {
    return (
      <div className="p-8 max-w-[620px] mx-auto">
        <LoadingSkeleton label="Loading document" rows={5} />
      </div>
    );
  }
  if (err) {
    return (
      <div className="p-8 max-w-[620px] mx-auto">
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
      <div className="p-8 max-w-[620px] mx-auto">
        <EmptyState
          title="Document not found"
          description="The document may have been deleted."
        />
      </div>
    );
  }
  return (
    <article className="doc-view">
      <h1 className="doc-view-title">{data.title || data.path}</h1>
      <div className="doc-view-meta">
        {data.doc_type} · v{data.version}
      </div>
    </article>
  );
}
