import type { Community, Entity } from '@/types/api'

interface Props {
  communities: Community[]
  members: Entity[]
  level: number
  loading: boolean
  detailLoading: boolean
  selectedId: string | null
  onLevel: (level: number) => void
  onSelect: (id: string) => void
}

const levels = [-1, 0, 1, 2, 3]

export default function CommunityList(props: Props) {
  const { communities, members, level, loading, detailLoading, selectedId, onLevel, onSelect } = props

  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'minmax(320px, 420px) minmax(0, 1fr)', gap: '1rem', minHeight: 0, flex: 1 }}>
      <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.9rem', minHeight: 0 }}>
        <div>
          <div style={{ fontSize: '0.74rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)', marginBottom: '0.35rem' }}>Communities</div>
          <div style={{ fontSize: '0.95rem', fontWeight: 700 }}>Cluster summaries by level</div>
        </div>
        <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap' }}>
          {levels.map((value) => (
            <button key={value} className={`mode-pill${level === value ? ' active' : ''}`} onClick={() => onLevel(value)}>
              {value < 0 ? 'all' : `L${value}`}
            </button>
          ))}
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.65rem', overflow: 'auto' }}>
          {loading && <div style={{ color: 'var(--text-muted)' }}>Loading communities…</div>}
          {communities.map((community) => (
            <button
              key={String(community.id)}
              onClick={() => onSelect(String(community.id))}
              className="card"
              style={{
                textAlign: 'left',
                background: selectedId === String(community.id) ? 'var(--nav-active-bg)' : 'var(--bg-card)',
                borderColor: selectedId === String(community.id) ? 'var(--color-accent)' : 'var(--border)',
                padding: '0.9rem',
                cursor: 'pointer',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.75rem', marginBottom: '0.4rem' }}>
                <div style={{ fontWeight: 700, fontSize: '0.8rem' }}>{community.title || `Community ${community.id}`}</div>
                <span className="badge">L{community.level}</span>
              </div>
              <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', lineHeight: 1.5 }}>{community.summary || 'No summary available.'}</div>
            </button>
          ))}
        </div>
      </div>
      <div className="card" style={{ minHeight: 0, overflow: 'auto' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.9rem' }}>
          <div style={{ fontSize: '0.85rem', fontWeight: 700 }}>Members</div>
          {detailLoading && <span className="badge">Loading…</span>}
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '0.75rem' }}>
          {members.length === 0 && <div style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>Select a community to inspect member entities.</div>}
          {members.map((member) => (
            <article key={String(member.id)} style={{ border: '1px solid var(--border)', borderRadius: 10, padding: '0.8rem', background: 'var(--bg-card)' }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.5rem', marginBottom: '0.35rem' }}>
                <div style={{ fontWeight: 700, fontSize: '0.78rem' }}>{member.name}</div>
                <span className="badge badge-orange">{member.type}</span>
              </div>
              <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', lineHeight: 1.5 }}>{member.description || 'No description available.'}</div>
            </article>
          ))}
        </div>
      </div>
    </div>
  )
}
