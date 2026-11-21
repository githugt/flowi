import { describe, expect, it } from 'vitest'
import { quoteFamily } from './fontfamily'

describe('quoteFamily', () => {
  it('quotes a single name', () => {
    expect(quoteFamily('JetBrains Mono')).toBe('"JetBrains Mono"')
    expect(quoteFamily('  Inter ')).toBe('"Inter"')
  })
  it('keeps a list or an already quoted name', () => {
    expect(quoteFamily('Inter, system-ui')).toBe('Inter, system-ui')
    expect(quoteFamily('"Fira Code"')).toBe('"Fira Code"')
  })
  it('refuses what could end the declaration, and empty text', () => {
    expect(quoteFamily('x; color: red')).toBe('')
    expect(quoteFamily('a"b')).toBe('')
    expect(quoteFamily('')).toBe('')
  })
})
