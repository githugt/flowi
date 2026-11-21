import { describe, expect, it } from 'vitest'
import { filterTraffic, statusTone, type TrafficRow } from './traffic'

const row = (seq: number, method: string, path: string, status: number, match = 'exact', request = ''): TrafficRow =>
  ({ seq, at: '10:00:00', method, path, status, match, request, duration: '1ms', remote: '', version: '', example: '', source: '', delay: '' })

const rows = [row(1, 'GET', '/users/1', 200, 'exact', 'Get user'), row(2, 'POST', '/orders', 503, 'fault'), row(3, 'GET', '/missing', 404, 'none')]

describe('statusTone', () => {
  it.each([[200, 'success'], [302, 'redirect'], [404, 'client'], [503, 'server']])('%i is %s', (status, tone) => {
    expect(statusTone(status)).toBe(tone)
  })
})

describe('filterTraffic', () => {
  it('keeps every call for a blank query', () => {
    expect(filterTraffic(rows, '', false)).toBe(rows)
  })

  it('matches words against the method, path, status and the request it matched', () => {
    expect(filterTraffic(rows, 'post orders', false).map((r) => r.seq)).toEqual([2])
    expect(filterTraffic(rows, '404', false).map((r) => r.seq)).toEqual([3])
    expect(filterTraffic(rows, 'get user', false).map((r) => r.seq)).toEqual([1])
  })

  // Before: only the method, path, status and request were searched, so "which call got the pinned example?" had no answer.
  it('also matches the example the call was answered with', () => {
    const withExample = [{ ...rows[0], example: 'two pets' }, rows[1]]

    expect(filterTraffic(withExample, 'two pets', false).map((r) => r.seq)).toEqual([1])
  })

  it('can keep only the failed calls (4xx and 5xx)', () => {
    expect(filterTraffic(rows, '', true).map((r) => r.seq)).toEqual([2, 3])
  })
})

describe('filterTraffic by mode', () => {
  const mk = (seq: number, status: number, match = 'example'): TrafficRow => ({ seq, at: '', method: 'GET', path: '/x', status, match, request: '', duration: '', remote: '', version: '', example: '', source: '', delay: '' })
  const calls = [mk(1, 200), mk(2, 404), mk(3, 503), mk(4, 500, 'fault')]

  it('keeps the 4xx, the 5xx or the injected faults', () => {
    // Before: only All and Failed, so the calls the replayer broke on purpose were mixed with real ones.
    expect(filterTraffic(calls, '', '4xx').map((r) => r.seq)).toEqual([2])
    expect(filterTraffic(calls, '', '5xx').map((r) => r.seq)).toEqual([3, 4])
    expect(filterTraffic(calls, '', 'faults').map((r) => r.seq)).toEqual([4])
    expect(filterTraffic(calls, '', 'failed').map((r) => r.seq)).toEqual([2, 3, 4])
    expect(filterTraffic(calls, '', 'all')).toBe(calls)
  })

  it('narrows by words inside a mode', () => {
    expect(filterTraffic(calls, '503', '5xx').map((r) => r.seq)).toEqual([3])
  })
})
