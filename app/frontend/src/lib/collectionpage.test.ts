import { describe, expect, it } from 'vitest'
import { type Drift, type Endpoint, type Impact, collectionOfRow, driftText, impactText, endpointMatches, groupCount, groupEndpoints, headline, levelChoices, levelShown, pageCollection, pageNode, runTone, unlistedProblems } from './collectionpage'

const state = (o: Partial<Parameters<typeof headline>[0]> = {}) => ({ operationsUsed: 0, operationsTotal: 0, failing: 0, neverRan: 0, broken: [], specTotal: 0, specErrors: 0, impactBroken: 0, driftMoved: 0, ...o })

describe('page node', () => {
  it('names one tab for each collection and reads the collection back', () => {
    expect(pageNode('C1')).toBe('screen:collection:C1')
    expect(pageCollection(pageNode('C1'))).toBe('C1')
    expect(pageCollection('screen:checks')).toBe('')
  })
  it('takes the collection of a sidebar row, with or without the c: prefix', () => {
    expect(collectionOfRow({ id: 'c:C1', kind: 'collection' })).toBe('C1')
    expect(collectionOfRow({ id: 'C1', kind: 'collection' })).toBe('C1')
    expect(collectionOfRow({ id: 'r:C1:r1', kind: 'request' })).toBe('')
    expect(collectionOfRow(undefined)).toBe('')
  })
})

describe('headline', () => {
  it('is calm when nothing needs a look', () => {
    expect(headline(state({ operationsUsed: 3, operationsTotal: 3 }))).toEqual({ text: 'Nothing needs a look.', tone: 'ok' })
  })
  it('puts a failing flow first and calls it bad', () => {
    const h = headline(state({ failing: 2, neverRan: 1, operationsTotal: 4, operationsUsed: 3 }))

    expect(h.text).toBe('2 flows failing · 1 flow never ran · 1 operation no flow calls')
    expect(h.tone).toBe('bad')
  })
  it('only warns for what is not covered or never ran', () => {
    expect(headline(state({ neverRan: 1 })).tone).toBe('warn')
  })
  it('calls an unreadable file bad', () => {
    expect(headline(state({ broken: ['pets/odd'] }))).toEqual({ text: '1 file that cannot be read', tone: 'bad' })
  })
})

describe('headline and the API description', () => {
  it('counts rule errors as bad and the other findings as notes', () => {
    expect(headline(state({ specTotal: 5, specErrors: 2 }))).toEqual({ text: '2 rule errors in the API description · 3 notes on the API description', tone: 'bad' })
  })
  it('only warns for notes', () => {
    expect(headline(state({ specTotal: 1 }))).toEqual({ text: '1 note on the API description', tone: 'warn' })
  })
})

describe('runTone', () => {
  it('maps a run status to a pill tone', () => {
    expect(runTone('passed')).toBe('ok')
    expect(runTone('failed')).toBe('bad')
    expect(runTone('skipped')).toBe('warn')
    expect(runTone('')).toBe('')
  })
})

describe('spec rule levels', () => {
  const rule = { name: 'path-case', about: '', default: 'off', level: 'error', chosen: true }
  it('offers the default first, named with its level', () => {
    expect(levelChoices(rule).map((c) => c.label)).toEqual(['Default (off)', 'Off', 'Note', 'Warning', 'Error'])
    expect(levelChoices(rule)[0].value).toBe('')
  })
  it('shows the choice, or the default when nothing was chosen', () => {
    expect(levelShown(rule)).toBe('error')
    expect(levelShown({ ...rule, chosen: false, level: 'off' })).toBe('')
  })
})

const ep = (o: Partial<Endpoint>): Endpoint => ({ node: 'n', path: 'p', name: '', summary: '', verb: 'GET', where: '/x', folder: '', tags: [], operation: '', versions: [], used: [], statuses: [], unread: 0, impact: { consumers: 0, broken: 0, unknown: 0, contract: '', notes: [] }, drift: { checked: false, at: '', via: [], new: 0, missing: 0, changed: 0, noSchema: false, notes: [] }, ...o })

describe('groupEndpoints', () => {
  const list = [
    ep({ path: 'a', where: '/otp', verb: 'GET', tags: ['test'], folder: 'otp' }),
    ep({ path: 'b', where: '/otp', verb: 'POST', tags: ['generate', 'mobile'] }),
    ep({ path: 'c', where: '/mobile', verb: 'POST', status: { name: 'review', color: '', icon: '' } }),
  ]
  it('groups by URL path, so the methods of one path sit together', () => {
    const g = groupEndpoints(list, 'path')

    expect(g.map((x) => [x.key, x.endpoints.length])).toEqual([['/mobile', 1], ['/otp', 2]])
  })
  it('puts an endpoint with several tags in each, and the untagged last', () => {
    const g = groupEndpoints(list, 'tags')

    expect(g.map((x) => x.key)).toEqual(['generate', 'mobile', 'test', 'No tag'])
    expect(g[0].endpoints[0].path).toBe('b')
  })
  it('groups by folder, the top level last', () => {
    expect(groupEndpoints(list, 'folder').map((x) => x.key)).toEqual(['otp', 'Top level'])
  })
  it('groups by status, those without one last', () => {
    expect(groupEndpoints(list, 'status').map((x) => x.key)).toEqual(['review', 'No status'])
  })
  it('counts methods for a path and endpoints for the rest', () => {
    expect(groupCount(1, 'path')).toBe('1 method')
    expect(groupCount(2, 'path')).toBe('2 methods')
    expect(groupCount(1, 'tags')).toBe('1 endpoint')
  })
})

describe('endpointMatches', () => {
  it('needs every word somewhere in the verb, path, name, summary, tags or operation', () => {
    const e = ep({ verb: 'POST', where: '/otp/{id}/call', summary: 'Send an OTP via call', tags: ['generate'] })

    expect(endpointMatches(e, 'post call')).toBe(true)
    expect(endpointMatches(e, 'generate otp')).toBe(true)
    expect(endpointMatches(e, 'get call')).toBe(false)
    expect(endpointMatches(e, '')).toBe(true)
  })
})

// Before: the page said which flows call an operation, not whether what they read from it still fits or whether it moved.
describe('headline and what reads an operation', () => {
  it('puts a reader that no longer fits among the bad things, and moved responses among the warnings', () => {
    expect(headline(state({ impactBroken: 2 }))).toEqual({ text: '2 operations that something reads wrongly', tone: 'bad' })
    expect(headline(state({ driftMoved: 1 }))).toEqual({ text: '1 operation whose responses moved', tone: 'warn' })
  })
})

const impact = (o: Partial<Impact> = {}): Impact => ({ consumers: 0, broken: 0, unknown: 0, contract: '', notes: [], ...o })
const drift = (o: Partial<Drift> = {}): Drift => ({ checked: true, at: '', via: [], new: 0, missing: 0, changed: 0, noSchema: false, notes: [], ...o })

describe('impactText', () => {
  it('says nothing reads an operation, then how many readers fit, then which do not', () => {
    expect(impactText(impact())).toEqual({ text: 'nothing reads its responses', tone: '' })
    expect(impactText(impact({ consumers: 3, contract: 'example' }))).toEqual({ text: '3 readers, all fit its pinned example', tone: 'ok' })
    expect(impactText(impact({ consumers: 1, contract: 'response' }))).toEqual({ text: '1 reader, all fit its last saved response', tone: 'ok' })
    expect(impactText(impact({ consumers: 3, broken: 1 }))).toEqual({ text: '3 readers, 1 no longer fits its contract', tone: 'bad' })
    expect(impactText(impact({ consumers: 2, unknown: 2 })).tone).toBe('warn')
  })
})

describe('driftText', () => {
  it('is quiet until a check ran, then says no change, nothing to compare with, or what moved', () => {
    expect(driftText(drift({ checked: false }))).toEqual({ text: 'not checked', tone: '' })
    expect(driftText(drift())).toEqual({ text: 'no change', tone: 'ok' })
    expect(driftText(drift({ noSchema: true })).tone).toBe('warn')
    expect(driftText(drift({ new: 2 }))).toEqual({ text: '2 moved: 2 new', tone: 'warn' })
    expect(driftText(drift({ new: 1, missing: 1, changed: 2 }))).toEqual({ text: '4 moved: 1 new, 1 missing, 2 changed', tone: 'bad' })
  })
})

describe('unlistedProblems', () => {
  it('counts the problems the cap cut off, not the notes that are listed', () => {
    // Before: three listed notes made the page claim fewer problems were in the Problems panel than there were.
    const listed = [{ severity: 'warning' }, { severity: 'note' }, { severity: 'note' }, { severity: 'note' }]

    expect(unlistedProblems(listed, 5)).toBe(4)
    expect(unlistedProblems([], 0)).toBe(0)
  })
})
