import { useMemo } from "react";
import type { GraphData } from "@/hooks/api/useGraph";
import { layoutGraph } from "@/lib/graph-layout";

interface Props { data: GraphData | undefined; maxNodes?: number; }

const COLOR: Record<string, string> = {
  entity: "var(--chart-1)",
  note: "var(--chart-3)",
  community: "var(--chart-2)",
};

const W = 220;
const H = 140;

export function GlanceView({ data, maxNodes = 60 }: Props) {
  const laid = useMemo(() => {
    if (!data) return null;
    const nodeIds = new Set(data.nodes.slice(0, maxNodes).map((n) => n.id));
    const trimmed: GraphData = {
      nodes: data.nodes.filter((n) => nodeIds.has(n.id)),
      edges: data.edges.filter((e) => nodeIds.has(e.source) && nodeIds.has(e.target)),
    };
    return layoutGraph(trimmed, W, H, 120);
  }, [data, maxNodes]);

  if (!data || !laid || laid.nodes.length === 0) {
    return (
      <div className="h-[140px] grid place-items-center text-xs text-muted-foreground font-mono">
        {data ? "no graph yet" : "loading…"}
      </div>
    );
  }

  const idx = Object.fromEntries(laid.nodes.map((n) => [n.id, n]));

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="w-full h-auto" aria-label="Graph preview">
      {laid.edges.map((e, i) => {
        const a = idx[e.source]; const b = idx[e.target];
        if (!a || !b) return null;
        return (
          <line
            key={i}
            x1={a.x}
            y1={a.y}
            x2={b.x}
            y2={b.y}
            stroke="var(--border)"
            strokeWidth={0.5}
            opacity={0.6}
          />
        );
      })}
      {laid.nodes.map((n) => (
        <circle
          key={n.id}
          cx={n.x}
          cy={n.y}
          r={2.5}
          fill={COLOR[n.kind] ?? COLOR.entity}
        />
      ))}
    </svg>
  );
}
