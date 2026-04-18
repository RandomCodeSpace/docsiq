import { Link } from "react-router-dom";

interface Props { target: string; label?: string; }

export function WikiLink({ target, label }: Props) {
  return (
    <Link
      to={`/notes/${encodeURIComponent(target)}`}
      className="text-[var(--color-accent)] underline decoration-dotted hover:decoration-solid"
    >
      {label ?? target}
    </Link>
  );
}
