import { NavLink } from "react-router-dom";
import { Home as HomeIcon, FileText, BookOpen, Network, Terminal } from "lucide-react";
import type { ComponentType } from "react";
import { cn } from "@/lib/utils";
import { useUIStore } from "@/stores/ui";
import { t } from "@/i18n";

interface NavItem {
  to: string;
  label: string;
  icon: ComponentType<{ size?: number }>;
  chord: string;
}

const ITEMS: NavItem[] = [
  { to: "/", label: t("nav.home"), icon: HomeIcon, chord: "G H" },
  { to: "/notes", label: t("nav.notes"), icon: FileText, chord: "G N" },
  { to: "/docs", label: t("nav.documents"), icon: BookOpen, chord: "G D" },
  { to: "/graph", label: t("nav.graph"), icon: Network, chord: "G G" },
  { to: "/mcp", label: t("nav.mcp"), icon: Terminal, chord: "G M" },
];

export function Sidebar() {
  const collapsed = useUIStore((s) => s.sidebarCollapsed);

  return (
    <aside
      role="navigation"
      aria-label="Primary"
      className={cn(
        "border-r border-border bg-card flex flex-col",
        collapsed ? "w-[56px]" : "w-[220px]",
      )}
    >
      <nav className="p-2 flex flex-col gap-1 flex-1" aria-label="Main">
        {ITEMS.map(({ to, label, icon: Icon, chord }) => (
          <NavLink
            key={to}
            to={to}
            end={to === "/"}
            className={({ isActive }) =>
              cn(
                "flex items-center gap-3 rounded-md px-3 py-1.5 text-sm",
                "hover:bg-muted transition-colors",
                isActive && "bg-muted text-foreground",
                !isActive && "text-muted-foreground",
              )
            }
            title={collapsed ? label : undefined}
          >
            <Icon size={16} />
            {!collapsed && (
              <>
                <span className="flex-1">{label}</span>
                <span className="font-mono text-[10px] text-muted-foreground">
                  {chord}
                </span>
              </>
            )}
          </NavLink>
        ))}
      </nav>
    </aside>
  );
}
