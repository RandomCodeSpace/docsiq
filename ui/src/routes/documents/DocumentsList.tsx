import { Link } from "react-router-dom";
import { useState } from "react";
import { useDocs } from "@/hooks/api/useDocs";
import { useProjectStore } from "@/stores/project";
import { formatRelativeTime } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { UploadModal } from "./UploadModal";

export default function DocumentsList() {
  const project = useProjectStore((s) => s.slug);
  const { data = [] } = useDocs(project);
  const [uploadOpen, setUploadOpen] = useState(false);

  if (data.length === 0) {
    return (
      <div className="docs-page">
        <div className="docs-page-head">
          <h1 className="docs-page-title">Documents</h1>
          <Button onClick={() => setUploadOpen(true)}>Upload</Button>
        </div>
        <UploadModal open={uploadOpen} onOpenChange={setUploadOpen} />
        <div className="docs-empty">
          <p className="docs-empty-title">No documents indexed yet.</p>
          <p className="docs-empty-desc">
            docsiq indexes from disk or URL into a GraphRAG knowledge base. Run the CLI against a
            folder of PDFs, Markdown, text, or a docs site.
          </p>
          <code className="docs-empty-code">docsiq index ~/path/to/docs --project {project}</code>
          <p className="docs-empty-hint">
            Requires an LLM provider (Azure OpenAI / OpenAI / Ollama) configured in
            <code className="mx-1 px-1 py-0.5 bg-muted rounded font-mono">~/.docsiq/config.yaml</code>.
            Notes and wikilinks work without a provider.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="docs-page">
      <div className="docs-page-head">
        <h1 className="docs-page-title">Documents</h1>
        <Button onClick={() => setUploadOpen(true)}>Upload</Button>
      </div>
      <UploadModal open={uploadOpen} onOpenChange={setUploadOpen} />
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Title</TableHead>
            <TableHead>Type</TableHead>
            <TableHead>Updated</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {data.map((d) => (
            <TableRow key={d.id}>
              <TableCell>
                <Link to={`/docs/${d.id}`} className="text-foreground underline decoration-dotted">
                  {d.title || d.path}
                </Link>
              </TableCell>
              <TableCell className="text-muted-foreground">{d.doc_type}</TableCell>
              <TableCell className="text-muted-foreground">
                {formatRelativeTime(d.updated_at * 1000)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
