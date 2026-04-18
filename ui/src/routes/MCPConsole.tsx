import { useMCP } from "@/hooks/api/useMCP";
import { useState } from "react";

const KNOWN_TOOLS = [
  "list_projects", "stats", "search_documents", "search_notes",
  "list_notes", "read_note", "write_note", "list_entities",
  "query_entity", "find_relationships", "get_graph_neighborhood",
  "get_entity_claims",
];

export default function MCPConsole() {
  const { history, invoke } = useMCP();
  const [tool, setTool] = useState("stats");
  const [args, setArgs] = useState("{}");
  const [err, setErr] = useState<string | null>(null);

  async function onRun() {
    setErr(null);
    let parsed: unknown;
    try { parsed = JSON.parse(args); } catch { setErr("Invalid JSON"); return; }
    await invoke(tool, parsed);
  }

  return (
    <div className="p-6 max-w-[1000px] mx-auto">
      <h1 className="text-xl font-semibold mb-4">MCP Console</h1>
      <div className="flex gap-2 mb-3">
        <select
          value={tool}
          onChange={(e) => setTool(e.currentTarget.value)}
          className="px-3 py-2 bg-[var(--color-surface-1)] border border-[var(--color-border)] rounded-md text-sm font-mono"
        >
          {KNOWN_TOOLS.map((t) => <option key={t}>{t}</option>)}
        </select>
        <input
          value={args}
          onChange={(e) => setArgs(e.currentTarget.value)}
          className="flex-1 px-3 py-2 bg-[var(--color-surface-1)] border border-[var(--color-border)] rounded-md text-sm font-mono"
          aria-label="Tool arguments (JSON)"
        />
        <button onClick={onRun} className="px-3 py-2 bg-[var(--color-accent)] text-[var(--color-accent-contrast)] rounded-md text-sm">
          Run
        </button>
      </div>
      {err && <p className="text-sm text-[var(--color-semantic-error)] mb-2">{err}</p>}
      <div className="space-y-2">
        {history.map((h) => (
          <details key={h.id} className="border border-[var(--color-border)] rounded-md">
            <summary className="cursor-pointer p-3 flex items-center gap-3">
              <span className="font-mono text-xs px-2 py-0.5 rounded bg-[var(--color-surface-2)]">{h.tool}</span>
              <span className="text-xs text-[var(--color-text-muted)]">{h.tookMs}ms</span>
              {h.error && <span className="text-xs text-[var(--color-semantic-error)] ml-auto">{h.error}</span>}
            </summary>
            <pre className="p-3 text-xs font-mono overflow-auto bg-[var(--color-surface-1)]">
{`args: ${JSON.stringify(h.args, null, 2)}
result: ${JSON.stringify(h.result, null, 2)}`}
            </pre>
          </details>
        ))}
        {history.length === 0 && (
          <p className="text-sm text-[var(--color-text-muted)]">No calls yet.</p>
        )}
      </div>
    </div>
  );
}
