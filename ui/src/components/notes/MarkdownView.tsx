import { renderMarkdown } from "@/lib/markdown";
import { WikiLink } from "./WikiLink";

export function MarkdownView({ source }: { source: string }) {
  const parts = renderMarkdown(source);
  return (
    <div className="prose-notes max-w-[620px]">
      {parts.map((p, i) =>
        p.kind === "html" ? (
          <div key={i} dangerouslySetInnerHTML={{ __html: p.content }} />
        ) : (
          <WikiLink key={i} target={p.content} label={p.label} />
        ),
      )}
    </div>
  );
}
