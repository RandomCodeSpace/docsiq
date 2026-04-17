import { useCallback, useEffect, useRef, useState } from 'react'
import { ChevronDown, ChevronRight, FileText, Folder, FolderOpen, Plus, RefreshCw } from 'lucide-react'
import type { TreeNode } from '@/types/api'

interface Props {
  tree: TreeNode[]
  activeKey: string | null
  loading: boolean
  onSelect: (key: string) => void
  onReload: () => void
  onCreate: (key: string, title: string, tags: string[]) => Promise<void> | void
}

// ---------------------------------------------------------------------------
// Key validation (mirrors backend rules: no `..`, no absolute paths, no NULs).
// ---------------------------------------------------------------------------
export function validateKey(raw: string): string | null {
  const key = raw.trim()
  if (!key) return 'Key is required.'
  if (key.includes('\0')) return 'Key must not contain null bytes.'
  if (key.startsWith('/') || /^[A-Za-z]:[\\/]/.test(key)) return 'Absolute paths are not allowed.'
  const parts = key.split(/[\\/]+/)
  if (parts.some((p) => p === '..')) return 'Key must not contain `..` segments.'
  if (parts.some((p) => p === '.')) return 'Key must not contain `.` segments.'
  if (parts.some((p) => p === '')) return 'Key must not contain empty segments.'
  if (!/^[\w\-./]+$/.test(key)) return 'Use letters, numbers, -, _, ., /.'
  return null
}

function parseTags(raw: string): string[] {
  return raw
    .split(',')
    .map((t) => t.trim().replace(/^#/, ''))
    .filter(Boolean)
}

// ---------------------------------------------------------------------------
// Modal for "new note" — used both from header `+` button and context menu.
// ---------------------------------------------------------------------------
interface CreateModalProps {
  initialKey: string
  submitLabel: string
  heading: string
  onClose: () => void
  onSubmit: (key: string, title: string, tags: string[]) => Promise<void> | void
}

function CreateNoteModal({ initialKey, submitLabel, heading, onClose, onSubmit }: CreateModalProps) {
  const [key, setKey] = useState(initialKey)
  const [title, setTitle] = useState('')
  const [tags, setTags] = useState('')
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const keyInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const el = keyInputRef.current
    if (el) {
      el.focus()
      el.setSelectionRange(el.value.length, el.value.length)
    }
  }, [])

  const submit = useCallback(async () => {
    const vErr = validateKey(key)
    if (vErr) {
      setErr(vErr)
      return
    }
    setBusy(true)
    setErr(null)
    try {
      await onSubmit(key.trim(), title.trim(), parseTags(tags))
      onClose()
    } catch (e) {
      setErr(String(e instanceof Error ? e.message : e))
    } finally {
      setBusy(false)
    }
  }, [key, title, tags, onSubmit, onClose])

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.preventDefault()
      onClose()
    } else if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      void submit()
    }
  }

  return (
    <div
      onClick={onClose}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.55)',
        zIndex: 100,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        onKeyDown={onKeyDown}
        className="card"
        style={{
          width: 420,
          maxWidth: '90vw',
          padding: '1rem',
          display: 'flex',
          flexDirection: 'column',
          gap: '0.6rem',
          background: 'var(--bg-card)',
          border: '1px solid var(--border)',
          borderRadius: 10,
        }}
      >
        <div style={{ fontSize: '0.72rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--accent-notes)' }}>
          {heading}
        </div>
        <label style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem', fontSize: '0.72rem', color: 'var(--text-secondary)' }}>
          Key (path)
          <input
            ref={keyInputRef}
            value={key}
            onChange={(e) => setKey(e.target.value)}
            placeholder="architecture/auth"
            style={{
              background: 'var(--bg-input)',
              border: '1px solid var(--border)',
              borderRadius: 6,
              padding: '0.4rem 0.5rem',
              color: 'var(--text-primary)',
              fontSize: '0.82rem',
              fontFamily: 'ui-monospace, monospace',
            }}
          />
        </label>
        <label style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem', fontSize: '0.72rem', color: 'var(--text-secondary)' }}>
          Title
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Auth architecture"
            style={{
              background: 'var(--bg-input)',
              border: '1px solid var(--border)',
              borderRadius: 6,
              padding: '0.4rem 0.5rem',
              color: 'var(--text-primary)',
              fontSize: '0.82rem',
            }}
          />
        </label>
        <label style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem', fontSize: '0.72rem', color: 'var(--text-secondary)' }}>
          Tags (comma-separated)
          <input
            value={tags}
            onChange={(e) => setTags(e.target.value)}
            placeholder="design, security"
            style={{
              background: 'var(--bg-input)',
              border: '1px solid var(--border)',
              borderRadius: 6,
              padding: '0.4rem 0.5rem',
              color: 'var(--text-primary)',
              fontSize: '0.82rem',
            }}
          />
        </label>
        {err && (
          <div style={{ color: 'var(--accent-error)', fontSize: '0.72rem' }}>{err}</div>
        )}
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.4rem', marginTop: '0.3rem' }}>
          <button
            type="button"
            className="theme-btn"
            onClick={onClose}
            disabled={busy}
            style={{ padding: '0.35rem 0.75rem', fontSize: '0.75rem' }}
          >
            Cancel
          </button>
          <button
            type="button"
            className="theme-btn"
            onClick={() => void submit()}
            disabled={busy}
            style={{
              padding: '0.35rem 0.75rem',
              fontSize: '0.75rem',
              background: 'var(--accent-notes-dim)',
              borderColor: 'var(--accent-notes)',
              color: 'var(--accent-notes)',
            }}
          >
            {busy ? 'Saving…' : submitLabel}
          </button>
        </div>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Context menu — simple positioned panel.
// ---------------------------------------------------------------------------
interface MenuState {
  x: number
  y: number
  folderPath: string
}

function ContextMenu({
  state,
  onClose,
  onNewNote,
  onNewSubfolder,
}: {
  state: MenuState
  onClose: () => void
  onNewNote: (folderPath: string) => void
  onNewSubfolder: (folderPath: string) => void
}) {
  useEffect(() => {
    const handler = () => onClose()
    window.addEventListener('click', handler)
    window.addEventListener('scroll', handler, true)
    window.addEventListener('resize', handler)
    return () => {
      window.removeEventListener('click', handler)
      window.removeEventListener('scroll', handler, true)
      window.removeEventListener('resize', handler)
    }
  }, [onClose])

  return (
    <div
      role="menu"
      style={{
        position: 'fixed',
        top: state.y,
        left: state.x,
        background: 'var(--bg-card)',
        border: '1px solid var(--border)',
        borderRadius: 6,
        padding: '0.25rem',
        zIndex: 90,
        minWidth: 180,
        boxShadow: '0 4px 20px rgba(0,0,0,0.4)',
      }}
      onClick={(e) => e.stopPropagation()}
    >
      <button
        type="button"
        onClick={() => {
          onNewNote(state.folderPath)
          onClose()
        }}
        style={menuItemStyle}
      >
        New note here
      </button>
      <button
        type="button"
        onClick={() => {
          onNewSubfolder(state.folderPath)
          onClose()
        }}
        style={menuItemStyle}
      >
        New subfolder
      </button>
    </div>
  )
}

const menuItemStyle: React.CSSProperties = {
  width: '100%',
  textAlign: 'left',
  background: 'transparent',
  border: 'none',
  color: 'var(--text-secondary)',
  fontSize: '0.78rem',
  padding: '0.4rem 0.6rem',
  borderRadius: 4,
  cursor: 'pointer',
}

// ---------------------------------------------------------------------------
// Row — now supports long-press + contextmenu for folder creation UX.
// ---------------------------------------------------------------------------
interface RowProps {
  node: TreeNode
  depth: number
  expanded: Set<string>
  toggle: (path: string) => void
  activeKey: string | null
  onSelect: (key: string) => void
  onFolderMenu: (x: number, y: number, folderPath: string) => void
}

function Row({ node, depth, expanded, toggle, activeKey, onSelect, onFolderMenu }: RowProps) {
  const isFolder = node.type === 'folder'
  const isOpen = expanded.has(node.path)
  const isActive = !isFolder && node.path === activeKey

  const longPressTimer = useRef<number | null>(null)
  const cancelLongPress = () => {
    if (longPressTimer.current !== null) {
      window.clearTimeout(longPressTimer.current)
      longPressTimer.current = null
    }
  }

  const onContextMenu = (e: React.MouseEvent) => {
    if (!isFolder) return
    e.preventDefault()
    onFolderMenu(e.clientX, e.clientY, node.path)
  }

  const onTouchStart = (e: React.TouchEvent) => {
    if (!isFolder) return
    const t = e.touches[0]
    const x = t.clientX
    const y = t.clientY
    longPressTimer.current = window.setTimeout(() => {
      onFolderMenu(x, y, node.path)
    }, 500)
  }

  return (
    <div>
      <button
        type="button"
        onClick={() => {
          if (isFolder) toggle(node.path)
          else onSelect(node.path)
        }}
        onContextMenu={onContextMenu}
        onTouchStart={onTouchStart}
        onTouchEnd={cancelLongPress}
        onTouchMove={cancelLongPress}
        onTouchCancel={cancelLongPress}
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
              onFolderMenu={onFolderMenu}
            />
          ))}
        </div>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main component.
// ---------------------------------------------------------------------------
export default function FolderTree({ tree, activeKey, loading, onSelect, onReload, onCreate }: Props) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [menu, setMenu] = useState<MenuState | null>(null)
  const [createState, setCreateState] = useState<
    | { mode: 'note'; initialKey: string }
    | { mode: 'subfolder'; initialKey: string }
    | null
  >(null)

  const toggle = (path: string) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(path)) next.delete(path)
      else next.add(path)
      return next
    })
  }

  const openFolderMenu = useCallback((x: number, y: number, folderPath: string) => {
    setMenu({ x, y, folderPath })
  }, [])

  const onSubmitCreate = useCallback(
    async (key: string, title: string, tags: string[]) => {
      const effectiveTitle = title || key.split('/').pop() || key
      await onCreate(key, effectiveTitle, tags)
    },
    [onCreate],
  )

  return (
    <div className="card" style={{ display: 'flex', flexDirection: 'column', minHeight: 0, gap: '0.5rem', padding: '0.75rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div style={{ fontSize: '0.72rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)' }}>
          Notes
        </div>
        <div style={{ display: 'flex', gap: '0.25rem' }}>
          <button
            type="button"
            className="theme-btn"
            onClick={() => setCreateState({ mode: 'note', initialKey: '' })}
            title="New note"
            style={{ padding: '0.25rem', color: 'var(--accent-notes)' }}
          >
            <Plus size={12} />
          </button>
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
            onFolderMenu={openFolderMenu}
          />
        ))}
      </div>
      {menu && (
        <ContextMenu
          state={menu}
          onClose={() => setMenu(null)}
          onNewNote={(folderPath) => setCreateState({ mode: 'note', initialKey: `${folderPath}/` })}
          onNewSubfolder={(folderPath) => setCreateState({ mode: 'subfolder', initialKey: `${folderPath}/` })}
        />
      )}
      {createState && createState.mode === 'note' && (
        <CreateNoteModal
          initialKey={createState.initialKey}
          heading="New note"
          submitLabel="Create note"
          onClose={() => setCreateState(null)}
          onSubmit={onSubmitCreate}
        />
      )}
      {createState && createState.mode === 'subfolder' && (
        <CreateNoteModal
          initialKey={`${createState.initialKey}.keep`}
          heading="New subfolder (creates a .keep note)"
          submitLabel="Create subfolder"
          onClose={() => setCreateState(null)}
          onSubmit={onSubmitCreate}
        />
      )}
    </div>
  )
}
