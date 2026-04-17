import { useCallback, useEffect, useState } from 'react'
import type { ProjectInfo } from '../types/api'

/**
 * useProjects — fetches the registered project list from GET /api/projects.
 * Falls back to [{slug: "_default", name: "_default"}] when the endpoint
 * errors so the UI is still usable against an unregistered server.
 */
export function useProjects() {
  const [projects, setProjects] = useState<ProjectInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const reload = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await fetch('/api/projects')
      const data = await res.json()
      const list: ProjectInfo[] = Array.isArray(data)
        ? data.map((p: unknown) => {
            if (typeof p === 'string') return { slug: p, name: p }
            const obj = p as { slug?: string; name?: string }
            return { slug: obj.slug ?? '_default', name: obj.name ?? obj.slug ?? '_default' }
          })
        : []
      setProjects(list.length > 0 ? list : [{ slug: '_default', name: '_default' }])
    } catch (e) {
      setError(String(e))
      setProjects([{ slug: '_default', name: '_default' }])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void reload()
  }, [reload])

  return { projects, loading, error, reload }
}

/**
 * useCurrentProject — stores the active project slug in the URL query
 * string (?project=<slug>), defaulting to `_default`. Updates push a new
 * history entry so reloads preserve context and the back button works.
 */
export function useCurrentProject() {
  const read = () => {
    if (typeof window === 'undefined') return '_default'
    const params = new URLSearchParams(window.location.search)
    return params.get('project') ?? '_default'
  }
  const [slug, setSlug] = useState<string>(read)

  useEffect(() => {
    const onPop = () => setSlug(read())
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  const set = useCallback((next: string) => {
    const url = new URL(window.location.href)
    url.searchParams.set('project', next)
    window.history.replaceState({}, '', url.toString())
    setSlug(next)
  }, [])

  return [slug, set] as const
}
