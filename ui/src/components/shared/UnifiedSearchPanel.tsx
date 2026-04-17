import { useCallback, useState } from 'react'
import { Search } from 'lucide-react'
import type { NoteHit, SearchResult } from '@/types/api'

type UnifiedHit =
  | { kind: 'note'; key: string; snippet?: string; score?: number }
  | { kind: 'doc'; title: string; path?: string; text?: string; score?: number }
  | { kind: 'entity'; name: string; type?: string }

interface Props {
  project: string
  onOpenNote: (key: string) => void
  onOpenDoc: () => void
  onOpenEntity?: (name: string) => void
}

/**
 * UnifiedSearchPanel — single input; dispatches in parallel to the docs
 * search (`POST /api/search`, mode=local) and the notes FTS endpoint
 * (`GET /api/projects/{slug}/search`). Results are merged + tagged so
 * the user can jump straight to the relevant view.
 */
export default function UnifiedSearchPanel({ project, onOpenNote, onOpenDoc, onOpenEntity }: Props) {
  const [q, setQ] = useState('')
  const [loading, setLoading] = useState(false)
  const [hits, setHits] = useState<UnifiedHit[]>([])
  const [err, setErr] = useState<string | null>(null)

  const run = useCallback(async () => {
    const query = q.trim()
    if (!query) {
      setHits([])
      return
    }
    setLoading(true)
    setErr(null)
    try {
      const [docRes, noteRes] = await Promise.all([
        fetch('/api/search', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ query, mode: 'local', top_k: 6 }),
        })
          .then((r) => (r.ok ? r.json() : []))
          .catch(() => []),
        project
          ? fetch(`/api/projects/${encodeURIComponent(project)}/search?q=${encodeURIComponent(query)}&limit=6`)
              .then((r) => (r.ok ? r.json() : { hits: [] }))
              .catch(() => ({ hits: [] }))
          : Promise.resolve({ hits: [] }),
      ])

      const docRows: SearchResult[] = Array.isArray(docRes) ? docRes : (docRes.results ?? [])
      const noteRows: NoteHit[] = Array.isArray(noteRes?.hits) ? noteRes.hits : []

      const merged: UnifiedHit[] = []
      for (const n of noteRows) {
        merged.push({ kind: 'note', key: n.key, snippet: n.snippet, score: n.score })
      }
      for (const d of docRows) {
        if (d.answer) continue
        merged.push({
          kind: 'doc',
          title: d.title || d.path || 'Document',
          path: d.path,
          text: d.text || d.chunk_text,
          score: d.score,
        })
        // Entities reported by the doc result are exposed as secondary hits.
        for (const e of d.entities ?? []) {
          merged.push({ kind: 'entity', name: e })
        }
      }
      setHits(merged)
    } catch (e) {
      setErr(String(e))
    } finally {
      setLoading(false)
    }
  }, [project, q])

  const badge = (kind: UnifiedHit['kind']) => {
    const map: Record<UnifiedHit['kind'], { label: string; color: string }> = {
      note: { label: 'note', color: 'var(--accent-notes)' },
      doc: { label: 'doc', color: 'var(--color-accent)' },
      entity: { label: 'entity', color: 'var(--color-warn)' },
    }
    const cfg = map[kind]
    return (
      <span className="badge" style={{ borderColor: cfg.color, color: cfg.color, fontSize: '0.62rem' }}>
        [{cfg.label}]
      </span>
    )
  }

  return (
    <div
      className="card"
      style={{ display: 'flex', flexDirection: 'column', gap: '0.55rem', padding: '0.7rem' }}
    >
      <form
        onSubmit={(e) => {
          e.preventDefault()
          void run()
        }}
        style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}
      >
        <Search size={14} style={{ color: 'var(--text-muted)' }} />
        <input
          className="search-input"
          style={{ flex: 1 }}
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Unified search across docs + notes…"
          spellCheck={false}
        />
        <button type="submit" className="mc-send-btn" disabled={loading || !q.trim()} style={{ padding: '0.4rem 0.8rem' }}>
          {loading ? '…' : 'Search'}
        </button>
      </form>
      {err && <div style={{ fontSize: '0.72rem', color: 'var(--accent-error)' }}>{err}</div>}
      {hits.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.35rem', maxHeight: 240, overflow: 'auto' }}>
          {hits.map((hit, i) => {
            if (hit.kind === 'note') {
              return (
                <button
                  key={`n-${hit.key}-${i}`}
                  type="button"
                  onClick={() => onOpenNote(hit.key)}
                  className="nav-link"
                  style={{ textAlign: 'left', display: 'flex', gap: '0.5rem', alignItems: 'flex-start', border: '1px solid var(--border)', borderRadius: 6, padding: '0.4rem 0.55rem', background: 'var(--bg-card)' }}
                >
                  {badge('note')}
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: '0.78rem', fontWeight: 600 }}>{hit.key}</div>
                    {hit.snippet && <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)', lineHeight: 1.5 }}>{hit.snippet}</div>}
                  </div>
                </button>
              )
            }
            if (hit.kind === 'doc') {
              return (
                <button
                  key={`d-${hit.title}-${i}`}
                  type="button"
                  onClick={() => onOpenDoc()}
                  className="nav-link"
                  style={{ textAlign: 'left', display: 'flex', gap: '0.5rem', alignItems: 'flex-start', border: '1px solid var(--border)', borderRadius: 6, padding: '0.4rem 0.55rem', background: 'var(--bg-card)' }}
                >
                  {badge('doc')}
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: '0.78rem', fontWeight: 600 }}>{hit.title}</div>
                    {hit.text && <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)', lineHeight: 1.5, whiteSpace: 'pre-wrap' }}>{hit.text.slice(0, 160)}</div>}
                  </div>
                </button>
              )
            }
            return (
              <button
                key={`e-${hit.name}-${i}`}
                type="button"
                onClick={() => onOpenEntity?.(hit.name)}
                className="nav-link"
                style={{ textAlign: 'left', display: 'flex', gap: '0.5rem', alignItems: 'center', border: '1px solid var(--border)', borderRadius: 6, padding: '0.35rem 0.55rem', background: 'var(--bg-card)' }}
              >
                {badge('entity')}
                <span style={{ fontSize: '0.78rem' }}>{hit.name}</span>
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}
