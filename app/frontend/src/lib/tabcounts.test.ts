import { describe, expect, it } from 'vitest'
import { sectionConfigured, sectionCounts } from './tabcounts'
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

describe('sectionConfigured', () => {
  // Before: the Receive, Binding, Proto and Settings tabs never showed the green dot.
  it('shows the dot for what an async request waits for and its bindings', () => {
    const waiting = viewWith({ async: { block: { Receive: { Count: 3, Timeout: 0, Match: '' }, Binding: [kv('group')] } } })
    const empty = viewWith({ async: { block: { Receive: { Count: 0, Timeout: 0, Match: '' }, Binding: null } } })

    expect(sectionConfigured(waiting, 'Receive')).toBe(true)
    expect(sectionConfigured(waiting, 'Binding')).toBe(true)
    expect(sectionConfigured(empty, 'Receive')).toBe(false)
    expect(sectionConfigured(empty, 'Binding')).toBe(false)
  })
  it('shows the dot for gRPC proto files and for what a gRPC request waits for', () => {
    const grpc = viewWith({ grpc: { block: { Proto: ['a.proto'], Import: null, Receive: { Count: 0, Timeout: 5, Match: '' } } } })

    expect(sectionConfigured(grpc, 'Proto')).toBe(true)
    expect(sectionConfigured(grpc, 'Receive')).toBe(true)
    expect(sectionConfigured(viewWith({ grpc: { block: { Proto: null, Import: null, Receive: null } } }), 'Proto')).toBe(false)
  })
  it('shows the dot for send settings', () => {
    expect(sectionConfigured(viewWith({ current: { Settings: { timeout: '5s' } } }), 'Settings')).toBe(true)
    expect(sectionConfigured(viewWith({ current: { Settings: {} } }), 'Settings')).toBe(false)
    expect(sectionConfigured(viewWith({ current: {} }), 'Settings')).toBe(false)
  })
})

describe('rows switched off', () => {
  it('should not count a switched-off parameter or header, and show no dot when all are off', () => {
    // Before: every row counted, so the badge promised entries the request did not send.
    const some = viewWith({ current: { Query: [kv('a'), { ...kv('b'), Disabled: true }], Headers: [kv('h'), { ...kv('i'), Disabled: true }] } })
    const none = viewWith({ current: { Query: [{ ...kv('a'), Disabled: true }], Headers: [{ ...kv('h'), Disabled: true }] } })

    expect(sectionCounts(some)).toMatchObject({ Params: 1, Headers: 1 })
    expect(sectionCounts(none)).toMatchObject({ Params: 0, Headers: 0 })
    expect(sectionConfigured(none, 'Params')).toBe(false)
    expect(sectionConfigured(none, 'Headers')).toBe(false)
    expect(sectionConfigured(some, 'Params')).toBe(true)
  })
})
