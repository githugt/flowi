import { describe, expect, it } from 'vitest'
import { CARD, categoryToken, clipText, colourToken, outcomeColour, portLine, portToken, toneToken } from './card'

describe('clipText', () => {
  it('leaves short text alone', () => expect(clipText('/health', 40)).toBe('/health'))
  it('cuts long text with an ellipsis at the limit', () => {
    const cut = clipText('x'.repeat(80), 40)
    expect(cut).toHaveLength(40)
    expect(cut.endsWith('…')).toBe(true)
  })
})

describe('toneToken', () => {
  it('maps each tone to a colour token and unknown ones to muted', () => {
    expect(toneToken('done')).toBe('--ok')
    expect(toneToken('fail')).toBe('--fail')
    expect(toneToken('case')).toBe('--accent')
    expect(toneToken('else')).toBe('--muted')
    expect(toneToken('whatever')).toBe('--muted')
  })
})

describe('colourToken', () => {
  it('uses the step colour when it is one of the six, else the category colour', () => {
    expect(colourToken('request', 'pink')).toBe('--c-pink')
    expect(colourToken('request', 'black')).toBe('--c-black')
    expect(colourToken('request', 'chartreuse')).toBe('--request')
    expect(colourToken('condition')).toBe('--container')
    expect(categoryToken('set')).toBe('--action')
  })
})

describe('portLine', () => {
  it('puts the first port on the header line and the next ones one gap lower', () => {
    expect(portLine(100, ['done', 'fail'], 'done')).toBe(100 + CARD.inPort)
    expect(portLine(100, ['done', 'fail'], 'fail')).toBe(100 + CARD.inPort + CARD.portGap)
    expect(portLine(100, ['done', 'fail'], undefined)).toBe(100 + CARD.inPort)
  })
})

describe('portToken', () => {
  it('uses the colour an outcome was given, else the colour of its tone', () => {
    expect(portToken({ tone: 'outcome', color: 'amber' })).toBe('--c-amber')
    expect(portToken({ tone: 'fail' })).toBe('--fail')
    expect(portToken({ tone: 'done', color: '' })).toBe('--ok')
  })
  it('ignores a colour that is not one of the step colours', () => {
    expect(portToken({ tone: 'outcome', color: 'mauve' })).toBe('--info')
  })
})

describe('outcomeColour', () => {
  it('is green for success, red for failure and aqua blue for a named outcome, unless one was given', () => {
    expect(outcomeColour('success')).toBe('green')
    expect(outcomeColour('failure')).toBe('red')
    expect(outcomeColour('not-found')).toBe('sky')
    expect(outcomeColour('not-found', 'amber')).toBe('amber')
  })
})
