import { useParams } from "react-router-dom";
import { useDoc } from "@/hooks/api/useDocs";
import { useProjectStore } from "@/stores/project";

export default function DocumentView() {
  const { id } = useParams();
  const project = useProjectStore((s) => s.slug);
  const { data, isLoading } = useDoc(project, id);
  if (isLoading) return <div className="p-8 text-sm text-muted-foreground">Loading…</div>;
  if (!data) return <div className="p-8 text-sm">Not found.</div>;
  return (
    <article className="doc-view">
      <h1 className="doc-view-title">{data.title || data.path}</h1>
      <div className="doc-view-meta">
        {data.doc_type} · v{data.version}
      </div>
    </article>
  );
}
