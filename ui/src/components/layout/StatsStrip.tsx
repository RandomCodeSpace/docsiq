import { formatCount, formatRelativeTime } from "@/lib/format";
import type { Stats } from "@/types/api";

interface Props { stats: Stats | undefined; delta?: { notes?: number } }

export function StatsStrip({ stats, delta }: Props) {
  const tiles: { label: string; value: string; delta?: number }[] = [
    { label: "Notes", value: stats ? formatCount(stats.notes) : "—", delta: delta?.notes },
    { label: "Documents", value: stats ? formatCount(stats.documents) : "—" },
    { label: "Entities", value: stats ? formatCount(stats.entities) : "—" },
    { label: "Communities", value: stats ? formatCount(stats.communities) : "—" },
    { label: "Last indexed", value: stats?.last_indexed ? formatRelativeTime(new Date(stats.last_indexed).getTime()) : "—" },
  ];
  return (
    <div role="region" aria-label="Project statistics" className="stats-strip">
      {tiles.map((tl) => (
        <div key={tl.label} className="stats-cell">
          <div className="stats-label">{tl.label}</div>
          <div className="stats-value-row">
            <span className="stats-value">{tl.value}</span>
            {tl.delta !== undefined && tl.delta > 0 && (
              <span className="stats-delta">+{tl.delta}</span>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
