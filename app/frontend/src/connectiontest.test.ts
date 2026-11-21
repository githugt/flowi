import { describe, expect, it } from 'vitest'

describe('Test connection on a message or database request', () => {
  it('should sit beside the server box of message and database requests and call the bridge', async () => {
    // Before: no protocol had a "Test connection"; a wrong broker or DSN showed only after a full send or a 30 s timeout.
    const editor = (await import('./RequestEditor.svelte?raw')).default as string
    const button = (await import('./ConnectionTest.svelte?raw')).default as string

    expect(button).toContain('RequestService.TestConnection(node)')
    expect(button).toContain('loading={info.busy}')
    expect(editor.match(/<ConnectionTest \{node\} info=\{view\.connection\} \/>/g)?.length).toBe(2)
    // A data file has no connection to test, so the file branch must not get the button.
    expect(editor).toContain('view.connection && !fileSource')
  })

  it('should show the verdict by role status in the colours for ok and failed', async () => {
    const button = (await import('./ConnectionTest.svelte?raw')).default as string

    expect(button).toContain('role="status"')
    expect(button).toContain('var(--ok)')
    expect(button).toContain('var(--fail)')
  })
})
