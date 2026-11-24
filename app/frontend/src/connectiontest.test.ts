import { describe, expect, it } from 'vitest'

describe('Test connection on every kind of request', () => {
  it('should sit beside the address of every kind of request and call the bridge', async () => {
    // Before: no protocol had a "Test connection"; a wrong broker or DSN showed only after a full send or a 30 s timeout, and
    // an HTTP request, a gRPC call and a data file had none at all.
    const editor = (await import('./RequestEditor.svelte?raw')).default as string
    const button = (await import('./ConnectionTest.svelte?raw')).default as string

    expect(button).toContain('RequestService.TestConnection(node)')
    expect(button).toContain('loading={info.busy}')
    expect(editor.match(/<ConnectionTest \{node\} info=\{view\.connection\}/g)?.length).toBe(4)
    for (const kind of ['kind="http"', 'kind="grpc"', "kind={fileSource ? 'file' : 'connect'}"]) expect(editor).toContain(kind)
  })

  it('should show the verdict by role status in the colours for ok and failed', async () => {
    const button = (await import('./ConnectionTest.svelte?raw')).default as string

    expect(button).toContain('role="status"')
    expect(button).toContain('var(--ok)')
    expect(button).toContain('var(--fail)')
  })
})
