import { describe, expect, it } from 'vitest'
import { CARD } from './card'
import { PORT_HIT, startsLink } from './porthit'
import card from '../StepCard.svelte?raw'

describe('port hit targets', () => {
  it('is 24px across, and neighbouring ports do not overlap', () => {
    // Before: the 12px port circle was the whole target.
    expect(PORT_HIT * 2).toBeGreaterThanOrEqual(24)
    expect(PORT_HIT * 2).toBeLessThanOrEqual(CARD.portGap)
  })

  it('starts a link from done, fail and outcomes only', () => {
    expect(startsLink('done')).toBe(true)
    expect(startsLink('fail')).toBe(true)
    expect(startsLink('outcome-ok')).toBe(true)
    expect(startsLink('case-0')).toBe(false)
  })

  it('puts an invisible target behind each port and each +', () => {
    expect(card).toContain('class="porthit"')
    expect(card).toContain('class="addhit"')
    expect(card).toContain('r={PORT_HIT}')
  })
})
