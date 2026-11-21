import { describe, expect, it } from 'vitest'
// @ts-ignore
import app from '../App.svelte?raw'
// @ts-ignore
import flowEditor from '../FlowEditor.svelte?raw'
// @ts-ignore
import segmented from '../Segmented.svelte?raw'

// Before: the tab strip had no tab semantics, an error showed as plain red text, the view switch declared a
// radiogroup with no radios, and run progress was not announced.
describe('accessibility markup', () => {
  it('gives the tab strip tab semantics and announces the app error', () => {
    expect(app).toContain('role="tablist"')
    expect(app).toMatch(/role="tab"[^>]*aria-selected/)
    expect(app).toMatch(/class="err" role="alert"/)
  })
  it('gives the view switch radios, and announces the run status', () => {
    expect(flowEditor).toContain('<Segmented label="View"')
    expect(segmented).toMatch(/role="radiogroup"/)
    expect(segmented).toMatch(/role="radio"\s+aria-checked/)
    expect(flowEditor).toMatch(/role="status" aria-live="polite"/)
  })
})

// @ts-ignore
import toggle from '../Toggle.svelte?raw'

describe('Toggle', () => {
  // Before: an off switch was a 9% line colour with a white knob: about 1.2:1 on a light theme, and in high contrast
  // it looked like an on switch.
  it('draws an off switch as an outline in the muted colour, which every theme keeps readable', () => {
    expect(toggle).toMatch(/\.track \{[^}]*background: transparent;[^}]*inset 0 0 0 1\.5px var\(--muted\)/)
    expect(toggle).toMatch(/\.knob \{[^}]*background: var\(--muted\)/)
  })
  it('tells on from off in forced colours by fill, not by colour alone', () => {
    expect(toggle).toMatch(/forced-colors: active[^}]*border: 2px solid ButtonText/)
    expect(toggle).toMatch(/\.on \.track \{ background: Highlight/)
  })
})

describe('startup', () => {
  // Before: until the tabs loaded, the main area said "Open a request or a flow from the sidebar" (and app.restored was never read).
  it('shows a status while the workspace is opening, not the empty-state text', () => {
    const at = app.indexOf('{:else if !app.restored}')
    expect(at).toBeGreaterThan(0)
    expect(app.indexOf('Open a request or a flow from the sidebar.')).toBeGreaterThan(at)
    expect(app.slice(at, at + 220)).toMatch(/role="status" aria-busy="true"/)
  })
})

// @ts-ignore
import codeView from '../CodeView.svelte?raw'
// @ts-ignore
import blueprints from '../BlueprintsScreen.svelte?raw'

describe('error text', () => {
  // Before: three screens and the tab opener showed String(e), which puts "Error: " in front of what Go said.
  it('shows the message of a failed call, not String(e)', () => {
    expect(codeView).not.toMatch(/String\(e\)/)
    expect(blueprints).not.toMatch(/String\(e\)/)
    expect(codeView).toContain("from './lib/errors'")
  })
})

describe('stale error', () => {
  // Before: app.error stayed until the next open(), whatever tab the person moved to.
  it('is cleared when the active tab changes', () => {
    expect(app).toMatch(/if \(now === shownOn\) return[\s\S]{0,60}app\.error = ''/)
  })
})
