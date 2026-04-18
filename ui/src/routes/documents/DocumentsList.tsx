import { Link } from "react-router-dom";
import { useState } from "react";
import { useDocs } from "@/hooks/api/useDocs";
import { useProjectStore } from "@/stores/project";
import { formatRelativeTime } from "@/lib/format";
import { UploadModal } from "./UploadModal";

export default function DocumentsList() {
  const project = useProjectStore((s) => s.slug);
  const { data = [] } = useDocs(project);
  const [uploadOpen, setUploadOpen] = useState(false);

  return (
    <div className="p-6 max-w-[1200px] mx-auto">
      <header className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-semibold">Documents</h1>
        <button
          onClick={() => setUploadOpen(true)}
          className="px-3 py-1.5 bg-[var(--color-accent)] text-[var(--color-accent-contrast)] rounded-md text-sm"
        >
          Upload
        </button>
      </header>
      <UploadModal open={uploadOpen} onOpenChange={setUploadOpen} />
      <table className="w-full text-sm font-mono border-collapse">
        <thead>
          <tr className="text-left text-xs uppercase tracking-wider text-[var(--color-text-muted)]">
            <th className="p-2">Title</th>
            <th className="p-2">Type</th>
            <th className="p-2">Updated</th>
          </tr>
        </thead>
        <tbody>
          {data.map((d) => (
            <tr key={d.id} className="border-t border-[var(--color-border)] hover:bg-[var(--color-surface-2)]">
              <td className="p-2">
                <Link to={`/docs/${d.id}`} className="text-[var(--color-text)] underline decoration-dotted">
                  {d.title || d.path}
                </Link>
              </td>
              <td className="p-2 text-[var(--color-text-muted)]">{d.doc_type}</td>
              <td className="p-2 text-[var(--color-text-muted)]">
                {formatRelativeTime(d.updated_at * 1000)}
              </td>
            </tr>
          ))}
          {data.length === 0 && (
            <tr><td colSpan={3} className="p-6 text-center text-sm text-[var(--color-text-muted)]">No documents indexed yet.</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
