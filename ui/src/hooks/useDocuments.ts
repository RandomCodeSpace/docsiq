import { useCallback, useEffect, useState } from 'react'
import type { Document } from '../types/api'

export function useDocuments() {
  const [documents, setDocuments] = useState<Document[]>([])
  const [docType, setDocType] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchDocuments = useCallback(async (nextType = docType) => {
    setLoading(true)
    setError(null)
    try {
      const params = new URLSearchParams()
      if (nextType) params.set('doc_type', nextType)
      const res = await fetch(`/api/documents${params.toString() ? `?${params.toString()}` : ''}`)
      const data = await res.json()
      setDocuments(Array.isArray(data) ? data : [])
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [docType])

  useEffect(() => {
    fetchDocuments()
  }, [fetchDocuments])

  const selectType = (nextType: string) => {
    setDocType(nextType)
    void fetchDocuments(nextType)
  }

  return { documents, docType, loading, error, refetch: fetchDocuments, selectType }
}
