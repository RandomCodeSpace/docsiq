import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { NoteHit } from '@/types/api'

// ---------------------------------------------------------------------------
// Mock the useNotesSearch hook so the component doesn't touch the network.
// ---------------------------------------------------------------------------
const mockState: {
  hits: NoteHit[]
  isLoading: boolean
  error: string | null
  search: ReturnType<typeof vi.fn>
} = {
  hits: [],
  isLoading: false,
  error: null,
  search: vi.fn(),
}

vi.mock('@/hooks/useNotes', () => ({
  useNotesSearch: () => mockState,
}))
vi.mock('../../../hooks/useNotes', () => ({
  useNotesSearch: () => mockState,
}))

// Import AFTER vi.mock so the mock is active.
import NotesSearchPanel from '../NotesSearchPanel'

beforeEach(() => {
  mockState.hits = []
  mockState.isLoading = false
  mockState.error = null
  mockState.search = vi.fn().mockResolvedValue(undefined)
})

describe('NotesSearchPanel', () => {
  it('typing fires search when user submits via Enter', async () => {
    render(<NotesSearchPanel project="proj" onOpenNote={vi.fn()} />)
    const input = screen.getByPlaceholderText(/search notes/i)
    await userEvent.type(input, 'hello{Enter}')
    expect(mockState.search).toHaveBeenCalledWith('hello', 50)
  })

  it('clicking Search button also fires search', async () => {
    render(<NotesSearchPanel project="proj" onOpenNote={vi.fn()} />)
    await userEvent.type(screen.getByPlaceholderText(/search notes/i), 'query')
    await userEvent.click(screen.getByRole('button', { name: /search/i }))
    expect(mockState.search).toHaveBeenCalledWith('query', 50)
  })

  it('renders results list from mocked hits', async () => {
    mockState.hits = [
      { key: 'notes/a', snippet: 'Alpha content', score: 0.9 },
      { key: 'notes/b', snippet: 'Beta content', score: 0.5 },
    ]
    render(<NotesSearchPanel project="proj" onOpenNote={vi.fn()} />)
    // Trigger a search so lastQuery is set and hits render.
    await userEvent.type(screen.getByPlaceholderText(/search notes/i), 'foo{Enter}')
    // Each key appears twice (title fallback + monospace key row). Both hits rendered.
    expect(screen.getAllByText('notes/a').length).toBeGreaterThan(0)
    expect(screen.getAllByText('notes/b').length).toBeGreaterThan(0)
    // Snippet-level content uniquely identifies each row.
    expect(screen.getByText(/Alpha content/)).toBeInTheDocument()
    expect(screen.getByText(/Beta content/)).toBeInTheDocument()
  })

  it('highlights matching terms with <mark> tags in snippets', async () => {
    mockState.hits = [
      { key: 'notes/a', snippet: 'the alpha was here and alpha again' },
    ]
    const { container } = render(<NotesSearchPanel project="proj" onOpenNote={vi.fn()} />)
    await userEvent.type(screen.getByPlaceholderText(/search notes/i), 'alpha{Enter}')
    const marks = container.querySelectorAll('mark')
    // Two instances of "alpha" should be wrapped in <mark>.
    expect(marks.length).toBeGreaterThanOrEqual(2)
    marks.forEach((m) => expect(m.textContent?.toLowerCase()).toBe('alpha'))
  })

  it('clicking a result calls onOpenNote(key)', async () => {
    const onOpenNote = vi.fn()
    mockState.hits = [{ key: 'notes/target', snippet: 'body' }]
    render(<NotesSearchPanel project="proj" onOpenNote={onOpenNote} />)
    await userEvent.type(screen.getByPlaceholderText(/search notes/i), 'body{Enter}')
    // The monospace key row is unique; grab any occurrence and click the enclosing button.
    const matches = screen.getAllByText('notes/target')
    const resultBtn = matches[0].closest('button')!
    await userEvent.click(resultBtn)
    expect(onOpenNote).toHaveBeenCalledWith('notes/target')
  })

  it('shows result count and elapsed ms after a search', async () => {
    mockState.hits = [
      { key: 'a', snippet: 's' },
      { key: 'b', snippet: 's' },
      { key: 'c', snippet: 's' },
    ]
    render(<NotesSearchPanel project="proj" onOpenNote={vi.fn()} />)
    await userEvent.type(screen.getByPlaceholderText(/search notes/i), 'test{Enter}')
    // Count shown: "3 results for "test""
    expect(screen.getByText(/3 results for/i)).toBeInTheDocument()
    // Elapsed ms separator "· NNN ms" is appended.
    expect(screen.getByText(/\d+ ms/)).toBeInTheDocument()
  })

  it('shows "no match" empty state after search with zero hits', async () => {
    mockState.hits = []
    render(<NotesSearchPanel project="proj" onOpenNote={vi.fn()} />)
    await userEvent.type(screen.getByPlaceholderText(/search notes/i), 'nope{Enter}')
    expect(screen.getByText(/no notes match/i)).toBeInTheDocument()
  })
})
