import { describe, expect, it } from 'vitest'
import { charWidth, clipWidth } from './measure'

const textWidth = (text: string) => Array.from(new Intl.Segmenter(undefined, { granularity: 'grapheme' }).segment(text), (s) => charWidth(s.segment)).reduce((a, b) => a + b, 0)

const cards = import.meta.glob(['../StepCard.svelte', '../EdgeCard.svelte', '../SequenceDiagram.svelte'], { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('measure', () => {
  it('weighs CJK and emoji double, narrow letters light and wide letters heavy', () => {
    expect(charWidth('漢')).toBe(2)
    expect(charWidth('😀')).toBe(2)
    expect(charWidth('🇫🇷')).toBe(2)
    expect(charWidth('é')).toBe(1)
    expect(charWidth('́')).toBe(0)
    expect(charWidth('i')).toBeLessThan(1)
    expect(charWidth('W')).toBeGreaterThan(1)
  })

  it('leaves what fits alone and ends a cut in an ellipsis', () => {
    expect(clipWidth('/health', 20)).toBe('/health')
    expect(clipWidth('x'.repeat(40), 10)).toBe('x'.repeat(9) + '…')
  })

  it('cuts CJK at half the characters', () => {
    // Before: 10 characters of CJK were as long as 10 letters, and overflowed the card by half again.
    const cut = clipWidth('漢字漢字漢字漢字漢字', 10)

    expect(cut).toBe('漢字漢字…')
    expect(textWidth(cut)).toBeLessThanOrEqual(10)
  })

  it('keeps more narrow letters and fewer wide capitals in the same room', () => {
    const narrow = clipWidth('illiliiliiliillilliiliiliil', 12)
    const wide = clipWidth('WMWMWMWMWMWMWMWMWMWM', 12)

    expect(narrow.length).toBeGreaterThan(wide.length)
    expect(textWidth(wide)).toBeLessThanOrEqual(12)
  })

  it('never splits an emoji or an accented letter, and always keeps one character', () => {
    expect(clipWidth('👍👍👍👍', 5)).toBe('👍👍…')
    expect(clipWidth('é'.repeat(8), 4)).toBe('é'.repeat(3) + '…')
    expect(clipWidth('漢字', 1)).toBe('漢…')
  })

  it('is what the cards and the diagram cut with', () => {
    for (const [file, text] of Object.entries(cards)) {
      expect(text, file).toContain('clipWidth(')
    }
  })
})
