import { describe, it, expect } from "vitest";
import { renderMarkdown } from "../markdown";

describe("renderMarkdown", () => {
  it("parses headings + paragraphs", () => {
    const parts = renderMarkdown("# Hello\n\nworld");
    expect(parts).toHaveLength(1);
    expect(parts[0].content).toMatch(/<h1/);
    expect(parts[0].content).toMatch(/<p>world/);
  });
  it("strips YAML frontmatter", () => {
    const parts = renderMarkdown("---\ntitle: hi\n---\n\nbody");
    expect(parts[0].content).toMatch(/<p>body/);
    expect(parts[0].content).not.toMatch(/title:/);
  });
  it("extracts plain wikilink", () => {
    const parts = renderMarkdown("see [[target]]!");
    const link = parts.find((p) => p.kind === "wikilink");
    expect(link?.content).toBe("target");
    expect(link?.label).toBeUndefined();
  });
  it("extracts aliased wikilink and renders alias", () => {
    const parts = renderMarkdown("see [[target|Alias]]!");
    const link = parts.find((p) => p.kind === "wikilink");
    expect(link?.content).toBe("target");
    expect(link?.label).toBe("Alias");
  });
  it("opens external links in new tab", () => {
    const parts = renderMarkdown("[g](https://example.com)");
    expect(parts[0].content).toMatch(/target="_blank"/);
    expect(parts[0].content).toMatch(/rel="noopener noreferrer"/);
  });
  it("adds loading=lazy to images", () => {
    const parts = renderMarkdown("![alt](/x.png)");
    expect(parts[0].content).toMatch(/loading="lazy"/);
  });
});
