import { describe, expect, it } from 'vitest'
import { collapsedOnOpen, loadPins, savePins, type Pins } from './sidebar'

const none: Pins = { flow: false, request: false }

describe('collapsedOnOpen', () => {
  it('collapses when a flow tab opens, unless the sidebar is pinned open for flows', () => {
    expect(collapsedOnOpen('flow', false, none)).toBe(true)
    expect(collapsedOnOpen('flow', false, { ...none, flow: true })).toBe(false)
  })
  it('leaves the sidebar as it was when a request tab opens', () => {
    expect(collapsedOnOpen('request', false, none)).toBe(false)
    expect(collapsedOnOpen('request', true, none)).toBe(true)
  })
  it('opens it again for a request when pinned open for requests', () => {
    expect(collapsedOnOpen('request', true, { ...none, request: true })).toBe(false)
  })
  it('leaves it as it was for a screen that is neither', () => {
    expect(collapsedOnOpen('other', true, none)).toBe(true)
    expect(collapsedOnOpen('other', false, { ...none, flow: true })).toBe(false)
  })
})

describe('pins', () => {
  const store = () => {
    const m = new Map<string, string>()
    return { getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v) }
  }
  it('keeps the choice for each tab kind', () => {
    const s = store()
    savePins(s, { flow: true, request: false })
    expect(loadPins(s)).toEqual({ flow: true, request: false })
  })
  it('starts unpinned when nothing was kept or what was kept is not readable', () => {
    expect(loadPins(store())).toEqual(none)
    const s = store()
    s.setItem('sidebar:pins', '{oops')
    expect(loadPins(s)).toEqual(none)
  })
  it('survives a store that refuses', () => {
    const broken = { getItem: () => { throw new Error('no') }, setItem: () => { throw new Error('no') } }
    expect(loadPins(broken)).toEqual(none)
    expect(() => savePins(broken, none)).not.toThrow()
  })
})
