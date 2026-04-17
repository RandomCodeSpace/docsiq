import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import NotesGraphView from '../NotesGraphView'
import type { NotesGraph } from '@/types/api'

function makeGraph(n: number, edges: Array<[string, string]> = []): NotesGraph {
  const nodes = Array.from({ length: n }, (_, i) => ({
    id: `n${i}`,
    key: `n${i}`,
    label: `Node ${i}`,
  }))
  return { nodes, edges: edges.map(([from, to]) => ({ from, to })) }
}

describe('NotesGraphView', () => {
  it('renders N <circle> elements for N input nodes', () => {
    const { container } = render(
      <NotesGraphView graph={makeGraph(5)} loading={false} error={null} onSelect={vi.fn()} />,
    )
    const circles = container.querySelectorAll('circle')
    expect(circles.length).toBe(5)
  })

  it('renders edges as <line> elements in the SVG', () => {
    const graph = makeGraph(3, [
      ['n0', 'n1'],
      ['n1', 'n2'],
    ])
    const { container } = render(
      <NotesGraphView graph={graph} loading={false} error={null} onSelect={vi.fn()} />,
    )
    const lines = container.querySelectorAll('svg line')
    expect(lines.length).toBe(2)
  })

  it('shows empty-state message when graph is empty', () => {
    render(<NotesGraphView graph={makeGraph(0)} loading={false} error={null} onSelect={vi.fn()} />)
    expect(screen.getByText(/no wikilinks yet/i)).toBeInTheDocument()
  })

  it('shows empty-state when graph prop is null (not loading)', () => {
    render(<NotesGraphView graph={null} loading={false} error={null} onSelect={vi.fn()} />)
    expect(screen.getByText(/no wikilinks yet/i)).toBeInTheDocument()
  })

  it('uses note-accent color token on node circles (var(--accent-notes))', () => {
    const { container } = render(
      <NotesGraphView graph={makeGraph(2)} loading={false} error={null} onSelect={vi.fn()} />,
    )
    const circles = container.querySelectorAll('circle')
    expect(circles.length).toBeGreaterThan(0)
    // Every node circle uses the accent-notes CSS variable for fill.
    circles.forEach((c) => {
      expect(c.getAttribute('fill')).toBe('var(--accent-notes)')
    })
  })

  it('uses accent-notes-dim token on edge lines', () => {
    const graph = makeGraph(2, [['n0', 'n1']])
    const { container } = render(
      <NotesGraphView graph={graph} loading={false} error={null} onSelect={vi.fn()} />,
    )
    const lines = container.querySelectorAll('svg line')
    expect(lines[0].getAttribute('stroke')).toBe('var(--accent-notes-dim)')
  })

  it('shows loading state', () => {
    render(<NotesGraphView graph={null} loading={true} error={null} onSelect={vi.fn()} />)
    expect(screen.getByText(/loading/i)).toBeInTheDocument()
  })

  it('shows error state', () => {
    render(
      <NotesGraphView graph={null} loading={false} error="boom" onSelect={vi.fn()} />,
    )
    expect(screen.getByText('boom')).toBeInTheDocument()
  })

  it('displays counts of nodes and links', () => {
    const graph = makeGraph(4, [
      ['n0', 'n1'],
      ['n1', 'n2'],
      ['n2', 'n3'],
    ])
    render(<NotesGraphView graph={graph} loading={false} error={null} onSelect={vi.fn()} />)
    // "4 notes · 3 links"
    expect(screen.getByText(/4 notes/i)).toBeInTheDocument()
    expect(screen.getByText(/3 links/i)).toBeInTheDocument()
  })
})
