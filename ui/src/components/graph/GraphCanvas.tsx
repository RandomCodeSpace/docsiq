import { useEffect, useMemo, useRef, useState } from "react";
import { zoom as d3zoom, type ZoomTransform, zoomIdentity } from "d3-zoom";
import { select } from "d3-selection";
import type { GraphData } from "@/hooks/api/useGraph";
import { layoutGraph } from "@/lib/graph-layout";

const COLOR: Record<string, string> = {
  entity: "var(--chart-1)",
  note: "var(--chart-3)",
  community: "var(--chart-2)",
};

export function GraphCanvas({ data }: { data: GraphData }) {
  const wrapRef = useRef<HTMLDivElement>(null);
  const svgRef = useRef<SVGSVGElement>(null);
  const [size, setSize] = useState({ w: 1200, h: 700 });
  const [transform, setTransform] = useState<ZoomTransform>(zoomIdentity);
  const [hover, setHover] = useState<string | null>(null);
  const [pinned, setPinned] = useState<string | null>(null);

  useEffect(() => {
    const wrap = wrapRef.current;
    if (!wrap) return;
    const measure = () => {
      const r = wrap.getBoundingClientRect();
      setSize({
        w: Math.max(320, Math.floor(r.width)),
        h: Math.max(240, Math.floor(r.height)),
      });
    };
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(wrap);
    return () => ro.disconnect();
  }, []);

  const laid = useMemo(() => layoutGraph(data, size.w, size.h, 240), [data, size]);

  // viewBox: tight bounds of content, expanded to match the container aspect
  // ratio so the graph fills the visible area (no letterboxing).
  const { idx, viewBox } = useMemo(() => {
    const index = Object.fromEntries(laid.nodes.map((n) => [n.id, n]));
    if (laid.nodes.length === 0) {
      return { idx: index, viewBox: [0, 0, size.w, size.h] as const };
    }
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (const n of laid.nodes) {
      if (n.x < minX) minX = n.x;
      if (n.y < minY) minY = n.y;
      if (n.x > maxX) maxX = n.x;
      if (n.y > maxY) maxY = n.y;
    }
    const pad = 40;
    let x = minX - pad;
    let y = minY - pad;
    let w = (maxX - minX) + pad * 2;
    let h = (maxY - minY) + pad * 2;
    // Expand viewBox to match the container's aspect ratio so the graph fills
    // the viewport instead of being letterboxed by preserveAspectRatio="meet".
    const containerAspect = size.w / size.h;
    const contentAspect = w / h;
    if (containerAspect > contentAspect) {
      const newW = h * containerAspect;
      x -= (newW - w) / 2;
      w = newW;
    } else {
      const newH = w / containerAspect;
      y -= (newH - h) / 2;
      h = newH;
    }
    return { idx: index, viewBox: [x, y, w, h] as const };
  }, [laid, size]);

  // Adjacency for hover-highlight.
  const adjacency = useMemo(() => {
    const m = new Map<string, Set<string>>();
    for (const e of laid.edges) {
      if (!m.has(e.source)) m.set(e.source, new Set());
      if (!m.has(e.target)) m.set(e.target, new Set());
      m.get(e.source)!.add(e.target);
      m.get(e.target)!.add(e.source);
    }
    return m;
  }, [laid.edges]);

  // Node radius by degree centrality — hubs get bigger, orphans stay small.
  // sqrt scaling keeps extremes sane (a 100-edge hub doesn't eat the screen).
  const radiusOf = useMemo(() => {
    const base = 3.5;
    const scale = 1.6;
    return (id: string): number => {
      const deg = adjacency.get(id)?.size ?? 0;
      return base + Math.sqrt(deg) * scale;
    };
  }, [adjacency]);

  // Wire d3-zoom (pan + wheel zoom + pinch on touch).
  useEffect(() => {
    const svg = svgRef.current;
    if (!svg) return;
    const behavior = d3zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.1, 40])
      .filter((event: Event) => {
        // Allow panning and pinching even with a node under the pointer —
        // `setPointerCapture` isn't needed since d3-zoom handles it. Only
        // block the dblclick-zoom which is unwanted on touch.
        return event.type !== "dblclick";
      })
      .on("zoom", (event) => {
        setTransform(event.transform);
      });
    select(svg).call(behavior);
    return () => {
      select(svg).on(".zoom", null);
    };
  }, []);

  const active = pinned ?? hover;
  const neighbors = active ? adjacency.get(active) : null;
  // If the active node has no edges, don't dim the rest — otherwise the whole
  // graph goes opaque for no useful reason.
  const dimAll = !!active && !!neighbors && neighbors.size > 0;

  function isNodeDimmed(id: string): boolean {
    if (!dimAll) return false;
    if (id === active) return false;
    return !(neighbors?.has(id));
  }
  function isEdgeDimmed(source: string, target: string): boolean {
    if (!dimAll) return false;
    return source !== active && target !== active;
  }

  const resetZoom = () => {
    const svg = svgRef.current;
    if (!svg) return;
    const behavior = d3zoom<SVGSVGElement, unknown>().scaleExtent([0.1, 40]);
    select(svg).call(behavior.transform, zoomIdentity);
    setTransform(zoomIdentity);
  };

  return (
    <div ref={wrapRef} className="graph-canvas-wrap">
      <div className="graph-meta">
        {data.nodes.length} nodes · {data.edges.length} edges
        {active && <span className="graph-meta-sep"> · {idx[active]?.label ?? active}</span>}
      </div>
      <button className="graph-reset" onClick={resetZoom} aria-label="Reset view">
        reset
      </button>
      <svg
        ref={svgRef}
        viewBox={`${viewBox[0]} ${viewBox[1]} ${viewBox[2]} ${viewBox[3]}`}
        className="graph-canvas"
        role="img"
        aria-label="Knowledge graph"
        preserveAspectRatio="xMidYMid meet"
        onPointerDown={() => {
          if (pinned) setPinned(null);
        }}
      >
        <g transform={transform.toString()}>
          {laid.edges.map((e, i) => {
            const a = idx[e.source];
            const b = idx[e.target];
            if (!a || !b) return null;
            const dimmed = isEdgeDimmed(e.source, e.target);
            return (
              <line
                key={i}
                x1={a.x}
                y1={a.y}
                x2={b.x}
                y2={b.y}
                stroke={dimmed ? "var(--border)" : "var(--muted-foreground)"}
                strokeWidth={dimmed ? 0.6 : 1.2}
                strokeOpacity={dimmed ? 0.15 : 0.6}
                strokeLinecap="round"
                pointerEvents="none"
              />
            );
          })}
          {laid.nodes.map((n) => {
            const dimmed = isNodeDimmed(n.id);
            const isActive = active === n.id;
            const r = radiusOf(n.id) + (isActive ? 2 : 0);
            return (
              <g
                key={n.id}
                data-node-id={n.id}
                className="graph-node"
                style={{ opacity: dimmed ? 0.2 : 1, cursor: "pointer" }}
                onPointerEnter={() => setHover(n.id)}
                onPointerLeave={() => setHover(null)}
                onClick={(e) => {
                  e.stopPropagation();
                  setPinned(pinned === n.id ? null : n.id);
                }}
              >
                <circle
                  cx={n.x}
                  cy={n.y}
                  r={r + 3}
                  fill="var(--background)"
                />
                <circle
                  cx={n.x}
                  cy={n.y}
                  r={r}
                  fill={COLOR[n.kind] ?? COLOR.entity}
                  stroke={isActive ? "var(--foreground)" : "var(--background)"}
                  strokeWidth={isActive ? 1.5 : 0.8}
                />
                <text
                  x={n.x + r + 4}
                  y={n.y + 3}
                  fontSize={10}
                  fontFamily="var(--font-mono)"
                  fill="var(--foreground)"
                  stroke="var(--background)"
                  strokeWidth={3}
                  paintOrder="stroke fill"
                  style={{ pointerEvents: "none", userSelect: "none" }}
                >
                  {truncate(disambiguateLabel(n.id, n.label), 36)}
                </text>
              </g>
            );
          })}
        </g>
      </svg>
    </div>
  );
}

function truncate(s: string, max: number): string {
  return s.length > max ? s.slice(0, max - 1) + "…" : s;
}

// Disambiguate labels. Notes commonly share leaf names across projects
// (every project has an `overview`). Show `<scope>/<leaf>` so the user
// can tell them apart at a glance.
function disambiguateLabel(id: string, label: string): string {
  const parts = id.split("/").filter(Boolean);
  if (parts.length <= 1) return label || id;
  const leaf = parts[parts.length - 1];
  const scope = parts.length >= 3 ? parts[parts.length - 2] : parts[0];
  // Prefer the backend-provided label only if it's the leaf; otherwise use it as-is.
  const shownLeaf = label && label !== leaf ? label : leaf;
  return `${scope}/${shownLeaf}`;
}
