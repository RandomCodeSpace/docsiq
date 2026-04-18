import { Link } from "react-router-dom";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { useNotesGraph } from "@/hooks/api/useGraph";
import { useMemo } from "react";

interface Props { project: string; open: boolean; onOpenChange: (v: boolean) => void; currentKey?: string; }

export function LinkPanel({ project, open, onOpenChange, currentKey }: Props) {
  const { data } = useNotesGraph(project);
  const { inbound, outbound } = useMemo(() => {
    if (!data || !currentKey) return { inbound: [] as string[], outbound: [] as string[] };
    const inb: string[] = [];
    const out: string[] = [];
    for (const e of data.edges) {
      if (e.target === currentKey) inb.push(e.source);
      if (e.source === currentKey) out.push(e.target);
    }
    return { inbound: Array.from(new Set(inb)), outbound: Array.from(new Set(out)) };
  }, [data, currentKey]);

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-[280px]">
        <SheetHeader className="mb-4">
          <SheetTitle className="font-mono text-xs uppercase tracking-wider text-[var(--color-text-muted)]">
            Links
          </SheetTitle>
        </SheetHeader>
        <div className="space-y-4 text-sm">
          <section>
            <h3 className="text-xs uppercase text-[var(--color-text-muted)] mb-1.5">Inbound</h3>
            {inbound.length === 0 && <p className="text-xs text-[var(--color-text-muted)]">—</p>}
            {inbound.map((k) => (
              <Link
                key={k}
                to={`/notes/${encodeURIComponent(k)}`}
                onClick={() => onOpenChange(false)}
                className="block px-2 py-1 rounded hover:bg-[var(--color-surface-2)]"
              >
                {k}
              </Link>
            ))}
          </section>
          <section>
            <h3 className="text-xs uppercase text-[var(--color-text-muted)] mb-1.5">Outbound</h3>
            {outbound.length === 0 && <p className="text-xs text-[var(--color-text-muted)]">—</p>}
            {outbound.map((k) => (
              <Link
                key={k}
                to={`/notes/${encodeURIComponent(k)}`}
                onClick={() => onOpenChange(false)}
                className="block px-2 py-1 rounded hover:bg-[var(--color-surface-2)]"
              >
                {k}
              </Link>
            ))}
          </section>
        </div>
      </SheetContent>
    </Sheet>
  );
}
