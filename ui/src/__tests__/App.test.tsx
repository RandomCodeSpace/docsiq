import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// ---------------------------------------------------------------------------
// Mock every hook used by App so we can render it in isolation.
// Mocks must be declared BEFORE App is imported.
// ---------------------------------------------------------------------------
vi.mock('@/hooks/useTheme', () => ({
  useTheme: () => ({ toggle: vi.fn(), theme: 'dark' }),
}))

vi.mock('@/hooks/useStats', () => ({
  useStats: () => ({ stats: null, loading: false, error: null }),
}))

vi.mock('@/hooks/useSearch', () => ({
  useSearch: () => ({ results: [], loading: false, error: null, search: vi.fn() }),
}))

vi.mock('@/hooks/useDocuments', () => ({
  useDocuments: () => ({
    documents: [],
    docType: null,
    loading: false,
    selectType: vi.fn(),
  }),
}))

vi.mock('@/hooks/useGraph', () => ({
  useGraph: () => ({
    graph: null,
    loading: false,
    error: null,
    status: 'ready',
    loadGraph: vi.fn(),
    setNoEntities: vi.fn(),
  }),
}))

vi.mock('@/hooks/useCommunities', () => ({
  useCommunities: () => ({
    communities: [],
    level: 0,
    members: [],
    selectedId: null,
    loading: false,
    detailLoading: false,
    selectLevel: vi.fn(),
    loadCommunity: vi.fn(),
  }),
}))

vi.mock('@/hooks/useUpload', () => ({
  useUpload: () => ({
    jobId: null,
    status: 'idle',
    messages: [],
    loading: false,
    upload: vi.fn(),
  }),
}))

// useProjects provides projects + useCurrentProject hook.
// currentProject is exposed from a stateful wrapper to reflect changes.
const currentProjectBus = { value: '_default', setter: (_: string) => {} }
vi.mock('@/hooks/useProjects', () => {
  return {
    useProjects: () => ({
      projects: [
        { slug: '_default', name: '_default' },
        { slug: 'other', name: 'Other Project' },
      ],
      loading: false,
      error: null,
      reload: vi.fn(),
    }),
    useCurrentProject: () => {
      // Read URL directly so App reflects the current ?project= value on first render.
      const params = new URLSearchParams(window.location.search)
      const initial = params.get('project') ?? '_default'
      const [, force] = (globalThis as any).__useState_forceRef ?? [null, null]
      void force
      // Simulate a stateful setter that mutates the URL, like the real hook.
      const set = (next: string) => {
        const url = new URL(window.location.href)
        url.searchParams.set('project', next)
        window.history.replaceState({}, '', url.toString())
        currentProjectBus.value = next
      }
      currentProjectBus.setter = set
      return [initial, set] as const
    },
  }
})

vi.mock('@/hooks/useNotes', () => ({
  useNote: () => ({ key: null, data: null, isLoading: false, error: null, load: vi.fn(), setKey: vi.fn() }),
  useNotesGraph: () => ({ graph: null, isLoading: false, error: null, reload: vi.fn() }),
  useNotesTree: () => ({ tree: [], isLoading: false, error: null, reload: vi.fn() }),
  deleteNote: vi.fn(),
  writeNote: vi.fn(),
}))

// Child components stubbed to keep this test focused on App URL/state wiring.
vi.mock('@/components/docs/StatsPanel', () => ({ default: () => <div data-testid="stats-panel" /> }))
vi.mock('@/components/docs/SearchPanel', () => ({ default: () => <div data-testid="search-panel" /> }))
vi.mock('@/components/docs/DocumentList', () => ({ default: () => <div data-testid="doc-list" /> }))
vi.mock('@/components/docs/GraphView', () => ({ default: () => <div data-testid="graph-view" /> }))
vi.mock('@/components/docs/CommunityList', () => ({ default: () => <div data-testid="community-list" /> }))
vi.mock('@/components/docs/UploadPanel', () => ({ default: () => <div data-testid="upload" /> }))
vi.mock('@/components/mcp/MCPConsole', () => ({ default: () => <div data-testid="mcp" /> }))
vi.mock('@/components/shared/JsonViewer', () => ({ default: () => <div data-testid="json" /> }))
vi.mock('@/components/shared/UnifiedSearchPanel', () => ({
  default: (props: { project: string }) => (
    <div data-testid="unified-search" data-project={props.project} />
  ),
}))
vi.mock('@/components/notes/FolderTree', () => ({ default: () => <div data-testid="folder-tree" /> }))
vi.mock('@/components/notes/NoteView', () => ({ default: () => <div data-testid="note-view" /> }))
vi.mock('@/components/notes/NoteEditor', () => ({ default: () => <div data-testid="note-editor" /> }))
vi.mock('@/components/notes/LinkPanel', () => ({ default: () => <div data-testid="link-panel" /> }))
vi.mock('@/components/notes/NotesGraphView', () => ({ default: () => <div data-testid="notes-graph" /> }))
vi.mock('@/components/notes/NotesSearchPanel', () => ({ default: () => <div data-testid="notes-search" /> }))

// Import App AFTER all mocks.
import App from '../App'

// ---------------------------------------------------------------------------
// Test URL helpers
// ---------------------------------------------------------------------------
function setUrl(search: string) {
  window.history.replaceState({}, '', `/?${search.replace(/^\?/, '')}`)
}

beforeEach(() => {
  setUrl('')
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('App — URL <-> state sync', () => {
  it('reads initial tab from ?tab=notes', () => {
    setUrl('tab=notes')
    render(<App />)
    // Notes view renders the notes folder tree (our stub).
    expect(screen.getByTestId('folder-tree')).toBeInTheDocument()
  })

  it('defaults tab to overview when ?tab= missing', () => {
    render(<App />)
    // Overview view renders the stats panel.
    expect(screen.getByTestId('stats-panel')).toBeInTheDocument()
  })

  it('reads initial project from ?project=other', () => {
    setUrl('project=other')
    render(<App />)
    const unified = screen.getByTestId('unified-search')
    expect(unified.getAttribute('data-project')).toBe('other')
  })

  it('changing tab updates the URL via window.history', async () => {
    const replaceSpy = vi.spyOn(window.history, 'replaceState')
    render(<App />)
    await userEvent.click(screen.getByRole('button', { name: /notes/i }))
    // After clicking Notes, history.replaceState should have been called with a URL
    // whose ?tab= param is "notes".
    const calls = replaceSpy.mock.calls
    const newestUrl = String(calls[calls.length - 1][2])
    expect(newestUrl).toMatch(/tab=notes/)
  })

  it('changing project updates the URL ?project= param', async () => {
    const projects = [
      { slug: '_default', name: '_default' },
      { slug: 'other', name: 'Other Project' },
    ]
    render(<App />)
    const select = screen.getByTitle('Active project') as HTMLSelectElement
    const replaceSpy = vi.spyOn(window.history, 'replaceState')
    await act(async () => {
      await userEvent.selectOptions(select, 'other')
    })
    // One of the recent replaceState calls carries project=other.
    const urls = replaceSpy.mock.calls.map((c) => String(c[2]))
    expect(urls.some((u) => /project=other/.test(u))).toBe(true)
    void projects
  })
})
