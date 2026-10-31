import { describe, expect, it } from 'vitest'
import { envSwatch } from './envcolours'

describe('envSwatch', () => {
  it('should give a CSS colour for a palette name', () => {
    expect(envSwatch('red')).toMatch(/^#[0-9a-f]{6}$/)
  })

  it.each([undefined, '', 'chartreuse'])('should give nothing for %j, so the default look is kept', (colour) => {
    expect(envSwatch(colour)).toBeUndefined()
  })
})
