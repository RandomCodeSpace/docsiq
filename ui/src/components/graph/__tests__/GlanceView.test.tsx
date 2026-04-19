import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { GlanceView } from "../GlanceView";

describe("GlanceView", () => {
  it("shows loading state on undefined", () => {
    const { getByText } = render(<GlanceView data={undefined} />);
    expect(getByText(/loading/i)).toBeInTheDocument();
  });
  it("renders N circles for N nodes (capped by maxNodes)", () => {
    const { container } = render(
      <GlanceView
        data={{
          nodes: Array.from({ length: 5 }, (_, i) => ({
            id: String(i), label: "n", kind: "entity",
          })),
          edges: [{ source: "0", target: "1" }],
        }}
      />,
    );
    expect(container.querySelectorAll("circle").length).toBe(5);
    expect(container.querySelectorAll("line").length).toBe(1);
  });
});
