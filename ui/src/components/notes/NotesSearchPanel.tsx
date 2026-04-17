import { useCallback, useEffect, useRef, useState } from 'react'
import { Search } from 'lucide-react'
import { useNotesSearch } from '@/hooks/useNotes'
import type { NoteHit } from '@/types/api'

interface Props {
  project: string
  onOpenNote: (key: string) => void
}

// Pull a display title: frontmatter `title:` if present, else first H1, else key.
function deriveTitle(key: string, snippet?: string): string {
  if (!snippet) return key
  const fm = snippet.match(/^title:\s*["']?([^"'\n]+)["']?/im)
  if (fm) return fm[1].trim()
  const h1 = snippet.match(/^#\s+(.+)$/m)
  if (h1) return h1[1].trim()
  return key
}

// Highlight matches inside a snippet. Splits the snippet on whitespace-delimited
// query terms and wraps matches in <mark>. Case-insensitive. Terms shorter than
// 2 chars are ignored to avoid noise.
function HighlightedSnippet({ text, query }: { text: string; query: string }) {
  if (!text) return null
  const terms = query
    .split(/\s+/)
    .map((t) => t.trim())
    .filter((t) => t.length >= 2)
  if (terms.length === 0) return <>{text}</>
  // Escape regex metacharacters in terms.
  const escaped = terms.map((t) => t.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
  const re = new RegExp(`(${escaped.join('|')})`, 'gi')
  const parts = text.split(re)
  return (
    <>
      {parts.map((p, i) =>
        re.test(p) ? (
          <mark
            key={i}
            style={{
              background: 'var(--accent-notes-dim)',
              color: 'var(--accent-notes)',
              padding: '0 2px',
              borderRadius: 2,
            }}
          >
            {p}
          </mark>
        ) : (
          <span key={i}>{p}</span>
        ),
      )}
    </>
  )
}

export default function NotesSearchPanel({ project, onOpenNote }: Props) {
  const { hits, isLoading, error, search } = useNotesSearch(project)
  const [query, setQuery] = useState('')
  const [lastQuery, setLastQuery] = useState('')
  const [elapsedMs, setElapsedMs] = useState<number | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  const doSearch = useCallback(async () => {
    const q = query.trim()
    if (!q) return
    const start = performance.now()
    await search(q, 50)
    setElapsedMs(Math.round(performance.now() - start))
    setLastQuery(q)
  }, [query, search])

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      void doSearch()
    }
  }

  return (
    <div className="card" style={{ display: 'flex', flexDirection: 'column', minHeight: 0, gap: '0.75rem', padding: '1rem', flex: 1 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', borderBottom: '1px solid var(--border)', paddingBottom: '0.6rem' }}>
        <div style={{ fontSize: '0.72rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--accent-notes)' }}>
          Notes search
        </div>
        <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>· {project}</div>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
        <Search size={14} style={{ color: 'var(--text-muted)' }} />
        <input
          ref={inputRef}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder="Search notes (FTS)…"
          style={{
            flex: 1,
            background: 'var(--bg-input)',
            border: '1px solid var(--border)',
            borderRadius: 6,
            padding: '0.45rem 0.6rem',
            color: 'var(--text-primary)',
            fontSize: '0.85rem',
          }}
        />
        <button
          type="button"
          className="theme-btn"
          onClick={() => void doSearch()}
          disabled={isLoading || !query.trim()}
          style={{ padding: '0.4rem 0.75rem', fontSize: '0.75rem' }}
        >
          {isLoading ? 'Searching…' : 'Search'}
        </button>
      </div>

      {lastQuery && (
        <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>
          {hits.length} result{hits.length === 1 ? '' : 's'} for “{lastQuery}”
          {typeof elapsedMs === 'number' && ` · ${elapsedMs} ms`}
        </div>
      )}

      {error && (
        <div style={{ color: 'var(--accent-error)', fontSize: '0.75rem' }}>Error: {error}</div>
      )}

      <div style={{ flex: 1, overflow: 'auto', minHeight: 0, display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
        {!lastQuery && !isLoading && (
          <div style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
            Type a query and press Enter. Notes are indexed with SQLite FTS5.
          </div>
        )}
        {lastQuery && hits.length === 0 && !isLoading && !error && (
          <div style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
            No notes match “{lastQuery}”.
          </div>
        )}
        {hits.map((hit: NoteHit) => {
          const title = deriveTitle(hit.key, hit.snippet)
          return (
            <button
              key={hit.key}
              type="button"
              onClick={() => onOpenNote(hit.key)}
              style={{
                textAlign: 'left',
                background: 'var(--bg-panel)',
                border: '1px solid var(--border)',
                borderRadius: 8,
                padding: '0.7rem 0.85rem',
                cursor: 'pointer',
                color: 'var(--text-secondary)',
                display: 'flex',
                flexDirection: 'column',
                gap: '0.3rem',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'baseline', gap: '0.5rem', justifyContent: 'space-between' }}>
                <div style={{ fontSize: '0.88rem', fontWeight: 600, color: 'var(--text-primary)' }}>{title}</div>
                {typeof hit.score === 'number' && (
                  <div style={{ fontSize: '0.65rem', color: 'var(--text-dim)' }}>
                    score {hit.score.toFixed(2)}
                  </div>
                )}
              </div>
              <div style={{ fontSize: '0.7rem', color: 'var(--accent-notes)', fontFamily: 'ui-monospace, monospace' }}>
                {hit.key}
              </div>
              {hit.snippet && (
                <div style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', lineHeight: 1.55, whiteSpace: 'pre-wrap' }}>
                  <HighlightedSnippet text={hit.snippet} query={lastQuery} />
                </div>
              )}
              {hit.tags && hit.tags.length > 0 && (
                <div style={{ display: 'flex', gap: '0.35rem', flexWrap: 'wrap', marginTop: '0.15rem' }}>
                  {hit.tags.map((t) => (
                    <span
                      key={t}
                      className="badge"
                      style={{ borderColor: 'var(--accent-notes)', color: 'var(--accent-notes)', fontSize: '0.62rem' }}
                    >
                      #{t}
                    </span>
                  ))}
                </div>
              )}
            </button>
          )
        })}
      </div>
    </div>
  )
}
