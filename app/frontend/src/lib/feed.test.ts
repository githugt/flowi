import { describe, expect, it } from 'vitest'
import { atEnd, droppedRows, isNewRun, messageMatches, toNdjson } from './feed'

describe('droppedRows', () => {
  it('drops nothing below the limit and the oldest above it', () => {
    expect(droppedRows(100, 5000)).toBe(0)
    expect(droppedRows(5000, 5000)).toBe(0)
    expect(droppedRows(5200, 5000)).toBe(200)
  })
})

describe('atEnd', () => {
  it('is true at the end or within the slack, false when scrolled up', () => {
    expect(atEnd(600, 400, 1000)).toBe(true)
    expect(atEnd(590, 400, 1000)).toBe(true)
    expect(atEnd(100, 400, 1000)).toBe(false)
  })
})

describe('toNdjson', () => {
  it('writes one object per line, in order', () => {
    const t = toNdjson([
      { direction: 'sent', at: '12:00:00', body: 'a' },
      { direction: 'received', at: '12:00:01', body: 'b\nc', key: 'k' },
    ])

    const lines = t.split('\n')

    expect(lines).toHaveLength(2)
    expect(JSON.parse(lines[1])).toMatchObject({ direction: 'received', body: 'b\nc', key: 'k' })
  })
})

describe('messageMatches', () => {
  // Before: a filter lowercased with toLowerCase, so "cafe" missed "Café", and it ran three times per message per key.
  it('looks in the body, key and type, ignoring case and accents', () => {
    const m = { body: 'Order Café', key: 'order-7', type: 'OrderCreated' }
    expect(messageMatches(m, '')).toBe(true)
    expect(messageMatches(m, 'cafe')).toBe(true)
    expect(messageMatches(m, 'ORDER-7')).toBe(true)
    expect(messageMatches(m, 'created')).toBe(true)
    expect(messageMatches(m, 'refund')).toBe(false)
    expect(messageMatches({ body: 'x' }, 'y')).toBe(false)
  })
})

describe('isNewRun', () => {
  // Before: rows were keyed by index, so after a re-run the open row or the pause pointed at another message.
  it('is true when the list has become shorter', () => {
    expect(isNewRun(12, 0)).toBe(true)
    expect(isNewRun(12, 3)).toBe(true)
    expect(isNewRun(12, 12)).toBe(false)
    expect(isNewRun(0, 5)).toBe(false)
  })
})
