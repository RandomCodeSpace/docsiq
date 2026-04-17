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

// ---------------------------------------------------------------------------
// Markdown renderer. Regex-based (no react-markdown). Covers:
//   headings, bold, italic, inline code, fenced code, unordered list items,
//   wikilinks, plus (new) links, images, blockquotes, tables, horizontal
//   rules, and inline math `\( ... \)` preserved as styled inline code.
//
// Skipped features (would need a real parser):
//   - Nested tables / nested blockquotes
//   - Ordered-list renumbering (we render `1. foo` as plain paragraphs)
//   - Task-list checkboxes (`- [ ]`)
//   - Setext headings (`====` underlines)
//   - Reference-style links
// ---------------------------------------------------------------------------

function isHorizontalRule(line: string): boolean {
  const s = line.trim()
  if (s.length < 3) return false
  return /^(?:-{3,}|\*{3,}|_{3,})$/.test(s)
}

function renderMarkdown(src: string, onNavigate: (key: string) => void): ReactNode[] {
  const out: ReactNode[] = []
  // Strip an optional YAML frontmatter block so our HR handling doesn't trip on the
  // leading `---`. Frontmatter uses `---` on line 1 and a matching `---` later.
  let working = src
  if (/^---\s*\r?\n/.test(working)) {
    const endRe = /\r?\n---\s*(?:\r?\n|$)/
    const m = endRe.exec(working)
    if (m && typeof m.index === 'number') {
      working = working.slice(m.index + m[0].length)
    }
  }
  const lines = working.split('\n')
  let inCode = false
  let codeBuf: string[] = []
  let i = 0

  const flushCode = (key: number) => {
    out.push(
      <pre key={`code-${key}`} style={{ background: 'var(--code-bg)', color: 'var(--code-text)', padding: '0.75rem', borderRadius: 6, overflow: 'auto', fontSize: '0.78rem', margin: '0.5rem 0' }}>
        <code>{codeBuf.join('\n')}</code>
      </pre>,
    )
    codeBuf = []
  }

  while (i < lines.length) {
    const line = lines[i]

    // Fenced code
    if (line.startsWith('```')) {
      if (inCode) flushCode(i)
      inCode = !inCode
      i += 1
      continue
    }
    if (inCode) {
      codeBuf.push(line)
      i += 1
      continue
    }

    // Horizontal rule
    if (isHorizontalRule(line)) {
      out.push(
        <hr
          key={i}
          style={{
            border: 'none',
            borderTop: '1px solid var(--border)',
            margin: '0.9rem 0',
          }}
        />,
      )
      i += 1
      continue
    }

    // Blockquote — collapse consecutive `>` lines into one <blockquote>.
    if (line.startsWith('>')) {
      const start = i
      const buf: string[] = []
      while (i < lines.length && lines[i].startsWith('>')) {
        buf.push(lines[i].replace(/^>\s?/, ''))
        i += 1
      }
      out.push(
        <blockquote
          key={`bq-${start}`}
          style={{
            borderLeft: '3px solid var(--accent-notes)',
            paddingLeft: '0.75rem',
            margin: '0.5rem 0',
            color: 'var(--text-secondary)',
            fontStyle: 'italic',
          }}
        >
          {buf.map((l, j) => (
            <div key={`${start}-${j}`} style={{ fontSize: '0.85rem', lineHeight: 1.7 }}>
              {renderInline(l, onNavigate, start * 1000 + j)}
            </div>
          ))}
        </blockquote>,
      )
      continue
    }

    // Table: `| h1 | h2 |\n| --- | --- |\n| c1 | c2 |`
    if (line.trim().startsWith('|') && i + 1 < lines.length && isTableSeparator(lines[i + 1])) {
      const tableStart = i
      const header = splitTableRow(line)
      i += 2 // skip header + separator
      const rows: string[][] = []
      while (i < lines.length && lines[i].trim().startsWith('|')) {
        rows.push(splitTableRow(lines[i]))
        i += 1
      }
      out.push(
        <div key={`tbl-${tableStart}`} style={{ overflow: 'auto', margin: '0.5rem 0' }}>
          <table style={{ borderCollapse: 'collapse', fontSize: '0.8rem', minWidth: '100%' }}>
            <thead>
              <tr>
                {header.map((cell, j) => (
                  <th
                    key={j}
                    style={{
                      borderBottom: '1px solid var(--border-strong)',
                      padding: '0.35rem 0.6rem',
                      textAlign: 'left',
                      color: 'var(--text-primary)',
                      fontWeight: 600,
                      background: 'var(--bg-card)',
                    }}
                  >
                    {renderInline(cell, onNavigate, tableStart * 1000 + j)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((row, r) => (
                <tr key={r}>
                  {row.map((cell, c) => (
                    <td
                      key={c}
                      style={{
                        borderBottom: '1px solid var(--border)',
                        padding: '0.3rem 0.6rem',
                        color: 'var(--text-secondary)',
                      }}
                    >
                      {renderInline(cell, onNavigate, tableStart * 10000 + r * 100 + c)}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>,
      )
      continue
    }

    // Headings
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
    i += 1
  }
  if (inCode) flushCode(lines.length)
  return out
}

// Table helpers ---------------------------------------------------------
function splitTableRow(line: string): string[] {
  let trimmed = line.trim()
  if (trimmed.startsWith('|')) trimmed = trimmed.slice(1)
  if (trimmed.endsWith('|')) trimmed = trimmed.slice(0, -1)
  return trimmed.split('|').map((c) => c.trim())
}

function isTableSeparator(line: string): boolean {
  const cells = splitTableRow(line)
  if (cells.length === 0) return false
  return cells.every((c) => /^:?-{3,}:?$/.test(c) || /^:?-+:?$/.test(c))
}

// ---------------------------------------------------------------------------
// Inline renderer: wikilinks, images, links, inline math, code, bold, italic.
// ---------------------------------------------------------------------------
function renderInline(src: string, onNavigate: (key: string) => void, baseKey: number): ReactNode[] {
  // Tokenize in order: wikilinks, images, links, inline math. We split by a
  // single combined regex so each top-level token is preserved.
  //
  // Order matters: images (`![..](..)`) must come before links (`[..](..)`).
  const topRe = /(\[\[[^\]]+\]\]|!\[[^\]]*\]\([^)\s]+\)|\[[^\]]+\]\([^)\s]+\)|\\\([^)]*?\\\))/g
  const parts = src.split(topRe)
  const nodes: ReactNode[] = []

  parts.forEach((part, i) => {
    if (!part) return

    // Wikilink
    const wl = part.match(/^\[\[([^\]]+)\]\]$/)
    if (wl) {
      const target = wl[1]
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

    // Image
    const img = part.match(/^!\[([^\]]*)\]\(([^)\s]+)\)$/)
    if (img) {
      const [, alt, url] = img
      nodes.push(
        <img
          key={`${baseKey}-img-${i}`}
          src={url}
          alt={alt}
          loading="lazy"
          style={{ maxWidth: '100%', height: 'auto', borderRadius: 4, display: 'block', margin: '0.3rem 0' }}
        />,
      )
      return
    }

    // Link
    const lnk = part.match(/^\[([^\]]+)\]\(([^)\s]+)\)$/)
    if (lnk) {
      const [, text, url] = lnk
      nodes.push(
        <a
          key={`${baseKey}-lnk-${i}`}
          href={url}
          target="_blank"
          rel="noopener noreferrer"
          style={{ color: 'var(--color-accent)', textDecoration: 'underline' }}
        >
          {text}
        </a>,
      )
      return
    }

    // Inline math \( ... \)
    const math = part.match(/^\\\((.*?)\\\)$/)
    if (math) {
      nodes.push(
        <code
          key={`${baseKey}-math-${i}`}
          className="math-inline"
          style={{
            background: 'var(--accent-notes-dim)',
            color: 'var(--accent-notes)',
            padding: '0.1rem 0.3rem',
            borderRadius: 4,
            fontSize: '0.8rem',
            fontFamily: 'ui-monospace, monospace',
          }}
        >
          {math[1]}
        </code>,
      )
      return
    }

    // Fallback: inline `code`, **bold**, *italic*
    const chunks = part.split(/(`[^`]+`|\*\*[^*]+\*\*|\*[^*]+\*)/g)
    chunks.forEach((c, j) => {
      if (!c) return
      if (c.startsWith('`') && c.endsWith('`') && c.length >= 2) {
        nodes.push(<code key={`${baseKey}-i-${i}-${j}`} style={{ background: 'var(--code-bg)', padding: '0.1rem 0.3rem', borderRadius: 4, fontSize: '0.8rem' }}>{c.slice(1, -1)}</code>)
      } else if (c.startsWith('**') && c.endsWith('**') && c.length >= 4) {
        nodes.push(<strong key={`${baseKey}-i-${i}-${j}`}>{c.slice(2, -2)}</strong>)
      } else if (c.startsWith('*') && c.endsWith('*') && c.length >= 2) {
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
