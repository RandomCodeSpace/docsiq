import { type ReactNode, useState } from "react";
import { Sidebar } from "./Sidebar";
import { TopBar } from "./TopBar";
import { SkipLink } from "./SkipLink";

export function Shell({ children }: { children: ReactNode }) {
  const [cmdOpen, setCmdOpen] = useState(false);

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
      {/* CommandPalette will mount in Wave 3 */}
      <span className="sr-only" aria-hidden>{cmdOpen ? "open" : "closed"}</span>
    </div>
  );
}
