import { EventRow } from "./EventRow";
import { t } from "@/i18n";
import type { ActivityEvent } from "@/hooks/api/useActivity";

interface Props { events: ActivityEvent[]; lastVisit: number; }

export function ActivityFeed({ events, lastVisit }: Props) {
  if (events.length === 0) {
    return (
      <div className="activity-empty">
        {t("home.nothingNew")}
      </div>
    );
  }
  return (
    <div className="activity-list">
      <h2 className="text-xs uppercase tracking-wider text-muted-foreground mb-2.5">
        {t("home.sinceLastVisit")}
      </h2>
      {events.map((e) => (
        <EventRow key={e.id} event={e} isNew={e.timestamp > lastVisit} />
      ))}
    </div>
  );
}
