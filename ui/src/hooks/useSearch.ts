import { useCallback, useRef, useState } from 'react'
import type { SearchRequest, SearchResult } from '../types/api'

export function useSearch() {
  const [results, setResults] = useState<SearchResult[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  const search = useCallback(async (req: SearchRequest) => {
    abortRef.current?.abort()
    const ctrl = new AbortController()
    abortRef.current = ctrl

    setLoading(true)
    setError(null)
    try {
      const res = await fetch('/api/search', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(req),
        signal: ctrl.signal,
      })
      const data = await res.json()
      // global mode returns { answer, results }; local returns array
      if (Array.isArray(data)) {
        setResults(data)
      } else {
        const merged: SearchResult[] = data.results ?? []
        if (data.answer) merged.unshift({ answer: data.answer })
        setResults(merged)
      }
    } catch (e) {
      if ((e as Error).name !== 'AbortError') setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  return { results, loading, error, search }
}
