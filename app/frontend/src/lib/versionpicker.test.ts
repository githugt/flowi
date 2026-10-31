import { describe, expect, it } from 'vitest'
import { pickerItems, pickerLabel, toggleOverlay } from './versionpicker'

const input = { versions: ['1', '2'], names: ['', 'beta'], def: '1', selected: '2', overlays: ['vip', 'slow'], stack: ['vip'] }

describe('pickerItems', () => {
  it('groups the versions, then the overlays with a None first', () => {
    const items = pickerItems(input)
    expect(items.map((i) => `${i.group}:${i.id || 'none'}`)).toEqual(['Versions:1', 'Versions:2', 'Overlays:none', 'Overlays:vip', 'Overlays:slow'])
  })
  it('marks the open version and the overlays laid over it', () => {
    const on = pickerItems(input).filter((i) => i.on).map((i) => i.id)
    expect(on).toEqual(['2', 'vip'])
  })
  it('labels a version with its name and says which is the default', () => {
    const [v1, v2] = pickerItems(input)
    expect(v1.label).toBe('1 (default)')
    expect(v2.label).toBe('2 · beta')
  })
  it('marks None when no overlay is laid over', () => {
    expect(pickerItems({ ...input, stack: [] }).find((i) => i.id === '' )?.on).toBe(true)
  })
})

describe('toggleOverlay', () => {
  it('stacks a new overlay after the ones already there, in order', () => {
    expect(toggleOverlay(['vip'], 'slow')).toBe('vip+slow')
  })
  it('takes an overlay out of the stack and keeps the order of the rest', () => {
    expect(toggleOverlay(['vip', 'slow', 'eu'], 'slow')).toBe('vip+eu')
  })
  it('clears everything for None', () => {
    expect(toggleOverlay(['vip', 'slow'], '')).toBe('')
  })
})

describe('pickerLabel', () => {
  it('shows the version, and the overlays after it', () => {
    expect(pickerLabel('2', ['vip', 'slow'])).toBe('2 + vip + slow')
    expect(pickerLabel('2', [])).toBe('2')
  })
})
