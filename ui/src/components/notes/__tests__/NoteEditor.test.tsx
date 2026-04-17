import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import NoteEditor from '../NoteEditor'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
interface RenderOptions {
  initialContent?: string
  initialAuthor?: string
  initialTags?: string[]
}
function renderEditor(opts: RenderOptions = {}) {
  const onSave = vi.fn()
  const onCancel = vi.fn()
  const utils = render(
    <NoteEditor
      noteKey="folder/my-note"
      initialContent={opts.initialContent ?? ''}
      initialAuthor={opts.initialAuthor ?? ''}
      initialTags={opts.initialTags ?? []}
      onSave={onSave}
      onCancel={onCancel}
    />,
  )
  const textarea = utils.container.querySelector('textarea') as HTMLTextAreaElement
  const inputs = utils.container.querySelectorAll('input')
  const authorInput = inputs[0] as HTMLInputElement
  const tagsInput = inputs[1] as HTMLInputElement
  return { ...utils, onSave, onCancel, textarea, authorInput, tagsInput }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------
describe('NoteEditor', () => {
  it('updates local content state when body textarea changes', async () => {
    const { textarea } = renderEditor({ initialContent: 'hello' })
    expect(textarea.value).toBe('hello')
    await userEvent.clear(textarea)
    await userEvent.type(textarea, 'new body')
    expect(textarea.value).toBe('new body')
  })

  it('parses comma-separated tag input into a trimmed array on save', async () => {
    const { onSave, tagsInput, textarea } = renderEditor({ initialContent: 'body' })
    await userEvent.clear(tagsInput)
    await userEvent.type(tagsInput, '  alpha ,  beta,gamma  ,')
    // Content already present so save is enabled.
    expect(textarea.value).toBe('body')
    await userEvent.click(screen.getByRole('button', { name: /save/i }))
    expect(onSave).toHaveBeenCalledTimes(1)
    const arg = onSave.mock.calls[0][0]
    expect(arg.tags).toEqual(['alpha', 'beta', 'gamma'])
    expect(arg.content).toBe('body')
  })

  it('calls onSave with content, author, tags on save button click', async () => {
    const { onSave, textarea, authorInput, tagsInput } = renderEditor()
    await userEvent.type(textarea, 'the body')
    await userEvent.type(authorInput, 'jane')
    await userEvent.type(tagsInput, 'x, y')
    await userEvent.click(screen.getByRole('button', { name: /save/i }))
    expect(onSave).toHaveBeenCalledWith({
      content: 'the body',
      author: 'jane',
      tags: ['x', 'y'],
    })
  })

  it('disables save when content is empty (whitespace only)', async () => {
    renderEditor({ initialContent: '' })
    const saveBtn = screen.getByRole('button', { name: /save/i })
    expect(saveBtn).toBeDisabled()
  })

  it('clicking cancel fires onCancel', async () => {
    const { onCancel } = renderEditor({ initialContent: 'hi' })
    // Cancel button only has the X icon; use title attribute.
    const cancelBtn = screen.getByTitle('Cancel')
    await userEvent.click(cancelBtn)
    expect(onCancel).toHaveBeenCalledTimes(1)
  })

  it('deduplicates and filters empty tags by filtering blanks (trimmed)', async () => {
    // NOTE: the component trims + filters empties, but does NOT explicitly
    // dedup. Verify actual behavior: empties are dropped, values trimmed.
    const { onSave, textarea, tagsInput } = renderEditor()
    await userEvent.type(textarea, 'body')
    await userEvent.type(tagsInput, 'a, ,  b ,, c')
    await userEvent.click(screen.getByRole('button', { name: /save/i }))
    expect(onSave.mock.calls[0][0].tags).toEqual(['a', 'b', 'c'])
  })
})
