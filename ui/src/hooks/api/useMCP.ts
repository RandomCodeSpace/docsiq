import { useState, useCallback } from "react";
import { apiFetch } from "@/lib/api-client";

export interface MCPCallRecord {
  id: string;
  tool: string;
  args: unknown;
  result?: unknown;
  error?: string;
  tookMs: number;
  timestamp: number;
}

export function useMCP() {
  const [history, setHistory] = useState<MCPCallRecord[]>([]);

  const invoke = useCallback(async (tool: string, args: unknown) => {
    const started = performance.now();
    const rec: MCPCallRecord = { id: crypto.randomUUID(), tool, args, tookMs: 0, timestamp: Date.now() };
    try {
      const result = await apiFetch<unknown>("/mcp", {
        method: "POST",
        body: JSON.stringify({
          jsonrpc: "2.0",
          id: 1,
          method: "tools/call",
          params: { name: tool, arguments: args },
        }),
      });
      rec.result = result;
    } catch (e) {
      rec.error = (e as Error).message;
    } finally {
      rec.tookMs = Math.round(performance.now() - started);
      setHistory((h) => [rec, ...h].slice(0, 50));
    }
  }, []);

  return { history, invoke };
}
