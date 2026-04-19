import type { ActivityEventKind } from "@/hooks/api/useActivity";

const STYLES: Record<ActivityEventKind, { label: string; className: string }> = {
  note_added: { label: "+ NOTE", className: "activity-badge-new" },
  note_updated: { label: "~ NOTE", className: "activity-badge-upd" },
  doc_indexed: { label: "INDEX", className: "activity-badge-doc" },
  doc_error: { label: "ERROR", className: "activity-badge-err" },
};

export function EventBadge({ kind }: { kind: ActivityEventKind }) {
  const { label, className } = STYLES[kind];
  return (
    <span className={`activity-badge ${className}`}>
      {label}
    </span>
  );
}
