import { useUIStore } from "@/stores/ui";
import { t } from "@/i18n";
import { PanelLeft } from "lucide-react";
import { ThemeToggle } from "./ThemeToggle";

interface TopBarProps {
  onCommandOpen: () => void;
}

export function TopBar({ onCommandOpen }: TopBarProps) {
  const toggle = useUIStore((s) => s.toggleSidebar);

  return (
    <header className="flex items-center gap-4 h-11 px-3 border-b border-[var(--color-border)] bg-[var(--color-surface-1)]">
      <button
        onClick={toggle}
        aria-label="Toggle sidebar"
        className="p-1.5 rounded-md hover:bg-[var(--color-surface-2)] transition-colors"
      >
        <PanelLeft size={16} />
      </button>
      <span className="font-mono text-sm text-[var(--color-text)]">docsiq</span>
      <span className="text-[var(--color-text-faint)]">/</span>
      <span className="font-mono text-sm text-[var(--color-text-muted)]">_default</span>
      <button
        onClick={onCommandOpen}
        aria-label="Open command palette"
        className="ml-auto flex items-center gap-2 px-3 py-1.5 rounded-md border border-[var(--color-border-strong)] bg-[var(--color-base)] text-sm text-[var(--color-text-muted)] hover:bg-[var(--color-surface-2)] transition-colors"
      >
        <span>{t("nav.search")}</span>
        <kbd className="font-mono text-[10px] px-1.5 py-0.5 rounded bg-[var(--color-surface-2)] border border-[var(--color-border)]">⌘K</kbd>
      </button>
      <ThemeToggle />
    </header>
  );
}
