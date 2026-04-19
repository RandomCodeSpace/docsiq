import { useUIStore } from "@/stores/ui";
import { useProjectStore } from "@/stores/project";
import { useProjects } from "@/hooks/api/useProjects";
import { t } from "@/i18n";
import { PanelLeft } from "lucide-react";
import { ThemeToggle } from "./ThemeToggle";

interface TopBarProps {
  onCommandOpen: () => void;
}

export function TopBar({ onCommandOpen }: TopBarProps) {
  const toggle = useUIStore((s) => s.toggleSidebar);
  const slug = useProjectStore((s) => s.slug);
  const setSlug = useProjectStore((s) => s.setSlug);
  const { data: projects } = useProjects();

  return (
    <header className="flex items-center gap-4 h-11 px-3 border-b border-border bg-card">
      <button
        onClick={toggle}
        aria-label="Toggle sidebar"
        className="p-1.5 rounded-md hover:bg-muted transition-colors"
      >
        <PanelLeft size={16} />
      </button>
      <span className="font-mono text-sm text-foreground">docsiq</span>
      <span className="text-muted-foreground">/</span>
      <select
        aria-label="Project"
        value={slug}
        onChange={(e) => setSlug(e.target.value)}
        className="font-mono text-sm text-muted-foreground bg-transparent border border-border rounded px-2 py-0.5 hover:bg-muted focus:outline-none focus-visible:ring-1 focus-visible:ring-primary"
      >
        {(projects?.length ? projects : [{ slug, name: slug }]).map((p) => (
          <option key={p.slug} value={p.slug}>{p.name || p.slug}</option>
        ))}
      </select>
      <button
        onClick={onCommandOpen}
        aria-label="Open command palette"
        className="ml-auto flex items-center gap-2 px-3 py-1.5 rounded-md border border-border bg-background text-sm text-muted-foreground hover:bg-muted transition-colors"
      >
        <span>{t("nav.search")}</span>
        <kbd className="font-mono text-[10px] px-1.5 py-0.5 rounded bg-muted border border-border">⌘K</kbd>
      </button>
      <ThemeToggle />
    </header>
  );
}
