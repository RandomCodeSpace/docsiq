import { renderMarkdown } from "@/lib/markdown";
import { WikiLink } from "./WikiLink";

export function MarkdownView({ source }: { source: string }) {
  const parts = renderMarkdown(source);
  return (
    <>
      {parts.map((p, i) =>
        p.kind === "html" ? (
          <div key={i} dangerouslySetInnerHTML={{ __html: p.content }} />
        ) : (
          <WikiLink key={i} target={p.content} label={p.label} />
        ),
      )}
    </>
  );
}
