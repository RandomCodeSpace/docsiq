import { useState } from 'react'
import { Save, X } from 'lucide-react'

interface Props {
  noteKey: string
  initialContent?: string
  initialAuthor?: string
  initialTags?: string[]
  onSave: (data: { content: string; author: string; tags: string[] }) => Promise<void> | void
  onCancel: () => void
}

export default function NoteEditor({
  noteKey,
  initialContent = '',
  initialAuthor = '',
  initialTags = [],
  onSave,
  onCancel,
}: Props) {
  const [content, setContent] = useState(initialContent)
  const [author, setAuthor] = useState(initialAuthor)
  const [tagsInput, setTagsInput] = useState(initialTags.join(', '))
  const [saving, setSaving] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  const submit = async () => {
    setSaving(true)
    setErr(null)
    try {
      const tags = tagsInput
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean)
      await onSave({ content, author, tags })
    } catch (e) {
      setErr(String(e))
    } finally {
      setSaving(false)
    }
  }

  return (
    <article className="card" style={{ display: 'flex', flexDirection: 'column', minHeight: 0, padding: '1rem', gap: '0.7rem', overflow: 'hidden' }}>
      <header style={{ display: 'flex', alignItems: 'center', gap: '0.6rem', borderBottom: '1px solid var(--border)', paddingBottom: '0.5rem' }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: '0.68rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)' }}>Editing</div>
          <div style={{ fontSize: '0.95rem', fontWeight: 700, overflow: 'hidden', textOverflow: 'ellipsis' }}>{noteKey}</div>
        </div>
        <button type="button" className="theme-btn" onClick={onCancel} title="Cancel" style={{ padding: '0.35rem 0.6rem' }}>
          <X size={13} />
        </button>
        <button
          type="button"
          className="mc-send-btn"
          onClick={() => void submit()}
          disabled={saving || !content.trim()}
          style={{ padding: '0.4rem 0.8rem', display: 'flex', alignItems: 'center', gap: '0.3rem' }}
        >
          <Save size={13} /> {saving ? 'Saving…' : 'Save'}
        </button>
      </header>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.6rem' }}>
        <label style={{ fontSize: '0.72rem', color: 'var(--text-muted)', display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
          Author
          <input className="search-input" value={author} onChange={(e) => setAuthor(e.target.value)} placeholder="architect" />
        </label>
        <label style={{ fontSize: '0.72rem', color: 'var(--text-muted)', display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
          Tags (comma-separated)
          <input className="search-input" value={tagsInput} onChange={(e) => setTagsInput(e.target.value)} placeholder="design, api" />
        </label>
      </div>
      <label style={{ fontSize: '0.72rem', color: 'var(--text-muted)', display: 'flex', flexDirection: 'column', gap: '0.25rem', flex: 1, minHeight: 0 }}>
        Content (Markdown, `[[wikilinks]]` supported)
        <textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          spellCheck={false}
          style={{
            flex: 1,
            minHeight: 240,
            width: '100%',
            background: 'var(--bg-input)',
            color: 'var(--text-primary)',
            border: '1px solid var(--border)',
            borderRadius: 8,
            padding: '0.7rem',
            fontFamily: 'var(--font-mono, monospace)',
            fontSize: '0.82rem',
            lineHeight: 1.6,
            resize: 'vertical',
          }}
        />
      </label>
      {err && <div style={{ fontSize: '0.75rem', color: 'var(--accent-error)' }}>{err}</div>}
    </article>
  )
}
