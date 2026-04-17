import { BookOpen, Database, FileText, GitBranch, Moon, Network, Search, Sun, Upload, Workflow } from 'lucide-react'
import type { ProjectInfo, Stats } from '@/types/api'
import { fmt } from '@/lib/utils'

export type DocsView =
  | 'overview'
  | 'search'
  | 'documents'
  | 'graph'
  | 'communities'
  | 'upload'
  | 'mcp'
  | 'notes'

interface Props {
  currentView: DocsView
  onViewChange: (view: DocsView) => void
  stats: Stats | null
  onThemeToggle: () => void
  projects?: ProjectInfo[]
  currentProject?: string
  onProjectChange?: (slug: string) => void
}

const items: { view: DocsView; label: string; icon: typeof Search }[] = [
  { view: 'overview', label: 'Overview', icon: Database },
  { view: 'search', label: 'Search', icon: Search },
  { view: 'documents', label: 'Documents', icon: BookOpen },
  { view: 'graph', label: 'Graph', icon: GitBranch },
  { view: 'communities', label: 'Communities', icon: Network },
  { view: 'notes', label: 'Notes', icon: FileText },
  { view: 'upload', label: 'Upload', icon: Upload },
  { view: 'mcp', label: 'MCP Console', icon: Workflow },
]

export default function TopNav({
  currentView,
  onViewChange,
  stats,
  onThemeToggle,
  projects,
  currentProject,
  onProjectChange,
}: Props) {
  return (
    <nav className="top-nav">
      <a className="logo" href="/">
        <BookOpen size={17} style={{ color: 'var(--color-accent)', flexShrink: 0 }} />
        <span className="logo-mark">DOCSCONTEXT</span>
        <span className="logo-ver">knowledge graph</span>
      </a>

      {items.map(({ view, label, icon: Icon }) => (
        <button key={view} className={`nav-link${currentView === view ? ' active' : ''}`} onClick={() => onViewChange(view)}>
          <Icon size={13} /> {label}
        </button>
      ))}

      {projects && onProjectChange && (
        <select
          value={currentProject ?? '_default'}
          onChange={(e) => onProjectChange(e.target.value)}
          title="Active project"
          style={{
            marginLeft: '0.75rem',
            background: 'var(--bg-input)',
            color: 'var(--text-primary)',
            border: '1px solid var(--border)',
            borderRadius: 6,
            padding: '0.3rem 0.5rem',
            fontSize: '0.74rem',
          }}
        >
          {projects.map((p) => (
            <option key={p.slug} value={p.slug}>
              {p.name}
            </option>
          ))}
        </select>
      )}

      <div className="stats" style={{ marginLeft: 'auto' }}>
        <div className="stat">
          <span className="stat-val">{fmt(stats?.documents ?? 0)}</span>
          <span className="stat-lbl">Docs</span>
        </div>
        <div className="stat">
          <span className="stat-val">{fmt(stats?.entities ?? 0)}</span>
          <span className="stat-lbl">Entities</span>
        </div>
        <div className="stat">
          <span className="stat-val">{fmt(stats?.communities ?? 0)}</span>
          <span className="stat-lbl">Groups</span>
        </div>
      </div>

      <button className="theme-btn" onClick={onThemeToggle} title="Toggle theme" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Sun size={15} className="icon-sun" />
        <Moon size={15} className="icon-moon" />
      </button>
    </nav>
  )
}
