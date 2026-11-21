import { describe, expect, it } from 'vitest'
// @ts-ignore
import stepCard from '../StepCard.svelte?raw'
// @ts-ignore
import edgeCard from '../EdgeCard.svelte?raw'
// @ts-ignore
import assertCards from '../AssertCards.svelte?raw'

// Before: the canvas cards used six pixel sizes, two of them (12.5 and 13.5) off any scale, written out in each file.
describe('canvas type scale', () => {
  it('writes no raw pixel font size in a card', () => {
    const raw = (name: string, src: string) => [...src.matchAll(/font-size:\s*[0-9.]+px/g)].map((m) => `${name}: ${m[0]}`)

    expect([...raw('StepCard', stepCard), ...raw('EdgeCard', edgeCard), ...raw('AssertCards', assertCards)]).toEqual([])
  })
})
