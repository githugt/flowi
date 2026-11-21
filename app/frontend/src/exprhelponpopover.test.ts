import { describe, expect, it } from 'vitest'
import help from './ExprHelp.svelte?raw'

describe('ExprHelp on Popover', () => {
  it('opens its help with Popover under the ?, and closes from there, with no window handler or clamp of its own', () => {
    // Before: it wrote a fixed box with z-index top, measured and clamped it itself, stopped pointer-downs and closed
    // on a window pointer-down and a window Escape handler.
    expect(help).toMatch(/<Popover [^>]*onclose=\{close\}/)
    expect(help).not.toContain('<svelte:window')
    expect(help).not.toContain('stopPropagation()}\n') // the button no longer stops its own pointer-down
    expect(help).not.toContain('keepInside')
    expect(help).not.toMatch(/z-index:\s*\d|z-index:\s*var/)
  })

  it('gives the focus back to the ? and keeps a click inside the help from reaching a label around the ?', () => {
    expect(help).toMatch(/function close\(\) \{\s*open = false\s*opener\?\.focus\(\)/)
    expect(help).toContain('class="help" onclick={(e) => e.stopPropagation()}')
  })
})
