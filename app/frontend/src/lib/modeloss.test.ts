import { describe, expect, it } from 'vitest'
import { modeLoss } from './modeloss'
import type { AsyncBlock } from './types'

const block = (over: Partial<AsyncBlock> = {}): AsyncBlock => ({ Protocol: 'kafka', Server: '', Channel: '', Send: null, Receive: null, Binding: null, ...over })
const send = (text: string) => ({ Body: { JSON: '', Text: text, File: '', Form: null, Multipart: null }, Headers: null })

describe('modeLoss', () => {
  // Before: choosing Receive deleted the written message, and choosing Send deleted the count, time limit and filter, silently.
  it('names the message a switch to Receive would delete', () => {
    expect(modeLoss(block({ Send: send('{"id":1}') }), 'receive')).toBe('the message you wrote')
  })
  it('names the receive settings a switch to Send would delete', () => {
    expect(modeLoss(block({ Receive: { Count: 3, Timeout: 0, Match: '' } }), 'send')).toContain('count, time limit and filter')
  })
  it('says nothing when only defaults would go', () => {
    expect(modeLoss(block({ Send: send('') }), 'receive')).toBe('')
    expect(modeLoss(block({ Receive: { Count: 0, Timeout: 0, Match: '' } }), 'send')).toBe('')
    expect(modeLoss(block({ Receive: { Count: 1, Timeout: 0, Match: '' } }), 'send')).toBe('')
  })
  it('says nothing for the mode that keeps both sides', () => {
    expect(modeLoss(block({ Send: send('x'), Receive: { Count: 5, Timeout: 9, Match: 'a' } }), 'send_receive')).toBe('')
  })
})
