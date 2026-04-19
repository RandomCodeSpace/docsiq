export interface Stats {
  documents: number;
  chunks: number;
  entities: number;
  relationships: number;
  communities: number;
  notes: number;
  last_indexed: string | null;
}

export interface Project { slug: string; name: string; }

export interface Note {
  key: string;
  content: string;
  author?: string;
  tags: string[];
  created_at: string;
  updated_at: string;
}

export interface NoteHit {
  key: string;
  title: string;
  snippet: string;
  tags: string[];
  rank: number;
}

export interface Document {
  id: string;
  path: string;
  title: string;
  doc_type: string;
  version: number;
  is_latest: boolean;
  created_at: number;
  updated_at: number;
}

export interface SearchHit {
  chunk_id: string;
  doc_id: string;
  doc_title: string;
  content: string;
  score: number;
}

export interface ApiError { error: string; request_id?: string; }
