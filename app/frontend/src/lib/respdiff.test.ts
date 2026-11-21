import { describe, expect, it } from 'vitest'
import { diffNotes, diffSummary, kindMark, type ResponseDiff } from './respdiff'
import pane from '../ResponsePane.svelte?raw'

const base: ResponseDiff = { older: '14:00:00 · 200', newer: '14:01:00 · 404', same: false, oldStatus: 200, newStatus: 404, headers: [], body: [], bodyMode: 'json', truncated: 0, hidden: 0, volatile: 0 }
const change = { path: '$.a', kind: 'changed', old: '1', new: '2' }

describe('diffSummary', () => {
  it('says what differs, in the order a reader looks: status, headers, body', () => {
    expect(diffSummary({ ...base, headers: [change, change], body: [change], truncated: 2 })).toBe('Status 200 → 404 · 2 headers · 3 body changes')
  })

  it('leaves out what is the same, and says when everything is', () => {
    expect(diffSummary({ ...base, oldStatus: 200, newStatus: 200, body: [change] })).toBe('1 body change')
    expect(diffSummary({ ...base, same: true })).toContain('Nothing differs')
  })

  it('counts lines for a text body', () => {
    expect(diffSummary({ ...base, oldStatus: 200, newStatus: 200, bodyMode: 'text', body: [change, change] })).toBe('2 line changes')
  })
})

describe('diffNotes', () => {
  it('explains what was left out', () => {
    expect(diffNotes({ ...base, truncated: 1, hidden: 2, volatile: 1, bodyMode: 'binary' })).toEqual([
      '1 more change not listed.',
      '2 values named like a credential were not compared.',
      '1 header that changes on every send (Date, a request id) was skipped.',
      'The body is binary: compared by size and digest only.',
    ])
  })
})

describe('kindMark', () => {
  it('marks added, removed and changed', () => expect(['added', 'removed', 'changed'].map(kindMark)).toEqual(['+', '−', '~']))
})

describe('the response pane', () => {
  it('has the button and the panel, and drops the comparison when the response changes', () => {
    expect(pane).toContain('Compare with previous')
    expect(pane).toContain('RequestService.ComparePrevious(view.node)')
    expect(pane).toContain('aria-label="Comparison with the previous response"')
    expect(pane).toMatch(/\$effect\(\(\) => \{\s*void \[view\.node, r\?\.at\]\s*compared = null/)
  })
})
