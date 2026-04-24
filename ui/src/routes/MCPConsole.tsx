import { useMCP, templateForTool, type MCPTool } from "@/hooks/api/useMCP";
import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { EmptyState, ErrorState, LoadingSkeleton } from "@/components/empty";

type ParamType = "string" | "number" | "integer" | "boolean" | "array" | "object";

interface ParamSpec {
  name: string;
  type: ParamType;
  description?: string;
  enum?: unknown[];
  required: boolean;
}

function normaliseParams(tool: MCPTool | undefined): ParamSpec[] {
  const props = tool?.inputSchema?.properties ?? {};
  const required = new Set(tool?.inputSchema?.required ?? []);
  return Object.entries(props).map(([name, p]) => ({
    name,
    type: (p.type as ParamType) ?? "string",
    description: p.description,
    enum: p.enum,
    required: required.has(name),
  }));
}

function coerce(value: string, type: ParamType): unknown {
  if (value === "") return undefined;
  if (type === "number" || type === "integer") {
    const n = Number(value);
    return Number.isFinite(n) ? n : value;
  }
  if (type === "boolean") return value === "true";
  if (type === "array" || type === "object") {
    try { return JSON.parse(value); } catch { return value; }
  }
  return value;
}

export default function MCPConsole() {
  const { history, invoke, tools, toolsError, refreshTools } = useMCP();
  const [selectedName, setSelectedName] = useState("");
  const [filter, setFilter] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const [rawJson, setRawJson] = useState("{}");
  const [useRaw, setUseRaw] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return tools;
    return tools.filter(
      (t) => t.name.toLowerCase().includes(q) || t.description?.toLowerCase().includes(q),
    );
  }, [tools, filter]);

  useEffect(() => {
    if (!selectedName && filtered.length) setSelectedName(filtered[0].name);
  }, [filtered, selectedName]);

  const selected = useMemo(() => tools.find((t) => t.name === selectedName), [tools, selectedName]);
  const params = useMemo(() => normaliseParams(selected), [selected]);

  useEffect(() => {
    setErr(null);
    setFields({});
    setRawJson(templateForTool(selected));
    setUseRaw(false);
  }, [selectedName, selected]);

  async function onRun() {
    if (!selected) return;
    setErr(null);
    let args: unknown;
    if (useRaw) {
      try { args = JSON.parse(rawJson); } catch { setErr("Invalid JSON"); return; }
    } else {
      const obj: Record<string, unknown> = {};
      for (const p of params) {
        const v = coerce(fields[p.name] ?? "", p.type);
        if (v !== undefined) obj[p.name] = v;
      }
      args = obj;
    }
    setBusy(true);
    try { await invoke(selected.name, args); } finally { setBusy(false); }
  }

  return (
    <div className="mcp-shell">
      <aside className="mcp-aside">
        <div className="mcp-aside-head">
          <div className="mcp-aside-title">
            <h1 className="mcp-aside-name">MCP Tools</h1>
            <span className="mcp-aside-count">{toolsError ? "error" : tools.length}</span>
          </div>
          <input
            value={filter}
            onChange={(e) => setFilter(e.currentTarget.value)}
            placeholder="filter…"
            className="mcp-filter"
            aria-label="Filter tools"
          />
          <button onClick={() => void refreshTools()} className="mcp-refresh">
            refresh
          </button>
        </div>
        {toolsError && <p className="mcp-error">{toolsError}</p>}
        <ul>
          {filtered.map((t) => (
            <li key={t.name}>
              <button
                onClick={() => setSelectedName(t.name)}
                className={cn("mcp-tool", t.name === selectedName && "mcp-tool-active")}
              >
                <div>{t.name}</div>
                {t.description && <div className="mcp-tool-desc">{t.description}</div>}
              </button>
            </li>
          ))}
        </ul>
      </aside>
      <section className="mcp-main">
        {!selected ? (
          <div className="mcp-content">
            {toolsError ? (
              <ErrorState
                title="MCP tools failed to load"
                message={toolsError}
                onRetry={() => void refreshTools()}
              />
            ) : tools.length === 0 ? (
              <LoadingSkeleton label="Loading MCP tools" rows={4} />
            ) : (
              <EmptyState
                title="Pick a tool"
                description="Select a tool from the left sidebar to run it."
              />
            )}
          </div>
        ) : (
          <div className="mcp-content">
            <header className="mb-4">
              <h2 className="mcp-title">{selected.name}</h2>
              {selected.description && <p className="mcp-desc">{selected.description}</p>}
            </header>

            <div className="mcp-panel">
              <div className="mcp-panel-head">
                <h3 className="mcp-panel-title">Arguments</h3>
                <label className="mcp-raw-toggle">
                  <input
                    type="checkbox"
                    checked={useRaw}
                    onChange={(e) => setUseRaw(e.currentTarget.checked)}
                  />
                  raw JSON
                </label>
              </div>
              {useRaw ? (
                <textarea
                  value={rawJson}
                  onChange={(e) => setRawJson(e.currentTarget.value)}
                  rows={Math.min(14, Math.max(4, rawJson.split("\n").length))}
                  className="mcp-textarea"
                  spellCheck={false}
                />
              ) : params.length === 0 ? (
                <p className="mcp-empty-params">(no parameters)</p>
              ) : (
                <div className="space-y-3">
                  {params.map((p) => (
                    <ParamField
                      key={p.name}
                      spec={p}
                      value={fields[p.name] ?? ""}
                      onChange={(v) => setFields((f) => ({ ...f, [p.name]: v }))}
                    />
                  ))}
                </div>
              )}
              <div className="mcp-run-row">
                <Button onClick={onRun} disabled={busy} size="sm">
                  {busy ? "running…" : "Run"}
                </Button>
                {err && <span className="text-xs text-destructive">{err}</span>}
              </div>
            </div>

            <section>
              <h3 className="mcp-panel-title mb-2">History</h3>
              {history.length === 0 && <p className="text-sm text-muted-foreground">No calls yet.</p>}
              <div className="mcp-history">
                {history.map((h) => (
                  <details key={h.id} className="mcp-history-item" open={h === history[0]}>
                    <summary className="mcp-history-head">
                      <span className="mcp-history-name">{h.tool}</span>
                      <span className="mcp-history-ms">{h.tookMs}ms</span>
                      {h.error ? (
                        <span className="mcp-history-err">{h.error}</span>
                      ) : (
                        <span className="mcp-history-ok">ok</span>
                      )}
                    </summary>
                    <pre className="mcp-history-body">
{`args:
${JSON.stringify(h.args, null, 2)}

result:
${JSON.stringify(h.result, null, 2)}`}
                    </pre>
                  </details>
                ))}
              </div>
            </section>
          </div>
        )}
      </section>
    </div>
  );
}

function ParamField({
  spec,
  value,
  onChange,
}: {
  spec: ParamSpec;
  value: string;
  onChange: (v: string) => void;
}) {
  const label = (
    <label className="param-label">
      <span className="param-name">{spec.name}</span>
      <span className="param-type">:{spec.type}</span>
      {spec.required && <span className="param-required">*</span>}
      {spec.description && <span className="param-desc">{spec.description}</span>}
    </label>
  );
  if (spec.enum?.length) {
    return (
      <div>
        {label}
        <select
          value={value}
          onChange={(e) => onChange(e.currentTarget.value)}
          className="param-input"
        >
          <option value="">—</option>
          {spec.enum.map((v) => (
            <option key={String(v)} value={String(v)}>{String(v)}</option>
          ))}
        </select>
      </div>
    );
  }
  if (spec.type === "boolean") {
    return (
      <div>
        {label}
        <select
          value={value}
          onChange={(e) => onChange(e.currentTarget.value)}
          className="param-input"
        >
          <option value="">—</option>
          <option value="true">true</option>
          <option value="false">false</option>
        </select>
      </div>
    );
  }
  if (spec.type === "array" || spec.type === "object") {
    return (
      <div>
        {label}
        <textarea
          value={value}
          onChange={(e) => onChange(e.currentTarget.value)}
          rows={3}
          placeholder={spec.type === "array" ? "[]" : "{}"}
          className="param-input"
          spellCheck={false}
        />
      </div>
    );
  }
  return (
    <div>
      {label}
      <input
        value={value}
        onChange={(e) => onChange(e.currentTarget.value)}
        type={spec.type === "number" || spec.type === "integer" ? "number" : "text"}
        className="param-input"
      />
    </div>
  );
}
