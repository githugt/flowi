import { describe, expect, it } from 'vitest'
import splitter from './Splitter.svelte?raw'
import app from './App.svelte?raw'
import footer from './Footer.svelte?raw'

describe('Splitter snap-to-collapse', () => {
  it('calls oncollapse on release past the snap distance, keeps the width at the minimum, and shows it with the failure colour', () => {
    expect(splitter).toContain('oncollapse?: () => void')
    expect(splitter).toContain('wantsCollapse(raw, min)')
    expect(splitter).toMatch(/if \(folding\) \{\s*folding = false\s*oncollapse\?\.\(\)/)
    expect(splitter).toContain('width = Math.min(max, Math.max(min, raw))')
    expect(splitter).toContain('.split.folding')
  })

  it('is wired for the two panes that can fold: the sidebar to its rail and the bottom panel closed', () => {
    expect(app).toMatch(/<Splitter name="sidebar"[^>]*oncollapse=\{fold\}/)
    expect(footer).toMatch(/<Splitter name="footer-height"[^>]*oncollapse=\{\(\) => \(open = null\)\}/)
  })
})
