import { describe, expect, it } from 'vitest'
import { collapsedOnOpen, loadMode, loadPins, saveMode, savePins, shouldExplainFold, type Pins } from './sidebar'
import { windowKeyAction } from './shortcuts'
import app from '../App.svelte?raw'

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

const mem = () => {
  const m = new Map<string, string>()
  return { getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v) }
}

describe('sidebar mode', () => {
  it('auto folds for a flow, open never folds, manual leaves it as it is', () => {
    // Before: the sidebar always folded for a flow, with no setting.
    expect(collapsedOnOpen('flow', false, none, 'auto')).toBe(true)
    expect(collapsedOnOpen('flow', true, none, 'open')).toBe(false)
    expect(collapsedOnOpen('flow', false, none, 'manual')).toBe(false)
    expect(collapsedOnOpen('flow', true, none, 'manual')).toBe(true)
    expect(collapsedOnOpen('flow', false, none)).toBe(true)
  })

  it('remembers the mode and falls back to auto', () => {
    const s = mem()
    expect(loadMode(s)).toBe('auto')
    saveMode(s, 'open')
    expect(loadMode(s)).toBe('open')
    s.setItem('sidebar:mode', 'nonsense')
    expect(loadMode(s)).toBe('auto')
  })

  it('explains the first automatic fold once', () => {
    const s = mem()

    expect(shouldExplainFold(s)).toBe(true)
    expect(shouldExplainFold(s)).toBe(false)
  })

  it('folds with Ctrl+B or Cmd+B, and not with Shift or Alt added', () => {
    const e = (o: object) => ({ key: 'b', ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, ...o })

    expect(windowKeyAction(e({ ctrlKey: true }), false)).toBe('toggle-sidebar')
    expect(windowKeyAction(e({ metaKey: true }), true)).toBe('toggle-sidebar')
    expect(windowKeyAction(e({ ctrlKey: true, shiftKey: true }), false)).toBe('')
    expect(windowKeyAction(e({}), false)).toBe('')
  })

  it('is wired: the window answers the toggle and tells why it folded', () => {
    expect(app).toContain('app.sidebarToggles')
    expect(app).toContain('shouldExplainFold(store)')
  })
})
