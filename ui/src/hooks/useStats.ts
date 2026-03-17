import { useEffect, useState } from 'react'
import type { Stats } from '../types/api'

export function useStats() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetch_ = () => {
    setLoading(true)
    fetch('/api/stats')
      .then(r => r.json())
      .then(data => { setStats(data); setError(null) })
      .catch(e => setError(String(e)))
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetch_() }, [])
  return { stats, loading, error, refetch: fetch_ }
}
