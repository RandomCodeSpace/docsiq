import { Outlet, useParams } from "react-router-dom";
import { useState } from "react";
import { TreeDrawer } from "@/components/notes/TreeDrawer";
import { LinkPanel } from "@/components/notes/LinkPanel";
import { useProjectStore } from "@/stores/project";
import { useHotkey } from "@/hooks/useHotkey";

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
      {!key && (
        <div className="p-8 max-w-[620px] mx-auto text-[var(--color-text-muted)] text-sm">
          Open the tree (<kbd className="font-mono text-xs px-1.5 py-0.5 border border-[var(--color-border)] rounded">⌘/</kbd>) or search (<kbd className="font-mono text-xs px-1.5 py-0.5 border border-[var(--color-border)] rounded">⌘K</kbd>) to select a note.
        </div>
      )}
    </div>
  );
}
