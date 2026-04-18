import { forceSimulation, forceManyBody, forceLink, forceCenter, forceX, forceY, type Simulation } from "d3-force";
import type { GraphData } from "@/hooks/api/useGraph";

export interface LaidOutNode { id: string; label: string; kind: string; x: number; y: number; }
export interface LaidOutEdge { source: string; target: string; }

export function layoutGraph(
  data: GraphData,
  width: number,
  height: number,
  ticks = 200,
): { nodes: LaidOutNode[]; edges: LaidOutEdge[] } {
  const nodes: LaidOutNode[] = data.nodes.map((n) => ({ id: n.id, label: n.label, kind: n.kind, x: 0, y: 0 }));
  const links = data.edges.map((e) => ({ source: e.source, target: e.target }));
  const sim: Simulation<LaidOutNode, LaidOutEdge> = forceSimulation(nodes)
    .force("charge", forceManyBody().strength(-60))
    .force("center", forceCenter(width / 2, height / 2))
    .force("link", forceLink(links).id((d: any) => d.id).distance(40).strength(0.6))
    .force("x", forceX(width / 2).strength(0.02))
    .force("y", forceY(height / 2).strength(0.02))
    .stop();
  for (let i = 0; i < ticks; i++) sim.tick();
  return {
    nodes,
    edges: links.map((l) => ({
      source: (l.source as any).id ?? l.source,
      target: (l.target as any).id ?? l.target,
    })),
  };
}
