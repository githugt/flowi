import { describe, expect, it } from 'vitest'
import { bandColour, envSwatch } from './envcolours'

describe('envSwatch', () => {
  it('should give a CSS colour for a palette name', () => {
    expect(envSwatch('red')).toMatch(/^#[0-9a-f]{6}$/)
  })

  it.each([undefined, '', 'chartreuse'])('should give nothing for %j, so the default look is kept', (colour) => {
    expect(envSwatch(colour)).toBeUndefined()
  })
})

describe('bandColour', () => {
  it('is the colour of the selected environment, and nothing for none or the default', () => {
    // Before: production safety rested on one small chip at the far right of the tab strip.
    const tree = { selected: 'prod', envColours: { prod: 'red', local: '' } }

    expect(bandColour(tree)).toBe(envSwatch('red'))
    expect(bandColour({ ...tree, selected: 'local' })).toBeUndefined()
    expect(bandColour({ ...tree, selected: '' })).toBeUndefined()
    expect(bandColour(null)).toBeUndefined()
  })

  it('turns the chip amber only when the open request needs an environment that is not chosen', async () => {
    // Before: the chip was amber whenever no environment was selected, even for a request with no {{env.…}} values.
    const app = (await import('../App.svelte?raw')).default as string
    const bar = (await import('../MissingBar.svelte?raw')).default as string

    expect(app).toContain('class:none={!app.tree.selected && app.envWanted}')
    expect(bar).toContain('app.envWanted = missing.vars.length > 0 && !missing.environment')
    expect(bar).toContain('return () => (app.envWanted = false)')
  })

  it('is drawn under the tab strip and over the footer', async () => {
    const app = (await import('../App.svelte?raw')).default as string
    const footer = (await import('../Footer.svelte?raw')).default as string

    expect(app).toContain('class="envband"')
    expect(footer).toContain('border-top: 2px solid var(--env, transparent)')
  })
})
