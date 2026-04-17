import { ArrowDownLeft, ArrowUpRight } from 'lucide-react'
import type { NotesGraph } from '@/types/api'

interface Props {
  activeKey: string | null
  graph: NotesGraph | null
  onNavigate: (key: string) => void
}

/**
 * LinkPanel — shows inbound + outbound wikilinks for the active note,
 * derived from the pre-built notes graph. Rendering happens off a
 * single graph fetch, so flipping between notes is instant.
 */
export default function LinkPanel({ activeKey, graph, onNavigate }: Props) {
  if (!activeKey) {
    return null
  }
  const nodes = graph?.nodes ?? []
  const edges = graph?.edges ?? []

  // node.id is the note key (server contract).
  const activeNode = nodes.find((n) => n.id === activeKey || n.key === activeKey)
  const activeId = activeNode?.id ?? activeKey

  const outlinks = edges.filter((e) => e.from === activeId).map((e) => e.to)
  const backlinks = edges.filter((e) => e.to === activeId).map((e) => e.from)

  const Section = ({ title, items, icon }: { title: string; items: string[]; icon: typeof ArrowUpRight }) => {
    const Icon = icon
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.3rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.35rem', fontSize: '0.68rem', textTransform: 'uppercase', letterSpacing: '0.1em', color: 'var(--text-dim)' }}>
          <Icon size={11} /> {title}
          <span className="badge" style={{ marginLeft: 'auto' }}>{items.length}</span>
        </div>
        {items.length === 0 ? (
          <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>None</div>
        ) : (
          items.map((k) => (
            <button
              key={`${title}-${k}`}
              type="button"
              onClick={() => onNavigate(k)}
              className="nav-link"
              style={{
                textAlign: 'left',
                fontSize: '0.76rem',
                padding: '0.3rem 0.5rem',
                border: '1px solid var(--border)',
                borderRadius: 6,
                background: 'var(--bg-card)',
                color: 'var(--text-secondary)',
              }}
            >
              {k.split('/').pop()}
              {k.includes('/') && (
                <span style={{ display: 'block', fontSize: '0.62rem', color: 'var(--text-dim)', marginTop: '0.1rem' }}>
                  {k.split('/').slice(0, -1).join('/')}
                </span>
              )}
            </button>
          ))
        )}
      </div>
    )
  }

  return (
    <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.8rem', padding: '0.75rem', minHeight: 0, overflow: 'auto' }}>
      <div style={{ fontSize: '0.72rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)' }}>Links</div>
      <Section title="Outbound" items={outlinks} icon={ArrowUpRight} />
      <Section title="Inbound" items={backlinks} icon={ArrowDownLeft} />
    </div>
  )
}
