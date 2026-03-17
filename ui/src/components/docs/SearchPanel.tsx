import { useState } from 'react'
import { Search } from 'lucide-react'
import type { SearchMode, SearchResult } from '@/types/api'
import JsonViewer from '@/components/shared/JsonViewer'

interface Props {
  loading: boolean
  error: string | null
  results: SearchResult[]
  onSearch: (query: string, mode: SearchMode, topK: number) => void
}

export default function SearchPanel({ loading, error, results, onSearch }: Props) {
  const [query, setQuery] = useState('')
  const [mode, setMode] = useState<SearchMode>('local')
  const [topK, setTopK] = useState(8)

  const answer = results.find((item) => item.answer)?.answer
  const rows = results.filter((item) => !item.answer)

  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'minmax(320px, 440px) minmax(0, 1fr)', gap: '1rem', minHeight: 0, flex: 1 }}>
      <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.9rem' }}>
        <div>
          <div style={{ fontSize: '0.74rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)', marginBottom: '0.35rem' }}>Semantic Search</div>
          <div style={{ fontSize: '0.95rem', fontWeight: 700 }}>Local and community-level retrieval</div>
        </div>
        <div className="search-wrap" style={{ maxWidth: 'none' }}>
          <Search size={13} />
          <input className="search-input" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Find claims, entities, and relationships..." spellCheck={false} />
        </div>
        <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
          {(['local', 'global'] as const).map((item) => (
            <button key={item} className={`mode-pill${mode === item ? ' active' : ''}`} onClick={() => setMode(item)}>
              {item}
            </button>
          ))}
        </div>
        <label style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem', fontSize: '0.74rem', color: 'var(--text-muted)' }}>
          Result count
          <input type="range" min={3} max={20} value={topK} onChange={(event) => setTopK(Number(event.target.value))} />
          <span className="badge">Top {topK}</span>
        </label>
        <button className="mc-send-btn" disabled={loading || !query.trim()} onClick={() => onSearch(query, mode, topK)}>
          {loading ? 'Searching…' : 'Run Search'}
        </button>
        {error && <div style={{ color: '#ef4444', fontSize: '0.74rem' }}>{error}</div>}
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', minHeight: 0 }}>
        {answer && <JsonViewer title="Synthesized answer" value={{ answer }} defaultOpen />}
        <div className="card" style={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.8rem' }}>
            <div style={{ fontSize: '0.8rem', fontWeight: 700 }}>Results</div>
            <span className="badge">{rows.length}</span>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.7rem' }}>
            {rows.length === 0 && <div style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>No results yet.</div>}
            {rows.map((item, index) => (
              <article key={`${item.document_id ?? 'x'}-${index}`} style={{ border: '1px solid var(--border)', borderRadius: 10, padding: '0.85rem', background: 'var(--bg-card)' }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.75rem', marginBottom: '0.45rem' }}>
                  <div style={{ fontSize: '0.8rem', fontWeight: 600 }}>{item.title || item.path || `Match ${index + 1}`}</div>
                  {typeof item.score === 'number' && <span className="badge badge-blue">{item.score.toFixed(3)}</span>}
                </div>
                <div style={{ fontSize: '0.74rem', color: 'var(--text-secondary)', lineHeight: 1.6, whiteSpace: 'pre-wrap' }}>{item.text || item.chunk_text || 'No snippet returned.'}</div>
                {!!item.entities?.length && (
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.35rem', marginTop: '0.65rem' }}>
                    {item.entities.map((entity) => <span key={entity} className="badge">{entity}</span>)}
                  </div>
                )}
              </article>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
