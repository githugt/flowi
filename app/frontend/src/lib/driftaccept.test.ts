import { describe, expect, it } from 'vitest'
import { acceptLabel, canAccept, pending, settledNote } from './driftaccept'
import type { DriftView } from './types'

const diff = { path: '$.region', kind: 'new' as const, fails: false }
const row = (o: Partial<DriftView['requests'][number]>): DriftView['requests'][number] => ({ request: 'pets/get', version: '1', status: 200, noSchema: false, accepted: false, differences: [], ...o })
const view = (requests: DriftView['requests']): DriftView => ({ runStatus: 'passed', failing: false, ci: '', requests })

describe('canAccept and pending', () => {
  it('should offer what moved or has no schema, and not what is clean or already accepted', () => {
    // Before: the drift check was a read-only report with nothing to accept.
    const d = view([row({ differences: [diff] }), row({ request: 'a', noSchema: true }), row({ request: 'b' }), row({ request: 'c', accepted: true })])

    expect(d.requests.map(canAccept)).toEqual([true, true, false, false])
    expect(pending(d)).toEqual(['pets/get', 'a'])
  })
})

describe('acceptLabel', () => {
  it('should say Learn schema for a response with none and Accept otherwise', () => {
    expect(acceptLabel(row({ noSchema: true }))).toBe('Learn schema')
    expect(acceptLabel(row({ differences: [diff] }))).toBe('Accept')
  })
})

describe('settledNote', () => {
  it('should count what was accepted, and say nothing before', () => {
    expect(settledNote(view([row({})]))).toBe('')
    expect(settledNote(view([row({ accepted: true })]))).toBe('1 request accepted: its stored schema now matches what the check saw.')
    expect(settledNote(view([row({ accepted: true }), row({ accepted: true })]))).toContain('2 requests accepted: their')
  })
})
