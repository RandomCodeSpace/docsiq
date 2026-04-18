import { lazy, Suspense, useEffect } from "react";
import { Route, Routes } from "react-router-dom";
import { Providers } from "@/components/layout/Providers";
import { Shell } from "@/components/layout/Shell";
import { initAuth } from "@/lib/api-client";
import Home from "@/routes/Home";

// Home is eager (first paint); everything else is split.
const NotesLayout = lazy(() => import("@/routes/notes/NotesLayout"));
const NoteView = lazy(() => import("@/routes/notes/NoteView"));
const NoteEditor = lazy(() => import("@/routes/notes/NoteEditor"));
const NotesSearch = lazy(() => import("@/routes/notes/NotesSearch"));
const DocumentsList = lazy(() => import("@/routes/documents/DocumentsList"));
const DocumentView = lazy(() => import("@/routes/documents/DocumentView"));
const Graph = lazy(() => import("@/routes/Graph"));
const MCPConsole = lazy(() => import("@/routes/MCPConsole"));

function RouteFallback() {
  return <div className="p-6 text-sm text-muted-foreground">loading…</div>;
}

export default function App() {
  useEffect(() => { initAuth(); }, []);
  return (
    <Providers>
      <Shell>
        <Suspense fallback={<RouteFallback />}>
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
        </Suspense>
      </Shell>
    </Providers>
  );
}

function NotFound() {
  return (
    <div className="p-6 text-muted-foreground">
      <h1 className="text-xl font-semibold text-foreground">Not found</h1>
      <p className="text-sm mt-2">No such page.</p>
    </div>
  );
}
