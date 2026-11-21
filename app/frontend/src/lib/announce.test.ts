import { describe, expect, it } from 'vitest'
import { sendAnnouncement } from './announce'

describe('sendAnnouncement', () => {
  it('says Sending while a send runs', () => expect(sendAnnouncement(true, undefined, '')).toBe('Sending'))
  it('gives the status and the time of a finished send', () => {
    expect(sendAnnouncement(false, { title: '200 OK', took: '142 ms', passed: true }, '')).toBe('200 OK, 142 ms')
  })
  it('says when checks failed', () => {
    expect(sendAnnouncement(false, { title: '500 Internal Server Error', took: '', passed: false }, '')).toBe('500 Internal Server Error. Checks failed')
  })
  it('says a send failed with no response', () => {
    expect(sendAnnouncement(false, undefined, 'connection refused')).toBe('Send failed. connection refused')
  })
  it('says nothing before any send', () => expect(sendAnnouncement(false, undefined, '')).toBe(''))
})
