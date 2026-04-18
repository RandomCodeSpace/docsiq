import { Link } from "react-router-dom";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { useNotes } from "@/hooks/api/useNotes";
import type { Note } from "@/types/api";

interface Props { project: string; open: boolean; onOpenChange: (v: boolean) => void; currentKey?: string; }

function groupByFolder(notes: Note[]) {
  const tree: Record<string, Note[]> = {};
  for (const n of notes) {
    const parts = n.key.split("/");
    const folder = parts.length === 1 ? "" : parts.slice(0, -1).join("/");
    (tree[folder] ??= []).push(n);
  }
  return tree;
}

export function TreeDrawer({ project, open, onOpenChange, currentKey }: Props) {
  const { data = [] } = useNotes(project);
  const grouped = groupByFolder(data);
  const folders = Object.keys(grouped).sort();
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="left" className="w-[300px] p-0">
        <SheetHeader className="px-4 py-3 border-b border-[var(--color-border)]">
          <SheetTitle className="font-mono text-xs uppercase tracking-wider text-[var(--color-text-muted)]">
            Notes
          </SheetTitle>
        </SheetHeader>
        <div className="p-2 overflow-auto text-sm">
          {folders.map((folder) => (
            <div key={folder || "(root)"} className="mb-2">
              {folder && (
                <div className="font-mono text-xs text-[var(--color-text-muted)] px-2 py-1">
                  {folder}/
                </div>
              )}
              {grouped[folder]
                .sort((a, b) => a.key.localeCompare(b.key))
                .map((n) => (
                  <Link
                    key={n.key}
                    to={`/notes/${encodeURIComponent(n.key)}`}
                    className={
                      "block px-2 py-1 rounded text-sm " +
                      (currentKey === n.key
                        ? "bg-[var(--color-surface-2)] text-[var(--color-text)]"
                        : "text-[var(--color-text-muted)] hover:bg-[var(--color-surface-2)]")
                    }
                    onClick={() => onOpenChange(false)}
                  >
                    {n.key.split("/").pop()}
                  </Link>
                ))}
            </div>
          ))}
          {folders.length === 0 && (
            <div className="p-2 text-xs text-[var(--color-text-muted)]">No notes yet.</div>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
