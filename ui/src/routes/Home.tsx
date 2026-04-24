import { useEffect, useMemo } from "react";
import { Link } from "react-router-dom";
import { ArrowUpRight } from "lucide-react";
import { StatsStrip } from "@/components/layout/StatsStrip";
import { ActivityFeed } from "@/components/activity/ActivityFeed";
import { GlanceView } from "@/components/graph/GlanceView";
import { useProjectStore } from "@/stores/project";
import { useStats } from "@/hooks/api/useStats";
import { useActivity } from "@/hooks/api/useActivity";
import { useNotes } from "@/hooks/api/useNotes";
import { useNotesGraph } from "@/hooks/api/useGraph";
import { useLastVisit } from "@/hooks/useLastVisit";
import { EmptyState, ErrorState, LoadingSkeleton } from "@/components/empty";

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

  const recentNotes = (notes.data ?? []).slice(0, 8);
  const mergedStats = useMemo(() => {
    const notesCount = notes.data?.length ?? 0;
    const base = stats.data ?? {
      documents: 0, chunks: 0, entities: 0, relationships: 0,
      communities: 0, notes: 0, last_indexed: null,
    };
    return { ...base, notes: notesCount };
  }, [stats.data, notes.data]);

  const activityError = activity.error as Error | null | undefined;
  const graphError = graph.error as Error | null | undefined;

  return (
    <div className="page">
      <StatsStrip stats={mergedStats} delta={{ notes: newCount }} />

      <div className="home-split">
        <div className="home-main">
          <div className="page-header">
            <div className="flex items-baseline gap-3">
              <h2 className="page-header-title">Activity</h2>
              <span className="page-header-meta">{activity.data?.length ?? 0} events</span>
            </div>
          </div>
          <div className="page-body">
            {activity.isLoading ? (
              <LoadingSkeleton label="Loading activity" rows={4} />
            ) : activityError ? (
              <ErrorState
                title="Could not load activity"
                message={activityError.message || "Unknown error"}
                onRetry={() => activity.refetch()}
              />
            ) : (activity.data?.length ?? 0) === 0 ? (
              <EmptyState
                title="No recent activity"
                description="Ingest a document or create a note to start the feed."
              />
            ) : (
              <ActivityFeed events={activity.data ?? []} lastVisit={lastVisit} />
            )}
          </div>
        </div>

        <aside className="home-rail">
          <section className="home-rail-top">
            <div className="section-head">
              <h2 className="section-title">Graph</h2>
              <Link to="/graph" className="section-link">
                open <ArrowUpRight className="size-3" />
              </Link>
            </div>
            <div className="graph-card">
              {graph.isLoading ? (
                <LoadingSkeleton label="Loading graph preview" rows={3} />
              ) : graphError ? (
                <ErrorState
                  title="Graph failed"
                  message={graphError.message || "Unknown error"}
                  onRetry={() => graph.refetch()}
                />
              ) : !graph.data || graph.data.nodes.length === 0 ? (
                <EmptyState
                  title="No graph yet"
                  description="Run an index to see entities and edges."
                />
              ) : (
                <GlanceView data={graph.data} />
              )}
            </div>
          </section>

          <section className="section flex-1">
            <div className="section-head">
              <h2 className="section-title">Recent notes</h2>
              <Link to="/notes" className="section-link">
                all <ArrowUpRight className="size-3" />
              </Link>
            </div>
            {notes.isLoading ? (
              <LoadingSkeleton label="Loading notes" rows={5} />
            ) : recentNotes.length === 0 ? (
              <EmptyState
                title="No notes yet"
                description="Create your first note to see it here."
              />
            ) : (
              <ul className="note-list">
                {recentNotes.map((n) => {
                  const parts = n.key.split("/");
                  const name = parts.pop() ?? n.key;
                  const folder = parts.join("/");
                  return (
                    <li key={n.key}>
                      <Link to={`/notes/${encodeURIComponent(n.key)}`} className="note-row">
                        <span className="note-row-name">{name}</span>
                        {folder && <span className="note-row-folder">{folder}</span>}
                      </Link>
                    </li>
                  );
                })}
              </ul>
            )}
          </section>
        </aside>
      </div>
    </div>
  );
}
