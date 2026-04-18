import { Link, Outlet, useParams } from "react-router-dom";
import { useMemo, useState } from "react";
import { TreeDrawer } from "@/components/notes/TreeDrawer";
import { LinkPanel } from "@/components/notes/LinkPanel";
import { useProjectStore } from "@/stores/project";
import { useHotkey } from "@/hooks/useHotkey";
import { useNotes } from "@/hooks/api/useNotes";

export default function NotesLayout() {
  const project = useProjectStore((s) => s.slug);
  const { key } = useParams();
  const [treeOpen, setTreeOpen] = useState(false);
  const [linksOpen, setLinksOpen] = useState(false);

  useHotkey("mod+/", () => setTreeOpen((v) => !v));
  useHotkey("mod+l", () => setLinksOpen((v) => !v));

  return (
    <div className="relative">
      <TreeDrawer project={project} open={treeOpen} onOpenChange={setTreeOpen} currentKey={key} />
      <LinkPanel project={project} open={linksOpen} onOpenChange={setLinksOpen} currentKey={key} />
      <Outlet />
      {!key && <NotesIndex project={project} />}
    </div>
  );
}

function NotesIndex({ project }: { project: string }) {
  const { data, isLoading } = useNotes(project);
  const groups = useMemo(() => {
    const byFolder: Record<string, string[]> = {};
    for (const n of data ?? []) {
      const parts = n.key.split("/");
      const folder = parts.length > 1 ? parts.slice(0, -1).join("/") : "(root)";
      (byFolder[folder] ??= []).push(n.key);
    }
    for (const k of Object.keys(byFolder)) byFolder[k].sort();
    return Object.entries(byFolder).sort(([a], [b]) => a.localeCompare(b));
  }, [data]);

  return (
    <div className="notes-index">
      <div className="notes-index-head">
        <h1 className="notes-index-title">Notes</h1>
        <span className="notes-index-count">
          {isLoading ? "loading…" : `${data?.length ?? 0} notes in ${groups.length} folders`}
        </span>
      </div>
      <p className="notes-index-hint">
        Open tree <kbd className="kbd">⌘/</kbd>
        &nbsp;· search <kbd className="kbd">⌘K</kbd>
      </p>
      <div className="notes-index-grid">
        {groups.map(([folder, keys]) => (
          <section key={folder} className="notes-folder">
            <h2 className="notes-folder-head">
              {folder} <span className="notes-folder-count">· {keys.length}</span>
            </h2>
            <ul className="notes-folder-list">
              {keys.map((k) => (
                <li key={k} className="notes-folder-item">
                  <Link to={`/notes/${encodeURIComponent(k)}`} className="notes-folder-link">
                    {k.split("/").pop()}
                  </Link>
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </div>
  );
}
