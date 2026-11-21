import { describe, expect, it } from 'vitest'
import screen from '../ReplayerScreen.svelte?raw'

describe('recorded requests card wiring', () => {
  it('should show what a run saved above the traffic, from the bridge view', () => {
    // Before: record mode ended with no result.
    expect(screen).toContain('view.recorded.count > 0')
    expect(screen).toContain('recordedHeadline(view.recorded)')
    expect(screen.indexOf('recordedHeadline(view.recorded)')).toBeLessThan(screen.indexOf('<TrafficLog'))
  })
})
