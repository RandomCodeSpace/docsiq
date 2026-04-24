import { useEffect } from "react";
import { useLocation, useParams } from "react-router-dom";

const STATIC_TITLES: Record<string, string> = {
  "/": "Home",
  "/notes": "Notes",
  "/notes/search": "Search notes",
  "/docs": "Documents",
  "/graph": "Graph",
  "/mcp": "MCP Console",
};

const SUFFIX = "docsiq";

function prettifyKey(raw: string): string {
  try {
    const decoded = decodeURIComponent(raw);
    const last = decoded.split("/").pop();
    return last && last.length > 0 ? last : decoded;
  } catch {
    return raw;
  }
}

/**
 * Sets document.title with a consistent " — docsiq" suffix.
 *
 * `parts` takes precedence over path-derived titles. Pass a list of parts
 * from most-specific to least-specific, e.g. ["Design doc v2", "Documents"]
 * produces "Design doc v2 — Documents — docsiq".
 */
export function useDocumentTitle(parts?: string[]): void {
  const { pathname } = useLocation();
  const params = useParams();

  useEffect(() => {
    const explicit = (parts ?? []).filter((p) => typeof p === "string" && p.trim().length > 0);
    let segments: string[] = [];

    if (explicit.length > 0) {
      segments = [...explicit, SUFFIX];
    } else {
      const label = STATIC_TITLES[pathname];
      if (label) {
        segments = [label, SUFFIX];
      } else if (pathname.startsWith("/notes/") && params.key) {
        segments = [prettifyKey(params.key), SUFFIX];
      } else if (pathname.startsWith("/docs/") && params.id) {
        // Route may pass its own title via `parts`; otherwise show a short id.
        segments = [`Document ${params.id.slice(0, 8)}`, SUFFIX];
      } else {
        segments = [SUFFIX];
      }
    }

    document.title =
      segments.length === 1 ? segments[0]! : segments.join(" — ");
  }, [pathname, params, parts]);
}
