import { describe, expect, it } from 'vitest'
import indexHtml from '../../index.html?raw'
import themeBoot from '../../public/theme-boot.js?raw'

// Before: the webview had no Content-Security-Policy, so any future injection could load a remote script or send data
// out. These checks keep the policy from being loosened by accident.
describe('index.html', () => {
  const csp = /http-equiv="Content-Security-Policy" content="([^"]+)"/.exec(indexHtml)?.[1] ?? ''
  const directive = (name: string) => csp.split(';').map((d) => d.trim()).find((d) => d.startsWith(name + ' ')) ?? ''

  it('has a policy that allows scripts from the app only', () => {
    expect(csp).not.toBe('')
    expect(directive('default-src')).toBe("default-src 'self'")
    expect(directive('script-src')).toBe("script-src 'self'")
    expect(csp).not.toContain('unsafe-eval')
    expect(directive('object-src')).toBe("object-src 'none'")
    expect(directive('base-uri')).toBe("base-uri 'none'")
  })

  it('keeps every script in a file, so the policy needs no inline exception', () => {
    const scripts = [...indexHtml.matchAll(/<script\b[^>]*>/g)].map((m) => m[0])
    expect(scripts.length).toBeGreaterThan(0)
    for (const tag of scripts) expect(tag).toContain('src=')
  })

  it('still sets the theme before the first paint, from its own file', () => {
    expect(indexHtml).toContain('src="/theme-boot.js"')
    expect(themeBoot).toContain("localStorage.getItem('flowi.theme')")
  })
})
