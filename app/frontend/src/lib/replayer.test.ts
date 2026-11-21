import { describe, expect, it } from 'vitest'
import { forwardLabel, forwardPreview, splitList, within } from './replayer'

const base = { forwardTo: '', forwardOnly: '', recordTo: '', recordHar: '' }
const targets = { env: 'dev', targets: [{ base: 'https://a.test', requests: 2 }, { base: 'https://b.test/v1', requests: 1 }, { base: '{{host}}', problem: 'no value', requests: 1 }] }
const texts = (l: { text: string }[]) => l.map((x) => x.text).join('\n')

describe('splitList', () => {
  it('drops blanks and trims', () => expect(splitList(' a , ,b,')).toEqual(['a', 'b']))
})

describe('within', () => {
  it('matches the same API and a path prefix, not a sibling host', () => {
    expect(within('https://a.test/', 'https://a.test')).toBe(true)
    expect(within('https://a.test/v1', 'https://a.test')).toBe(true)
    expect(within('https://a.test.evil', 'https://a.test')).toBe(false)
  })
})

describe('forwardPreview', () => {
  it('Given replay only, Then unmatched calls are not found', () => {
    expect(texts(forwardPreview({ ...base, forward: '' }, null))).toContain('404')
  })
  it('Given a HAR file, Then it is mentioned in every mode', () => {
    for (const forward of ['', 'pass', 'save'] as const) expect(texts(forwardPreview({ ...base, forward, recordHar: '/t/s.har' }, targets))).toContain('/t/s.har')
  })
  it('Given one API, Then everything goes there', () => {
    const t = texts(forwardPreview({ ...base, forward: 'save', forwardTo: 'https://x.test' }, targets))
    expect(t).toContain('Every call with no saved answer is sent to https://x.test')
    expect(t).toContain('in Drafts')
  })
  it('Given an allow-list, Then APIs outside it are marked', () => {
    const t = texts(forwardPreview({ ...base, forward: 'pass', forwardOnly: 'https://a.test' }, targets))
    expect(t).toContain('https://a.test (2 requests): forwarded.')
    expect(t).toContain('https://b.test/v1 (1 request): not in the list')
    expect(t).toContain('{{host}} (1 request): no value')
  })
  it('Given a listed API no request calls, Then it says so', () => {
    expect(texts(forwardPreview({ ...base, forward: 'pass', forwardOnly: 'https://zzz.test' }, targets))).toContain('no saved request calls this API')
  })
})

describe('forwardLabel', () => {
  it('names the mode and where', () => {
    expect(forwardLabel({ forward: '', forwardTo: '', forwardOnly: '' })).toBe('listening')
    expect(forwardLabel({ forward: 'pass', forwardTo: 'https://x.test', forwardOnly: '' })).toContain('https://x.test')
    expect(forwardLabel({ forward: 'save', forwardTo: '', forwardOnly: '' })).toContain('their own APIs')
  })
})

describe('forwardLabel faults', () => {
  it('names the faults and in-turn answering that are on', () => {
    const base = { forward: '' as const, forwardTo: '', forwardOnly: '' }

    expect(forwardLabel({ ...base, faultErrorPercent: '20', sequence: true })).toBe('listening · errors 20% · answers in turn')
    expect(forwardLabel({ ...base, faultDropPercent: '5' })).toBe('listening · drops 5%')
    expect(forwardLabel({ ...base, faultErrorPercent: '0' })).toBe('listening')
  })
  it('leaves the faults out while forwarding, where they do not apply', () => {
    expect(forwardLabel({ forward: 'pass', forwardTo: 'https://x.test', forwardOnly: '', faultErrorPercent: '50' })).toBe('passing calls with no saved answer to https://x.test')
  })
})
