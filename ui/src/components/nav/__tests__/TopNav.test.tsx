import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import TopNav from '../TopNav'
import type { ProjectInfo, Stats } from '@/types/api'

const stubStats: Stats = {
  documents: 1,
  entities: 2,
  relationships: 3,
  communities: 4,
  chunks: 5,
  vectors: 6,
} as unknown as Stats

function renderNav(
  overrides: {
    projects?: ProjectInfo[]
    currentProject?: string
    onProjectChange?: (slug: string) => void
    onViewChange?: (v: string) => void
    currentView?: string
  } = {},
) {
  const onViewChange = overrides.onViewChange ?? vi.fn()
  const onThemeToggle = vi.fn()
  const onProjectChange = overrides.onProjectChange ?? vi.fn()
  const utils = render(
    <TopNav
      currentView={(overrides.currentView ?? 'overview') as any}
      onViewChange={onViewChange as any}
      stats={stubStats}
      onThemeToggle={onThemeToggle}
      projects={overrides.projects}
      currentProject={overrides.currentProject}
      onProjectChange={onProjectChange}
    />,
  )
  return { ...utils, onViewChange, onThemeToggle, onProjectChange }
}

describe('TopNav', () => {
  it('renders Documents and Notes tabs', () => {
    renderNav()
    expect(screen.getByRole('button', { name: /documents/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /notes/i })).toBeInTheDocument()
  })

  it('renders the project selector with options from the projects prop', () => {
    const projects: ProjectInfo[] = [
      { slug: 'alpha', name: 'Alpha Project' },
      { slug: 'beta', name: 'Beta Project' },
    ]
    renderNav({ projects, currentProject: 'alpha' })
    const select = screen.getByTitle('Active project') as HTMLSelectElement
    expect(select).toBeInTheDocument()
    expect(select.value).toBe('alpha')
    // Options rendered.
    expect(screen.getByRole('option', { name: 'Alpha Project' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Beta Project' })).toBeInTheDocument()
  })

  it('clicking Notes tab calls onViewChange("notes")', async () => {
    const { onViewChange } = renderNav()
    await userEvent.click(screen.getByRole('button', { name: /notes/i }))
    expect(onViewChange).toHaveBeenCalledWith('notes')
  })

  it('selecting a project option fires onProjectChange(slug)', async () => {
    const projects: ProjectInfo[] = [
      { slug: 'alpha', name: 'Alpha' },
      { slug: 'beta', name: 'Beta' },
    ]
    const onProjectChange = vi.fn()
    renderNav({ projects, currentProject: 'alpha', onProjectChange })
    const select = screen.getByTitle('Active project') as HTMLSelectElement
    await userEvent.selectOptions(select, 'beta')
    expect(onProjectChange).toHaveBeenCalledWith('beta')
  })

  it('omits the project selector when projects prop is undefined', () => {
    renderNav({ projects: undefined })
    expect(screen.queryByTitle('Active project')).not.toBeInTheDocument()
  })

  it('applies active class to the current view tab', () => {
    renderNav({ currentView: 'notes' })
    const notesBtn = screen.getByRole('button', { name: /notes/i })
    expect(notesBtn.className).toMatch(/active/)
    const docsBtn = screen.getByRole('button', { name: /documents/i })
    expect(docsBtn.className).not.toMatch(/active/)
  })
})
