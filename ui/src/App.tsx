import { useEffect } from "react";
import { Route, Routes } from "react-router-dom";
import { Providers } from "@/components/layout/Providers";
import { Shell } from "@/components/layout/Shell";
import { initAuth } from "@/lib/api-client";

import Home from "@/routes/Home";
import NotesLayout from "@/routes/notes/NotesLayout";
import NoteView from "@/routes/notes/NoteView";
import NoteEditor from "@/routes/notes/NoteEditor";
import NotesSearch from "@/routes/notes/NotesSearch";
import DocumentsList from "@/routes/documents/DocumentsList";
import DocumentView from "@/routes/documents/DocumentView";
import Graph from "@/routes/Graph";
import MCPConsole from "@/routes/MCPConsole";

export default function App() {
  useEffect(() => { initAuth(); }, []);
  return (
    <Providers>
      <Shell>
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/notes" element={<NotesLayout />}>
            <Route path="search" element={<NotesSearch />} />
            <Route path=":key" element={<NoteView />} />
            <Route path=":key/edit" element={<NoteEditor />} />
          </Route>
          <Route path="/docs" element={<DocumentsList />} />
          <Route path="/docs/:id" element={<DocumentView />} />
          <Route path="/graph" element={<Graph />} />
          <Route path="/mcp" element={<MCPConsole />} />
          <Route path="*" element={<NotFound />} />
        </Routes>
      </Shell>
    </Providers>
  );
}

function NotFound() {
  return (
    <div className="p-6 text-[var(--color-text-muted)]">
      <h1 className="text-xl font-semibold text-[var(--color-text)]">Not found</h1>
      <p className="text-sm mt-2">No such page.</p>
    </div>
  );
}
