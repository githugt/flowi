import { describe, expect, it } from 'vitest'
import { breakdownText, hiddenCount, visibleIdeas, type Idea } from './home'

const idea = (id: string): Idea => ({ id, title: id, why: '', action: 'screen:x', label: 'Go' })

describe('visibleIdeas', () => {
  it('leaves out the dismissed ones and keeps the order', () => {
    const got = visibleIdeas([idea('a'), idea('b'), idea('c')], new Set(['b']), 5)
    expect(got.shown.map((i) => i.id)).toEqual(['a', 'c'])
  })

  it('shows at most the limit and says how many more there are', () => {
    const got = visibleIdeas([idea('a'), idea('b'), idea('c'), idea('d'), idea('e')], new Set(), 3)
    expect(got.shown.map((i) => i.id)).toEqual(['a', 'b', 'c'])
    expect(got.more).toBe(2)
  })

  it('does not count dismissed ideas as more', () => {
    expect(visibleIdeas([idea('a'), idea('b')], new Set(['a', 'b']), 3)).toEqual({ shown: [], more: 0 })
  })
})

describe('breakdownText', () => {
  it('names the kinds of request that exist, and only those', () => {
    expect(breakdownText({ http: 12, messaging: 2, grpc: 0, datasets: 1 })).toBe('12 HTTP · 2 messaging · 1 data')
  })

  it('is empty when there are no requests', () => {
    expect(breakdownText({ http: 0, messaging: 0, grpc: 0, datasets: 0 })).toBe('')
  })
})

describe('hiddenCount', () => {
  it('counts the dismissed ideas that are still on offer', () => {
    expect(hiddenCount([idea('a'), idea('b'), idea('c')], new Set(['a', 'c']))).toBe(2)
  })

  it('ignores a dismissed idea that is no longer offered', () => {
    // The user has since used the feature, so the rule stopped suggesting it: there is nothing to bring back.
    expect(hiddenCount([idea('a')], new Set(['gone']))).toBe(0)
  })
})
