import { useCallback, useState } from 'react'
import type { GraphNeighborhood } from '../types/api'

export function useGraph() {
  const [graph, setGraph] = useState<GraphNeighborhood | null>(null)
  const [entity, setEntity] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const loadGraph = useCallback(async (nextEntity: string, depth = 2) => {
    if (!nextEntity.trim()) return
    setEntity(nextEntity)
    setLoading(true)
    setError(null)
    try {
      const params = new URLSearchParams({ entity: nextEntity, depth: String(depth) })
      const res = await fetch(`/api/graph/neighborhood?${params.toString()}`)
      const data = await res.json()
      setGraph({
        center: nextEntity,
        nodes: (data.nodes ?? []).map((node: any) => ({
          id: String(node.id),
          label: node.label ?? node.name ?? String(node.id),
          type: node.type ?? 'Other',
          description: node.description,
          rank: node.rank,
        })),
        edges: (data.edges ?? []).map((edge: any) => ({
          id: String(edge.id ?? `${edge.from}-${edge.to}`),
          from: String(edge.from),
          to: String(edge.to),
          label: edge.label,
          weight: edge.weight,
        })),
      })
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  return { graph, entity, loading, error, loadGraph }
}
