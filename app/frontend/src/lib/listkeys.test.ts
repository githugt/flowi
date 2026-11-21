import { describe, expect, it } from 'vitest'
import { listKeyTarget } from './listkeys'

describe('listKeyTarget', () => {
  it('moves one row and wraps round the ends', () => {
    // Before: the palette and the dialogs stopped at the ends; the method picker wrapped.
    expect(listKeyTarget('ArrowDown', 0, 3)).toBe(1)
    expect(listKeyTarget('ArrowDown', 2, 3)).toBe(0)
    expect(listKeyTarget('ArrowUp', 0, 3)).toBe(2)
    expect(listKeyTarget('ArrowUp', 2, 3)).toBe(1)
  })
  it('starts from the first or last row when nothing is chosen', () => {
    expect(listKeyTarget('ArrowDown', -1, 4)).toBe(0)
    expect(listKeyTarget('ArrowUp', -1, 4)).toBe(3)
  })
  it('stays at 0 in an empty list, and leaves other keys alone', () => {
    expect(listKeyTarget('ArrowDown', 0, 0)).toBe(0)
    expect(listKeyTarget('a', 0, 3)).toBeNull()
    expect(listKeyTarget('Enter', 0, 3)).toBeNull()
  })
})

// @ts-ignore
import dropdown from '../Dropdown.svelte?raw'
// @ts-ignore
import menu from '../Menu.svelte?raw'

describe('Dropdown keys', () => {
  // Before: the dropdown's options could not be reached with the arrow keys and the focus stayed on the button.
  // They are the Menu's now: the dropdown opens one, and the Menu moves the focus and gives it back on Escape.
  it('moves the focus with the arrow keys, and returns it on Escape', () => {
    expect(dropdown).toMatch(/<Menu /)
    expect(menu).toContain("e.key === 'ArrowDown'")
    expect(menu).toMatch(/e\.key === 'Escape'\) onclose\(\)/)
    expect(menu).toContain('opener.focus')
  })

  it('starts the arrow keys from the chosen option', () => {
    expect(menu).toContain('button[aria-checked="true"]')
  })
})
