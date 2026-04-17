import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import UnifiedSearchPanel from '../UnifiedSearchPanel'

// ---------------------------------------------------------------------------
// Fetch mocking — the component calls fetch('/api/search') + fetch('/api/projects/{p}/search')
// ---------------------------------------------------------------------------
function mockFetch(handler: (url: string, init?: RequestInit) => unknown) {
  const fn = vi.fn(async (url: string, init?: RequestInit) => {
    const body = handler(url, init)
    return {
      ok: true,
      json: async () => body,
    } as Response
  })
  // @ts-expect-error assigning mock
  global.fetch = fn
  return fn
}

beforeEach(() => {
  // Default: both endpoints return empty shapes.
  mockFetch(() => ({}))
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('UnifiedSearchPanel', () => {
  it('dispatches parallel fetch to /api/search and /api/projects/{p}/search', async () => {
    const fetchFn = mockFetch((url) => {
      if (url === '/api/search') return { results: [] }
      return { hits: [] }
    })
    render(
      <UnifiedSearchPanel project="proj-1" onOpenNote={vi.fn()} onOpenDoc={vi.fn()} />,
    )
    await userEvent.type(screen.getByPlaceholderText(/unified search/i), 'hello')
    await userEvent.click(screen.getByRole('button', { name: /search/i }))

    await waitFor(() => {
      expect(fetchFn).toHaveBeenCalledTimes(2)
    })
    const urls = fetchFn.mock.calls.map((c) => c[0])
    expect(urls).toContain('/api/search')
    expect(urls.some((u) => String(u).startsWith('/api/projects/proj-1/search'))).toBe(true)
  })

  it('POSTs to /api/search with mode=local body', async () => {
    const fetchFn = mockFetch((url) => (url === '/api/search' ? { results: [] } : { hits: [] }))
    render(
      <UnifiedSearchPanel project="p" onOpenNote={vi.fn()} onOpenDoc={vi.fn()} />,
    )
    await userEvent.type(screen.getByPlaceholderText(/unified search/i), 'query1')
    await userEvent.click(screen.getByRole('button', { name: /search/i }))
    await waitFor(() => expect(fetchFn).toHaveBeenCalled())

    const docCall = fetchFn.mock.calls.find((c) => c[0] === '/api/search')!
    expect(docCall[1]).toMatchObject({ method: 'POST' })
    const body = JSON.parse((docCall[1] as RequestInit).body as string)
    expect(body).toMatchObject({ query: 'query1', mode: 'local' })
  })

  it('renders merged results with [note] / [doc] / [entity] labels visible', async () => {
    mockFetch((url) => {
      if (url === '/api/search') {
        return {
          results: [
            {
              title: 'Doc Title',
              path: '/x',
              text: 'body text',
              entities: ['Alice'],
              score: 0.7,
            },
          ],
        }
      }
      return {
        hits: [{ key: 'notes/x', snippet: 'snippet body' }],
      }
    })
    render(
      <UnifiedSearchPanel project="p" onOpenNote={vi.fn()} onOpenDoc={vi.fn()} onOpenEntity={vi.fn()} />,
    )
    await userEvent.type(screen.getByPlaceholderText(/unified search/i), 'term')
    await userEvent.click(screen.getByRole('button', { name: /search/i }))

    await waitFor(() => {
      expect(screen.getByText('[note]')).toBeInTheDocument()
    })
    expect(screen.getByText('[doc]')).toBeInTheDocument()
    expect(screen.getByText('[entity]')).toBeInTheDocument()
    // Titles / keys visible.
    expect(screen.getByText('notes/x')).toBeInTheDocument()
    expect(screen.getByText('Doc Title')).toBeInTheDocument()
    expect(screen.getByText('Alice')).toBeInTheDocument()
  })

  it('renders nothing (empty results state) when both endpoints return empty', async () => {
    mockFetch((url) => (url === '/api/search' ? { results: [] } : { hits: [] }))
    const { container } = render(
      <UnifiedSearchPanel project="p" onOpenNote={vi.fn()} onOpenDoc={vi.fn()} />,
    )
    await userEvent.type(screen.getByPlaceholderText(/unified search/i), 'nope')
    await userEvent.click(screen.getByRole('button', { name: /search/i }))
    // Wait for fetch to resolve; no hit buttons should be rendered.
    await waitFor(() => {
      // The results dropdown is not rendered when hits is empty.
      expect(container.querySelectorAll('button[type="button"]').length).toBe(0)
    })
  })

  it('clicking a note result calls onOpenNote(key)', async () => {
    const onOpenNote = vi.fn()
    mockFetch((url) => {
      if (url === '/api/search') return { results: [] }
      return { hits: [{ key: 'notes/click-me', snippet: 's' }] }
    })
    render(
      <UnifiedSearchPanel project="p" onOpenNote={onOpenNote} onOpenDoc={vi.fn()} />,
    )
    await userEvent.type(screen.getByPlaceholderText(/unified search/i), 'q')
    await userEvent.click(screen.getByRole('button', { name: /search/i }))
    await waitFor(() => expect(screen.getByText('notes/click-me')).toBeInTheDocument())
    const btn = screen.getByText('notes/click-me').closest('button')!
    await userEvent.click(btn)
    expect(onOpenNote).toHaveBeenCalledWith('notes/click-me')
  })

  it('clicking a doc result calls onOpenDoc', async () => {
    const onOpenDoc = vi.fn()
    mockFetch((url) => {
      if (url === '/api/search') {
        return { results: [{ title: 'D', path: '/d', text: 't' }] }
      }
      return { hits: [] }
    })
    render(
      <UnifiedSearchPanel project="p" onOpenNote={vi.fn()} onOpenDoc={onOpenDoc} />,
    )
    await userEvent.type(screen.getByPlaceholderText(/unified search/i), 'q')
    await userEvent.click(screen.getByRole('button', { name: /search/i }))
    await waitFor(() => expect(screen.getByText('D')).toBeInTheDocument())
    const btn = screen.getByText('D').closest('button')!
    await userEvent.click(btn)
    expect(onOpenDoc).toHaveBeenCalled()
  })
})
