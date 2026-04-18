import { useEffect, useMemo } from "react";
import { StatsStrip } from "@/components/layout/StatsStrip";
import { ActivityFeed } from "@/components/activity/ActivityFeed";
import { GlanceView } from "@/components/graph/GlanceView";
import { useProjectStore } from "@/stores/project";
import { useStats } from "@/hooks/api/useStats";
import { useActivity } from "@/hooks/api/useActivity";
import { useNotes } from "@/hooks/api/useNotes";
import { useNotesGraph } from "@/hooks/api/useGraph";
import { useLastVisit } from "@/hooks/useLastVisit";
import { t } from "@/i18n";

export default function Home() {
  const project = useProjectStore((s) => s.slug);
  const stats = useStats(project);
  const activity = useActivity(project);
  const notes = useNotes(project);
  const graph = useNotesGraph(project);
  const { lastVisit, touch } = useLastVisit();

  const newCount = useMemo(() => {
    if (!activity.data) return 0;
    return activity.data.filter((e) => e.kind === "note_added" && e.timestamp > lastVisit).length;
  }, [activity.data, lastVisit]);

  useEffect(() => () => { touch(); }, [touch]);

  const recentNotes = (notes.data ?? []).slice(0, 5);

  return (
    <div className="p-6 max-w-[1400px] mx-auto">
      <StatsStrip stats={stats.data} delta={{ notes: newCount }} />
      <div className="grid grid-cols-1 lg:grid-cols-[1fr_320px] gap-5">
        <ActivityFeed events={activity.data ?? []} lastVisit={lastVisit} />
        <aside className="flex flex-col gap-4">
          <section aria-label={t("home.graphGlance")} className="border border-[var(--color-border)] rounded-md p-3">
            <h2 className="text-[10px] uppercase tracking-wider text-[var(--color-text-muted)] mb-2.5">
              {t("home.graphGlance")}
            </h2>
            <GlanceView data={graph.data} />
          </section>
          <section aria-label={t("home.pinnedNotes")} className="border border-[var(--color-border)] rounded-md p-3">
            <h2 className="text-[10px] uppercase tracking-wider text-[var(--color-text-muted)] mb-2.5">
              {t("home.pinnedNotes")}
            </h2>
            <ul className="text-sm text-[var(--color-text)] font-mono space-y-1.5">
              {recentNotes.map((n) => (
                <li key={n.key} className="truncate">{n.key}</li>
              ))}
              {recentNotes.length === 0 && (
                <li className="text-[var(--color-text-muted)]">—</li>
              )}
            </ul>
          </section>
        </aside>
      </div>
    </div>
  );
}
