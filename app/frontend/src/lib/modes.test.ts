import { describe, expect, it } from 'vitest'
import { lostText, modesOf } from './modes'

describe('modesOf', () => {
  it('offers the three ways to call for any of them', () => {
    for (const t of ['request', 'fetch', 'webhook']) expect(modesOf(t).map((o) => o.node)).toEqual(['request', 'fetch', 'webhook'])
  })
  it('offers two ways to loop', () => {
    expect(modesOf('run_till').map((o) => o.label)).toEqual(['Over a list or N times', 'Until something holds'])
  })
  it('offers nothing for a kind with one way', () => {
    expect(modesOf('delay')).toEqual([])
  })
})

describe('lostText', () => {
  it('names what is removed and says undo brings it back', () => {
    expect(lostText(['version', 'with'])).toContain('version, with')
  })
  it('is empty when nothing is lost', () => {
    expect(lostText([])).toBe('')
  })
})
