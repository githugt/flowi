import { describe, expect, it } from 'vitest'
import { filterTree, firstLeaf, highlightParts } from './treefilter'
import type { TreeNode } from './types'

const leaf = (id: string, label: string, method = ''): TreeNode => ({ id, label, kind: 'request', method })
const group = (id: string, label: string, children: TreeNode[]): TreeNode => ({ id, label, kind: 'folder', children })

const roots: TreeNode[] = [
  group('c1', 'Shop', [group('c1/orders', 'Orders', [leaf('o1', 'List orders', 'GET'), leaf('o2', 'Create order', 'POST')]), leaf('p1', 'Ping', 'GET')]),
  group('c2', 'Billing', [leaf('b1', 'Invoice', 'GET')]),
]

const ids = (nodes: TreeNode[]): string[] => nodes.flatMap((n) => [n.id, ...ids(n.children ?? [])])

describe('filterTree', () => {
  it('returns the tree itself for an empty or blank query', () => {
    expect(filterTree(roots, '')).toBe(roots)
    expect(filterTree(roots, '   ')).toBe(roots)
  })

  it('keeps a matching row with the groups that lead to it, and drops the rest', () => {
    const found = filterTree(roots, 'create')

    expect(ids(found)).toEqual(['c1', 'c1/orders', 'o2'])
  })

  it('keeps everything under a group whose own name matches', () => {
    const found = filterTree(roots, 'orders')

    expect(ids(found)).toEqual(['c1', 'c1/orders', 'o1', 'o2'])
  })

  it('matches without regard to case and on the method too', () => {
    expect(ids(filterTree(roots, 'INVOICE'))).toEqual(['c2', 'b1'])
    expect(ids(filterTree(roots, 'post'))).toEqual(['c1', 'c1/orders', 'o2'])
  })

  it('needs every word of the query to match', () => {
    expect(ids(filterTree(roots, 'list orders'))).toEqual(['c1', 'c1/orders', 'o1'])
    expect(filterTree(roots, 'list invoice')).toEqual([])
  })

  it('does not change the tree it was given', () => {
    filterTree(roots, 'ping')

    expect(roots[0].children).toHaveLength(2)
  })
})

describe('firstLeaf', () => {
  it('finds the first row that is not a group', () => {
    expect(firstLeaf(filterTree(roots, 'orders'))?.id).toBe('o1')
  })

  it('is undefined when nothing matched', () => {
    expect(firstLeaf([])).toBeUndefined()
  })
})

describe('highlightParts', () => {
  it('splits a label into the stretches that match a word and the rest', () => {
    expect(highlightParts('List orders', 'ord')).toEqual([{ text: 'List ', hit: false }, { text: 'ord', hit: true }, { text: 'ers', hit: false }])
  })

  it('marks every word, ignoring case, and merges words that touch or overlap', () => {
    expect(highlightParts('Create Order', 'ORDER create')).toEqual([{ text: 'Create', hit: true }, { text: ' ', hit: false }, { text: 'Order', hit: true }])
    expect(highlightParts('abcd', 'ab bc')).toEqual([{ text: 'abc', hit: true }, { text: 'd', hit: false }])
  })

  it('returns the label whole when nothing in it matches or the query is blank', () => {
    expect(highlightParts('Ping', 'zzz')).toEqual([{ text: 'Ping', hit: false }])
    expect(highlightParts('Ping', '  ')).toEqual([{ text: 'Ping', hit: false }])
  })
})
