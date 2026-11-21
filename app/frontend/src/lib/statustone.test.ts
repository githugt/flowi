import { describe, expect, it } from 'vitest'
import { statusPill, statusTone } from './statustone'

describe('statusTone', () => {
  it('reads 2xx as ok, 3xx and 1xx as warn, 4xx and 5xx as bad', () => {
    expect(statusTone(200)).toBe('ok')
    expect(statusTone(204)).toBe('ok')
    expect(statusTone(301)).toBe('warn')
    expect(statusTone(101)).toBe('warn')
    expect(statusTone(404)).toBe('bad')
    expect(statusTone(503)).toBe('bad')
  })
  it('reads the number at the start of a pill text', () => {
    expect(statusTone('301 Moved Permanently')).toBe('warn')
    expect(statusTone('200 OK')).toBe('ok')
  })
  it('falls back to passed when there is no number', () => {
    expect(statusTone('', true)).toBe('ok')
    expect(statusTone('error', false)).toBe('bad')
    expect(statusTone(0)).toBe('')
    expect(statusTone(undefined)).toBe('')
  })
})

describe('statusPill', () => {
  // Before: a 200 whose assertions failed showed a green "200" unless the Assert tab was open.
  it('says a check failed when the status is fine but the send did not pass', () => {
    expect(statusPill(200, false)).toEqual({ text: '200 · check failed', tone: 'bad' })
    expect(statusPill('301 Moved', false).tone).toBe('bad')
  })
  it('leaves a passing or an already failing status alone', () => {
    expect(statusPill(200, true)).toEqual({ text: '200', tone: 'ok' })
    expect(statusPill(500, false)).toEqual({ text: '500', tone: 'bad' })
  })
})
