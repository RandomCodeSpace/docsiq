import { useState, useCallback, useRef, useEffect } from "react";

export interface MCPCallRecord {
  id: string;
  tool: string;
  args: unknown;
  result?: unknown;
  error?: string;
  tookMs: number;
  timestamp: number;
}

export interface MCPTool {
  name: string;
  description?: string;
  inputSchema?: {
    type?: string;
    properties?: Record<string, { type?: string; description?: string; enum?: unknown[] }>;
    required?: string[];
  };
}

async function rpc(
  sessionId: string | null,
  body: unknown,
): Promise<{ json: unknown; sessionId: string | null }> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "Accept": "application/json, text/event-stream",
  };
  if (sessionId) headers["Mcp-Session-Id"] = sessionId;

  const res = await fetch("/mcp", {
    method: "POST",
    credentials: "include",
    headers,
    body: JSON.stringify(body),
  });
  const newSession = res.headers.get("Mcp-Session-Id") ?? sessionId;

  const text = await res.text();
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${text.slice(0, 200)}`);
  if (!text) return { json: null, sessionId: newSession };

  const ct = res.headers.get("content-type") ?? "";
  if (ct.includes("text/event-stream")) {
    for (const line of text.split(/\r?\n/)) {
      if (line.startsWith("data: ")) {
        return { json: JSON.parse(line.slice(6)), sessionId: newSession };
      }
    }
    throw new Error("MCP SSE response had no data frame");
  }
  return { json: JSON.parse(text), sessionId: newSession };
}

export function useMCP() {
  const [history, setHistory] = useState<MCPCallRecord[]>([]);
  const [tools, setTools] = useState<MCPTool[]>([]);
  const [toolsError, setToolsError] = useState<string | null>(null);
  const sessionRef = useRef<string | null>(null);
  const initialisedRef = useRef(false);

  async function ensureInitialised() {
    if (initialisedRef.current) return;
    const { sessionId: sid1 } = await rpc(null, {
      jsonrpc: "2.0",
      id: 1,
      method: "initialize",
      params: {
        protocolVersion: "2025-03-26",
        capabilities: {},
        clientInfo: { name: "docsiq-ui", version: "0.1" },
      },
    });
    sessionRef.current = sid1;
    await rpc(sessionRef.current, { jsonrpc: "2.0", method: "notifications/initialized" });
    initialisedRef.current = true;
  }

  const refreshTools = useCallback(async () => {
    try {
      await ensureInitialised();
      const { json } = await rpc(sessionRef.current, {
        jsonrpc: "2.0",
        id: Date.now(),
        method: "tools/list",
      });
      const r = json as { result?: { tools?: MCPTool[] } };
      setTools(r?.result?.tools ?? []);
      setToolsError(null);
    } catch (e) {
      setToolsError((e as Error).message);
      initialisedRef.current = false;
      sessionRef.current = null;
    }
  }, []);

  useEffect(() => { void refreshTools(); }, [refreshTools]);

  const invoke = useCallback(async (tool: string, args: unknown) => {
    const started = performance.now();
    const rec: MCPCallRecord = {
      id: crypto.randomUUID(),
      tool,
      args,
      tookMs: 0,
      timestamp: Date.now(),
    };
    try {
      await ensureInitialised();
      const { json } = await rpc(sessionRef.current, {
        jsonrpc: "2.0",
        id: Date.now(),
        method: "tools/call",
        params: { name: tool, arguments: args },
      });
      const r = json as { error?: { message: string }; result?: unknown };
      if (r?.error) throw new Error(r.error.message);
      rec.result = r?.result;
    } catch (e) {
      rec.error = (e as Error).message;
      initialisedRef.current = false;
      sessionRef.current = null;
    } finally {
      rec.tookMs = Math.round(performance.now() - started);
      setHistory((h) => [rec, ...h].slice(0, 50));
    }
  }, []);

  return { history, invoke, tools, toolsError, refreshTools };
}

export function templateForTool(tool: MCPTool | undefined): string {
  if (!tool?.inputSchema?.properties) return "{}";
  const props = tool.inputSchema.properties;
  const required = new Set(tool.inputSchema.required ?? []);
  const keys = Object.keys(props);
  if (keys.length === 0) return "{}";
  const obj: Record<string, unknown> = {};
  for (const k of keys) {
    if (required.size && !required.has(k)) continue;
    const t = props[k].type;
    obj[k] = t === "number" || t === "integer" ? 0
      : t === "boolean" ? false
      : t === "array" ? []
      : t === "object" ? {}
      : "";
  }
  return JSON.stringify(obj, null, 2);
}
