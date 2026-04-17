import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { useProjects, useCurrentProject } from '../useProjects'

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
    json: async () => body,
    text: async () => JSON.stringify(body),
  })
}

describe('useProjects', () => {
  it('fires fetch at /api/projects', async () => {
    mockFetch(200, [])
    renderHook(() => useProjects())
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/api/projects')
    })
  })

  it('returns project list on 200', async () => {
    mockFetch(200, [{ slug: 'alpha', name: 'Alpha' }, { slug: 'beta', name: 'Beta' }])
    const { result } = renderHook(() => useProjects())
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.projects).toEqual([
      { slug: 'alpha', name: 'Alpha' },
      { slug: 'beta', name: 'Beta' },
    ])
  })

  it('maps string entries into {slug,name}', async () => {
    mockFetch(200, ['foo', 'bar'])
    const { result } = renderHook(() => useProjects())
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.projects).toEqual([
      { slug: 'foo', name: 'foo' },
      { slug: 'bar', name: 'bar' },
    ])
  })

  it('falls back to _default on error', async () => {
    ;(global.fetch as any).mockRejectedValueOnce(new Error('500'))
    const { result } = renderHook(() => useProjects())
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.projects).toEqual([{ slug: '_default', name: '_default' }])
    expect(result.current.error).toContain('500')
  })

  it('falls back to _default on empty response', async () => {
    mockFetch(200, [])
    const { result } = renderHook(() => useProjects())
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.projects).toEqual([{ slug: '_default', name: '_default' }])
  })

  it('is loading before fetch resolves', () => {
    ;(global.fetch as any).mockImplementationOnce(() => new Promise(() => {}))
    const { result } = renderHook(() => useProjects())
    expect(result.current.loading).toBe(true)
  })

  it('reload() fires a new fetch', async () => {
    mockFetch(200, [{ slug: 'a', name: 'A' }])
    const { result } = renderHook(() => useProjects())
    await waitFor(() => expect(result.current.loading).toBe(false))
    mockFetch(200, [{ slug: 'b', name: 'B' }])
    await act(async () => {
      await result.current.reload()
    })
    expect((global.fetch as any).mock.calls.length).toBe(2)
    expect(result.current.projects).toEqual([{ slug: 'b', name: 'B' }])
  })
})

describe('useCurrentProject', () => {
  const origHref = window.location.href

  beforeEach(() => {
    window.history.replaceState({}, '', '/')
  })
  afterEach(() => {
    window.history.replaceState({}, '', origHref)
  })

  it('defaults to _default when no query param', () => {
    window.history.replaceState({}, '', '/')
    const { result } = renderHook(() => useCurrentProject())
    expect(result.current[0]).toBe('_default')
  })

  it('reads ?project=<slug> from URL', () => {
    window.history.replaceState({}, '', '/?project=myslug')
    const { result } = renderHook(() => useCurrentProject())
    expect(result.current[0]).toBe('myslug')
  })

  it('setProject updates URL via history.replaceState', () => {
    window.history.replaceState({}, '', '/')
    const spy = vi.spyOn(window.history, 'replaceState')
    const { result } = renderHook(() => useCurrentProject())
    act(() => {
      result.current[1]('newslug')
    })
    expect(result.current[0]).toBe('newslug')
    expect(spy).toHaveBeenCalled()
    const calledUrl = spy.mock.calls[spy.mock.calls.length - 1][2] as string
    expect(calledUrl).toContain('project=newslug')
    expect(window.location.search).toContain('project=newslug')
  })

  it('reacts to popstate events', () => {
    window.history.replaceState({}, '', '/?project=one')
    const { result } = renderHook(() => useCurrentProject())
    expect(result.current[0]).toBe('one')
    act(() => {
      window.history.replaceState({}, '', '/?project=two')
      window.dispatchEvent(new PopStateEvent('popstate'))
    })
    expect(result.current[0]).toBe('two')
  })
})
