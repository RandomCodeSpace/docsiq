import { useCallback, useEffect, useState } from 'react'
import type { Note, NoteHit, NoteReadResponse, NotesGraph, TreeNode } from '../types/api'

// ---- Lists ----------------------------------------------------------

/** useNotes — lists note keys for a project. */
export function useNotes(project: string) {
  const [notes, setNotes] = useState<string[]>([])
  const [isLoading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const reload = useCallback(async () => {
    if (!project) return
    setLoading(true)
    setError(null)
    try {
      const res = await fetch(`/api/projects/${encodeURIComponent(project)}/notes`)
      const data = await res.json()
      const list = Array.isArray(data) ? data : (data.keys ?? [])
      setNotes(list)
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [project])

  useEffect(() => {
    void reload()
  }, [reload])

  return { notes, isLoading, error, reload }
}

// ---- Single note ----------------------------------------------------

/** useNote — fetches a single note by key. Exposes loadNote(key) to swap. */
export function useNote(project: string, initialKey: string | null) {
  const [key, setKey] = useState<string | null>(initialKey)
  const [data, setData] = useState<NoteReadResponse | null>(null)
  const [isLoading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(
    async (nextKey: string) => {
      if (!project || !nextKey) return
      setKey(nextKey)
      setLoading(true)
      setError(null)
      try {
        const res = await fetch(
          `/api/projects/${encodeURIComponent(project)}/notes/${encodeURI(nextKey)}`,
        )
        if (!res.ok) {
          setData(null)
          setError(`${res.status} ${res.statusText}`)
        } else {
          const json = await res.json()
          setData(json as NoteReadResponse)
        }
      } catch (e) {
        setError(String(e))
      } finally {
        setLoading(false)
      }
    },
    [project],
  )

  useEffect(() => {
    if (initialKey) void load(initialKey)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [project, initialKey])

  return { key, data, isLoading, error, load, setKey }
}

// ---- Write / delete -------------------------------------------------

export async function writeNote(
  project: string,
  key: string,
  content: string,
  author?: string,
  tags?: string[],
): Promise<Note> {
  const res = await fetch(
    `/api/projects/${encodeURIComponent(project)}/notes/${encodeURI(key)}`,
    {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content, author: author ?? '', tags: tags ?? [] }),
    },
  )
  if (!res.ok) {
    const txt = await res.text()
    throw new Error(`write note failed: ${res.status} ${txt}`)
  }
  return res.json()
}

export async function deleteNote(project: string, key: string): Promise<void> {
  const res = await fetch(
    `/api/projects/${encodeURIComponent(project)}/notes/${encodeURI(key)}`,
    { method: 'DELETE' },
  )
  if (!res.ok) {
    const txt = await res.text()
    throw new Error(`delete note failed: ${res.status} ${txt}`)
  }
}

// ---- Graph / tree / search -----------------------------------------

export function useNotesGraph(project: string) {
  const [graph, setGraph] = useState<NotesGraph | null>(null)
  const [isLoading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const reload = useCallback(async () => {
    if (!project) return
    setLoading(true)
    setError(null)
    try {
      const res = await fetch(`/api/projects/${encodeURIComponent(project)}/graph`)
      const data = await res.json()
      setGraph({
        nodes: Array.isArray(data?.nodes) ? data.nodes : [],
        edges: Array.isArray(data?.edges) ? data.edges : [],
      })
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [project])

  useEffect(() => {
    void reload()
  }, [reload])

  return { graph, isLoading, error, reload }
}

export function useNotesTree(project: string) {
  const [tree, setTree] = useState<TreeNode[]>([])
  const [isLoading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const reload = useCallback(async () => {
    if (!project) return
    setLoading(true)
    setError(null)
    try {
      const res = await fetch(`/api/projects/${encodeURIComponent(project)}/tree`)
      const data = await res.json()
      const list = Array.isArray(data) ? data : (data.tree ?? [])
      setTree(list as TreeNode[])
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [project])

  useEffect(() => {
    void reload()
  }, [reload])

  return { tree, isLoading, error, reload }
}

export function useNotesSearch(project: string) {
  const [hits, setHits] = useState<NoteHit[]>([])
  const [isLoading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const search = useCallback(
    async (query: string, limit = 20) => {
      if (!project || !query) {
        setHits([])
        return
      }
      setLoading(true)
      setError(null)
      try {
        const u = new URL(`/api/projects/${encodeURIComponent(project)}/search`, window.location.origin)
        u.searchParams.set('q', query)
        u.searchParams.set('limit', String(limit))
        const res = await fetch(u.pathname + u.search)
        const data = await res.json()
        setHits(Array.isArray(data?.hits) ? data.hits : [])
      } catch (e) {
        setError(String(e))
      } finally {
        setLoading(false)
      }
    },
    [project],
  )

  return { hits, isLoading, error, search, setHits }
}
