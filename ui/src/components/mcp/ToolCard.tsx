import { Play, Terminal } from 'lucide-react'
import type { MCPTool } from '@/types/api'

interface Props {
  tool: MCPTool
  index: number
  onCall: (index: number) => void
  onRPC: (index: number) => void
}

export default function ToolCard({ tool, index, onCall, onRPC }: Props) {
  const props = tool.inputSchema?.properties || {}
  const req = tool.inputSchema?.required || []
  const paramCount = Object.keys(props).length

  return (
    <div className="mc-tool-card">
      <div style={{ position: 'absolute', inset: '0 0 auto 0', height: 2, background: 'linear-gradient(90deg, var(--color-accent), var(--color-accent-hover))' }} />
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '0.75rem', marginBottom: '0.4rem' }}>
        <span style={{ fontFamily: 'ui-monospace, monospace', fontSize: '0.8rem', fontWeight: 700, color: 'var(--text-primary)' }}>
          {tool.name}
        </span>
        {paramCount > 0 && (
          <span className="mc-badge" style={{ fontSize: '0.6rem' }}>{paramCount}p</span>
        )}
      </div>
      <p style={{ fontSize: '0.72rem', color: 'var(--text-muted)', lineHeight: 1.55, marginBottom: '0.75rem', minHeight: '3.2em' }}>
        {tool.description || 'No description provided.'}
      </p>
      {paramCount > 0 && (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.3rem', marginBottom: '0.8rem' }}>
          {Object.entries(props).map(([key, value]) => (
            <span key={key} className={`mc-param-tag ${req.includes(key) ? 'mc-param-req' : 'mc-param-opt'}`}>
              {key}
              <span style={{ opacity: 0.45, marginLeft: 2 }}>:{value.type ?? 'any'}</span>
            </span>
          ))}
        </div>
      )}
      <div style={{ display: 'flex', gap: '0.4rem', marginTop: 'auto' }}>
        <button className="mc-btn-call" onClick={() => onCall(index)} style={{ display: 'flex', alignItems: 'center', gap: '0.35rem' }}>
          <Play size={10} /> Call
        </button>
        <button className="mc-btn-rpc" onClick={() => onRPC(index)} style={{ display: 'flex', alignItems: 'center', gap: '0.35rem' }}>
          <Terminal size={10} /> JSON-RPC
        </button>
      </div>
    </div>
  )
}
