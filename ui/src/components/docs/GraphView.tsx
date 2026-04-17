import { useMemo, useState } from 'react'
import { Orbit } from 'lucide-react'
import type { GraphNeighborhood, NotesGraph } from '@/types/api'
import type { GraphStatus } from '@/hooks/useGraph'

interface Props {
  graph: GraphNeighborhood | null
  loading: boolean
  error: string | null
  status: GraphStatus
  onLoad: (entity: string, depth: number) => void
  /** Optional notes graph for overlay mode. When present, a toggle button
   *  lets the user merge notes/wikilink edges into the same SVG. */
  notesGraph?: NotesGraph | null
}

const typeColors: Record<string, string> = {
  Person: '#f59e0b',
  Organization: '#3b82f6',
  Concept: '#a78bfa',
  Location: '#10b981',
  Event: '#f43f5e',
  Technology: '#06b6d4',
  Other: '#64748b',
}

export default function GraphView({ graph, loading, error, status, onLoad, notesGraph }: Props) {
  const [entity, setEntity] = useState('')
  const [depth, setDepth] = useState(2)
  const [showNotes, setShowNotes] = useState(false)

  const layout = useMemo(() => {
    if (!graph?.nodes?.length) return []
    const radius = 170
    return graph.nodes.map((node, index) => {
      const angle = (Math.PI * 2 * index) / graph.nodes.length
      return {
        ...node,
        x: 220 + Math.cos(angle) * radius,
        y: 220 + Math.sin(angle) * radius,
        color: typeColors[node.type] || typeColors.Other,
      }
    })
  }, [graph])

  const edgeMap = useMemo(() => {
    const byId = new Map(layout.map((node) => [node.id, node]))
    return (graph?.edges ?? []).map((edge) => ({ edge, from: byId.get(edge.from), to: byId.get(edge.to) })).filter((item) => item.from && item.to)
  }, [graph, layout])

  // Notes overlay: place note nodes on an outer ring so they visibly
  // separate from the entity neighborhood. Purely cosmetic — the two
  // graphs don't share an ID space, so edges only exist within each set.
  const notesLayout = useMemo(() => {
    if (!showNotes || !notesGraph?.nodes?.length) return []
    const radius = 210
    return notesGraph.nodes.map((node, index) => {
      const angle = (Math.PI * 2 * index) / notesGraph.nodes.length + Math.PI / 6
      return {
        id: node.id,
        label: node.label ?? node.key ?? node.id,
        x: 220 + Math.cos(angle) * radius,
        y: 220 + Math.sin(angle) * radius,
      }
    })
  }, [showNotes, notesGraph])

  const notesEdges = useMemo(() => {
    if (!showNotes || !notesGraph?.edges?.length) return []
    const byId = new Map(notesLayout.map((n) => [n.id, n]))
    return notesGraph.edges
      .map((e) => ({ edge: e, from: byId.get(e.from), to: byId.get(e.to) }))
      .filter((x) => x.from && x.to)
  }, [showNotes, notesGraph, notesLayout])

  return (
    <div style={{ display: 'grid', gridTemplateColumns: '320px minmax(0, 1fr)', gap: '1rem', minHeight: 0, flex: 1 }}>
      <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.9rem' }}>
        <div>
          <div style={{ fontSize: '0.74rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)', marginBottom: '0.35rem' }}>Neighborhood Graph</div>
          <div style={{ fontSize: '0.95rem', fontWeight: 700 }}>Inspect entity relationships</div>
        </div>
        <input className="search-input" style={{ paddingLeft: '10px' }} value={entity} onChange={(event) => setEntity(event.target.value)} placeholder="Entity name" spellCheck={false} />
        <label style={{ display: 'flex', flexDirection: 'column', gap: '0.35rem', fontSize: '0.74rem', color: 'var(--text-muted)' }}>
          Depth
          <input type="range" min={1} max={4} value={depth} onChange={(event) => setDepth(Number(event.target.value))} />
          <span className="badge">Depth {depth}</span>
        </label>
        <button className="mc-send-btn" disabled={loading || !entity.trim()} onClick={() => onLoad(entity, depth)}>
          {loading ? 'Loading…' : 'Load Graph'}
        </button>
        {notesGraph && (
          <button
            type="button"
            className="nav-link"
            onClick={() => setShowNotes((s) => !s)}
            style={{
              justifyContent: 'center',
              border: '1px solid var(--border)',
              borderRadius: 8,
              padding: '0.45rem 0.6rem',
              color: showNotes ? 'var(--accent-notes)' : 'var(--text-secondary)',
              borderColor: showNotes ? 'var(--accent-notes)' : 'var(--border)',
            }}
          >
            {showNotes ? 'Hide notes graph' : 'Show notes graph'}
          </button>
        )}
        {error && <div style={{ color: '#ef4444', fontSize: '0.74rem' }}>{error}</div>}
      </div>
      <div className="card" style={{ minHeight: 0, overflow: 'hidden', display: 'grid', gridTemplateRows: '1fr auto', gap: '0.9rem' }}>
        <div style={{ position: 'relative', minHeight: 420, overflow: 'hidden', borderRadius: 12, background: 'radial-gradient(circle at top, rgba(56,189,248,0.12), transparent 35%), linear-gradient(180deg, var(--bg-card), var(--bg-base))', border: '1px solid var(--border)' }}>
          {!graph?.nodes?.length && (
            <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--text-muted)', textAlign: 'center', padding: '1rem' }}>
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '0.5rem' }}>
                <Orbit size={16} />
                {status === 'no_entities' && <span>No entities indexed yet. Upload documents to build the graph.</span>}
                {status === 'not_found' && <span>Entity not found. Try a different name.</span>}
                {status === 'idle' && <span>Enter an entity name to explore its neighborhood.</span>}
                {(status === 'loaded' || status === 'error') && !graph?.nodes?.length && <span>No graph loaded</span>}
              </div>
            </div>
          )}
          {graph?.nodes?.length || notesLayout.length ? (
            <svg viewBox="0 0 440 440" style={{ width: '100%', height: '100%' }}>
              {edgeMap.map(({ edge, from, to }, index) => (
                <g key={`${edge.from}-${edge.to}-${index}`}>
                  <line x1={from!.x} y1={from!.y} x2={to!.x} y2={to!.y} stroke="var(--border-hover)" strokeWidth={1.5} />
                  {edge.label && <text x={(from!.x + to!.x) / 2} y={(from!.y + to!.y) / 2 - 4} fill="var(--text-muted)" fontSize="10" textAnchor="middle">{edge.label}</text>}
                </g>
              ))}
              {notesEdges.map(({ edge, from, to }, i) => (
                <line
                  key={`n-${edge.from}-${edge.to}-${i}`}
                  x1={from!.x}
                  y1={from!.y}
                  x2={to!.x}
                  y2={to!.y}
                  stroke="var(--accent-notes-dim)"
                  strokeWidth={1}
                  strokeDasharray="3 3"
                />
              ))}
              {layout.map((node) => (
                <g key={node.id}>
                  <circle cx={node.x} cy={node.y} r={14} fill={node.color} stroke="var(--bg-base)" strokeWidth={2} />
                  <text x={node.x} y={node.y + 28} fill="var(--text-primary)" fontSize="11" textAnchor="middle">{node.label.slice(0, 16)}</text>
                </g>
              ))}
              {notesLayout.map((node) => (
                <g key={`n-${node.id}`}>
                  <circle cx={node.x} cy={node.y} r={10} fill="var(--accent-notes)" stroke="var(--bg-base)" strokeWidth={2} />
                  <text x={node.x} y={node.y + 22} fill="var(--accent-notes)" fontSize="10" textAnchor="middle">
                    {node.label.split('/').pop()?.slice(0, 14)}
                  </text>
                </g>
              ))}
            </svg>
          ) : null}
        </div>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.45rem' }}>
          {layout.map((node) => (
            <span key={node.id} className="badge" style={{ borderColor: node.color, color: 'var(--text-secondary)' }}>{node.label}</span>
          ))}
        </div>
      </div>
    </div>
  )
}
