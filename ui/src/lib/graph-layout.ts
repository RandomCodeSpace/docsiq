import { forceSimulation, forceManyBody, forceLink, forceCenter, forceX, forceY, type Simulation } from "d3-force";
import type { GraphData } from "@/hooks/api/useGraph";

export interface LaidOutNode { id: string; label: string; kind: string; x: number; y: number; }
export interface LaidOutEdge { source: string; target: string; }

export function layoutGraph(
  data: GraphData,
  width: number,
  height: number,
  ticks = 300,
): { nodes: LaidOutNode[]; edges: LaidOutEdge[] } {
  const nodes: LaidOutNode[] = data.nodes.map((n) => ({ id: n.id, label: n.label, kind: n.kind, x: 0, y: 0 }));
  const links = data.edges.map((e) => ({ source: e.source, target: e.target }));

  // Detect orphans (no incident edges). Anchor them harder to the centre —
  // otherwise forceManyBody pushes them infinitely away with nothing to pull
  // them back.
  const degree = new Map<string, number>();
  for (const l of links) {
    degree.set(l.source, (degree.get(l.source) ?? 0) + 1);
    degree.set(l.target, (degree.get(l.target) ?? 0) + 1);
  }
  const orphanStrength = (n: any) => ((degree.get(n.id) ?? 0) === 0 ? 0.25 : 0.06);

  const sim: Simulation<LaidOutNode, LaidOutEdge> = forceSimulation(nodes)
    .force("charge", forceManyBody().strength(-45).distanceMax(260))
    .force("center", forceCenter(width / 2, height / 2))
    .force("link", forceLink(links).id((d: any) => d.id).distance(42).strength(0.7))
    .force("x", forceX(width / 2).strength(orphanStrength as any))
    .force("y", forceY(height / 2).strength(orphanStrength as any))
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
