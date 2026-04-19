import { useEffect, useMemo, useRef } from "react";
import { Graph as Cosmos, type GraphConfigInterface } from "@cosmograph/cosmos";
import type { GraphData, GraphNode } from "@/hooks/api/useGraph";

interface CosmosNode {
  id: string;
  label: string;
  kind: GraphNode["kind"];
  color?: string;
}

interface CosmosLink {
  source: string;
  target: string;
}

function readTokens() {
  const style = getComputedStyle(document.documentElement);
  const read = (name: string) => style.getPropertyValue(name).trim();
  return {
    background: read("--background") || "#0f1115",
    primary: read("--primary") || "#3ecf8e",
    border: read("--border") || "#262a33",
    chart1: read("--chart-1") || "#3ecf8e",
    chart2: read("--chart-2") || "#6ba6ff",
    chart3: read("--chart-3") || "#b08fe8",
  };
}

export function GraphCanvas({ data }: { data: GraphData }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const graphRef = useRef<Cosmos<CosmosNode, CosmosLink> | null>(null);

  const { nodes, links } = useMemo(() => {
    const tokens = typeof document === "undefined" ? null : readTokens();
    const kindColor: Record<GraphNode["kind"], string> = tokens
      ? { entity: tokens.chart1, note: tokens.chart3, community: tokens.chart2 }
      : { entity: "#3ecf8e", note: "#b08fe8", community: "#6ba6ff" };
    const trimmedNodes: CosmosNode[] = data.nodes.map((n) => ({
      id: n.id,
      label: n.label,
      kind: n.kind,
      color: kindColor[n.kind] ?? kindColor.entity,
    }));
    const ids = new Set(trimmedNodes.map((n) => n.id));
    const trimmedLinks: CosmosLink[] = data.edges
      .filter((e) => ids.has(e.source) && ids.has(e.target))
      .map((e) => ({ source: e.source, target: e.target }));
    return { nodes: trimmedNodes, links: trimmedLinks };
  }, [data]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const tokens = readTokens();

    // cosmograph uses regl (WebGL2). WebGPU support is slated for cosmos v2;
    // until then WebGL2 is the universal path — every WebGPU-capable browser
    // also exposes WebGL2.
    const config: GraphConfigInterface<CosmosNode, CosmosLink> = {
      backgroundColor: tokens.background,
      nodeColor: (n) => n.color ?? tokens.chart1,
      nodeSize: 4,
      nodeGreyoutOpacity: 0.15,
      linkColor: tokens.border,
      linkWidth: 0.6,
      linkGreyoutOpacity: 0.05,
      linkArrows: false,
      simulation: {
        decay: 1500,
        gravity: 0.2,
        repulsion: 0.4,
        linkDistance: 8,
        linkSpring: 1,
      },
      showFPSMonitor: false,
      pixelRatio: window.devicePixelRatio || 1,
    };

    const graph = new Cosmos(canvas, config);
    graphRef.current = graph;
    graph.setData(nodes, links);
    graph.fitView();

    return () => {
      graph.destroy();
      graphRef.current = null;
    };
  }, [nodes, links]);

  return <canvas ref={canvasRef} className="graph-canvas" />;
}
