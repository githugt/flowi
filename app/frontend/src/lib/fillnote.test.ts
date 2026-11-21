import { describe, expect, it } from 'vitest'
import { fillNote } from './fillnote'
import type { RequestVersion } from './types'

const v = (over: Partial<RequestVersion>): RequestVersion =>
  ({ Label: '', Method: 'POST', URL: '/pets', Query: null, Headers: [{ key: 'a', value: '1' }, { key: 'b', value: '2' }], Body: { JSON: '{}', Text: '', File: '', Form: null, Multipart: null }, Extract: null, Assert: ['status == 200'], Variables: null, Docs: '', ...over }) as RequestVersion

describe('fillNote', () => {
  it('lists what is replaced and what stays', () => {
    const t = fillNote(v({}), false)
    expect(t).toContain('Replaces: method POST, URL, 2 headers, body.')
    expect(t).toContain('Keeps 1 check, outcomes, variables and hooks.')
  })

  it('leaves out what the request does not have', () => {
    expect(fillNote(v({ URL: '', Headers: null, Body: { JSON: '', Text: '', File: '', Form: null, Multipart: null } }), false)).toContain('Replaces: method POST.')
  })

  it('says a whole request file keeps the name and other versions', () => {
    expect(fillNote(v({}), true)).toContain('Keeps the name and the other versions.')
  })

  it('says nothing when no request is open', () => {
    expect(fillNote(undefined, false)).toBe('')
  })
})
