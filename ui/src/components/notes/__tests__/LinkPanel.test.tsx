import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import LinkPanel from '../LinkPanel'
import type { NotesGraph } from '@/types/api'

function makeGraph(nodes: string[], edges: Array<[string, string]>): NotesGraph {
  return {
    nodes: nodes.map((id) => ({ id, key: id, label: id })),
    edges: edges.map(([from, to]) => ({ from, to })),
  }
}

describe('LinkPanel', () => {
  it('renders nothing when activeKey is null', () => {
    const { container } = render(
      <LinkPanel activeKey={null} graph={makeGraph(['a'], [])} onNavigate={vi.fn()} />,
    )
    expect(container.firstChild).toBeNull()
  })

  it('renders outbound links from the active key', () => {
    const graph = makeGraph(['a', 'b', 'c'], [['a', 'b'], ['a', 'c']])
    render(<LinkPanel activeKey="a" graph={graph} onNavigate={vi.fn()} />)
    expect(screen.getByText('Outbound')).toBeInTheDocument()
    // Two outbound destinations.
    expect(screen.getByRole('button', { name: /b/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /c/ })).toBeInTheDocument()
  })

  it('renders inbound (backlinks) pointing at the active key', () => {
    const graph = makeGraph(
      ['target', 'src1', 'src2'],
      [
        ['src1', 'target'],
        ['src2', 'target'],
      ],
    )
    render(<LinkPanel activeKey="target" graph={graph} onNavigate={vi.fn()} />)
    expect(screen.getByText('Inbound')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /src1/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /src2/ })).toBeInTheDocument()
  })

  it('shows "None" under each section when there are no links', () => {
    const graph = makeGraph(['lonely'], [])
    render(<LinkPanel activeKey="lonely" graph={graph} onNavigate={vi.fn()} />)
    // Both sections render an empty-state "None" message.
    const nones = screen.getAllByText('None')
    expect(nones.length).toBe(2)
  })

  it('clicking a link calls onNavigate with the target key', async () => {
    const onNavigate = vi.fn()
    const graph = makeGraph(['a', 'b'], [['a', 'b']])
    render(<LinkPanel activeKey="a" graph={graph} onNavigate={onNavigate} />)
    await userEvent.click(screen.getByRole('button', { name: /b/ }))
    expect(onNavigate).toHaveBeenCalledWith('b')
  })

  it('handles null graph prop (empty lists)', () => {
    render(<LinkPanel activeKey="any" graph={null} onNavigate={vi.fn()} />)
    // Both outbound + inbound show None.
    expect(screen.getAllByText('None').length).toBe(2)
  })
})
