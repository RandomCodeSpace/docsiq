import { GraphCanvas } from "@/components/graph/GraphCanvas";
import { useNotesGraph } from "@/hooks/api/useGraph";
import { useProjectStore } from "@/stores/project";

export default function Graph() {
  const project = useProjectStore((s) => s.slug);
  const { data, isLoading } = useNotesGraph(project);
  if (isLoading) return <div className="p-8 text-sm text-muted-foreground">Loading graph…</div>;
  if (!data || data.nodes.length === 0) {
    return <div className="p-8 text-sm text-muted-foreground">No graph data for this project.</div>;
  }
  return (
    <div className="graph-page">
      <GraphCanvas data={data} />
    </div>
  );
}
