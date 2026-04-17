import { useState } from 'react'
import { ChevronDown, ChevronRight, FileText, Folder, FolderOpen, RefreshCw } from 'lucide-react'
import type { TreeNode } from '@/types/api'

interface Props {
  tree: TreeNode[]
  activeKey: string | null
  loading: boolean
  onSelect: (key: string) => void
  onReload: () => void
}

function Row({
  node,
  depth,
  expanded,
  toggle,
  activeKey,
  onSelect,
}: {
  node: TreeNode
  depth: number
  expanded: Set<string>
  toggle: (path: string) => void
  activeKey: string | null
  onSelect: (key: string) => void
}) {
  const isFolder = node.type === 'folder'
  const isOpen = expanded.has(node.path)
  // note keys are the `.md`-stripped path, which matches node.path for notes.
  const isActive = !isFolder && node.path === activeKey

  return (
    <div>
      <button
        type="button"
        onClick={() => {
          if (isFolder) toggle(node.path)
          else onSelect(node.path)
        }}
        className="nav-link"
        style={{
          width: '100%',
          justifyContent: 'flex-start',
          gap: '0.35rem',
          paddingLeft: `${0.4 + depth * 0.85}rem`,
          fontSize: '0.78rem',
          textAlign: 'left',
          background: isActive ? 'var(--nav-active-bg)' : 'transparent',
          color: isActive ? 'var(--accent-notes)' : 'var(--text-secondary)',
          borderBottom: 'none',
        }}
      >
        {isFolder ? (
          <>
            {isOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
            {isOpen ? <FolderOpen size={13} /> : <Folder size={13} />}
          </>
        ) : (
          <>
            <span style={{ width: 12 }} />
            <FileText size={13} style={{ color: 'var(--accent-notes)' }} />
          </>
        )}
        <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {node.name}
        </span>
        {!isFolder && typeof node.link_count === 'number' && node.link_count > 0 && (
          <span className="badge" style={{ marginLeft: 'auto', fontSize: '0.6rem' }}>
            {node.link_count}
          </span>
        )}
      </button>
      {isFolder && isOpen && node.children && (
        <div>
          {node.children.map((child) => (
            <Row
              key={child.path}
              node={child}
              depth={depth + 1}
              expanded={expanded}
              toggle={toggle}
              activeKey={activeKey}
              onSelect={onSelect}
            />
          ))}
        </div>
      )}
    </div>
  )
}

export default function FolderTree({ tree, activeKey, loading, onSelect, onReload }: Props) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set())

  const toggle = (path: string) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(path)) next.delete(path)
      else next.add(path)
      return next
    })
  }

  return (
    <div className="card" style={{ display: 'flex', flexDirection: 'column', minHeight: 0, gap: '0.5rem', padding: '0.75rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div style={{ fontSize: '0.72rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)' }}>
          Notes
        </div>
        <button
          type="button"
          className="theme-btn"
          onClick={onReload}
          title="Refresh tree"
          style={{ padding: '0.25rem' }}
        >
          <RefreshCw size={12} />
        </button>
      </div>
      <div style={{ flex: 1, overflow: 'auto', minHeight: 0 }}>
        {loading && tree.length === 0 && (
          <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)', padding: '0.5rem' }}>Loading…</div>
        )}
        {!loading && tree.length === 0 && (
          <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)', padding: '0.5rem' }}>No notes yet.</div>
        )}
        {tree.map((node) => (
          <Row
            key={node.path}
            node={node}
            depth={0}
            expanded={expanded}
            toggle={toggle}
            activeKey={activeKey}
            onSelect={onSelect}
          />
        ))}
      </div>
    </div>
  )
}
