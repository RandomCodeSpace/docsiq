import { useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { useProjectStore } from "@/stores/project";
import { useCommandSearch } from "@/hooks/api/useCommand";

interface Props { open: boolean; onOpenChange: (v: boolean) => void; }

export function CommandPalette({ open, onOpenChange }: Props) {
  const [q, setQ] = useState("");
  const navigate = useNavigate();
  const project = useProjectStore((s) => s.slug);
  const { data } = useCommandSearch(project, q);

  const close = () => { onOpenChange(false); setQ(""); };

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange}>
      <Command shouldFilter={false}>
        <CommandInput
          value={q}
          onValueChange={setQ}
          placeholder="Search notes, docs, entities…"
          autoFocus
        />
        <CommandList>
          <CommandEmpty>No results.</CommandEmpty>
          {!q && (
            <div className="py-6 text-center text-sm text-muted-foreground">
              Type to search.
            </div>
          )}

          <CommandGroup heading="Pages">
            <CommandItem onSelect={() => { navigate("/"); close(); }}>Home</CommandItem>
            <CommandItem onSelect={() => { navigate("/notes"); close(); }}>Notes</CommandItem>
            <CommandItem onSelect={() => { navigate("/docs"); close(); }}>Documents</CommandItem>
            <CommandItem onSelect={() => { navigate("/graph"); close(); }}>Graph</CommandItem>
            <CommandItem onSelect={() => { navigate("/mcp"); close(); }}>MCP console</CommandItem>
          </CommandGroup>

          {data && data.notes.length > 0 && (
            <CommandGroup heading="Notes">
              {data.notes.slice(0, 5).map((n) => (
                <CommandItem
                  key={`note-${n.key}`}
                  onSelect={() => { navigate(`/notes/${n.key}`); close(); }}
                >
                  <span className="font-mono text-[10px] px-1.5 mr-2 rounded bg-[var(--color-surface-2)] text-[var(--color-text-muted)]">NOTE</span>
                  {n.title || n.key}
                </CommandItem>
              ))}
            </CommandGroup>
          )}

          {data && data.docs.length > 0 && (
            <CommandGroup heading="Documents">
              {data.docs.slice(0, 5).map((d) => (
                <CommandItem
                  key={`doc-${d.chunk_id}`}
                  onSelect={() => { navigate(`/docs/${d.doc_id}`); close(); }}
                >
                  <span className="font-mono text-[10px] px-1.5 mr-2 rounded bg-[var(--color-surface-2)] text-[var(--color-text-muted)]">DOC</span>
                  {d.doc_title}
                </CommandItem>
              ))}
            </CommandGroup>
          )}
        </CommandList>
      </Command>
    </CommandDialog>
  );
}
