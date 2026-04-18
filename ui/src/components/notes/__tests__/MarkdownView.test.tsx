import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { MarkdownView } from "../MarkdownView";

describe("MarkdownView", () => {
  it("renders wikilinks as clickable router links with alias", () => {
    render(<MemoryRouter><MarkdownView source="pre [[target|Alias]] post" /></MemoryRouter>);
    const link = screen.getByRole("link", { name: "Alias" }) as HTMLAnchorElement;
    expect(link.getAttribute("href")).toBe("/notes/target");
  });
});
