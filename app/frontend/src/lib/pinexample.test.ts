import { describe, expect, it } from 'vitest'
import { overlayDeleteText, replaceExampleText } from './pinexample'

describe('replaceExampleText', () => {
  it('Given a typed name When asking Then the dialog names it and offers Replace', () => {
    // Before: pinning wrote over an example of the same name without a word.
    const d = replaceExampleText(' ok ')

    expect(d.text).toContain('named ok is already pinned')
    expect(d.action).toBe('Replace')
  })
  it('Given no name When asking Then it names the status the example would take', () => {
    expect(replaceExampleText('', 200).text).toContain('named 200 is')
    expect(replaceExampleText('', '404 Not Found').text).toContain('named 404 is')
  })
})

describe('overlayDeleteText', () => {
  it('Given no flow uses it When asking Then it only says the responses stay', () => {
    expect(overlayDeleteText('vip', [])).toBe('Delete vip? Responses kept for it stay in the data folder.')
  })
  it('Given one flow When asking Then it names the flow', () => {
    // Before: the dialog never said which flows named the overlay.
    expect(overlayDeleteText('vip', ['signup'])).toContain('The flow signup has a step that lays it over a request')
  })
  it('Given several flows When asking Then it counts them and lists the first five', () => {
    const t = overlayDeleteText('vip', ['a', 'b', 'c', 'd', 'e', 'f', 'g'])

    expect(t).toContain('7 flows (a, b, c, d, e and 2 more) have steps that lay it')
  })
})
