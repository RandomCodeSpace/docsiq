export interface Stats {
  documents: number
  chunks: number
  embeddings?: number
  entities: number
  relationships: number
  claims?: number
  communities: number
}

export interface Document {
  id: string
  path: string
  title: string
  doc_type: string
  file_hash?: string
  structured?: string
  version?: number
  canonical_id?: string
  is_latest?: boolean
  created_at: number
  updated_at: number
}

export type SearchMode = 'local' | 'global'

export interface SearchRequest {
  query: string
  mode: SearchMode
  top_k: number
  graph_depth?: number
  community_level?: number
}

export interface SearchResult {
  chunk_id?: string | number
  document_id?: string | number
  title?: string
  path?: string
  score?: number
  text?: string
  chunk_text?: string
  answer?: string
  entities?: string[]
  communities?: string[]
}

export interface GraphNode {
  id: string
  label: string
  type: string
  description?: string
  rank?: number
  x?: number
  y?: number
}

export interface GraphEdge {
  id?: string
  from: string
  to: string
  label?: string
  weight?: number
}

export interface GraphNeighborhood {
  nodes: GraphNode[]
  edges: GraphEdge[]
  center?: string
}

export interface Entity {
  id: string
  name: string
  type: string
  description?: string
  rank?: number
  community_id?: string
}

export interface Community {
  id: string
  level: number
  parent_id?: string
  title?: string
  summary?: string
  rank?: number
}

export interface MCPTool {
  name: string
  description: string
  inputSchema?: {
    properties?: Record<string, { type?: string; description?: string }>
    required?: string[]
  }
}
