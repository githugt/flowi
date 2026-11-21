import { describe, expect, it } from 'vitest'
import { LEVEL_CARDS } from './telemetrycards'
import screen from '../TelemetryScreen.svelte?raw'

describe('telemetry cards', () => {
  it('has one card per level, off first, each saying what it sends', () => {
    expect(LEVEL_CARDS.map((c) => c.id)).toEqual(['off', 'minimal', 'full'])
    for (const c of LEVEL_CARDS) expect(c.summary.length, c.id).toBeGreaterThan(10)
  })

  it('shows the exact payload under its own card, with no separate preview tabs', () => {
    // Before: a tab strip changed only a preview, so clicking "Full" there looked like choosing it.
    expect(screen).toContain('Show exactly what is sent')
    expect(screen).toContain('view.previews[c.id]')
    expect(screen).not.toContain('looking')
    expect(screen).not.toContain('class="tabs"')
    expect(screen).toContain('use:radiogroup')
  })
})
