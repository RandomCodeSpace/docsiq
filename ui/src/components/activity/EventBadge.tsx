import type { ActivityEventKind } from "@/hooks/api/useActivity";

const STYLES: Record<ActivityEventKind, { label: string; color: string }> = {
  note_added: { label: "+ NOTE", color: "var(--color-semantic-new)" },
  note_updated: { label: "~ NOTE", color: "var(--color-semantic-new)" },
  doc_indexed: { label: "INDEX", color: "var(--color-semantic-index)" },
  doc_error: { label: "ERROR", color: "var(--color-semantic-error)" },
};

export function EventBadge({ kind }: { kind: ActivityEventKind }) {
  const { label, color } = STYLES[kind];
  return (
    <span
      className="font-mono text-[10px] px-1.5 py-0.5 rounded"
      style={{ color, borderColor: color, border: "1px solid" }}
    >
      {label}
    </span>
  );
}
