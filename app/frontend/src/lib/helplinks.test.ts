import { describe, expect, it } from 'vitest'
import { helpUrl } from './helplinks'
import { mainItems } from './mainmenu'
import { isWebUrl } from './openurl'

describe('helpUrl', () => {
  it('gives a web address for the documentation and for a report', () => {
    expect(isWebUrl(helpUrl('docs')!)).toBe(true)
    expect(isWebUrl(helpUrl('report')!)).toBe(true)
  })
  it('gives nothing for an action that is not a link', () => {
    expect(helpUrl('welcome')).toBeUndefined()
  })
  it('is reachable from the Help items of the window menu', () => {
    const actions = mainItems.map((i) => i.action)

    expect(actions).toEqual(expect.arrayContaining(['docs', 'report', 'shortcuts']))
  })
})
