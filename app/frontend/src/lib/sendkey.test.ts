import { describe, expect, it } from 'vitest'
import { isSendKey } from './sendkey'

const key = (over: Partial<Parameters<typeof isSendKey>[0]>) => ({ key: 'Enter', ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, repeat: false, ...over })

describe('isSendKey', () => {
  it.each([
    ['Ctrl+Enter', key({ ctrlKey: true }), true],
    ['Cmd+Enter', key({ metaKey: true }), true],
    ['Enter alone', key({}), false],
    ['Ctrl+Shift+Enter', key({ ctrlKey: true, shiftKey: true }), false],
    ['Ctrl+Alt+Enter', key({ ctrlKey: true, altKey: true }), false],
    ['a held Ctrl+Enter repeating', key({ ctrlKey: true, repeat: true }), false],
    ['Ctrl+Enter that confirms an IME composition', key({ ctrlKey: true, isComposing: true }), false],
    ['Ctrl+S', key({ ctrlKey: true, key: 's' }), false],
  ])('Given %s Then %s', (_, e, want) => {
    expect(isSendKey(e)).toBe(want)
  })
})
