import { Pencil, Trash2 } from 'lucide-react'
import type { ReactNode } from 'react'
import type { NoteReadResponse } from '@/types/api'

interface Props {
  note: NoteReadResponse | null
  loading: boolean
  error: string | null
  onNavigate: (key: string) => void
  onEdit: () => void
  onDelete: () => void
}

/**
 * Minimal markdown renderer. We intentionally do NOT pull in
 * `react-markdown` / `remark` here — it would double the bundle size
 * for a fairly narrow feature. The rules cover headings, bold, italic,
 * inline code, fenced code, unordered list items, and wikilinks.
 */
function renderMarkdown(src: string, onNavigate: (key: string) => void): ReactNode[] {
  const out: ReactNode[] = []
  const lines = src.split('\n')
  let inCode = false
  let codeBuf: string[] = []

  const flushCode = (i: number) => {
    out.push(
      <pre key={`code-${i}`} style={{ background: 'var(--code-bg)', color: 'var(--code-text)', padding: '0.75rem', borderRadius: 6, overflow: 'auto', fontSize: '0.78rem', margin: '0.5rem 0' }}>
        <code>{codeBuf.join('\n')}</code>
      </pre>,
    )
    codeBuf = []
  }

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (line.startsWith('```')) {
      if (inCode) flushCode(i)
      inCode = !inCode
      continue
    }
    if (inCode) {
      codeBuf.push(line)
      continue
    }
    if (line.startsWith('# ')) {
      out.push(<h1 key={i} style={{ fontSize: '1.4rem', margin: '0.8rem 0 0.5rem' }}>{renderInline(line.slice(2), onNavigate, i)}</h1>)
    } else if (line.startsWith('## ')) {
      out.push(<h2 key={i} style={{ fontSize: '1.1rem', margin: '0.7rem 0 0.4rem' }}>{renderInline(line.slice(3), onNavigate, i)}</h2>)
    } else if (line.startsWith('### ')) {
      out.push(<h3 key={i} style={{ fontSize: '0.95rem', margin: '0.6rem 0 0.35rem' }}>{renderInline(line.slice(4), onNavigate, i)}</h3>)
    } else if (line.startsWith('- ') || line.startsWith('* ')) {
      out.push(<li key={i} style={{ marginLeft: '1.2rem', fontSize: '0.85rem', lineHeight: 1.7 }}>{renderInline(line.slice(2), onNavigate, i)}</li>)
    } else if (line.trim() === '') {
      out.push(<div key={i} style={{ height: '0.5rem' }} />)
    } else {
      out.push(<p key={i} style={{ fontSize: '0.85rem', lineHeight: 1.7, margin: '0.2rem 0' }}>{renderInline(line, onNavigate, i)}</p>)
    }
  }
  if (inCode) flushCode(lines.length)
  return out
}

function renderInline(src: string, onNavigate: (key: string) => void, baseKey: number): ReactNode[] {
  // Split on wikilinks first; then handle bold, italic, code inside non-link parts.
  const parts = src.split(/(\[\[[^\]]+\]\])/g)
  const nodes: ReactNode[] = []
  parts.forEach((part, i) => {
    const m = part.match(/^\[\[([^\]]+)\]\]$/)
    if (m) {
      const target = m[1]
      nodes.push(
        <a
          key={`${baseKey}-wl-${i}`}
          onClick={(e) => {
            e.preventDefault()
            onNavigate(target)
          }}
          style={{
            color: 'var(--accent-notes)',
            borderBottom: '1px dashed var(--accent-notes)',
            cursor: 'pointer',
            textDecoration: 'none',
          }}
        >
          {target.split('/').pop()}
        </a>,
      )
      return
    }
    // inline markdown: `code`, **bold**, *italic*
    const chunks = part.split(/(`[^`]+`|\*\*[^*]+\*\*|\*[^*]+\*)/g)
    chunks.forEach((c, j) => {
      if (!c) return
      if (c.startsWith('`') && c.endsWith('`')) {
        nodes.push(<code key={`${baseKey}-i-${i}-${j}`} style={{ background: 'var(--code-bg)', padding: '0.1rem 0.3rem', borderRadius: 4, fontSize: '0.8rem' }}>{c.slice(1, -1)}</code>)
      } else if (c.startsWith('**') && c.endsWith('**')) {
        nodes.push(<strong key={`${baseKey}-i-${i}-${j}`}>{c.slice(2, -2)}</strong>)
      } else if (c.startsWith('*') && c.endsWith('*')) {
        nodes.push(<em key={`${baseKey}-i-${i}-${j}`}>{c.slice(1, -1)}</em>)
      } else {
        nodes.push(<span key={`${baseKey}-i-${i}-${j}`}>{c}</span>)
      }
    })
  })
  return nodes
}

export default function NoteView({ note, loading, error, onNavigate, onEdit, onDelete }: Props) {
  if (loading) {
    return <div className="card" style={{ padding: '1rem', fontSize: '0.78rem', color: 'var(--text-muted)' }}>Loading note…</div>
  }
  if (error) {
    return <div className="card" style={{ padding: '1rem', fontSize: '0.78rem', color: 'var(--accent-error)' }}>Error: {error}</div>
  }
  if (!note) {
    return <div className="card" style={{ padding: '1rem', fontSize: '0.78rem', color: 'var(--text-muted)' }}>Select a note from the tree or create a new one.</div>
  }

  const { note: n } = note
  return (
    <article className="card" style={{ display: 'flex', flexDirection: 'column', minHeight: 0, padding: '1rem', gap: '0.6rem', overflow: 'hidden' }}>
      <header style={{ display: 'flex', alignItems: 'center', gap: '0.6rem', borderBottom: '1px solid var(--border)', paddingBottom: '0.6rem' }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: '0.68rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)' }}>Note</div>
          <div style={{ fontSize: '0.95rem', fontWeight: 700, overflow: 'hidden', textOverflow: 'ellipsis' }}>{n.key}</div>
          <div style={{ display: 'flex', gap: '0.4rem', marginTop: '0.3rem', flexWrap: 'wrap' }}>
            {n.author && <span className="badge">by {n.author}</span>}
            {n.tags?.map((t) => (
              <span key={t} className="badge" style={{ borderColor: 'var(--accent-notes)', color: 'var(--accent-notes)' }}>
                #{t}
              </span>
            ))}
          </div>
        </div>
        <button type="button" className="theme-btn" onClick={onEdit} title="Edit" style={{ padding: '0.35rem 0.6rem' }}>
          <Pencil size={13} />
        </button>
        <button type="button" className="theme-btn" onClick={onDelete} title="Delete" style={{ padding: '0.35rem 0.6rem', color: 'var(--accent-error)' }}>
          <Trash2 size={13} />
        </button>
      </header>
      <div style={{ overflow: 'auto', flex: 1, minHeight: 0 }}>{renderMarkdown(n.content ?? '', onNavigate)}</div>
    </article>
  )
}
