import { describe, expect, it } from 'vitest'
import { rememberTheme, themeEntry, THEME_KEY } from './themecache'

const fake = () => {
  const m = new Map<string, string>()
  return { m, getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v), removeItem: (k: string) => void m.delete(k) }
}

describe('themecache', () => {
  it('stores the theme with its mode', () => {
    // Before: the first paint was the dark default whatever theme was chosen.
    const s = fake()
    rememberTheme(s, 'solarized-light')
    expect(JSON.parse(s.m.get(THEME_KEY)!)).toEqual({ theme: 'solarized-light', mode: 'light' })
  })
  it('forgets the entry when the window follows the system', () => {
    const s = fake()
    rememberTheme(s, 'light')
    rememberTheme(s, 'system')
    expect(s.m.has(THEME_KEY)).toBe(false)
  })
  it('does not throw when storage does', () => {
    const bad = { getItem: () => null, setItem: () => { throw new Error('full') }, removeItem: () => { throw new Error('full') } }
    expect(() => rememberTheme(bad, 'dark')).not.toThrow()
    expect(themeEntry('')).toBe('')
  })
  it('is read by the boot script (a file, loaded by index.html before the app) under the same key', async () => {
    // @ts-ignore node module in a vitest-only test
    const { readFileSync } = await import('node:fs')
    const html = readFileSync(new URL('../../index.html', import.meta.url), 'utf8')
    const boot = readFileSync(new URL('../../public/theme-boot.js', import.meta.url), 'utf8')
    expect(boot).toContain(`getItem('${THEME_KEY}')`)
    expect(boot).toContain('dataset.theme')
    // Before: the script was inline; the Content-Security-Policy allows no inline script, so it is a file.
    expect(html.indexOf('src="/theme-boot.js"')).toBeGreaterThan(-1)
    expect(html.indexOf('src="/theme-boot.js"')).toBeLessThan(html.indexOf('/src/main.ts'))
  })
})
