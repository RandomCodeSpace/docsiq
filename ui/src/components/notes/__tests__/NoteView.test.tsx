import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import NoteView from '../NoteView'
import type { NoteReadResponse } from '@/types/api'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function makeNote(content: string, overrides: Partial<NoteReadResponse['note']> = {}): NoteReadResponse {
  return {
    note: {
      key: 'test/note',
      content,
      author: 'tester',
      tags: ['a', 'b'],
      ...overrides,
    },
    outlinks: [],
    project: 'default',
  }
}

function renderView(content: string, onNavigate = vi.fn()) {
  const note = makeNote(content)
  const onEdit = vi.fn()
  const onDelete = vi.fn()
  const utils = render(
    <NoteView
      note={note}
      loading={false}
      error={null}
      onNavigate={onNavigate}
      onEdit={onEdit}
      onDelete={onDelete}
    />,
  )
  return { ...utils, onNavigate, onEdit, onDelete }
}

// ---------------------------------------------------------------------------
// Required cases
// ---------------------------------------------------------------------------

describe('NoteView — markdown rendering', () => {
  it('renders h1, h2, h3 headings', () => {
    const { container } = renderView('# One\n## Two\n### Three\n')
    expect(container.querySelector('h1')).toHaveTextContent('One')
    expect(container.querySelector('h2')).toHaveTextContent('Two')
    expect(container.querySelector('h3')).toHaveTextContent('Three')
  })

  it('renders bold, italic, and inline code', () => {
    const { container } = renderView('This is **bold** and *italic* and `mono`.')
    const strong = container.querySelector('strong')
    const em = container.querySelector('em')
    const code = container.querySelector('code')
    expect(strong).toHaveTextContent('bold')
    expect(em).toHaveTextContent('italic')
    expect(code).toHaveTextContent('mono')
  })

  it('renders wikilinks [[target]] as clickable and calls onNavigate', async () => {
    const { onNavigate } = renderView('See [[target]] now.')
    const link = screen.getByText('target')
    expect(link.tagName).toBe('A')
    await userEvent.click(link)
    expect(onNavigate).toHaveBeenCalledWith('target')
    expect(onNavigate).toHaveBeenCalledTimes(1)
  })

  it('displays tail-segment for slashed wikilinks but navigates to full target', async () => {
    // NoteView does NOT implement [[target|alias]] aliased form.
    // It DOES take the trailing path segment: [[a/b/c]] -> shows "c", navigates "a/b/c".
    const { onNavigate } = renderView('Jump to [[folder/sub/leaf]].')
    const link = screen.getByText('leaf')
    await userEvent.click(link)
    expect(onNavigate).toHaveBeenCalledWith('folder/sub/leaf')
  })

  it('renders markdown links [text](url) with target="_blank" and rel containing "noopener"', () => {
    renderView('Visit [Site](https://example.com) please.')
    const a = screen.getByRole('link', { name: 'Site' }) as HTMLAnchorElement
    expect(a.getAttribute('href')).toBe('https://example.com')
    expect(a.getAttribute('target')).toBe('_blank')
    expect(a.getAttribute('rel')).toMatch(/noopener/)
  })

  it('renders images ![alt](url) with loading="lazy" and alt attribute', () => {
    const { container } = renderView('![pretty pic](https://cdn.example.com/a.png)')
    const img = container.querySelector('img') as HTMLImageElement
    expect(img).not.toBeNull()
    expect(img.getAttribute('src')).toBe('https://cdn.example.com/a.png')
    expect(img.getAttribute('alt')).toBe('pretty pic')
    expect(img.getAttribute('loading')).toBe('lazy')
  })

  it('renders blockquotes > line as <blockquote>', () => {
    const { container } = renderView('> quoted thought\n> continues')
    const bq = container.querySelector('blockquote')
    expect(bq).not.toBeNull()
    expect(bq?.textContent).toMatch(/quoted thought/)
    expect(bq?.textContent).toMatch(/continues/)
  })

  it('renders GitHub-style tables with header, separator, rows into <table><td>', () => {
    const md = [
      '| Col A | Col B |',
      '| --- | --- |',
      '| a1 | b1 |',
      '| a2 | b2 |',
    ].join('\n')
    const { container } = renderView(md)
    const table = container.querySelector('table')
    expect(table).not.toBeNull()
    const ths = table!.querySelectorAll('th')
    expect(ths[0]).toHaveTextContent('Col A')
    expect(ths[1]).toHaveTextContent('Col B')
    const tds = table!.querySelectorAll('td')
    expect(tds.length).toBe(4)
    expect(tds[0]).toHaveTextContent('a1')
    expect(tds[3]).toHaveTextContent('b2')
  })

  it('renders --- / *** / ___ at start of line as <hr>', () => {
    const { container } = renderView('before\n\n---\n\nmid\n\n***\n\ntail\n\n___\n')
    const hrs = container.querySelectorAll('hr')
    expect(hrs.length).toBe(3)
  })

  it('renders inline math \\( ... \\) as <code class="math-inline">', () => {
    const { container } = renderView('Pythag: \\(a^2 + b^2 = c^2\\) holds.')
    const math = container.querySelector('code.math-inline')
    expect(math).not.toBeNull()
    expect(math).toHaveTextContent('a^2 + b^2 = c^2')
  })

  it('strips YAML frontmatter from display but keeps body', () => {
    const md = '---\ntitle: Hidden\nauthor: me\n---\n# Visible Heading\nBody text here.'
    const { container } = renderView(md)
    // Body renders
    expect(container.querySelector('h1')).toHaveTextContent('Visible Heading')
    expect(container.textContent).toMatch(/Body text here\./)
    // Frontmatter keys should NOT appear
    expect(container.textContent).not.toMatch(/title: Hidden/)
    expect(container.textContent).not.toMatch(/author: me/)
  })

  it('handles empty body without throwing', () => {
    expect(() => renderView('')).not.toThrow()
    // Also explicitly null-safe: component uses `note.content ?? ''`.
    const onNavigate = vi.fn()
    expect(() =>
      render(
        <NoteView
          note={{
            note: { key: 'k', content: '' },
            outlinks: [],
            project: 'default',
          }}
          loading={false}
          error={null}
          onNavigate={onNavigate}
          onEdit={vi.fn()}
          onDelete={vi.fn()}
        />,
      ),
    ).not.toThrow()
  })

  it('renders unordered list "- item" / "* item" as <li>', () => {
    const { container } = renderView('- first\n- second\n* third')
    const lis = container.querySelectorAll('li')
    expect(lis.length).toBe(3)
    expect(lis[0]).toHaveTextContent('first')
    expect(lis[1]).toHaveTextContent('second')
    expect(lis[2]).toHaveTextContent('third')
  })
})

// ---------------------------------------------------------------------------
// Edge cases — robustness
// ---------------------------------------------------------------------------

describe('NoteView — edge cases', () => {
  it('does not confuse body ---" with frontmatter: inline hr still renders', () => {
    // No leading frontmatter, so the --- inside the body should still be an <hr>.
    const md = '# Title\n\nIntro paragraph.\n\n---\n\nAfter rule.'
    const { container } = renderView(md)
    expect(container.querySelectorAll('hr').length).toBe(1)
    expect(container.textContent).toMatch(/After rule\./)
  })

  it('renders very long content (~10 KB) without throwing or crashing', () => {
    const block = '# Heading\n\nParagraph with **bold** and [[ref]] and `code`.\n\n'
    const big = block.repeat(200) // ~10 KB
    expect(() => renderView(big)).not.toThrow()
    // Sanity: at least one rendered h1 exists.
    const { container } = renderView(big)
    expect(container.querySelectorAll('h1').length).toBeGreaterThan(0)
  })

  it('degrades gracefully on nested emphasis ***both*** (no throw)', () => {
    const { container } = renderView('Consider ***both*** carefully.')
    // Regex-based renderer can't nest strong+em, so it renders something —
    // either a <strong> with a leading '*' or plain text. The contract here is
    // "does not crash and full word is still visible somewhere."
    expect(container.textContent).toMatch(/both/)
  })

  it('falls back to paragraphs for malformed table (missing separator row)', () => {
    const md = [
      '| Col A | Col B |',
      '| a1 | b1 |',
    ].join('\n')
    const { container } = renderView(md)
    // No <table> should render because the separator row is missing.
    expect(container.querySelector('table')).toBeNull()
    // Content still visible as regular paragraphs.
    expect(container.textContent).toMatch(/Col A/)
    expect(container.textContent).toMatch(/a1/)
  })
})

// ---------------------------------------------------------------------------
// Non-markdown surface: loading / error / empty-note states
// ---------------------------------------------------------------------------

describe('NoteView — non-content states', () => {
  it('shows loading placeholder when loading', () => {
    render(
      <NoteView
        note={null}
        loading={true}
        error={null}
        onNavigate={vi.fn()}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
      />,
    )
    expect(screen.getByText(/loading note/i)).toBeInTheDocument()
  })

  it('shows error message when error is set', () => {
    render(
      <NoteView
        note={null}
        loading={false}
        error="boom"
        onNavigate={vi.fn()}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
      />,
    )
    expect(screen.getByText(/error: boom/i)).toBeInTheDocument()
  })

  it('shows empty-selection hint when note is null', () => {
    render(
      <NoteView
        note={null}
        loading={false}
        error={null}
        onNavigate={vi.fn()}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
      />,
    )
    expect(screen.getByText(/select a note/i)).toBeInTheDocument()
  })
})
