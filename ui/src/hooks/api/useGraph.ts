import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { qk } from "./keys";

export interface GraphNode { id: string; label: string; kind: "entity" | "note" | "community"; }
export interface GraphEdge { source: string; target: string; }
export interface GraphData { nodes: GraphNode[]; edges: GraphEdge[]; }

export function useNotesGraph(project: string) {
  return useQuery({
    queryKey: qk.notesGraph(project),
    queryFn: () =>
      apiFetch<GraphData>(`/api/projects/${encodeURIComponent(project)}/graph`),
  });
}
