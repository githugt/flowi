import { describe, expect, it } from 'vitest'
import { bodyPaths, inTemplate, suggestFor, type Item } from './exprcomplete'

const vocab: Item[] = [
  { insert: 'status', detail: '' },
  { insert: 'body.', detail: '' },
  { insert: 'body.items[0].id', detail: '' },
  { insert: 'len(', detail: '' },
  { insert: 'all_messages', detail: '' },
]
const at = (text: string, caret = text.length) => suggestFor(text, caret, vocab)

describe('suggestFor', () => {
  it('offers the entries that start with the word before the caret', () => {
    const s = at('sta')

    expect(s?.items.map((i) => i.insert)).toEqual(['status'])
    expect(s).toMatchObject({ from: 0, to: 3 })
  })

  it('replaces only the last word of a longer test', () => {
    const s = at('status == 200 and bo')

    expect(s?.from).toBe(18)
    expect(s?.items.map((i) => i.insert)).toContain('body.')
  })

  it('completes a dotted path with array positions', () => {
    const s = at('body.items[0].')

    expect(s?.items.map((i) => i.insert)).toEqual(['body.items[0].id'])
  })

  it('ignores case', () => {
    expect(at('ALL')?.items[0].insert).toBe('all_messages')
  })

  it('offers nothing inside a quoted text', () => {
    expect(at('body.name == "sta')).toBeNull()
  })

  it('offers nothing inside an open template, which has its own completion', () => {
    expect(at('Bearer {{sta')).toBeNull()
  })

  it('offers nothing after a space or for a finished word', () => {
    expect(at('status == ')).toBeNull()
    expect(at('status')).toBeNull()
  })

  it('works from the caret, not the end of the text', () => {
    const s = at('sta == 200', 3)

    expect(s?.items[0].insert).toBe('status')
  })
})

describe('inTemplate', () => {
  it('is true only between {{ and }}', () => {
    expect(inTemplate('a {{b', 5)).toBe(true)
    expect(inTemplate('a {{b}} c', 9)).toBe(false)
  })
})

describe('bodyPaths', () => {
  it('lists the fields of a body, a list by its first item', () => {
    const paths = bodyPaths({ id: 1, user: { name: 'a' }, items: [{ sku: 'x' }], 'x-id': 2 }, 'body', 'last response')

    expect(paths.map((p) => p.insert)).toEqual(['body', 'body.id', 'body.user', 'body.user.name', 'body.items', 'body.items[0]', 'body.items[0].sku'])
  })

  it('stops at the depth limit', () => {
    const paths = bodyPaths({ a: { b: { c: 1 } } }, 'body', '', 2)

    expect(paths.map((p) => p.insert)).toEqual(['body', 'body.a', 'body.a.b'])
  })
})
