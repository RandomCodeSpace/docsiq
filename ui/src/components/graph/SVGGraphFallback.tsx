import { useEffect, useMemo, useRef, useState } from "react";
import type { GraphData } from "@/hooks/api/useGraph";
import { layoutGraph } from "@/lib/graph-layout";

interface Props { data: GraphData; reason: "no-webgl2" | "init-failed" }

const COLOR: Record<string, string> = {
  entity: "var(--chart-1)",
  note: "var(--chart-3)",
  community: "var(--chart-2)",
};

export function SVGGraphFallback({ data, reason }: Props) {
  const wrapRef = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ w: 1200, h: 700 });

  useEffect(() => {
    const wrap = wrapRef.current;
    if (!wrap) return;
    const measure = () => {
      const r = wrap.getBoundingClientRect();
      setSize({ w: Math.max(320, Math.floor(r.width)), h: Math.max(240, Math.floor(r.height)) });
    };
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(wrap);
    return () => ro.disconnect();
  }, []);

  const laid = useMemo(() => layoutGraph(data, size.w, size.h, 180), [data, size]);
  const idx = useMemo(
    () => Object.fromEntries(laid.nodes.map((n) => [n.id, n])),
    [laid.nodes],
  );

  const reasonText =
    reason === "no-webgl2"
      ? "WebGL2 unavailable — rendering with SVG."
      : "WebGL2 renderer failed — rendering with SVG.";

  return (
    <div ref={wrapRef} className="graph-canvas-wrap">
      <div className="graph-fallback-banner">{reasonText}</div>
      <svg
        viewBox={`0 0 ${size.w} ${size.h}`}
        className="graph-canvas"
        role="img"
        aria-label="Knowledge graph (SVG fallback)"
      >
        {laid.edges.map((e, i) => {
          const a = idx[e.source];
          const b = idx[e.target];
          if (!a || !b) return null;
          return (
            <line
              key={i}
              x1={a.x}
              y1={a.y}
              x2={b.x}
              y2={b.y}
              stroke="var(--border)"
              strokeWidth={0.6}
              opacity={0.6}
            />
          );
        })}
        {laid.nodes.map((n) => (
          <g key={n.id}>
            <circle cx={n.x} cy={n.y} r={3.5} fill={COLOR[n.kind] ?? COLOR.entity} />
            <title>{n.label || n.id}</title>
          </g>
        ))}
      </svg>
    </div>
  );
}
