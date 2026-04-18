import { useParams } from "react-router-dom";
import { useDoc } from "@/hooks/api/useDocs";
import { useProjectStore } from "@/stores/project";

export default function DocumentView() {
  const { id } = useParams();
  const project = useProjectStore((s) => s.slug);
  const { data, isLoading } = useDoc(project, id);
  if (isLoading) return <div className="p-8 text-sm text-[var(--color-text-muted)]">Loading…</div>;
  if (!data) return <div className="p-8 text-sm">Not found.</div>;
  return (
    <article className="p-8 max-w-[720px] mx-auto">
      <h1 className="text-xl font-semibold">{data.title || data.path}</h1>
      <div className="mt-2 font-mono text-xs text-[var(--color-text-muted)]">
        {data.doc_type} · v{data.version}
      </div>
    </article>
  );
}
