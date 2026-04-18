import { Button } from "@/components/ui/button";
import { SidebarTrigger } from "@/components/ui/sidebar";
import { Search } from "lucide-react";
import { useLocation } from "react-router-dom";
import { ThemeToggle } from "@/components/layout/ThemeToggle";

interface Props { onCommandOpen: () => void }

const TITLES: Record<string, string> = {
  "/": "Home",
  "/notes": "Notes",
  "/docs": "Documents",
  "/graph": "Graph",
  "/mcp": "MCP Console",
};

export function SiteHeader({ onCommandOpen }: Props) {
  const { pathname } = useLocation();
  const title =
    TITLES[pathname] ??
    (pathname.startsWith("/notes") ? "Notes"
      : pathname.startsWith("/docs") ? "Documents"
      : "docsiq");

  return (
    <header className="site-header">
      <div className="site-header-left">
        <SidebarTrigger className="site-header-mobile-trigger" />
        <h1 className="site-header-title">{title}</h1>
      </div>
      <div className="site-header-actions">
        <Button
          variant="outline"
          size="sm"
          onClick={onCommandOpen}
          className="site-header-search"
        >
          <Search className="size-4" />
          <span className="hidden sm:inline">Search</span>
          <kbd className="site-header-kbd">
            <span className="text-xs">⌘</span>K
          </kbd>
        </Button>
        <ThemeToggle />
      </div>
    </header>
  );
}
