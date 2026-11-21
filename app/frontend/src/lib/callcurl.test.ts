import { describe, expect, it } from 'vitest'
import { callCurl } from './callcurl'
import log from '../TrafficLog.svelte?raw'

describe('callCurl', () => {
  const sent = { headers: [{ name: 'Content-Type', value: 'application/json' }, { name: 'Host', value: 'x' }, { name: 'Content-Length', value: '7' }], body: '{"a":1}', truncated: false }

  it('repeats the call against the replayer, without the headers the connection adds', () => {
    // Before: a call could be read in the log but not repeated.
    expect(callCurl('127.0.0.1:8080', { method: 'POST', path: '/orders?x=1' }, sent)).toBe(
      `curl -X POST 'http://127.0.0.1:8080/orders?x=1' -H 'Content-Type: application/json' --data-raw '{"a":1}'`,
    )
  })

  it('quotes a single quote in a value', () => {
    expect(callCurl('h:1', { method: 'POST', path: '/' }, { headers: [], body: "it's", truncated: false })).toContain(`--data-raw 'it'\\''s'`)
  })

  it('leaves out a body that was cut short, and works without what was sent', () => {
    expect(callCurl('h:1', { method: 'POST', path: '/' }, { ...sent, truncated: true })).not.toContain('--data-raw')
    expect(callCurl('http://h:1/', { method: 'GET', path: '/a' })).toBe(`curl -X GET 'http://h:1/a'`)
    expect(callCurl('', { method: 'GET', path: '/a' })).toBe(`curl -X GET 'http://127.0.0.1:8080/a'`)
  })

  it('is offered on an open call in the traffic log', () => {
    expect(log).toContain('Copy as curl')
    expect(log).toContain('callCurl(addr')
  })
})
