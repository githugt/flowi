import { describe, expect, it } from 'vitest'

// Before: Hint and ExprHelp each drew their own "?" button. ExprHelp's font line (`font: 600 10px/1 inherit`) was
// invalid CSS, so the browser dropped it and the mark sat low with its dot on the ring, while Hint's was centred. Both
// now draw HelpButton, and any new help has to as well, so the look is written once.
const components = import.meta.glob('../*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

const files = Object.entries(components).map(([path, text]) => ({ file: path.replace('../', ''), text }))

describe('the round "?" help button', () => {
  it('is drawn only by HelpButton: no other component has a button that is just a "?"', () => {
    const own = files.filter((f) => f.file !== 'HelpButton.svelte' && /<button\b[^>]*>\s*\?\s*<\/button>/.test(f.text)).map((f) => f.file)

    expect(own).toEqual([])
  })

  it('is what Hint and ExprHelp draw', () => {
    const users = files.filter((f) => f.file === 'Hint.svelte' || f.file === 'ExprHelp.svelte')

    expect(users).toHaveLength(2)
    for (const f of users) expect(f.text, f.file).toContain('<HelpButton')
  })

  it('centres the mark by its box with a valid font, so it cannot land on the ring', () => {
    const button = files.find((f) => f.file === 'HelpButton.svelte')?.text ?? ''

    expect(button).toContain('place-items: center')
    expect(button).toMatch(/font: 600 10px\/1 var\(--font, system-ui\)/)
    expect(button).not.toMatch(/font:[^;]*\binherit\b/)
  })

  it('keeps the click and tooltip variants apart by what the button says it does', () => {
    const button = files.find((f) => f.file === 'HelpButton.svelte')?.text ?? ''

    expect(button).toContain("[aria-haspopup] { cursor: pointer; }")
    expect(button).toContain("[aria-expanded='true']")
  })
})
