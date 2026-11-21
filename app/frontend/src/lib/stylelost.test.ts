import { describe, expect, it } from 'vitest'

// Before: a clean-up that removed the style rules a migration had orphaned also removed `.addr` (the request bar's
// outline and its row layout) and `summary` (the settings groups), which were still in use. Nothing failed: svelte-check
// only reports a rule with no element, never an element with no rule, so the method, the address and Send stacked in
// a column until someone looked at the screen.
//
// A class used by a wrapper element must keep a rule in the component's own <style>. The list is the wrappers whose
// whole job is layout; add one when a component gains such a wrapper.
const components = import.meta.glob('../*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

const WRAPPERS: Record<string, string[]> = {
  'RequestEditor.svelte': ['.addr', '.url', '.url2', '.send', '.hookbadge', '.edited', '.panels', '.left', '.right', '.secnav', '.sectabs'],
  'SettingsScreen.svelte': ['h3, summary', '.sub summary', '.swatches', '.tabbar'],
  'ResponsePane.svelte': ['.cards', '.hcard', '.timeline', '.tick', '.chead'],
}

const styleOf = (text: string): string => /<style[^>]*>([\s\S]*?)<\/style>/.exec(text)?.[1] ?? ''
const source = (file: string): string => components[`../${file}`] ?? ''

describe('layout wrappers keep their style rules', () => {
  for (const [file, selectors] of Object.entries(WRAPPERS)) {
    it(`${file} styles ${selectors.join(' ')}`, () => {
      const style = styleOf(source(file))
      const missing = selectors.filter((s) => !new RegExp(`(^|\\n)\\s*${s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*[{:]`).test(style))

      expect(source(file)).not.toBe('')
      expect(missing).toEqual([])
    })
  }

  it('lays the request bar out as one row, with an outline', () => {
    const style = styleOf(source('RequestEditor.svelte'))
    const addr = /(^|\n)\s*\.addr\s*\{([^}]*)\}/.exec(style)?.[2] ?? ''

    expect(addr).toContain('display: flex')
    expect(addr).toContain('border: 1px solid var(--line)')
    expect(style).toMatch(/\.addr:focus-within/)
    expect(style).toMatch(/\.addr \.send\s*\{/)
  })

  it('keeps the tab strips as they were drawn', () => {
    const tabs = styleOf(source('Tabs.svelte'))

    // Before: the shared Tabs padded a strip over a pane by 12px with a gap, so the fourth tab of the response pane ran off
    // its edge, and a full-width tab (the side panel's) put its label at the top of the bar instead of in the middle.
    expect(tabs).toMatch(/\.md\.bare button\s*\{\s*padding:\s*0 10px/)
    expect(tabs).toMatch(/\.fill button\s*\{[^}]*align-items:\s*center/)
  })

  it('draws the JSON-RPC offer as a link, as it was', () => {
    // Before: the migration made it a ghost button, grey text where every other offer in the bar is an accent link.
    expect(source('RequestEditor.svelte')).toMatch(/<Button variant="link"[^\n]*Make it a JSON-RPC call<\/Button>/)
  })
})
