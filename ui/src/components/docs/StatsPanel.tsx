import { Activity, Boxes, FileText, GitBranch, Network, Sparkles } from 'lucide-react'
import type { Stats } from '@/types/api'
import { fmt } from '@/lib/utils'

interface Props {
  stats: Stats | null
  loading?: boolean
}

const cards = [
  { key: 'documents', label: 'Documents', icon: FileText },
  { key: 'chunks', label: 'Chunks', icon: Boxes },
  { key: 'entities', label: 'Entities', icon: Sparkles },
  { key: 'relationships', label: 'Relations', icon: GitBranch },
  { key: 'communities', label: 'Communities', icon: Network },
  { key: 'embeddings', label: 'Embeddings', icon: Activity },
] as const

export default function StatsPanel({ stats, loading }: Props) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '0.9rem' }}>
      {cards.map(({ key, label, icon: Icon }) => (
        <div key={key} className="card" style={{ position: 'relative', overflow: 'hidden' }}>
          <div style={{ position: 'absolute', inset: '0 auto auto 0', width: 56, height: 56, borderRadius: '50%', background: 'radial-gradient(circle, var(--accent-glow), transparent 70%)', transform: 'translate(-22%, -22%)' }} />
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}>
            <span className="badge">{label}</span>
            <Icon size={15} style={{ color: 'var(--color-accent)' }} />
          </div>
          <div style={{ fontSize: '1.8rem', fontWeight: 700, lineHeight: 1 }}>
            {loading ? '…' : fmt(Number(stats?.[key] ?? 0))}
          </div>
        </div>
      ))}
    </div>
  )
}
