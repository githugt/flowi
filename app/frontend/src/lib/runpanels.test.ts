import { describe, expect, it } from 'vitest'
import { LIVE, closeRunPanel, runPanelsOf, toggleRunPanel } from './runpanels.svelte'

describe('which run the steps and timeline panels show', () => {
  // Before: Steps and Timeline were toolbar toggles that could only show the last run.
  it('Given nothing is open When a run is chosen Then that panel shows it and the other stays closed', () => {
    toggleRunPanel('flow-a', 'steps', 'RUN1')

    expect(runPanelsOf('flow-a')).toEqual({ steps: 'RUN1', timeline: null })
  })

  it('Given a panel shows a run When the same run is chosen again Then it closes, and another run replaces it', () => {
    toggleRunPanel('flow-b', 'timeline', LIVE)
    toggleRunPanel('flow-b', 'timeline', 'RUN2')
    const replaced = runPanelsOf('flow-b').timeline
    toggleRunPanel('flow-b', 'timeline', 'RUN2')

    expect(replaced).toBe('RUN2')
    expect(runPanelsOf('flow-b').timeline).toBeNull()
  })

  it('Given a flow with its own panels When another flow opens one Then they do not share it, and close leaves the other panel', () => {
    toggleRunPanel('flow-c', 'steps', 'RUN1')
    toggleRunPanel('flow-c', 'timeline', 'RUN1')
    closeRunPanel('flow-c', 'steps')

    expect(runPanelsOf('flow-c')).toEqual({ steps: null, timeline: 'RUN1' })
    expect(runPanelsOf('flow-d')).toEqual({ steps: null, timeline: null })
  })
})
