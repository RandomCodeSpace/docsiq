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
        <div className="docs-empty">No documents indexed yet.</div>
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
