import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { ActivityFeed } from "../ActivityFeed";

describe("ActivityFeed", () => {
  it("shows empty state on no events", () => {
    render(<MemoryRouter><ActivityFeed events={[]} lastVisit={0} /></MemoryRouter>);
    expect(screen.getByText(/nothing new/i)).toBeInTheDocument();
  });
  it("renders events and highlights ones newer than lastVisit", () => {
    const now = Date.now();
    render(
      <MemoryRouter>
        <ActivityFeed
          events={[
            { id: "1", kind: "note_added", title: "jwt", timestamp: now, href: "/notes/jwt" },
            { id: "2", kind: "doc_indexed", title: "api.md", timestamp: now - 3600_000, href: "/docs/1" },
          ]}
          lastVisit={now - 1800_000}
        />
      </MemoryRouter>,
    );
    expect(screen.getByText("+ NOTE")).toBeInTheDocument();
    expect(screen.getByText("INDEX")).toBeInTheDocument();
    expect(screen.getByText("jwt")).toBeInTheDocument();
    expect(screen.getByText("api.md")).toBeInTheDocument();
  });
});
