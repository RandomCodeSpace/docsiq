import { useEffect } from "react";
import { useLocation, useParams } from "react-router-dom";

const TITLES: Record<string, string> = {
  "/": "Home",
  "/notes": "Notes",
  "/notes/search": "Search notes",
  "/docs": "Documents",
  "/graph": "Graph",
  "/mcp": "MCP Console",
};

function prettifyKey(raw: string): string {
  try {
    return decodeURIComponent(raw).split("/").pop() || raw;
  } catch {
    return raw;
  }
}

export function useDocumentTitle() {
  const { pathname } = useLocation();
  const params = useParams();
  useEffect(() => {
    let label = TITLES[pathname];
    if (!label) {
      if (pathname.startsWith("/notes/") && params.key) {
        label = prettifyKey(params.key);
      } else if (pathname.startsWith("/docs/") && params.id) {
        label = `Document ${params.id.slice(0, 8)}`;
      } else {
        label = "docsiq";
      }
    }
    document.title = label === "docsiq" ? "docsiq" : `${label} — docsiq`;
  }, [pathname, params]);
}
