import { useCallback, useEffect, useState } from 'react'
import TopNav, { type DocsView } from '@/components/nav/TopNav'
import StatsPanel from '@/components/docs/StatsPanel'
import SearchPanel from '@/components/docs/SearchPanel'
import DocumentList from '@/components/docs/DocumentList'
import GraphView from '@/components/docs/GraphView'
import CommunityList from '@/components/docs/CommunityList'
import UploadPanel from '@/components/docs/UploadPanel'
import MCPConsole from '@/components/mcp/MCPConsole'
import JsonViewer from '@/components/shared/JsonViewer'
import UnifiedSearchPanel from '@/components/shared/UnifiedSearchPanel'
import FolderTree from '@/components/notes/FolderTree'
import NoteView from '@/components/notes/NoteView'
import NoteEditor from '@/components/notes/NoteEditor'
import LinkPanel from '@/components/notes/LinkPanel'
import NotesGraphView from '@/components/notes/NotesGraphView'
import NotesSearchPanel from '@/components/notes/NotesSearchPanel'
import { useTheme } from '@/hooks/useTheme'
import { useStats } from '@/hooks/useStats'
import { useSearch } from '@/hooks/useSearch'
import { useDocuments } from '@/hooks/useDocuments'
import { useGraph } from '@/hooks/useGraph'
import { useCommunities } from '@/hooks/useCommunities'
import { useUpload } from '@/hooks/useUpload'
import { useCurrentProject, useProjects } from '@/hooks/useProjects'
import {
  deleteNote,
  useNote,
  useNotesGraph,
  useNotesTree,
  writeNote,
} from '@/hooks/useNotes'

// ---- URL <-> tab sync ----------------------------------------------

const VALID_VIEWS: DocsView[] = [
  'overview', 'search', 'documents', 'graph', 'communities', 'upload', 'mcp', 'notes',
]

function readTabFromUrl(): DocsView {
  if (typeof window === 'undefined') return 'overview'
  const params = new URLSearchParams(window.location.search)
  const t = params.get('tab')
  // Legacy kgraph URLs used `tab=docs`; collapse that to the overview.
  if (t === 'docs') return 'overview'
  if (t && (VALID_VIEWS as string[]).includes(t)) return t as DocsView
  return 'overview'
}

function writeTabToUrl(view: DocsView) {
  const url = new URL(window.location.href)
  url.searchParams.set('tab', view)
  window.history.replaceState({}, '', url.toString())
}

type NotesView = 'tree' | 'search'

const VALID_NOTES_VIEWS: NotesView[] = ['tree', 'search']

function readNotesViewFromUrl(): NotesView {
  if (typeof window === 'undefined') return 'tree'
  const v = new URLSearchParams(window.location.search).get('view')
  return v && (VALID_NOTES_VIEWS as string[]).includes(v) ? (v as NotesView) : 'tree'
}

function writeNotesViewToUrl(v: NotesView) {
  const url = new URL(window.location.href)
  url.searchParams.set('view', v)
  window.history.replaceState({}, '', url.toString())
}

export default function App() {
  const { toggle } = useTheme()
  const { stats, loading: statsLoading, error: statsError } = useStats()
  const { results, loading: searchLoading, error: searchError, search } = useSearch()
  const { documents, docType, loading: docsLoading, selectType } = useDocuments()
  const { graph, loading: graphLoading, error: graphError, status: graphStatus, loadGraph, setNoEntities } = useGraph()
  const {
    communities, level, members, selectedId,
    loading: communitiesLoading, detailLoading, selectLevel, loadCommunity,
  } = useCommunities()
  const { jobId, status, messages, loading: uploadLoading, upload } = useUpload()

  // --- Multi-project state --------------------------------------------
  const { projects } = useProjects()
  const [currentProject, setCurrentProject] = useCurrentProject()

  // --- Tab state synced to URL ----------------------------------------
  const [view, setView] = useState<DocsView>(readTabFromUrl)
  const changeView = useCallback((next: DocsView) => {
    setView(next)
    writeTabToUrl(next)
  }, [])
  useEffect(() => {
    const onPop = () => setView(readTabFromUrl())
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  // --- Notes subsystem ------------------------------------------------
  const { tree, isLoading: treeLoading, reload: reloadTree } = useNotesTree(currentProject)
  const { graph: notesGraph, reload: reloadNotesGraph } = useNotesGraph(currentProject)
  const [activeNoteKey, setActiveNoteKey] = useState<string | null>(null)
  const [editMode, setEditMode] = useState(false)
  const [notesView, setNotesView] = useState<NotesView>(readNotesViewFromUrl)
  const changeNotesView = useCallback((next: NotesView) => {
    setNotesView(next)
    writeNotesViewToUrl(next)
  }, [])
  const { data: activeNote, isLoading: noteLoading, error: noteError, load: loadNote } = useNote(
    currentProject,
    activeNoteKey,
  )

  // Reset note selection + editor state whenever the project changes.
  useEffect(() => {
    setActiveNoteKey(null)
    setEditMode(false)
  }, [currentProject])

  const openNote = useCallback(
    (key: string) => {
      setActiveNoteKey(key)
      setEditMode(false)
      void loadNote(key)
      if (view !== 'notes') changeView('notes')
      if (notesView !== 'tree') changeNotesView('tree')
    },
    [loadNote, view, changeView, notesView, changeNotesView],
  )

  const createNote = useCallback(
    async (key: string, title: string, tags: string[]) => {
      const heading = title && title !== '.keep' ? title : key.split('/').pop() || key
      const body = `# ${heading}\n\n`
      await writeNote(currentProject, key, body, undefined, tags)
      reloadTree()
      reloadNotesGraph()
      setActiveNoteKey(key)
      setEditMode(false)
      void loadNote(key)
      if (notesView !== 'tree') changeNotesView('tree')
    },
    [currentProject, reloadTree, reloadNotesGraph, loadNote, notesView, changeNotesView],
  )

  const saveActiveNote = useCallback(
    async (data: { content: string; author: string; tags: string[] }) => {
      const key = activeNoteKey
      if (!key) return
      await writeNote(currentProject, key, data.content, data.author, data.tags)
      setEditMode(false)
      await loadNote(key)
      reloadTree()
      reloadNotesGraph()
    },
    [activeNoteKey, currentProject, loadNote, reloadTree, reloadNotesGraph],
  )

  const deleteActiveNote = useCallback(async () => {
    const key = activeNoteKey
    if (!key) return
    if (!window.confirm(`Delete note ${key}?`)) return
    await deleteNote(currentProject, key)
    setActiveNoteKey(null)
    reloadTree()
    reloadNotesGraph()
  }, [activeNoteKey, currentProject, reloadTree, reloadNotesGraph])

  // --- Docs graph auto-load (unchanged from pre-Phase-4) --------------
  useEffect(() => {
    if (!graph && !graphLoading && graphStatus === 'idle') {
      fetch('/api/entities?limit=1')
        .then((res) => (res.ok ? res.json() : null))
        .then((data) => {
          const first = Array.isArray(data) ? data[0] : null
          if (first?.name) {
            void loadGraph(first.name, 2)
          } else {
            setNoEntities()
          }
        })
        .catch(() => setNoEntities())
    }
  }, [graph, graphLoading, graphStatus, loadGraph, setNoEntities])

  return (
    <>
      <TopNav
        currentView={view}
        onViewChange={changeView}
        stats={stats}
        onThemeToggle={toggle}
        projects={projects}
        currentProject={currentProject}
        onProjectChange={setCurrentProject}
      />
      <main className="main-content" style={{ padding: '1rem', gap: '1rem' }}>
        {/* Unified search sits above every tab as a shortcut layer. */}
        <UnifiedSearchPanel
          project={currentProject}
          onOpenNote={openNote}
          onOpenDoc={() => changeView('search')}
          onOpenEntity={() => changeView('graph')}
        />

        {view === 'overview' && (
          <div style={{ display: 'grid', gridTemplateRows: 'auto auto 1fr', gap: '1rem', minHeight: 0 }}>
            <section className="card" style={{ display: 'grid', gridTemplateColumns: '1.35fr 0.65fr', gap: '1rem', overflow: 'hidden', position: 'relative' }}>
              <div style={{ position: 'absolute', inset: 0, background: 'radial-gradient(circle at top left, rgba(56,189,248,0.14), transparent 40%), linear-gradient(180deg, transparent, rgba(255,255,255,0.02))', pointerEvents: 'none' }} />
              <div style={{ position: 'relative', zIndex: 1 }}>
                <div style={{ fontSize: '0.74rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)', marginBottom: '0.5rem' }}>MCP Console Dashboard</div>
                <h1 style={{ fontSize: '2rem', lineHeight: 1.05, marginBottom: '0.75rem' }}>Search documents, inspect graph structure, browse project notes, and operate the ingestion pipeline from one surface.</h1>
                <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: 1.7, maxWidth: 620 }}>
                  DocsContext combines semantic retrieval, entity relationships, community summaries, wiki-style notes, and MCP tooling. This redesign unifies kgraph's notes subsystem into the docscontext console.
                </p>
              </div>
              <div style={{ position: 'relative', zIndex: 1, display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem' }}>
                {[
                  ['Retrieval', 'Local and global search'],
                  ['Graph', 'Neighborhood exploration'],
                  ['Notes', 'Wiki + wikilinks per project'],
                  ['Ingest', 'Drop files into the pipeline'],
                ].map(([title, copy]) => (
                  <div key={title} style={{ border: '1px solid var(--border)', borderRadius: 12, padding: '0.9rem', background: 'rgba(255,255,255,0.02)' }}>
                    <div style={{ fontSize: '0.76rem', fontWeight: 700, marginBottom: '0.35rem' }}>{title}</div>
                    <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', lineHeight: 1.5 }}>{copy}</div>
                  </div>
                ))}
              </div>
            </section>
            <StatsPanel stats={stats} loading={statsLoading} />
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', minHeight: 0 }}>
              <JsonViewer title="Stats response" value={stats ?? {}} defaultOpen />
              <JsonViewer title="Status" value={{ statsError, uploadStatus: status, graphLoaded: Boolean(graph?.nodes?.length), project: currentProject }} defaultOpen />
            </div>
          </div>
        )}

        {view === 'search' && (
          <SearchPanel loading={searchLoading} error={searchError} results={results} onSearch={(query, mode, topK) => void search({ query, mode, top_k: topK })} />
        )}

        {view === 'documents' && (
          <DocumentList documents={documents} docType={docType} loading={docsLoading} onFilter={selectType} />
        )}

        {view === 'graph' && (
          <GraphView
            graph={graph}
            loading={graphLoading}
            error={graphError}
            status={graphStatus}
            onLoad={(entity, depth) => void loadGraph(entity, depth)}
            notesGraph={notesGraph}
          />
        )}

        {view === 'communities' && (
          <CommunityList
            communities={communities}
            members={members}
            level={level}
            loading={communitiesLoading}
            detailLoading={detailLoading}
            selectedId={selectedId}
            onLevel={selectLevel}
            onSelect={(id) => void loadCommunity(id)}
          />
        )}

        {view === 'upload' && (
          <UploadPanel jobId={jobId} status={status} messages={messages} loading={uploadLoading} onUpload={(files) => void upload(files)} />
        )}

        {view === 'mcp' && <MCPConsole />}

        {view === 'notes' && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem', minHeight: 0, flex: 1 }}>
            <div style={{ display: 'flex', gap: '0.25rem', alignItems: 'center' }}>
              {(['tree', 'search'] as NotesView[]).map((v) => (
                <button
                  key={v}
                  type="button"
                  className={`nav-link${notesView === v ? ' active' : ''}`}
                  onClick={() => changeNotesView(v)}
                  style={{ fontSize: '0.72rem', padding: '0.3rem 0.7rem' }}
                >
                  {v === 'tree' ? 'Tree view' : 'Search'}
                </button>
              ))}
            </div>
            {notesView === 'tree' && (
              <div style={{ display: 'grid', gridTemplateColumns: '240px minmax(0, 1fr) 240px', gap: '1rem', minHeight: 0, flex: 1 }}>
                <FolderTree
                  tree={tree}
                  activeKey={activeNoteKey}
                  loading={treeLoading}
                  onSelect={openNote}
                  onReload={reloadTree}
                  onCreate={createNote}
                />
                <div style={{ display: 'grid', gridTemplateRows: '1fr auto', gap: '1rem', minHeight: 0 }}>
                  {editMode && activeNoteKey ? (
                    <NoteEditor
                      noteKey={activeNoteKey}
                      initialContent={activeNote?.note.content ?? ''}
                      initialAuthor={activeNote?.note.author ?? ''}
                      initialTags={activeNote?.note.tags ?? []}
                      onSave={saveActiveNote}
                      onCancel={() => setEditMode(false)}
                    />
                  ) : (
                    <NoteView
                      note={activeNote}
                      loading={noteLoading}
                      error={noteError}
                      onNavigate={openNote}
                      onEdit={() => setEditMode(true)}
                      onDelete={() => void deleteActiveNote()}
                    />
                  )}
                  <NotesGraphView
                    graph={notesGraph}
                    loading={false}
                    error={null}
                    onSelect={openNote}
                  />
                </div>
                <LinkPanel activeKey={activeNoteKey} graph={notesGraph} onNavigate={openNote} />
              </div>
            )}
            {notesView === 'search' && (
              <NotesSearchPanel project={currentProject} onOpenNote={openNote} />
            )}
          </div>
        )}
      </main>
      <footer className="status-bar">
        <div className="status-item"><span className="status-key">Q</span> semantic query surface</div>
        <div className="status-item"><span className="status-key">G</span> graph neighborhood</div>
        <div className="status-item"><span className="status-key">N</span> notes · {currentProject}</div>
        <div className="status-item"><span className="status-key">M</span> MCP tools</div>
      </footer>
    </>
  )
}
