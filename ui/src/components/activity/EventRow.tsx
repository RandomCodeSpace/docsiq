import { Link } from "react-router-dom";
import { EventBadge } from "./EventBadge";
import { formatRelativeTime } from "@/lib/format";
import type { ActivityEvent } from "@/hooks/api/useActivity";

export function EventRow({ event, isNew }: { event: ActivityEvent; isNew: boolean }) {
  return (
    <Link
      to={event.href}
      className="activity-row"
      style={isNew ? { background: "var(--muted)" } : undefined}
    >
      <EventBadge kind={event.kind} />
      <div className="activity-body">
        <div className="activity-title">
          {event.title}
        </div>
        {event.detail && <div className="activity-detail">{event.detail}</div>}
      </div>
      <span className="activity-time">
        {formatRelativeTime(event.timestamp)}
      </span>
    </Link>
  );
}
