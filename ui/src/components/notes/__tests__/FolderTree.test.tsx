import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import FolderTree from '../FolderTree'
import type { TreeNode } from '@/types/api'

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------
const sampleTree: TreeNode[] = [
  {
    path: 'architecture',
    name: 'architecture',
    type: 'folder',
    children: [
      {
        path: 'architecture/auth',
        name: 'auth',
        type: 'note',
        link_count: 3,
      },
    ],
  },
  {
    path: 'readme',
    name: 'readme',
    type: 'note',
  },
]

function renderTree(overrides: Partial<React.ComponentProps<typeof FolderTree>> = {}) {
  const props = {
    tree: sampleTree,
    activeKey: null,
    loading: false,
    onSelect: vi.fn(),
    onReload: vi.fn(),
    onCreate: vi.fn(),
    ...overrides,
  }
  const utils = render(<FolderTree {...props} />)
  return { ...utils, props }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------
describe('FolderTree', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders folder and file nodes', () => {
    renderTree()
    // Top-level folder is rendered
    expect(screen.getByRole('button', { name: /architecture/i })).toBeInTheDocument()
    // Top-level note (leaf) is rendered
    expect(screen.getByRole('button', { name: /readme/i })).toBeInTheDocument()
  })

  it('expanding a folder reveals nested note', async () => {
    const user = userEvent.setup()
    renderTree()
    // Nested child not visible initially
    expect(screen.queryByRole('button', { name: /^auth/i })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /architecture/i }))
    // Now visible
    expect(screen.getByRole('button', { name: /auth/i })).toBeInTheDocument()
  })

  it('click on a file calls onSelect with key', async () => {
    const user = userEvent.setup()
    const { props } = renderTree()
    await user.click(screen.getByRole('button', { name: /readme/i }))
    expect(props.onSelect).toHaveBeenCalledTimes(1)
    expect(props.onSelect).toHaveBeenCalledWith('readme')
  })

  it('click on a folder does NOT trigger onSelect (only toggles)', async () => {
    const user = userEvent.setup()
    const { props } = renderTree()
    await user.click(screen.getByRole('button', { name: /architecture/i }))
    expect(props.onSelect).not.toHaveBeenCalled()
  })

  it('+ button opens the create-note modal', async () => {
    const user = userEvent.setup()
    renderTree()
    const plusBtn = screen.getByTitle('New note')
    await user.click(plusBtn)
    // Modal heading
    expect(screen.getByText(/new note/i)).toBeInTheDocument()
    // Modal inputs
    expect(screen.getByPlaceholderText(/architecture\/auth/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /create note/i })).toBeInTheDocument()
  })

  it('refresh button calls onReload', async () => {
    const user = userEvent.setup()
    const { props } = renderTree()
    await user.click(screen.getByTitle('Refresh tree'))
    expect(props.onReload).toHaveBeenCalledTimes(1)
  })

  it('right-click / context menu on folder offers "new note here"', async () => {
    const user = userEvent.setup()
    renderTree()
    const folderBtn = screen.getByRole('button', { name: /architecture/i })
    // userEvent.pointer with contextmenu button
    await user.pointer({ keys: '[MouseRight]', target: folderBtn })
    const menu = await screen.findByRole('menu')
    expect(within(menu).getByRole('button', { name: /new note here/i })).toBeInTheDocument()
    expect(within(menu).getByRole('button', { name: /new subfolder/i })).toBeInTheDocument()
  })

  it('context menu "new note here" opens modal pre-filled with folder path', async () => {
    const user = userEvent.setup()
    renderTree()
    const folderBtn = screen.getByRole('button', { name: /architecture/i })
    await user.pointer({ keys: '[MouseRight]', target: folderBtn })
    const menu = await screen.findByRole('menu')
    await user.click(within(menu).getByRole('button', { name: /new note here/i }))
    const keyInput = screen.getByPlaceholderText(/architecture\/auth/) as HTMLInputElement
    expect(keyInput.value).toBe('architecture/')
  })

  it('rejects invalid keys inline (..) and does NOT call onCreate', async () => {
    const user = userEvent.setup()
    const { props } = renderTree()
    await user.click(screen.getByTitle('New note'))
    const keyInput = screen.getByPlaceholderText(/architecture\/auth/)
    await user.clear(keyInput)
    await user.type(keyInput, 'foo/../bar')
    await user.click(screen.getByRole('button', { name: /create note/i }))
    expect(screen.getByText(/`\.\.`/)).toBeInTheDocument()
    expect(props.onCreate).not.toHaveBeenCalled()
  })

  it('rejects absolute paths inline and does NOT call onCreate', async () => {
    const user = userEvent.setup()
    const { props } = renderTree()
    await user.click(screen.getByTitle('New note'))
    const keyInput = screen.getByPlaceholderText(/architecture\/auth/)
    await user.clear(keyInput)
    await user.type(keyInput, '/etc/passwd')
    await user.click(screen.getByRole('button', { name: /create note/i }))
    expect(screen.getByText(/absolute paths are not allowed/i)).toBeInTheDocument()
    expect(props.onCreate).not.toHaveBeenCalled()
  })

  it('rejects empty key inline', async () => {
    const user = userEvent.setup()
    const { props } = renderTree()
    await user.click(screen.getByTitle('New note'))
    await user.click(screen.getByRole('button', { name: /create note/i }))
    expect(screen.getByText(/key is required/i)).toBeInTheDocument()
    expect(props.onCreate).not.toHaveBeenCalled()
  })

  it('Escape closes modal without creating', async () => {
    const user = userEvent.setup()
    const { props } = renderTree()
    await user.click(screen.getByTitle('New note'))
    const keyInput = screen.getByPlaceholderText(/architecture\/auth/)
    await user.type(keyInput, 'notes/foo')
    expect(keyInput).toBeInTheDocument()
    await user.keyboard('{Escape}')
    // Modal is gone
    expect(screen.queryByPlaceholderText(/architecture\/auth/)).not.toBeInTheDocument()
    expect(props.onCreate).not.toHaveBeenCalled()
  })

  it('Enter submits the modal with valid input', async () => {
    const user = userEvent.setup()
    const onCreate = vi.fn().mockResolvedValue(undefined)
    renderTree({ onCreate })
    await user.click(screen.getByTitle('New note'))
    const keyInput = screen.getByPlaceholderText(/architecture\/auth/)
    await user.type(keyInput, 'notes/foo')
    const titleInput = screen.getByPlaceholderText(/auth architecture/i)
    await user.type(titleInput, 'Foo')
    const tagsInput = screen.getByPlaceholderText(/design, security/i)
    await user.type(tagsInput, 'a, b')
    await user.keyboard('{Enter}')
    expect(onCreate).toHaveBeenCalledTimes(1)
    expect(onCreate).toHaveBeenCalledWith('notes/foo', 'Foo', ['a', 'b'])
  })

  it('Enter with no title uses last path segment as fallback title', async () => {
    const user = userEvent.setup()
    const onCreate = vi.fn().mockResolvedValue(undefined)
    renderTree({ onCreate })
    await user.click(screen.getByTitle('New note'))
    const keyInput = screen.getByPlaceholderText(/architecture\/auth/)
    await user.type(keyInput, 'notes/bar')
    await user.keyboard('{Enter}')
    expect(onCreate).toHaveBeenCalledTimes(1)
    expect(onCreate).toHaveBeenCalledWith('notes/bar', 'bar', [])
  })

  it('clicking Create-note button submits via mouse as well', async () => {
    const user = userEvent.setup()
    const onCreate = vi.fn().mockResolvedValue(undefined)
    renderTree({ onCreate })
    await user.click(screen.getByTitle('New note'))
    await user.type(screen.getByPlaceholderText(/architecture\/auth/), 'hello')
    await user.click(screen.getByRole('button', { name: /create note/i }))
    expect(onCreate).toHaveBeenCalledWith('hello', 'hello', [])
  })

  it('Cancel button closes modal without creating', async () => {
    const user = userEvent.setup()
    const { props } = renderTree()
    await user.click(screen.getByTitle('New note'))
    await user.type(screen.getByPlaceholderText(/architecture\/auth/), 'notes/foo')
    await user.click(screen.getByRole('button', { name: /^cancel$/i }))
    expect(screen.queryByPlaceholderText(/architecture\/auth/)).not.toBeInTheDocument()
    expect(props.onCreate).not.toHaveBeenCalled()
  })

  it('shows "Loading…" when loading with empty tree', () => {
    renderTree({ tree: [], loading: true })
    expect(screen.getByText(/loading/i)).toBeInTheDocument()
  })

  it('shows empty state when not loading and tree is empty', () => {
    renderTree({ tree: [], loading: false })
    expect(screen.getByText(/no notes yet/i)).toBeInTheDocument()
  })

  it('renders link count badge for notes that have links', async () => {
    const user = userEvent.setup()
    renderTree()
    await user.click(screen.getByRole('button', { name: /architecture/i }))
    // The auth note has link_count=3
    const authBtn = screen.getByRole('button', { name: /auth/i })
    expect(within(authBtn).getByText('3')).toBeInTheDocument()
  })
})
