import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import {
  useNotes,
  useNotesGraph,
  useNotesTree,
  useNotesSearch,
  writeNote,
  deleteNote,
} from '../useNotes'

beforeEach(() => {
  global.fetch = vi.fn()
})
afterEach(() => {
  vi.restoreAllMocks()
})

function mockFetch(status: number, body: unknown) {
  ;(global.fetch as any).mockResolvedValueOnce({
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 200 ? 'OK' : 'Error',
    json: async () => body,
    text: async () => JSON.stringify(body),
  })
}

describe('useNotes', () => {
  it('fires the correct fetch URL for a project', async () => {
    mockFetch(200, { keys: [] })
    renderHook(() => useNotes('myproj'))
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/api/projects/myproj/notes')
    })
  })

  it('returns list of note keys on 200', async () => {
    mockFetch(200, { keys: ['a.md', 'b.md'] })
    const { result } = renderHook(() => useNotes('p'))
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.notes).toEqual(['a.md', 'b.md'])
    expect(result.current.error).toBeNull()
  })

  it('handles bare array response', async () => {
    mockFetch(200, ['x.md'])
    const { result } = renderHook(() => useNotes('p'))
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.notes).toEqual(['x.md'])
  })

  it('exposes error on fetch failure', async () => {
    ;(global.fetch as any).mockRejectedValueOnce(new Error('boom'))
    const { result } = renderHook(() => useNotes('p'))
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.error).toContain('boom')
  })

  it('is loading before fetch resolves', () => {
    ;(global.fetch as any).mockImplementationOnce(() => new Promise(() => {}))
    const { result } = renderHook(() => useNotes('p'))
    expect(result.current.isLoading).toBe(true)
  })

  it('reload() fires a new fetch', async () => {
    mockFetch(200, { keys: [] })
    const { result } = renderHook(() => useNotes('p'))
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    mockFetch(200, { keys: ['n.md'] })
    await act(async () => {
      await result.current.reload()
    })
    expect((global.fetch as any).mock.calls.length).toBe(2)
    expect(result.current.notes).toEqual(['n.md'])
  })

  it('encodes project slug in URL', async () => {
    mockFetch(200, [])
    renderHook(() => useNotes('my proj/x'))
    await waitFor(() => expect(global.fetch).toHaveBeenCalled())
    expect((global.fetch as any).mock.calls[0][0]).toBe('/api/projects/my%20proj%2Fx/notes')
  })
})

describe('writeNote', () => {
  it('PUTs to the correct URL with JSON body', async () => {
    mockFetch(200, { key: 'foo.md', content: 'hi' })
    const note = await writeNote('p', 'foo.md', 'hi', 'me', ['t1'])
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/projects/p/notes/foo.md',
      expect.objectContaining({
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    const [, init] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(init.body)).toEqual({ content: 'hi', author: 'me', tags: ['t1'] })
    expect(note).toEqual({ key: 'foo.md', content: 'hi' })
  })

  it('defaults author and tags', async () => {
    mockFetch(200, {})
    await writeNote('p', 'k', 'c')
    const [, init] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(init.body)).toEqual({ content: 'c', author: '', tags: [] })
  })

  it('throws on non-ok response', async () => {
    mockFetch(500, { err: 'fail' })
    await expect(writeNote('p', 'k', 'c')).rejects.toThrow(/write note failed: 500/)
  })
})

describe('deleteNote', () => {
  it('uses DELETE method at the correct URL', async () => {
    mockFetch(200, {})
    await deleteNote('p', 'foo.md')
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/projects/p/notes/foo.md',
      expect.objectContaining({ method: 'DELETE' }),
    )
  })

  it('throws on non-ok response', async () => {
    mockFetch(500, { err: 'bad' })
    await expect(deleteNote('p', 'k')).rejects.toThrow(/delete note failed: 500/)
  })
})

// useNotesGraph, useNotesTree, useNotesSearch live in useNotes.ts —
// not in separate files. Tests are included here.

describe('useNotesGraph', () => {
  it('fires fetch at /graph and parses nodes/edges', async () => {
    mockFetch(200, { nodes: [{ id: 'a' }], edges: [{ source: 'a', target: 'b' }] })
    const { result } = renderHook(() => useNotesGraph('p'))
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect((global.fetch as any).mock.calls[0][0]).toBe('/api/projects/p/graph')
    expect(result.current.graph).toEqual({
      nodes: [{ id: 'a' }],
      edges: [{ source: 'a', target: 'b' }],
    })
  })

  it('normalises missing fields to empty arrays', async () => {
    mockFetch(200, {})
    const { result } = renderHook(() => useNotesGraph('p'))
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.graph).toEqual({ nodes: [], edges: [] })
  })

  it('exposes error on fetch rejection', async () => {
    ;(global.fetch as any).mockRejectedValueOnce(new Error('net'))
    const { result } = renderHook(() => useNotesGraph('p'))
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.error).toContain('net')
  })

  it('reload() fires another fetch', async () => {
    mockFetch(200, { nodes: [], edges: [] })
    const { result } = renderHook(() => useNotesGraph('p'))
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    mockFetch(200, { nodes: [{ id: 'x' }], edges: [] })
    await act(async () => {
      await result.current.reload()
    })
    expect((global.fetch as any).mock.calls.length).toBe(2)
    expect(result.current.graph?.nodes).toEqual([{ id: 'x' }])
  })
})

describe('useNotesTree', () => {
  it('fires fetch at /tree', async () => {
    mockFetch(200, { tree: [] })
    renderHook(() => useNotesTree('p'))
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/api/projects/p/tree')
    })
  })

  it('returns tree list on 200', async () => {
    mockFetch(200, { tree: [{ name: 'root', type: 'dir', children: [] }] })
    const { result } = renderHook(() => useNotesTree('p'))
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.tree).toHaveLength(1)
  })

  it('handles bare array tree response', async () => {
    mockFetch(200, [{ name: 'a', type: 'file' }])
    const { result } = renderHook(() => useNotesTree('p'))
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.tree).toHaveLength(1)
  })

  it('exposes error on failure', async () => {
    ;(global.fetch as any).mockRejectedValueOnce(new Error('x'))
    const { result } = renderHook(() => useNotesTree('p'))
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.error).toContain('x')
  })

  it('is loading before fetch resolves', () => {
    ;(global.fetch as any).mockImplementationOnce(() => new Promise(() => {}))
    const { result } = renderHook(() => useNotesTree('p'))
    expect(result.current.isLoading).toBe(true)
  })

  it('reload() fires another fetch', async () => {
    mockFetch(200, { tree: [] })
    const { result } = renderHook(() => useNotesTree('p'))
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    mockFetch(200, { tree: [{ name: 'n', type: 'file' }] })
    await act(async () => {
      await result.current.reload()
    })
    expect((global.fetch as any).mock.calls.length).toBe(2)
  })
})

describe('useNotesSearch', () => {
  it('fires fetch with q and limit params', async () => {
    mockFetch(200, { hits: [] })
    const { result } = renderHook(() => useNotesSearch('p'))
    await act(async () => {
      await result.current.search('hello', 5)
    })
    const url = (global.fetch as any).mock.calls[0][0] as string
    expect(url).toContain('/api/projects/p/search')
    expect(url).toContain('q=hello')
    expect(url).toContain('limit=5')
  })

  it('returns hits on 200', async () => {
    mockFetch(200, { hits: [{ key: 'a.md', score: 1 }] })
    const { result } = renderHook(() => useNotesSearch('p'))
    await act(async () => {
      await result.current.search('x')
    })
    expect(result.current.hits).toEqual([{ key: 'a.md', score: 1 }])
  })

  it('skips fetch when query is empty', async () => {
    const { result } = renderHook(() => useNotesSearch('p'))
    await act(async () => {
      await result.current.search('')
    })
    expect(global.fetch).not.toHaveBeenCalled()
    expect(result.current.hits).toEqual([])
  })

  it('exposes error on fetch rejection', async () => {
    ;(global.fetch as any).mockRejectedValueOnce(new Error('down'))
    const { result } = renderHook(() => useNotesSearch('p'))
    await act(async () => {
      await result.current.search('q')
    })
    expect(result.current.error).toContain('down')
  })
})
