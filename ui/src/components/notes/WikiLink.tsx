import { Link } from "react-router-dom";
import { useProjectStore } from "@/stores/project";

// Mirror of Go's ParseWikilink: targets matching projects/<slug>/<rest>
// are cross-project references.
const CROSS_RE = /^projects\/([a-zA-Z0-9_-]+)\/(.+)$/;

interface ParsedWikilink {
  crossProject: boolean;
  project: string;  // "" for same-project
  key: string;      // note key within the target project (or raw target)
  fullKey: string;  // the raw target string
}

function parseWikilink(target: string): ParsedWikilink {
  const m = CROSS_RE.exec(target);
  if (m) {
    return { crossProject: true, project: m[1], key: m[2], fullKey: target };
  }
  return { crossProject: false, project: "", key: target, fullKey: target };
}

interface Props { target: string; label?: string; missing?: boolean; }

export function WikiLink({ target, label, missing }: Props) {
  const setSlug = useProjectStore((s) => s.setSlug);
  const parsed = parseWikilink(target);

  const classNames = [
    "wikilink",
    parsed.crossProject ? "wikilink-cross" : "",
    missing ? "wikilink-missing" : "",
  ]
    .filter(Boolean)
    .join(" ");

  function handleClick() {
    if (parsed.crossProject) {
      setSlug(parsed.project);
    }
  }

  return (
    <Link
      to={`/notes/${encodeURIComponent(parsed.fullKey)}`}
      className={classNames}
      onClick={handleClick}
    >
      {parsed.crossProject && (
        <span className="wikilink-cross-chip">{parsed.project}</span>
      )}
      {label ?? parsed.key}
    </Link>
  );
}
