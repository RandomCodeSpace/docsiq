import { useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import { Play, X } from 'lucide-react'
import type { MCPTool } from '@/types/api'
import { colorJSON } from '@/lib/utils'

interface Props {
  tool: MCPTool
  onClose: () => void
  onCall: (name: string, args: Record<string, unknown>) => Promise<unknown>
}

function buildDefaultArgs(tool: MCPTool): Record<string, unknown> {
  const args: Record<string, unknown> = {}
  const props = tool.inputSchema?.properties || {}
  const req = tool.inputSchema?.required || []
  for (const [key, value] of Object.entries(props)) {
    args[key] = req.includes(key) ? (value.type === 'number' ? 0 : value.type === 'boolean' ? false : '') : null
  }
  return args
}

export default function ToolCallModal({ tool, onClose, onCall }: Props) {
  const [argsText, setArgsText] = useState(() => JSON.stringify(buildDefaultArgs(tool), null, 2))
  const [resultHTML, setResultHTML] = useState('')
  const [calling, setCalling] = useState(false)
  const [timing, setTiming] = useState('')
  const [error, setError] = useState('')

  const handleCall = async () => {
    let args: Record<string, unknown>
    try {
      args = JSON.parse(argsText || '{}')
    } catch (e) {
      setError(`Invalid JSON: ${String(e)}`)
      return
    }
    setCalling(true)
    setError('')
    const t0 = performance.now()
    try {
      const result = await onCall(tool.name, args)
      setResultHTML(colorJSON(result))
      setTiming(`${Math.round(performance.now() - t0)}ms`)
    } catch (e) {
      setResultHTML('')
      setError(String(e))
    } finally {
      setCalling(false)
    }
  }

  return (
    <Dialog.Root open onOpenChange={(open) => !open && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="mc-overlay" />
        <Dialog.Content className="mc-modal" style={{ position: 'fixed', top: '50%', left: '50%', transform: 'translate(-50%, -50%)', width: 'min(880px, calc(100vw - 2rem))', maxHeight: '84vh', display: 'flex', flexDirection: 'column', zIndex: 50 }}>
          <div style={{ padding: '0.9rem 1.25rem', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '1rem' }}>
            <div>
              <Dialog.Title asChild>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.45rem', fontSize: '0.82rem', fontWeight: 700 }}>
                  <Play size={12} style={{ color: 'var(--color-accent)' }} />
                  <span>Call</span>
                  <code style={{ background: 'transparent', padding: 0, color: 'var(--color-accent)' }}>{tool.name}</code>
                </div>
              </Dialog.Title>
              <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', marginTop: '0.25rem' }}>{tool.description}</div>
            </div>
            <Dialog.Close asChild>
              <button className="mc-btn-icon" style={{ width: 28, padding: 0, justifyContent: 'center' }}>
                <X size={13} />
              </button>
            </Dialog.Close>
          </div>
          {error && (
            <div style={{ padding: '0.6rem 1.25rem', background: 'rgba(239,68,68,0.08)', borderBottom: '1px solid rgba(239,68,68,0.2)', color: '#ef4444', fontSize: '0.72rem' }}>
              {error}
            </div>
          )}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', minHeight: 0, flex: 1 }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.55rem', padding: '1rem', borderRight: '1px solid var(--border)' }}>
              <label style={{ fontSize: '0.62rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)', fontWeight: 700 }}>Arguments</label>
              <textarea className="mc-textarea" style={{ flex: 1, minHeight: '16rem' }} value={argsText} onChange={(event) => setArgsText(event.target.value)} spellCheck={false} />
              <button className="mc-send-btn" disabled={calling} onClick={handleCall}>
                {calling ? 'Executing…' : 'Execute Tool'}
              </button>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.55rem', padding: '1rem', minHeight: 0 }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <label style={{ fontSize: '0.62rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)', fontWeight: 700 }}>Result</label>
                {timing && <span className="mc-badge">{timing}</span>}
              </div>
              <pre className="mc-code" style={{ flex: 1, minHeight: '16rem', overflow: 'auto', padding: '0.9rem' }} dangerouslySetInnerHTML={{ __html: resultHTML || '<span style="color:var(--text-dim)">—</span>' }} />
            </div>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
