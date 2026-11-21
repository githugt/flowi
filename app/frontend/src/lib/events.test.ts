import { describe, expect, it } from 'vitest'
import { eventNames } from './events'

// The test runner has no Node typings in the project, so fs is imported the way tokens.test.ts does.
async function goEmits(): Promise<Set<string>> {
  // @ts-ignore
  const { readdirSync, readFileSync } = (await import('node:fs')) as {
    readdirSync: (url: URL) => string[]
    readFileSync: (url: URL, enc: string) => string
  }
  const dirs = ['../../../internal/bridge/', '../../../']
  const names = new Set<string>()
  for (const dir of dirs) {
    for (const f of readdirSync(new URL(dir, import.meta.url))) {
      if (!f.endsWith('.go') || f.endsWith('_test.go')) continue
      const text = readFileSync(new URL(dir + f, import.meta.url), 'utf8')
      for (const m of text.matchAll(/\bemit\("([a-z-]+)"/g)) names.add(m[1])
    }
  }
  return names
}

describe('event names', () => {
  // Before: the names were bare strings at 20 listening sites, with nothing to compare them with what Go emits.
  it('lists every event Go emits, and Go emits every listed one', async () => {
    const emitted = await goEmits()

    expect(emitted.size).toBeGreaterThan(20) // the scan still finds them
    expect([...emitted].filter((n) => !(eventNames as readonly string[]).includes(n))).toEqual([])
    expect(eventNames.filter((n) => !emitted.has(n))).toEqual([])
  })
  it('has no name twice', () => {
    expect(new Set(eventNames).size).toBe(eventNames.length)
  })
})
