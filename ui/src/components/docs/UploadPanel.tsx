import { useRef } from 'react'
import { UploadCloud } from 'lucide-react'

interface Props {
  jobId: string
  status: string
  messages: string[]
  loading: boolean
  onUpload: (files: File[]) => void
}

export default function UploadPanel({ jobId, status, messages, loading, onUpload }: Props) {
  const inputRef = useRef<HTMLInputElement | null>(null)

  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'minmax(320px, 420px) minmax(0, 1fr)', gap: '1rem', minHeight: 0, flex: 1 }}>
      <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.9rem', justifyContent: 'space-between' }}>
        <div>
          <div style={{ fontSize: '0.74rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)', marginBottom: '0.35rem' }}>Ingest</div>
          <div style={{ fontSize: '0.95rem', fontWeight: 700, marginBottom: '0.55rem' }}>Drop new source material into the graph</div>
          <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)', lineHeight: 1.6 }}>Upload markdown, PDFs, DOCX, text, and crawled web exports. The backend will chunk, embed, extract entities, and connect claims.</div>
        </div>
        <button
          onClick={() => inputRef.current?.click()}
          style={{
            minHeight: 180,
            borderRadius: 14,
            border: '1px dashed var(--border-hover)',
            background: 'linear-gradient(180deg, rgba(56,189,248,0.08), transparent 60%), var(--bg-card)',
            color: 'var(--text-primary)',
            cursor: 'pointer',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '0.75rem',
          }}
        >
          <UploadCloud size={24} style={{ color: 'var(--color-accent)' }} />
          <span style={{ fontWeight: 700 }}>{loading ? 'Uploading…' : 'Select files'}</span>
          <span style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>Click to browse or replace the current batch</span>
        </button>
        <input ref={inputRef} type="file" multiple style={{ display: 'none' }} onChange={(event) => onUpload(Array.from(event.target.files ?? []))} />
      </div>
      <div className="card" style={{ minHeight: 0, overflow: 'auto' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.85rem' }}>
          <div style={{ fontSize: '0.85rem', fontWeight: 700 }}>Job Progress</div>
          <span className="badge">{status}</span>
        </div>
        {jobId && <div style={{ marginBottom: '0.7rem', fontSize: '0.73rem', color: 'var(--text-muted)' }}>Job: <code>{jobId}</code></div>}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.55rem' }}>
          {messages.length === 0 && <div style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>No upload activity yet.</div>}
          {messages.map((message, index) => (
            <div key={`${message}-${index}`} style={{ padding: '0.7rem 0.85rem', border: '1px solid var(--border)', borderRadius: 10, background: 'var(--bg-card)', fontSize: '0.73rem', color: 'var(--text-secondary)' }}>
              {message}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
