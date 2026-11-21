import { describe, expect, it } from 'vitest'
import { paintOrder } from './zorder'
import canvas from '../Canvas.svelte?raw'

const u = (id: string, card = true) => ({ id, card })
const ids = (l: { id: string }[]) => l.map((x) => x.id)

describe('paintOrder', () => {
  const units = [u('a'), u('b'), u('c'), u('d')]

  it('draws the selected cards after the others and the dragged one last', () => {
    // Before: layout order, so a card dragged onto another slid under it.
    expect(ids(paintOrder(units, new Set(['b']), undefined, (x) => x.card))).toEqual(['a', 'c', 'd', 'b'])
    expect(ids(paintOrder(units, new Set(['b', 'a']), 'a', (x) => x.card))).toEqual(['c', 'd', 'b', 'a'])
  })

  it('keeps the order inside a group and changes nothing with nothing selected', () => {
    expect(ids(paintOrder(units, new Set(), undefined, (x) => x.card))).toEqual(['a', 'b', 'c', 'd'])
    expect(ids(paintOrder(units, new Set(['d', 'b']), undefined, (x) => x.card))).toEqual(['a', 'c', 'b', 'd'])
  })

  it('leaves containers where they are, under their cards', () => {
    const mixed = [u('box', false), u('x'), u('y')]

    expect(ids(paintOrder(mixed, new Set(['box', 'x']), 'box', (m) => m.card))).toEqual(['box', 'y', 'x'])
  })

  it('does not change the list it is given', () => {
    const copy = [...units]
    paintOrder(units, new Set(['a']), 'a', (x) => x.card)

    expect(units).toEqual(copy)
  })

  it('is what the canvas draws its cards from', () => {
    expect(canvas).toContain('paintOrder(units')
    expect(canvas).toContain('{#each painted as u (u.id)}')
  })
})
