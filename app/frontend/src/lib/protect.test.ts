import { describe, expect, it } from 'vitest'
import { runNeedsConfirm, sendNeedsConfirm, sendWrites, writesQuestion, writesUnknownQuestion } from './protect'

describe('sendWrites', () => {
  it('treats GET, HEAD and OPTIONS as reads, in any case', () => {
    for (const m of ['GET', 'head', 'Options']) expect(sendWrites({ kind: 'http', method: m }), m).toBe(false)
    for (const m of ['POST', 'put', 'PATCH', 'DELETE']) expect(sendWrites({ kind: 'http', method: m }), m).toBe(true)
  })
  it('treats a GraphQL query as a read even though it is a POST, and a mutation as a write', () => {
    expect(sendWrites({ kind: 'http', method: 'POST', graphql: 'query' })).toBe(false)
    expect(sendWrites({ kind: 'http', method: 'POST', graphql: 'subscription' })).toBe(false)
    expect(sendWrites({ kind: 'http', method: 'POST', graphql: 'mutation' })).toBe(true)
    expect(sendWrites({ kind: 'http', method: 'POST', graphql: '' }), 'a document that cannot be told apart').toBe(true)
    expect(sendWrites({ kind: 'http', method: 'PUT', graphql: 'query' }), 'a method GraphQL does not use').toBe(true)
  })
  it('treats messages and gRPC calls as writes, and a query only when it may write', () => {
    expect(sendWrites({ kind: 'async' })).toBe(true)
    expect(sendWrites({ kind: 'grpc' })).toBe(true)
    expect(sendWrites({ kind: 'dataset', writes: false })).toBe(false)
    expect(sendWrites({ kind: 'dataset', writes: true })).toBe(true)
  })
})

describe('sendNeedsConfirm', () => {
  it('asks only for a write in a protected environment', () => {
    expect(sendNeedsConfirm('prod', ['prod'], { kind: 'http', method: 'POST' })).toBe(true)
    expect(sendNeedsConfirm('prod', ['prod'], { kind: 'http', method: 'GET' })).toBe(false)
    expect(sendNeedsConfirm('staging', ['prod'], { kind: 'http', method: 'POST' })).toBe(false)
    expect(sendNeedsConfirm('', ['prod'], { kind: 'http', method: 'POST' })).toBe(false)
  })
  it('asks before any flow run in a protected environment', () => {
    expect(runNeedsConfirm('prod', ['prod'])).toBe(true)
    expect(runNeedsConfirm('staging', ['prod'])).toBe(false)
  })
})

describe('the question before a flow runs in a protected environment', () => {
  it('should name the writing steps and fold a long list into "and N more"', () => {
    // Before: the question was one generic sentence.
    const lines = Array.from({ length: 9 }, (_, i) => `s${i}: POST (r${i})`)

    const text = writesQuestion('prod', lines)

    expect(text.split('\n')[0]).toBe('prod is a protected environment. This flow changes data in 9 steps:')
    expect(text).toContain('• s0: POST (r0)')
    expect(text).toContain('• s5: POST (r5)')
    expect(text).not.toContain('s6')
    expect(text.endsWith('and 3 more')).toBe(true)
  })

  it('should say "step" for one and keep the old warning when the list could not be read', () => {
    const one = writesQuestion('prod', ['a: PUT (x)'])

    const unknown = writesUnknownQuestion('prod')

    expect(one).toContain('in 1 step:')
    expect(one).not.toContain('more')
    expect(unknown).toContain('could not be checked')
  })
})
