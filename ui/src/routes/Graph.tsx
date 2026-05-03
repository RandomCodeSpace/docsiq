import { useState } from "react";
import { GraphCanvas } from "@/components/graph/GraphCanvas";
import { useEntityGraph, useNotesGraph } from "@/hooks/api/useGraph";
import { useProjectStore } from "@/stores/project";
import { EmptyState, ErrorState, LoadingSkeleton } from "@/components/empty";

type View = "entity" | "notes";

export default function Graph() {
  const project = useProjectStore((s) => s.slug);
  const entity = useEntityGraph(project);
  const notes = useNotesGraph(project);

  // Default view: entity graph if it has nodes, else notes graph. Honour
  // an explicit user toggle once made.
  const [override, setOverride] = useState<View | null>(null);
  const entityHasNodes = (entity.data?.nodes.length ?? 0) > 0;
  const view: View = override ?? (entityHasNodes ? "entity" : "notes");
  const active = view === "entity" ? entity : notes;
  const data = active.data;
  const err = active.error as Error | null | undefined;

  const Toggle = () => (
    <div className="graph-toggle flex gap-2 p-3 border-b text-sm">
      <button
        type="button"
        onClick={() => setOverride("entity")}
        aria-pressed={view === "entity"}
        className={`px-3 py-1 rounded ${view === "entity" ? "bg-foreground text-background" : "hover:bg-muted"}`}
      >
        Entity graph
        {entity.data && ` · ${entity.data.nodes.length}`}
      </button>
      <button
        type="button"
        onClick={() => setOverride("notes")}
        aria-pressed={view === "notes"}
        className={`px-3 py-1 rounded ${view === "notes" ? "bg-foreground text-background" : "hover:bg-muted"}`}
      >
        Notes graph
        {notes.data && ` · ${notes.data.nodes.length}`}
      </button>
    </div>
  );

  if (active.isLoading) {
    return (
      <div className="graph-page">
        <Toggle />
        <div className="p-8">
          <LoadingSkeleton label="Loading graph" rows={4} />
        </div>
      </div>
    );
  }
  if (err) {
    return (
      <div className="graph-page">
        <Toggle />
        <div className="p-8">
          <ErrorState
            title="Graph failed to load"
            message={err.message || "Unknown error"}
            onRetry={() => active.refetch()}
          />
        </div>
      </div>
    );
  }
  if (!data || data.nodes.length === 0) {
    return (
      <div className="graph-page">
        <Toggle />
        <div className="p-8">
          <EmptyState
            title={view === "entity" ? "No entity graph yet" : "No notes graph yet"}
            description={
              view === "entity"
                ? "Run `docsiq index <path>` followed by `docsiq index --finalize` to extract entities and relationships."
                : "Add markdown notes with [[wikilinks]] under this project to build the notes graph."
            }
          />
        </div>
      </div>
    );
  }
  return (
    <div className="graph-page">
      <Toggle />
      <GraphCanvas data={data} />
    </div>
  );
}
