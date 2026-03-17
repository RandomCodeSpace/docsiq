import { useCallback, useEffect, useState } from 'react'
import type { Community, Entity } from '../types/api'

export function useCommunities() {
  const [communities, setCommunities] = useState<Community[]>([])
  const [level, setLevel] = useState(-1)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [members, setMembers] = useState<Entity[]>([])
  const [loading, setLoading] = useState(true)
  const [detailLoading, setDetailLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchCommunities = useCallback(async (nextLevel = level) => {
    setLoading(true)
    setError(null)
    try {
      const params = new URLSearchParams()
      if (nextLevel >= 0) params.set('level', String(nextLevel))
      const res = await fetch(`/api/communities${params.toString() ? `?${params.toString()}` : ''}`)
      const data = await res.json()
      setCommunities(Array.isArray(data) ? data : [])
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [level])

  useEffect(() => {
    fetchCommunities()
  }, [fetchCommunities])

  const selectLevel = (nextLevel: number) => {
    setLevel(nextLevel)
    void fetchCommunities(nextLevel)
  }

  const loadCommunity = async (id: string) => {
    setSelectedId(id)
    setDetailLoading(true)
    try {
      const res = await fetch(`/api/communities/${id}`)
      const data = await res.json()
      setMembers(Array.isArray(data.members) ? data.members : [])
    } catch (e) {
      setError(String(e))
      setMembers([])
    } finally {
      setDetailLoading(false)
    }
  }

  return {
    communities,
    level,
    members,
    selectedId,
    loading,
    detailLoading,
    error,
    selectLevel,
    loadCommunity,
  }
}
