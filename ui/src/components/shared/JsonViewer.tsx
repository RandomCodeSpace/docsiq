import { useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { colorJSON } from '@/lib/utils'

interface Props {
  title: string
  value: unknown
  defaultOpen?: boolean
}

export default function JsonViewer({ title, value, defaultOpen = false }: Props) {
  const [open, setOpen] = useState(defaultOpen)

  return (
    <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
      <button
        onClick={() => setOpen((v) => !v)}
        style={{
          width: '100%',
          display: 'flex',
          alignItems: 'center',
          gap: '0.5rem',
          padding: '0.85rem 1rem',
          background: 'transparent',
          border: 'none',
          borderBottom: open ? '1px solid var(--border)' : 'none',
          color: 'var(--text-primary)',
          cursor: 'pointer',
          textAlign: 'left',
        }}
      >
        {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        <span style={{ fontSize: '0.78rem', fontWeight: 600 }}>{title}</span>
      </button>
      {open && (
        <pre
          className="mc-code"
          style={{ margin: 0, padding: '1rem', border: 'none', borderRadius: 0, overflow: 'auto' }}
          dangerouslySetInnerHTML={{ __html: colorJSON(value) }}
        />
      )}
    </div>
  )
}
