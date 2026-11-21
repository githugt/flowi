import { describe, expect, it } from 'vitest'
import { triggerCurl } from './triggercurl'

describe('triggerCurl', () => {
  it('posts to the flow route with a bearer token', () => {
    expect(triggerCurl('127.0.0.1:8081', 'nightly', 'tok')).toBe(`curl -X POST -H 'Authorization: Bearer tok' 'http://127.0.0.1:8081/flows/nightly/run'`)
  })
  it('keeps a scheme the address already has, and drops a trailing slash', () => {
    expect(triggerCurl('https://ci.example.test/', 'a', 't')).toContain(`'https://ci.example.test/flows/a/run'`)
  })
  it('encodes a flow name that is not a plain word', () => {
    expect(triggerCurl('h:1', 'my flow', 't')).toContain('/flows/my%20flow/run')
  })
  it('writes a placeholder when no flow is named', () => {
    expect(triggerCurl('h:1', '', 't')).toContain('/flows/%3Cflow%3E/run')
  })
  it('quotes a single quote in the token so the shell reads it whole', () => {
    expect(triggerCurl('h:1', 'a', "it's")).toContain(`'Authorization: Bearer it'\\''s'`)
  })
})
