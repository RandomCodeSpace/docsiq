import MarkdownIt from "markdown-it";

export interface MarkdownPart {
  kind: "html" | "wikilink";
  content: string;
  label?: string;
}

const WIKILINK = /\[\[([^\]|]+?)(?:\|([^\]]+))?\]\]/g;

export function createMd() {
  const md = new MarkdownIt({ html: false, linkify: true, breaks: false });
  const defaultRender = md.renderer.rules.link_open ||
    ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options));
  md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
    const href = tokens[idx].attrGet("href") ?? "";
    if (/^https?:\/\//.test(href)) {
      tokens[idx].attrSet("target", "_blank");
      tokens[idx].attrSet("rel", "noopener noreferrer");
    }
    return defaultRender(tokens, idx, options, env, self);
  };
  const defaultImg = md.renderer.rules.image ||
    ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options));
  md.renderer.rules.image = (tokens, idx, options, env, self) => {
    tokens[idx].attrSet("loading", "lazy");
    return defaultImg(tokens, idx, options, env, self);
  };
  return md;
}

const md = createMd();

export function renderMarkdown(source: string): MarkdownPart[] {
  let body = source;
  if (body.startsWith("---\n")) {
    const end = body.indexOf("\n---", 4);
    if (end > 0) body = body.slice(end + 4).replace(/^\n/, "");
  }

  const parts: MarkdownPart[] = [];
  let lastIndex = 0;
  for (const m of body.matchAll(WIKILINK)) {
    const idx = m.index ?? 0;
    if (idx > lastIndex) {
      parts.push({ kind: "html", content: md.render(body.slice(lastIndex, idx)) });
    }
    parts.push({ kind: "wikilink", content: m[1].trim(), label: m[2]?.trim() });
    lastIndex = idx + m[0].length;
  }
  if (lastIndex < body.length) {
    parts.push({ kind: "html", content: md.render(body.slice(lastIndex)) });
  }
  return parts;
}
