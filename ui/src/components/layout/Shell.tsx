import { type ReactNode, useState } from "react";
import { useNavigate } from "react-router-dom";
import { AppSidebar } from "@/components/app-sidebar";
import { SiteHeader } from "@/components/site-header";
import { SkipLink } from "./SkipLink";
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import { useHotkey } from "@/hooks/useHotkey";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import { CommandPalette } from "@/components/command/CommandPalette";

export function Shell({ children }: { children: ReactNode }) {
  const [cmdOpen, setCmdOpen] = useState(false);
  const navigate = useNavigate();
  useDocumentTitle();

  useHotkey("mod+k", () => setCmdOpen((v) => !v));
  useHotkey("g,h", () => navigate("/"));
  useHotkey("g,n", () => navigate("/notes"));
  useHotkey("g,d", () => navigate("/docs"));
  useHotkey("g,g", () => navigate("/graph"));
  useHotkey("g,m", () => navigate("/mcp"));

  return (
    <SidebarProvider
      style={
        {
          "--sidebar-width": "calc(var(--spacing) * 72)",
          "--header-height": "calc(var(--spacing) * 12)",
        } as React.CSSProperties
      }
    >
      <SkipLink />
      <AppSidebar variant="inset" />
      <SidebarInset>
        <SiteHeader onCommandOpen={() => setCmdOpen(true)} />
        <main
          id="main"
          role="main"
          tabIndex={-1}
          className="flex flex-1 flex-col"
        >
          {children}
        </main>
      </SidebarInset>
      <CommandPalette open={cmdOpen} onOpenChange={setCmdOpen} />
    </SidebarProvider>
  );
}
