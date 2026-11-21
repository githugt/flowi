import { describe, expect, it } from 'vitest'
import { REOPEN_LIMIT, rememberClosed, takeReopen, type ClosedTab } from './closedtabs'

const doc = (node: string): ClosedTab => ({ node, kind: 'request' })

describe('rememberClosed', () => {
  it('puts the newest first and keeps one entry per document', () => {
    // Before: a closed tab could only be found again in the tree.
    const list = rememberClosed(rememberClosed(rememberClosed([], doc('a')), doc('b')), doc('a'))
    expect(list.map((t) => t.node)).toEqual(['a', 'b'])
  })
  it('ignores screens and hooks editors', () => {
    expect(rememberClosed([], { node: 'screen:home', kind: 'screen' })).toEqual([])
    expect(rememberClosed([], { node: 'hooks:x', kind: 'hooks' })).toEqual([])
  })
  it('keeps only the last few', () => {
    let list: ClosedTab[] = []
    for (let i = 0; i < REOPEN_LIMIT + 5; i++) list = rememberClosed(list, doc('n' + i))
    expect(list).toHaveLength(REOPEN_LIMIT)
    expect(list[0].node).toBe('n' + (REOPEN_LIMIT + 4))
  })
})

describe('takeReopen', () => {
  it('takes the newest usable tab and leaves the older ones', () => {
    const got = takeReopen([doc('a'), doc('b'), doc('c')], () => true)
    expect(got?.tab.node).toBe('a')
    expect(got?.rest.map((t) => t.node)).toEqual(['b', 'c'])
  })
  it('skips a tab that is open again or gone', () => {
    const got = takeReopen([doc('a'), doc('b'), doc('c')], (n) => n === 'c')
    expect(got?.tab.node).toBe('c')
    expect(got?.rest).toEqual([])
  })
  it('is null when nothing can come back', () => {
    expect(takeReopen([doc('a')], () => false)).toBeNull()
    expect(takeReopen([], () => true)).toBeNull()
  })
})
