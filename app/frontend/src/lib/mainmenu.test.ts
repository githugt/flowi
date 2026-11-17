import { describe, expect, it } from 'vitest'
import { actionForKey, isLinux, mainItems } from './mainmenu'

const press = (key: string, o: Partial<Parameters<typeof actionForKey>[0]> = {}) => actionForKey({ key, code: '', ctrlKey: true, shiftKey: false, altKey: false, metaKey: false, ...o })

describe('actionForKey', () => {
  it('maps the shortcuts of the menu', () => {
    expect(press('n')).toBe('new-request')
    expect(press('N', { shiftKey: true })).toBe('new-flow')
    expect(press('k')).toBe('jump')
    expect(press('P', { shiftKey: true })).toBe('palette')
    expect(press('w')).toBe('close-tab')
    expect(press('}', { shiftKey: true, code: 'BracketRight' })).toBe('next-tab')
    expect(press('{', { shiftKey: true, code: 'BracketLeft' })).toBe('previous-tab')
    expect(press(',')).toBe('settings')
  })
  it('leaves other keys alone, with or without Ctrl', () => {
    expect(press('s')).toBe('')
    expect(press('n', { ctrlKey: false })).toBe('')
    expect(press('n', { altKey: true })).toBe('')
    expect(press('c', { shiftKey: true })).toBe('')
  })
  it('names an action for every shortcut it lists', () => {
    for (const it of mainItems.filter((i) => i.shortcut)) expect(it.action).not.toBe('')
  })
})

describe('isLinux', () => {
  it('is true for desktop Linux only', () => {
    expect(isLinux('Mozilla/5.0 (X11; Linux x86_64)')).toBe(true)
    expect(isLinux('Mozilla/5.0 (Linux; Android 14)')).toBe(false)
    expect(isLinux('Mozilla/5.0 (Windows NT 10.0)')).toBe(false)
  })
})
