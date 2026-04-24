import { renderHook } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, it, expect, afterEach } from "vitest";
import { useDocumentTitle } from "../useDocumentTitle";

function Wrapper({
  path,
  url,
  parts,
}: {
  path: string;
  url: string;
  parts?: string[];
}) {
  function Inner() {
    useDocumentTitle(parts);
    return null;
  }
  return (
    <MemoryRouter initialEntries={[url]}>
      <Routes>
        <Route path={path} element={<Inner />} />
      </Routes>
    </MemoryRouter>
  );
}

describe("useDocumentTitle", () => {
  afterEach(() => {
    document.title = "docsiq";
  });

  it("sets Home title", () => {
    renderHook(() => {}, { wrapper: () => <Wrapper path="/" url="/" /> });
    expect(document.title).toBe("Home — docsiq");
  });

  it("sets Notes list title", () => {
    renderHook(() => {}, { wrapper: () => <Wrapper path="/notes" url="/notes" /> });
    expect(document.title).toBe("Notes — docsiq");
  });

  it("sets note-key title from the URL when no parts passed", () => {
    renderHook(() => {}, {
      wrapper: () => (
        <Wrapper path="/notes/:key" url="/notes/folder%2Fhello" />
      ),
    });
    expect(document.title).toBe("hello — docsiq");
  });

  it("sets Documents list title", () => {
    renderHook(() => {}, { wrapper: () => <Wrapper path="/docs" url="/docs" /> });
    expect(document.title).toBe("Documents — docsiq");
  });

  it("honours caller-provided parts for a document view", () => {
    renderHook(() => {}, {
      wrapper: () => (
        <Wrapper
          path="/docs/:id"
          url="/docs/abc"
          parts={["Design doc v2", "Documents"]}
        />
      ),
    });
    expect(document.title).toBe("Design doc v2 — Documents — docsiq");
  });

  it("sets Graph title", () => {
    renderHook(() => {}, { wrapper: () => <Wrapper path="/graph" url="/graph" /> });
    expect(document.title).toBe("Graph — docsiq");
  });

  it("sets MCP Console title", () => {
    renderHook(() => {}, { wrapper: () => <Wrapper path="/mcp" url="/mcp" /> });
    expect(document.title).toBe("MCP Console — docsiq");
  });

  it("falls back to `docsiq` on unknown paths with no parts", () => {
    renderHook(() => {}, {
      wrapper: () => <Wrapper path="/weird" url="/weird" />,
    });
    expect(document.title).toBe("docsiq");
  });
});
