import { describe, expect, it } from 'vitest'
import { themeMode } from './theme'

describe('themeMode', () => {
  it.each(['light', 'flowi-light', 'solarized-light', 'anuppuccin-light'])('%s is light', (name) => {
    expect(themeMode(name)).toBe('light')
  })

  it.each(['dark', 'classic', 'flowi-dark', 'high-contrast', 'solarized-dark', 'nord', 'anuppuccin-dark'])('%s is dark', (name) => {
    expect(themeMode(name)).toBe('dark')
  })
})
