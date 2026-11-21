import { describe, expect, it } from 'vitest'

// The test runner has no Node typings; package.json is read from disk.
// @ts-ignore
const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
const pkg = JSON.parse(readFileSync(new URL('../../package.json', import.meta.url), 'utf8')) as { scripts: Record<string, string> }

describe('the type-check script', () => {
  it('fails on a warning, so an unused style rule or a missing label cannot come back', () => {
    // Before: `npm run check` printed warnings and passed, and the count crept up between clean-ups.
    expect(pkg.scripts.check).toContain('svelte-check')
    expect(pkg.scripts.check).toContain('--fail-on-warnings')
  })

  it('lints the style blocks too, so a duplicate rule or a typo in a property cannot come back', () => {
    // Before: Stylelint was not set up, and eleven components had a selector written twice or a property declared twice.
    expect(pkg.scripts.check).toContain('npm run lint:css')
    expect(pkg.scripts['lint:css']).toContain('stylelint')
    expect(pkg.scripts['lint:css']).toContain('svelte')
  })
})
