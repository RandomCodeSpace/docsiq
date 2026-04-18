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
          <SheetTitle className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
            Links
          </SheetTitle>
        </SheetHeader>
        <div>
          <section className="link-panel-section">
            <h3 className="link-panel-heading">Inbound</h3>
            {inbound.length === 0 && <p className="link-panel-empty">—</p>}
            <ul className="link-panel-list">
              {inbound.map((k) => (
                <li key={k}>
                  <Link
                    to={`/notes/${encodeURIComponent(k)}`}
                    onClick={() => onOpenChange(false)}
                    className="link-panel-item"
                  >
                    {k}
                  </Link>
                </li>
              ))}
            </ul>
          </section>
          <section className="link-panel-section">
            <h3 className="link-panel-heading">Outbound</h3>
            {outbound.length === 0 && <p className="link-panel-empty">—</p>}
            <ul className="link-panel-list">
              {outbound.map((k) => (
                <li key={k}>
                  <Link
                    to={`/notes/${encodeURIComponent(k)}`}
                    onClick={() => onOpenChange(false)}
                    className="link-panel-item"
                  >
                    {k}
                  </Link>
                </li>
              ))}
            </ul>
          </section>
        </div>
      </SheetContent>
    </Sheet>
  );
}
