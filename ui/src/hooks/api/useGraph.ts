import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { qk } from "./keys";

export interface GraphNode { id: string; label: string; kind: "entity" | "note" | "community"; }
export interface GraphEdge { source: string; target: string; }
export interface GraphData { nodes: GraphNode[]; edges: GraphEdge[]; }

interface RawGraphNode { key?: string; id?: string; title?: string; label?: string; folder?: string; tags?: string[]; kind?: string; }
interface RawGraphEdge { source: string; target: string; }
interface RawGraphResponse { nodes?: RawGraphNode[]; edges?: RawGraphEdge[]; }

export function useNotesGraph(project: string) {
  return useQuery({
    queryKey: qk.notesGraph(project),
    queryFn: async (): Promise<GraphData> => {
      const res = await apiFetch<RawGraphResponse | null>(
        `/api/projects/${encodeURIComponent(project)}/graph`,
      );
      const rawNodes = res?.nodes ?? [];
      const rawEdges = res?.edges ?? [];
      const nodes: GraphNode[] = rawNodes.map((n) => ({
        id: n.id ?? n.key ?? "",
        label: n.label ?? n.title ?? n.key ?? "",
        kind: (n.kind as GraphNode["kind"]) ?? "note",
      }));
      const ids = new Set(nodes.map((n) => n.id));
      const edges: GraphEdge[] = rawEdges
        .filter((e) => ids.has(e.source) && ids.has(e.target))
        .map((e) => ({ source: e.source, target: e.target }));
      return { nodes, edges };
    },
  });
}
