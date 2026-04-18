import { formatCount, formatRelativeTime } from "@/lib/format";
import { t } from "@/i18n";
import type { Stats } from "@/types/api";

interface Props { stats: Stats | undefined; delta?: { notes?: number }; }

const CARD = "flex-1 border border-[var(--color-border)] rounded-md p-3 font-mono";
const LABEL = "text-[10px] uppercase tracking-wider text-[var(--color-text-muted)]";
const VALUE = "text-xl text-[var(--color-text)] mt-1";
const DELTA = "text-xs text-[var(--color-accent)]";

export function StatsStrip({ stats, delta }: Props) {
  const tiles = [
    { label: t("home.stats.notes"), value: stats ? formatCount(stats.notes) : "—", delta: delta?.notes },
    { label: t("home.stats.docs"), value: stats ? formatCount(stats.documents) : "—" },
    { label: t("home.stats.entities"), value: stats ? formatCount(stats.entities) : "—" },
    { label: t("home.stats.communities"), value: stats ? formatCount(stats.communities) : "—" },
    { label: t("home.stats.updated"), value: stats?.last_indexed ? formatRelativeTime(new Date(stats.last_indexed).getTime()) : "—" },
  ];
  return (
    <div role="region" aria-label="Project statistics" className="flex gap-3 mb-5 flex-wrap">
      {tiles.map((tl) => (
        <div key={tl.label} className={CARD}>
          <div className={LABEL}>{tl.label}</div>
          <div className={VALUE}>
            {tl.value}
            {tl.delta !== undefined && tl.delta > 0 && (
              <span className={`${DELTA} ml-2`}>+{tl.delta}</span>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
