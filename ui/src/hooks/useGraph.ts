import { useCallback, useState } from 'react'
import type { GraphNeighborhood } from '../types/api'

export type GraphStatus = 'idle' | 'loaded' | 'not_found' | 'no_entities' | 'error'

export function useGraph() {
  const [graph, setGraph] = useState<GraphNeighborhood | null>(null)
  const [entity, setEntity] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [status, setStatus] = useState<GraphStatus>('idle')

  const loadGraph = useCallback(async (nextEntity: string, depth = 2) => {
    if (!nextEntity.trim()) return
    setEntity(nextEntity)
    setLoading(true)
    setError(null)
    try {
      const params = new URLSearchParams({ entity: nextEntity, depth: String(depth) })
      const res = await fetch(`/api/graph/neighborhood?${params.toString()}`)
      if (res.status === 404) {
        setGraph(null)
        setStatus('not_found')
        return
      }
      if (!res.ok) {
        const body = await res.text()
        throw new Error(body || `HTTP ${res.status}`)
      }
      const data = await res.json()
      const nodes = (data.nodes ?? []).map((node: any) => ({
        id: String(node.id),
        label: node.label ?? node.name ?? String(node.id),
        type: node.type ?? 'Other',
        description: node.description,
        rank: node.rank,
      }))
      const edges = (data.edges ?? []).map((edge: any) => ({
        id: String(edge.id ?? `${edge.from}-${edge.to}`),
        from: String(edge.from),
        to: String(edge.to),
        label: edge.label,
        weight: edge.weight,
      }))
      setGraph({ center: nextEntity, nodes, edges })
      setStatus(nodes.length > 0 ? 'loaded' : 'loaded')
    } catch (e) {
      setError(String(e))
      setStatus('error')
    } finally {
      setLoading(false)
    }
  }, [])

  const setNoEntities = useCallback(() => {
    setStatus('no_entities')
  }, [])

  return { graph, entity, loading, error, status, loadGraph, setNoEntities }
}
