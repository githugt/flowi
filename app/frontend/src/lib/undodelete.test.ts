import { describe, expect, it } from 'vitest'

describe('undoing a delete', () => {
  it('should offer Undo on a toast that stays eight seconds and calls TrashService.Undo', async () => {
    // Before: a delete in the tree was permanent, so each one asked "This cannot be undone" first.
    const app = (await import('./app.svelte?raw')).default as string
    const toasts = (await import('../Toasts.svelte?raw')).default as string
    const ui = (await import('./ui.svelte?raw')).default as string

    expect(app).toContain("onEvent('deleted'")
    expect(app).toContain('const UNDO_MS = 8000')
    expect(app).toContain("label: 'Undo'")
    expect(app).toContain('TrashService.Undo(ids)')
    expect(ui).toContain('options.life ?? toastLife(text, level)')
    expect(toasts).toContain('t.action.label')
  })
})
