import { type ReactNode, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { TopBar } from "./TopBar";
import { SkipLink } from "./SkipLink";
import { useUIStore } from "@/stores/ui";
import { useHotkey } from "@/hooks/useHotkey";
import { CommandPalette } from "@/components/command/CommandPalette";

export function Shell({ children }: { children: ReactNode }) {
  const [cmdOpen, setCmdOpen] = useState(false);
  const toggleSidebar = useUIStore((s) => s.toggleSidebar);
  const navigate = useNavigate();

  useHotkey("mod+\\", () => toggleSidebar());
  useHotkey("mod+k", () => setCmdOpen((v) => !v));
  useHotkey("g,h", () => navigate("/"));
  useHotkey("g,n", () => navigate("/notes"));
  useHotkey("g,d", () => navigate("/docs"));
  useHotkey("g,g", () => navigate("/graph"));
  useHotkey("g,m", () => navigate("/mcp"));

  return (
    <div className="min-h-screen flex flex-col">
      <SkipLink />
      <TopBar onCommandOpen={() => setCmdOpen(true)} />
      <div className="flex flex-1 min-h-0">
        <Sidebar />
        <main
          id="main"
          role="main"
          tabIndex={-1}
          className="flex-1 min-w-0 overflow-auto"
        >
          {children}
        </main>
      </div>
      <CommandPalette open={cmdOpen} onOpenChange={setCmdOpen} />
    </div>
  );
}
