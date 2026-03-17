import { useState } from 'react'
import { Plug, RefreshCw } from 'lucide-react'
import { useMCP } from '@/hooks/useMCP'
import type { MCPTool } from '@/types/api'
import ToolCard from './ToolCard'
import ToolCallModal from './ToolCallModal'
import RPCPopup from './RPCPopup'

const statusColors = {
  idle: '#555',
  connecting: '#eab308',
  connected: '#22c55e',
  error: '#ef4444',
} as const

export default function MCPConsole() {
  const { status, tools, error, call, connect, send } = useMCP()
  const [callTool, setCallTool] = useState<MCPTool | null>(null)
  const [rpcTool, setRpcTool] = useState<MCPTool | null>(null)
  const dotColor = statusColors[status]

  const sendRPC = async (body: unknown) => send(body)

  return (
    <div className="mcp-view">
      <div className="mc-status-bar">
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.55rem' }}>
          <span style={{ position: 'relative', display: 'inline-flex', width: 8, height: 8 }}>
            <span style={{ position: 'absolute', inset: 0, borderRadius: '50%', background: dotColor, opacity: status === 'connected' ? 0.4 : 0, animation: status === 'connected' ? 'mc-ping 2s cubic-bezier(0,0,0.2,1) infinite' : 'none' }} />
            <span style={{ width: 8, height: 8, borderRadius: '50%', background: dotColor }} />
          </span>
          <span style={{ fontSize: '0.78rem', fontWeight: 600, color: dotColor, textTransform: 'capitalize' }}>{status}</span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.72rem', color: 'var(--text-dim)' }}>
          <Plug size={11} style={{ opacity: 0.5 }} />
          <code style={{ fontFamily: 'ui-monospace, monospace', padding: '0.15rem 0.4rem', borderRadius: 4, background: 'var(--bg-card)', border: '1px solid var(--border)', color: 'var(--text-secondary)' }}>
            {window.location.origin}/mcp
          </code>
        </div>
        <span className="mc-badge">HTTP Streamable MCP · JSON-RPC 2.0</span>
        <div style={{ marginLeft: 'auto' }}>
          <button className="mc-btn-icon" onClick={() => void connect()}>
            <RefreshCw size={12} /> Reconnect
          </button>
        </div>
      </div>
      <div style={{ padding: '0.7rem 1.25rem', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <span style={{ fontSize: '0.66rem', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.14em', color: 'var(--text-dim)' }}>Available Tools</span>
        <span style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>{tools.length} discovered</span>
      </div>
      <div style={{ flex: 1, overflow: 'auto', padding: '1rem', display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: '0.8rem', alignContent: 'start' }}>
        {status === 'error' && (
          <div className="card" style={{ gridColumn: '1 / -1', textAlign: 'center', padding: '2.5rem 1rem' }}>
            <div style={{ fontSize: '0.9rem', fontWeight: 700, color: '#ef4444', marginBottom: '0.45rem' }}>Connection failed</div>
            <div style={{ color: 'var(--text-muted)', marginBottom: '0.85rem' }}>{error || 'Could not reach the MCP endpoint.'}</div>
            <code>docscontext serve</code>
          </div>
        )}
        {status === 'connected' && tools.map((tool, index) => (
          <ToolCard key={tool.name} tool={tool} index={index} onCall={(next) => setCallTool(tools[next])} onRPC={(next) => setRpcTool(tools[next])} />
        ))}
      </div>
      {callTool && <ToolCallModal tool={callTool} onClose={() => setCallTool(null)} onCall={async (name, args) => (await call('tools/call', { name, arguments: args })).result ?? null} />}
      {rpcTool && <RPCPopup tool={rpcTool} onClose={() => setRpcTool(null)} onSend={sendRPC} />}
    </div>
  )
}
