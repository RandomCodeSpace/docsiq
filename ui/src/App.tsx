import { useEffect, useState } from 'react'
import TopNav, { type DocsView } from '@/components/nav/TopNav'
import StatsPanel from '@/components/docs/StatsPanel'
import SearchPanel from '@/components/docs/SearchPanel'
import DocumentList from '@/components/docs/DocumentList'
import GraphView from '@/components/docs/GraphView'
import CommunityList from '@/components/docs/CommunityList'
import UploadPanel from '@/components/docs/UploadPanel'
import MCPConsole from '@/components/mcp/MCPConsole'
import JsonViewer from '@/components/shared/JsonViewer'
import { useTheme } from '@/hooks/useTheme'
import { useStats } from '@/hooks/useStats'
import { useSearch } from '@/hooks/useSearch'
import { useDocuments } from '@/hooks/useDocuments'
import { useGraph } from '@/hooks/useGraph'
import { useCommunities } from '@/hooks/useCommunities'
import { useUpload } from '@/hooks/useUpload'

export default function App() {
  const { toggle } = useTheme()
  const { stats, loading: statsLoading, error: statsError } = useStats()
  const { results, loading: searchLoading, error: searchError, search } = useSearch()
  const { documents, docType, loading: docsLoading, selectType } = useDocuments()
  const { graph, loading: graphLoading, error: graphError, status: graphStatus, loadGraph, setNoEntities } = useGraph()
  const {
    communities,
    level,
    members,
    selectedId,
    loading: communitiesLoading,
    detailLoading,
    selectLevel,
    loadCommunity,
  } = useCommunities()
  const { jobId, status, messages, loading: uploadLoading, upload } = useUpload()
  const [view, setView] = useState<DocsView>('overview')

  useEffect(() => {
    if (!graph && !graphLoading && graphStatus === 'idle') {
      fetch('/api/entities?limit=1')
        .then((res) => res.ok ? res.json() : null)
        .then((data) => {
          const first = Array.isArray(data) ? data[0] : null
          if (first?.name) {
            void loadGraph(first.name, 2)
          } else {
            setNoEntities()
          }
        })
        .catch(() => setNoEntities())
    }
  }, [graph, graphLoading, graphStatus, loadGraph, setNoEntities])

  return (
    <>
      <TopNav currentView={view} onViewChange={setView} stats={stats} onThemeToggle={toggle} />
      <main className="main-content" style={{ padding: '1rem', gap: '1rem' }}>
        {view === 'overview' && (
          <div style={{ display: 'grid', gridTemplateRows: 'auto auto 1fr', gap: '1rem', minHeight: 0 }}>
            <section className="card" style={{ display: 'grid', gridTemplateColumns: '1.35fr 0.65fr', gap: '1rem', overflow: 'hidden', position: 'relative' }}>
              <div style={{ position: 'absolute', inset: 0, background: 'radial-gradient(circle at top left, rgba(56,189,248,0.14), transparent 40%), linear-gradient(180deg, transparent, rgba(255,255,255,0.02))', pointerEvents: 'none' }} />
              <div style={{ position: 'relative', zIndex: 1 }}>
                <div style={{ fontSize: '0.74rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)', marginBottom: '0.5rem' }}>MCP Console Dashboard</div>
                <h1 style={{ fontSize: '2rem', lineHeight: 1.05, marginBottom: '0.75rem' }}>Search documents, inspect graph structure, and operate the ingestion pipeline from one surface.</h1>
                <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: 1.7, maxWidth: 620 }}>
                  DocsContext combines semantic retrieval, entity relationships, community summaries, and MCP tooling. This redesign aligns the product with the shared console pattern used across the other agents while keeping the knowledge graph workflows in the foreground.
                </p>
              </div>
              <div style={{ position: 'relative', zIndex: 1, display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem' }}>
                {[
                  ['Retrieval', 'Local and global search'],
                  ['Graph', 'Neighborhood exploration'],
                  ['Communities', 'Hierarchical summaries'],
                  ['Ingest', 'Drop files into the pipeline'],
                ].map(([title, copy]) => (
                  <div key={title} style={{ border: '1px solid var(--border)', borderRadius: 12, padding: '0.9rem', background: 'rgba(255,255,255,0.02)' }}>
                    <div style={{ fontSize: '0.76rem', fontWeight: 700, marginBottom: '0.35rem' }}>{title}</div>
                    <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', lineHeight: 1.5 }}>{copy}</div>
                  </div>
                ))}
              </div>
            </section>
            <StatsPanel stats={stats} loading={statsLoading} />
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', minHeight: 0 }}>
              <JsonViewer title="Stats response" value={stats ?? {}} defaultOpen />
              <JsonViewer title="Status" value={{ statsError, uploadStatus: status, graphLoaded: Boolean(graph?.nodes?.length) }} defaultOpen />
            </div>
          </div>
        )}

        {view === 'search' && (
          <SearchPanel loading={searchLoading} error={searchError} results={results} onSearch={(query, mode, topK) => void search({ query, mode, top_k: topK })} />
        )}

        {view === 'documents' && (
          <DocumentList documents={documents} docType={docType} loading={docsLoading} onFilter={selectType} />
        )}

        {view === 'graph' && (
          <GraphView graph={graph} loading={graphLoading} error={graphError} status={graphStatus} onLoad={(entity, depth) => void loadGraph(entity, depth)} />
        )}

        {view === 'communities' && (
          <CommunityList
            communities={communities}
            members={members}
            level={level}
            loading={communitiesLoading}
            detailLoading={detailLoading}
            selectedId={selectedId}
            onLevel={selectLevel}
            onSelect={(id) => void loadCommunity(id)}
          />
        )}

        {view === 'upload' && (
          <UploadPanel jobId={jobId} status={status} messages={messages} loading={uploadLoading} onUpload={(files) => void upload(files)} />
        )}

        {view === 'mcp' && <MCPConsole />}
      </main>
      <footer className="status-bar">
        <div className="status-item"><span className="status-key">Q</span> semantic query surface</div>
        <div className="status-item"><span className="status-key">G</span> graph neighborhood</div>
        <div className="status-item"><span className="status-key">M</span> MCP tools</div>
      </footer>
    </>
  )
}
