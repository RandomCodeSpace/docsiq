import { EventRow } from "./EventRow";
import { t } from "@/i18n";
import type { ActivityEvent } from "@/hooks/api/useActivity";

interface Props { events: ActivityEvent[]; lastVisit: number; }

export function ActivityFeed({ events, lastVisit }: Props) {
  if (events.length === 0) {
    return (
      <div className="border border-dashed border-[var(--color-border)] rounded-md p-6 text-center text-sm text-[var(--color-text-muted)]">
        {t("home.nothingNew")}
      </div>
    );
  }
  return (
    <div>
      <h2 className="text-xs uppercase tracking-wider text-[var(--color-text-muted)] mb-2.5">
        {t("home.sinceLastVisit")}
      </h2>
      <div className="flex flex-col gap-1.5">
        {events.map((e) => (
          <EventRow key={e.id} event={e} isNew={e.timestamp > lastVisit} />
        ))}
      </div>
    </div>
  );
}
