import { useMemo } from "react";
import type { GraphData } from "@/hooks/api/useGraph";

interface Props { data: GraphData | undefined; maxNodes?: number; }

const COLOR: Record<string, string> = {
  entity: "var(--color-semantic-new)",
  note: "var(--color-semantic-graph)",
  community: "var(--color-semantic-index)",
};

export function GlanceView({ data, maxNodes = 30 }: Props) {
  const layout = useMemo(() => {
    if (!data) return null;
    const nodes = data.nodes.slice(0, maxNodes);
    const n = nodes.length;
    const radius = 60;
    const placed = nodes.map((node, i) => ({
      node,
      x: 110 + radius * Math.cos((2 * Math.PI * i) / n),
      y: 70 + radius * Math.sin((2 * Math.PI * i) / n),
    }));
    const idx: Record<string, { x: number; y: number }> = {};
    placed.forEach((p) => (idx[p.node.id] = { x: p.x, y: p.y }));
    const edges = data.edges.filter((e) => idx[e.source] && idx[e.target]).slice(0, 60);
    return { placed, edges, idx };
  }, [data, maxNodes]);

  if (!data || !layout) {
    return (
      <div className="h-[140px] grid place-items-center text-xs text-[var(--color-text-muted)] font-mono">
        loading…
      </div>
    );
  }

  return (
    <svg viewBox="0 0 220 140" className="w-full h-auto" aria-label="Graph preview">
      {layout.edges.map((e, i) => (
        <line
          key={i}
          x1={layout.idx[e.source].x}
          y1={layout.idx[e.source].y}
          x2={layout.idx[e.target].x}
          y2={layout.idx[e.target].y}
          stroke="var(--color-border-strong)"
          strokeWidth={0.6}
        />
      ))}
      {layout.placed.map((p) => (
        <circle
          key={p.node.id}
          cx={p.x}
          cy={p.y}
          r={4}
          fill={COLOR[p.node.kind] ?? COLOR.entity}
        />
      ))}
    </svg>
  );
}
