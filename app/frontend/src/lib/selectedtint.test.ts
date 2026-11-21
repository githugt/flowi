import { describe, expect, it } from 'vitest'

// "Selected" has one strength, --tint-sel, so a chosen row looks the same in every list and never like a hover.
// Before: eleven lists spelled the tint themselves, at 14, 22, 24 or 25 per cent.
const components = import.meta.glob('../*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

// Background tints only: a glow or a canvas fill is not a row's selected state.
const SELECTOR =/\.(sel|selected|on|cur|active)\b[^{]*\{[^}]*background:[^;}]*color-mix\(in srgb, var\(--accent\) \d+%/

describe('the selected tint', () => {
  it('is the shared token in every selected-row rule', () => {
    const own = Object.entries(components)
      .flatMap(([file, text]) => text.split('\n').map((line, i) => ({ at: `${file.replace('../', '')}:${i + 1}`, line })))
      .filter(({ line }) => SELECTOR.test(line))
      .map(({ at }) => at)

    expect(own).toEqual([])
  })
})
