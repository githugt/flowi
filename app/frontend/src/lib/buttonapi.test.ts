import { describe, expect, it } from 'vitest'
import button from '../Button.svelte?raw'
import iconButton from '../IconButton.svelte?raw'
import matrix from '../RequestMatrix.svelte?raw'

describe('button API', () => {
  it('has loading and pressed, a spinner that keeps the label, and no click while loading', () => {
    // Before: no loading, icon-only or pressed form: "Sending…" was plain text and every screen did its own.
    expect(button).toContain('loading?: boolean')
    expect(button).toContain('pressed?: boolean')
    expect(button).toContain('aria-busy={loading || undefined}')
    expect(button).toContain('disabled={rest.disabled || loading}')
    expect(button).toContain('class="spin"')
    expect(button).toContain('aria-pressed={pressed}')
  })

  it('has an icon-only button whose label cannot be left out', () => {
    expect(iconButton).toContain('label: string')
    expect(iconButton).not.toContain('label?:')
  })

  it('is used for the matrix Run while it sends', () => {
    expect(matrix).toContain('loading={busy}')
  })
})
