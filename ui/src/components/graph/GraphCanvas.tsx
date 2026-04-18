import { useMemo } from "react";
import { layoutGraph } from "@/lib/graph-layout";
import type { GraphData } from "@/hooks/api/useGraph";

const COLOR: Record<string, string> = {
  entity: "var(--color-semantic-new)",
  note: "var(--color-semantic-graph)",
  community: "var(--color-semantic-index)",
};

export function GraphCanvas({ data, width = 1200, height = 700 }: { data: GraphData; width?: number; height?: number }) {
  const laid = useMemo(() => layoutGraph(data, width, height), [data, width, height]);
  const idx = useMemo(() => Object.fromEntries(laid.nodes.map((n) => [n.id, n])), [laid.nodes]);
  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="w-full h-full" role="img" aria-label="Entity + notes graph">
      {laid.edges.map((e, i) => {
        const a = idx[e.source]; const b = idx[e.target];
        if (!a || !b) return null;
        return <line key={i} x1={a.x} y1={a.y} x2={b.x} y2={b.y} stroke="var(--color-border-strong)" strokeWidth={0.7} />;
      })}
      {laid.nodes.map((n) => (
        <g key={n.id}>
          <circle cx={n.x} cy={n.y} r={4.5} fill={COLOR[n.kind] ?? COLOR.entity} />
          <title>{n.label || n.id}</title>
        </g>
      ))}
    </svg>
  );
}
