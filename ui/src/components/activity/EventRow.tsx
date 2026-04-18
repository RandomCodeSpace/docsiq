import { Link } from "react-router-dom";
import { EventBadge } from "./EventBadge";
import { formatRelativeTime } from "@/lib/format";
import type { ActivityEvent } from "@/hooks/api/useActivity";

export function EventRow({ event, isNew }: { event: ActivityEvent; isNew: boolean }) {
  return (
    <Link
      to={event.href}
      className="flex items-center gap-3 px-3 py-2 rounded-md border border-[var(--color-border)] hover:bg-[var(--color-surface-2)] transition-colors"
      style={isNew ? { background: "var(--color-surface-2)" } : undefined}
    >
      <EventBadge kind={event.kind} />
      <span className="flex-1 truncate text-sm text-[var(--color-text)]">
        {event.title}
        {event.detail && <span className="ml-2 text-[var(--color-text-muted)]">· {event.detail}</span>}
      </span>
      <span className="font-mono text-xs text-[var(--color-text-muted)]">
        {formatRelativeTime(event.timestamp)}
      </span>
    </Link>
  );
}
