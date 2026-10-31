import { describe, expect, it } from 'vitest'
import { arrowEdges, arrowKey, arrowMarker, arrowOutcomes, outcomeOfPort, outcomesOfCard, toggled } from './arrows'
import type { Edge, Face } from './types'

const ok: Edge = { from: 'a', to: 'b' }
const fail: Edge = { from: 'a', to: 'b', fail: true }
const named = (outcome: string): Edge => ({ from: 'a', to: 'b', outcome })

describe('arrowOutcomes', () => {
  it('reads a plain arrow as success, a failure arrow as failure and a named one by its name', () => {
    expect(arrowOutcomes([ok], 'a', 'b')).toEqual(['success'])
    expect(arrowOutcomes([fail], 'a', 'b')).toEqual(['failure'])
    expect(arrowOutcomes([named('not-found')], 'a', 'b')).toEqual(['not-found'])
  })
  it('lists every outcome of an arrow taken more than one way', () => {
    expect(arrowOutcomes([ok, fail, named('auth')], 'a', 'b')).toEqual(['success', 'failure', 'auth'])
  })
  it('does not count a goto route or an edge between other cards', () => {
    expect(arrowOutcomes([{ ...fail, goto: true }, { from: 'x', to: 'b' }], 'a', 'b')).toEqual([])
  })
})

describe('toggled', () => {
  it('adds an outcome that is off and takes away one that is on, keeping the others', () => {
    expect(toggled(['success'], 'failure')).toEqual(['success', 'failure'])
    expect(toggled(['success', 'failure'], 'success')).toEqual(['failure'])
    expect(toggled(['failure'], 'failure')).toEqual([])
  })
})

describe('arrowKey and arrowEdges', () => {
  it('give every edge of one arrow the same key, and a goto route its own', () => {
    expect(arrowKey(ok)).toBe(arrowKey(fail))
    expect(arrowKey(named('x'))).toBe(arrowKey(ok))
    expect(arrowKey({ ...fail, goto: true })).not.toBe(arrowKey(fail))
    expect(arrowEdges([ok, fail, named('x'), { from: 'b', to: 'c' }], arrowKey(ok))).toHaveLength(3)
  })
})

describe('outcomes of a card', () => {
  const face = { ports: [{ id: 'done', label: 'Done', tone: 'done' }, { id: 'fail', label: 'Fail', tone: 'fail' }, { id: 'outcome-not-found', label: 'not-found', tone: 'outcome' }] } as unknown as Face
  it('names the outcome of each port, in order', () => {
    expect(outcomesOfCard(face)).toEqual(['success', 'failure', 'not-found'])
  })
  it('offers success and failure for a card that has no ports of its own', () => {
    expect(outcomesOfCard(undefined)).toEqual(['success', 'failure'])
  })
  it('maps a port id to its outcome and the other way round is the card port', () => {
    expect(outcomeOfPort('done')).toBe('success')
    expect(outcomeOfPort('fail')).toBe('failure')
    expect(outcomeOfPort('outcome-auth')).toBe('auth')
    expect(outcomeOfPort('case-0')).toBe('')
  })
})

describe('arrowMarker', () => {
  it('is the ring of the outcome colour when it has one, else of the failure red, the named blue or the plain grey', () => {
    expect(arrowMarker({}, 'amber')).toBe('arrow-c-amber')
    expect(arrowMarker({ fail: true })).toBe('arrow-fail')
    expect(arrowMarker({ outcome: 'ok' })).toBe('arrow-c-sky')
    expect(arrowMarker({})).toBe('arrow')
  })
  it('ignores a colour that is not one of the step colours', () => {
    expect(arrowMarker({ fail: true }, 'mauve')).toBe('arrow-fail')
  })
})
