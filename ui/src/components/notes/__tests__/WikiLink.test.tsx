import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { WikiLink } from "../WikiLink";

// We mock the project store so tests don't need a real Zustand provider.
const mockSetSlug = vi.fn();

vi.mock("@/stores/project", () => ({
  useProjectStore: (selector: (s: { slug: string; setSlug: (s: string) => void }) => unknown) =>
    selector({ slug: "_default", setSlug: mockSetSlug }),
}));

function renderWikiLink(props: { target: string; label?: string; missing?: boolean }) {
  return render(
    <MemoryRouter>
      <WikiLink {...props} />
    </MemoryRouter>,
  );
}

describe("WikiLink — same-project", () => {
  beforeEach(() => mockSetSlug.mockClear());

  it("renders a link with correct href", () => {
    renderWikiLink({ target: "architecture/jwt" });
    const link = screen.getByRole("link") as HTMLAnchorElement;
    expect(link.getAttribute("href")).toBe("/notes/architecture%2Fjwt");
  });

  it("shows label when provided", () => {
    renderWikiLink({ target: "foo", label: "Foo Page" });
    expect(screen.getByRole("link", { name: "Foo Page" })).toBeInTheDocument();
  });

  it("does not call setSlug on click", () => {
    renderWikiLink({ target: "bar" });
    fireEvent.click(screen.getByRole("link"));
    expect(mockSetSlug).not.toHaveBeenCalled();
  });

  it("applies wikilink-missing class when missing=true", () => {
    renderWikiLink({ target: "gone", missing: true });
    const link = screen.getByRole("link");
    expect(link.className).toContain("wikilink-missing");
  });

  it("does not apply wikilink-cross class for same-project", () => {
    renderWikiLink({ target: "ordinary" });
    const link = screen.getByRole("link");
    expect(link.className).not.toContain("wikilink-cross");
  });
});

describe("WikiLink — cross-project", () => {
  beforeEach(() => mockSetSlug.mockClear());

  it("renders a link pointing to the full cross-project key", () => {
    renderWikiLink({ target: "projects/docsiq/internal/pipeline" });
    const link = screen.getByRole("link") as HTMLAnchorElement;
    expect(link.getAttribute("href")).toBe(
      "/notes/projects%2Fdocsiq%2Finternal%2Fpipeline",
    );
  });

  it("applies wikilink-cross class", () => {
    renderWikiLink({ target: "projects/docsiq/internal/pipeline" });
    const link = screen.getByRole("link");
    expect(link.className).toContain("wikilink-cross");
  });

  it("renders project chip with the slug text", () => {
    renderWikiLink({ target: "projects/other-proj/design" });
    // The chip is a <span> inside the link showing the project slug.
    expect(screen.getByText("other-proj")).toBeInTheDocument();
  });

  it("calls setSlug with the target project on click", () => {
    renderWikiLink({ target: "projects/other-proj/design" });
    fireEvent.click(screen.getByRole("link"));
    expect(mockSetSlug).toHaveBeenCalledWith("other-proj");
    expect(mockSetSlug).toHaveBeenCalledTimes(1);
  });

  it("shows label instead of key when alias provided", () => {
    renderWikiLink({ target: "projects/B/x", label: "B's design" });
    expect(screen.getByRole("link", { name: /B's design/ })).toBeInTheDocument();
  });

  it("applies wikilink-missing class when cross-project target is missing", () => {
    renderWikiLink({ target: "projects/ghost/note", missing: true });
    const link = screen.getByRole("link");
    expect(link.className).toContain("wikilink-cross");
    expect(link.className).toContain("wikilink-missing");
  });
});
