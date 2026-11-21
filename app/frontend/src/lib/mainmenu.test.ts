import { describe, expect, it } from 'vitest'
import { actionForKey, isLinux, mainItems, opensMenu, pageAnswersShortcuts } from './mainmenu'

const press = (key: string, o: Partial<Parameters<typeof actionForKey>[0]> = {}) => actionForKey({ key, code: '', ctrlKey: true, shiftKey: false, altKey: false, metaKey: false, ...o })

describe('actionForKey', () => {
  it('maps the shortcuts of the menu', () => {
    expect(press('n')).toBe('new-request')
    expect(press('N', { shiftKey: true })).toBe('new-flow')
    expect(press('k')).toBe('jump')
    expect(press('P', { shiftKey: true })).toBe('palette')
    expect(press('w')).toBe('close-tab')
    expect(press('T', { shiftKey: true })).toBe('reopen-tab')
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

describe('mainItems', () => {
  // Before: the Help item "Telemetry…" opened the usage ping; people looking for OpenTelemetry tracing were misled.
  it('names the usage ping Usage reporting, not Telemetry', () => {
    const labels = mainItems.map((i) => i.label ?? '')
    expect(labels).toContain('Usage reporting…')
    expect(labels.some((l) => /telemetry/i.test(l))).toBe(false)
  })
})

describe('actionForKey on other layouts', () => {
  const press = (key: string, code: string, shift = false) => ({ key, code, ctrlKey: true, shiftKey: shift, altKey: false, metaKey: false })

  // Before: Ctrl+N and Ctrl+W did nothing on a Russian or Greek layout, where the key reads 'т' or 'ω'.
  it('follows the key position when the layout does not type Latin letters', () => {
    expect(actionForKey(press('т', 'KeyN'))).toBe('new-request')
    expect(actionForKey(press('ц', 'KeyW'))).toBe('close-tab')
    expect(actionForKey(press('Т', 'KeyN', true))).toBe('new-flow')
    expect(actionForKey(press('б', 'Comma'))).toBe('settings')
  })
  it('follows the letter on a Latin layout that moves the keys', () => {
    expect(actionForKey(press('n', 'KeyB'))).toBe('new-request') // Dvorak-like: the key says n
    expect(actionForKey(press('w', 'KeyN'))).toBe('close-tab')
  })
})

describe('pageAnswersShortcuts', () => {
  // Before: only isLinux answered, so Windows had no Ctrl+N in the page while its menu bar was hidden.
  it('is true on Linux and Windows, false on a Mac or a phone', () => {
    expect(pageAnswersShortcuts('Mozilla/5.0 (X11; Linux x86_64)')).toBe(true)
    expect(pageAnswersShortcuts('Mozilla/5.0 (Windows NT 10.0; Win64; x64)')).toBe(true)
    expect(pageAnswersShortcuts('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)')).toBe(false)
    expect(pageAnswersShortcuts('Mozilla/5.0 (Linux; Android 14)')).toBe(false)
  })
})

describe('opensMenu', () => {
  const press = (o: Partial<Parameters<typeof opensMenu>[0]>) => ({ key: '', code: '', altKey: false, ctrlKey: false, metaKey: false, shiftKey: false, ...o })
  // Before: the Linux menu button could only be clicked.
  it('is F10 or Alt+M, by key place so a Cyrillic layout works', () => {
    expect(opensMenu(press({ key: 'F10', code: 'F10' }))).toBe(true)
    expect(opensMenu(press({ key: 'ь', code: 'KeyM', altKey: true }))).toBe(true)
    expect(opensMenu(press({ key: 'm', code: 'KeyM' }))).toBe(false)
    expect(opensMenu(press({ key: 'F10', code: 'F10', shiftKey: true }))).toBe(false)
  })
})

describe('actionForKey by layout', () => {
  // Before: the rule (letters by key, brackets by code, Alt ignored) was only in the code, and tested on one layout.
  const press = (key: string, code: string, o: Partial<Parameters<typeof actionForKey>[0]> = {}) => actionForKey({ key, code, ctrlKey: true, shiftKey: false, altKey: false, metaKey: false, ...o })

  it('Ctrl+N is a new request on every layout, by what the key types or where it is', () => {
    expect(press('n', 'KeyN')).toBe('new-request') // US
    expect(press('n', 'KeyL')).toBe('new-request') // Dvorak: the N is where L is on a US board
    expect(press('т', 'KeyN')).toBe('new-request') // Russian
    expect(press('ν', 'KeyN')).toBe('new-request') // Greek
  })
  it('does not take another Latin letter that sits on the N key of this layout', () => {
    expect(press('l', 'KeyN')).toBe('') // Dvorak's L is on the US N place: not Ctrl+N
  })
  it('takes brackets and comma by place, on any layout', () => {
    expect(press('ü', 'BracketLeft', { shiftKey: true })).toBe('previous-tab') // German
    expect(press('+', 'BracketRight', { shiftKey: true })).toBe('next-tab') // German
    expect(press(',', 'Comma')).toBe('settings')
    expect(press('б', 'Comma')).toBe('settings') // Russian
  })
  it('ignores any press with Alt, so AltGr characters do not fire a shortcut', () => {
    expect(press('n', 'KeyN', { altKey: true })).toBe('')
    expect(press('@', 'KeyQ', { altKey: true })).toBe('')
  })
})
