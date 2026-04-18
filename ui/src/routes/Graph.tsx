import { GraphCanvas } from "@/components/graph/GraphCanvas";
import { useNotesGraph } from "@/hooks/api/useGraph";
import { useProjectStore } from "@/stores/project";

export default function Graph() {
  const project = useProjectStore((s) => s.slug);
  const { data, isLoading } = useNotesGraph(project);
  if (isLoading) return <div className="p-8 text-sm text-[var(--color-text-muted)]">Loading graph…</div>;
  if (!data) return <div className="p-8 text-sm">No graph data.</div>;
  return (
    <div className="h-[calc(100vh-44px)] p-4">
      <GraphCanvas data={data} />
    </div>
  );
}
