import { GraphCanvas } from "@/components/graph/GraphCanvas";
import { useNotesGraph } from "@/hooks/api/useGraph";
import { useProjectStore } from "@/stores/project";
import { EmptyState, ErrorState, LoadingSkeleton } from "@/components/empty";

export default function Graph() {
  const project = useProjectStore((s) => s.slug);
  const { data, isLoading, error, refetch } = useNotesGraph(project);
  const err = error as Error | null | undefined;

  if (isLoading) {
    return (
      <div className="graph-page p-8">
        <LoadingSkeleton label="Loading graph" rows={4} />
      </div>
    );
  }
  if (err) {
    return (
      <div className="graph-page p-8">
        <ErrorState
          title="Graph failed to load"
          message={err.message || "Unknown error"}
          onRetry={() => refetch()}
        />
      </div>
    );
  }
  if (!data || data.nodes.length === 0) {
    return (
      <div className="graph-page p-8">
        <EmptyState
          title="No graph data for this project"
          description="Ingest or index a document to build the graph."
        />
      </div>
    );
  }
  return (
    <div className="graph-page">
      <GraphCanvas data={data} />
    </div>
  );
}
