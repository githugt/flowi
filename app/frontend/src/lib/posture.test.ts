import { describe, expect, it } from 'vitest'
import { posture, weakLines } from './posture'

describe('posture', () => {
  it('ticks the safe values and warns about the weaker ones, with the safe value to restore', () => {
    const lines = posture([
      { name: 'ui.snippet_exclude_secrets', value: 'false' },
      { name: 'datasets.allow_writes', value: 'false' },
      { name: 'http.redirect_credentials', value: 'keep' },
      { name: 'timeout', value: '30s' },
    ])

    // Before: no screen said whether these choices were weaker than the defaults.
    expect(lines.map((l) => [l.name, l.ok])).toEqual([
      ['ui.snippet_exclude_secrets', false],
      ['datasets.allow_writes', true],
      ['http.redirect_credentials', false],
    ])
    expect(lines[0].safe).toBe('true')
    expect(lines[2].safe).toBe('strip')
  })
  it('skips a setting that is not in the list', () => {
    expect(posture([])).toEqual([])
  })
  // Before: each weaker choice had its own Fix, and putting several back meant clicking through them one by one.
  it('lists only the weaker lines for one reset, and none when everything is safe', () => {
    const weak = weakLines(posture([
      { name: 'ui.snippet_exclude_secrets', value: 'false' },
      { name: 'datasets.allow_outside_collection', value: 'true' },
      { name: 'secrets.host_check', value: 'block' },
    ]))

    expect(weak.map((l) => [l.name, l.safe])).toEqual([['ui.snippet_exclude_secrets', 'true'], ['datasets.allow_outside_collection', 'false']])
    expect(weakLines(posture([{ name: 'secrets.host_check', value: 'block' }]))).toEqual([])
  })
})
