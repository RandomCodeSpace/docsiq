import { useMemo } from 'react'
import type { NotesGraph } from '@/types/api'

interface Props {
  graph: NotesGraph | null
  loading: boolean
  error: string | null
  onSelect: (key: string) => void
}

/**
 * NotesGraphView — SVG circle-layout of the notes wikilink graph.
 *
 * Decision: uses SVG (same approach as docs GraphView) rather than
 * vis-network. vis-network is vendored into ui/vendor/ but the docs
 * UI never actually loads it; keeping SVG avoids loading a ~900KB
 * vendor script just for this panel.
 */
export default function NotesGraphView({ graph, loading, error, onSelect }: Props) {
  const layout = useMemo(() => {
    const nodes = graph?.nodes ?? []
    if (!nodes.length) return []
    const radius = 170
    return nodes.map((node, index) => {
      const angle = (Math.PI * 2 * index) / nodes.length
      return {
        ...node,
        x: 220 + Math.cos(angle) * radius,
        y: 220 + Math.sin(angle) * radius,
      }
    })
  }, [graph])

  const edgeMap = useMemo(() => {
    const byId = new Map(layout.map((n) => [n.id, n]))
    return (graph?.edges ?? [])
      .map((e) => ({ edge: e, from: byId.get(e.from), to: byId.get(e.to) }))
      .filter((x) => x.from && x.to)
  }, [graph, layout])

  return (
    <div className="card" style={{ display: 'flex', flexDirection: 'column', minHeight: 0, padding: '0.75rem', gap: '0.6rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <div style={{ fontSize: '0.72rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)' }}>Notes graph</div>
          <div style={{ fontSize: '0.85rem', fontWeight: 700 }}>{layout.length} notes · {edgeMap.length} links</div>
        </div>
      </div>
      <div style={{ flex: 1, minHeight: 240, position: 'relative', background: 'var(--bg-base)', border: '1px solid var(--border)', borderRadius: 8, overflow: 'hidden' }}>
        {loading && (
          <div style={{ position: 'absolute', inset: 0, display: 'grid', placeItems: 'center', fontSize: '0.75rem', color: 'var(--text-muted)' }}>Loading…</div>
        )}
        {error && !loading && (
          <div style={{ position: 'absolute', inset: 0, display: 'grid', placeItems: 'center', fontSize: '0.75rem', color: 'var(--accent-error)' }}>{error}</div>
        )}
        {!loading && layout.length === 0 && (
          <div style={{ position: 'absolute', inset: 0, display: 'grid', placeItems: 'center', fontSize: '0.75rem', color: 'var(--text-muted)' }}>No wikilinks yet.</div>
        )}
        {layout.length > 0 && (
          <svg viewBox="0 0 440 440" style={{ width: '100%', height: '100%' }}>
            {edgeMap.map(({ edge, from, to }, i) => (
              <line
                key={`${edge.from}-${edge.to}-${i}`}
                x1={from!.x}
                y1={from!.y}
                x2={to!.x}
                y2={to!.y}
                stroke="var(--accent-notes-dim)"
                strokeWidth={1}
              />
            ))}
            {layout.map((node) => (
              <g key={node.id} style={{ cursor: 'pointer' }} onClick={() => onSelect(node.id)}>
                <circle cx={node.x} cy={node.y} r={12} fill="var(--accent-notes)" stroke="var(--bg-base)" strokeWidth={2} />
                <text x={node.x} y={node.y + 26} fill="var(--text-primary)" fontSize="10" textAnchor="middle">
                  {(node.label ?? node.key ?? node.id).split('/').pop()?.slice(0, 16)}
                </text>
              </g>
            ))}
          </svg>
        )}
      </div>
    </div>
  )
}
