import { describe, expect, it } from 'vitest'
import help from '../ExprHelp.svelte?raw'

describe('the expression help', () => {
  it('copies a name when its row is clicked', () => {
    // Before: the rows were read-only text; a name had to be selected by hand.
    expect(help).toContain("copy(r.name, r.name)")
  })
  it('moves focus into itself and gives it back to the ? on Escape', () => {
    // It is a Popover now: the Popover moves the focus in (lib/modal) and closes on Escape through `onclose`.
    expect(help).toContain('<Popover')
    expect(help).toContain('opener?.focus()')
    expect(help).toContain('onclose={close}')
  })
})
