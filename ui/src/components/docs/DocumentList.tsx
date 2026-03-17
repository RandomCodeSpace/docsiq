import type { Document } from '@/types/api'

interface Props {
  documents: Document[]
  docType: string
  loading: boolean
  onFilter: (docType: string) => void
}

const filters = ['', 'markdown', 'pdf', 'docx', 'txt', 'web']

export default function DocumentList({ documents, docType, loading, onFilter }: Props) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', minHeight: 0, flex: 1 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '1rem', flexWrap: 'wrap' }}>
        <div>
          <div style={{ fontSize: '0.74rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)', marginBottom: '0.35rem' }}>Corpus</div>
          <div style={{ fontSize: '0.95rem', fontWeight: 700 }}>Indexed documents and versions</div>
        </div>
        <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap' }}>
          {filters.map((filter) => (
            <button key={filter || 'all'} className={`mode-pill${docType === filter ? ' active' : ''}`} onClick={() => onFilter(filter)}>
              {filter || 'all'}
            </button>
          ))}
        </div>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '0.9rem', overflow: 'auto' }}>
        {loading && <div style={{ color: 'var(--text-muted)' }}>Loading documents…</div>}
        {!loading && documents.map((doc) => (
          <article key={String(doc.id)} className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.7rem' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.75rem' }}>
              <div style={{ fontSize: '0.82rem', fontWeight: 700 }}>{doc.title || doc.path}</div>
              <span className="badge badge-purple">{doc.doc_type || 'unknown'}</span>
            </div>
            <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', wordBreak: 'break-all' }}>{doc.path}</div>
            <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
              {'version' in doc && <span className="badge">v{String((doc as any).version)}</span>}
              {'is_latest' in doc && (doc as any).is_latest && <span className="badge badge-green">latest</span>}
            </div>
          </article>
        ))}
      </div>
    </div>
  )
}
