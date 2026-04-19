export const en = {
  common: {
    loading: "Loading…",
    error: "Something went wrong.",
    retry: "Retry",
    cancel: "Cancel",
    save: "Save",
    delete: "Delete",
    close: "Close",
  },
  nav: {
    home: "Home",
    notes: "Notes",
    documents: "Documents",
    graph: "Graph",
    mcp: "MCP console",
    search: "Search or jump to…",
    searchShort: "Search",
    skipToMain: "Skip to main content",
  },
  home: {
    sinceLastVisit: "Since your last visit",
    nothingNew: "Nothing new since your last visit.",
    viewFullActivity: "View full activity",
    pinnedNotes: "Pinned notes",
    graphGlance: "Graph glance",
    stats: {
      notes: "Notes",
      docs: "Docs",
      entities: "Entities",
      communities: "Communities",
      updated: "Updated",
    },
  },
  notes: {
    writtenBy: "Written by",
    linksIn: "in",
    linksOut: "out",
    noContent: "This note has no content.",
    invalidKey: "Invalid key — use letters, digits, /, -, _",
  },
} as const;

export type Messages = typeof en;
