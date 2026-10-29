import { describe, expect, it } from 'vitest'
import { sectionCounts } from './tabcounts'
import type { RequestView } from './types'

const kv = (key: string) => ({ Key: key, Value: 'x', Description: '' })

// A view with only what the counts read; the cast keeps the tests to what they are about.
function viewWith(parts: Record<string, unknown>): RequestView {
  return { bodyKind: 'none', hookChain: [], current: {}, ...parts } as unknown as RequestView
}

describe('sectionCounts', () => {
  it('counts the rows of each list section', () => {
    const view = viewWith({ current: { Query: [kv('a'), kv('b')], Headers: [kv('h')], Extract: [{}], Assert: [{}, {}, {}] }, hookChain: ['x'] })

    const counts = sectionCounts(view)

    expect(counts).toMatchObject({ Params: 2, Headers: 1, Extract: 1, Assert: 3, Hooks: 1, Variables: 0 })
  })

  it('counts nothing for sections the request leaves out', () => {
    const view = viewWith({ current: { Query: null, Headers: null } })

    expect(sectionCounts(view)).toMatchObject({ Params: 0, Headers: 0, Assert: 0 })
  })

  it('counts form fields only when the body is a form', () => {
    const form = viewWith({ bodyKind: 'form', current: { Body: { Form: [kv('a'), kv('b')] } } })
    const text = viewWith({ bodyKind: 'text', current: { Body: { Form: [kv('a')] } } })

    expect(sectionCounts(form).Body).toBe(2)
    expect(sectionCounts(text).Body).toBe(0)
  })

  it('counts the headers a message sends, not the connection ones', () => {
    const sends = viewWith({ async: { headersAreConnection: false, block: { Send: { Headers: [kv('a'), kv('b')] } } }, current: { Headers: [kv('c')] } })
    const connection = viewWith({ async: { headersAreConnection: true, block: {} }, current: { Headers: [kv('c')] } })

    expect(sectionCounts(sends).Headers).toBe(2)
    expect(sectionCounts(connection).Headers).toBe(1)
  })
})
