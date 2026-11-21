import { describe, expect, it } from 'vitest'
import { headerValues, implicitHeaders } from './headernames'

const noBody = { JSON: '', Text: '', File: '', Form: null, Multipart: null }

describe('headerValues', () => {
  it('suggests the content types for Content-Type in any case', () => {
    // Before: a header value was free text with no suggestions.
    const values = headerValues(' content-TYPE ')
    expect(values).toContain('application/json')
  })
  it('suggests nothing for a header with free-text values', () => {
    expect(headerValues('X-Request-Id')).toEqual([])
  })
})

describe('implicitHeaders', () => {
  it('lists the Content-Type of a JSON body', () => {
    // Before: the Content-Type Flowi added was invisible.
    const out = implicitHeaders({ body: { ...noBody, JSON: '{}' }, headers: [], hasAuth: false })
    expect(out).toEqual([{ name: 'Content-Type', value: 'application/json', source: 'from Body' }])
  })
  it('uses the text type with a charset for a text body and the form type for form rows', () => {
    expect(implicitHeaders({ body: { ...noBody, Text: 'hi' }, headers: [], hasAuth: false })[0].value).toBe('text/plain; charset=utf-8')
    expect(implicitHeaders({ body: { ...noBody, Form: [{ Key: 'a', Value: 'b' }] }, headers: [], hasAuth: false })[0].value).toBe('application/x-www-form-urlencoded')
  })
  it('lists nothing for Content-Type when the request sets its own', () => {
    const out = implicitHeaders({ body: { ...noBody, JSON: '{}' }, headers: ['content-type'], hasAuth: false })
    expect(out).toEqual([])
  })
  it('lists the Authorization header when an auth scheme is chosen, unless the request sets it', () => {
    expect(implicitHeaders({ body: noBody, headers: [], hasAuth: true }).map((h) => h.name)).toEqual(['Authorization'])
    expect(implicitHeaders({ body: noBody, headers: ['Authorization'], hasAuth: true })).toEqual([])
  })
  it('lists nothing for an empty request', () => {
    expect(implicitHeaders({ body: noBody, headers: [], hasAuth: false })).toEqual([])
  })
})
