import { describe, expect, it } from 'vitest'
import { filterChoices, filtersNote, filterTerms, hasTerm, withoutFilters, withoutPrefix } from './sidefilter'
import { filterTree } from './treefilter'
import { parseQuery } from './devstatus'
import type { TreeNode } from './types'

const st = (name: string) => ({ name, color: 'blue', icon: 'code' })
const req = (id: string, label: string, type: string, typeName: string, where: string, extra: Partial<TreeNode> = {}): TreeNode => ({ id, label, kind: 'request', method: 'GET', type, typeName, where, ...extra })
const roots: TreeNode[] = [
  { id: 'c:hv', label: 'Hyperverge', kind: 'collection', children: [
    { id: 'c:hv/requests', label: 'Requests', kind: 'requests', children: [
      { id: 'fo', label: 'Face Match - India', kind: 'folder', where: 'Hyperverge', children: [req('r1', 'Match', 'http', 'HTTP', 'Hyperverge / Face Match - India', { tags: ['face'], status: st('testing') })] },
      req('r2', 'Token', 'http', 'HTTP', 'Hyperverge', { tags: ['auth'] }),
    ] },
  ] },
  { id: 'c:msg', label: 'Test Collection', kind: 'collection', children: [
    { id: 'c:msg/requests', label: 'Requests', kind: 'requests', children: [
      req('r3', 'Call', 'grpc', 'gRPC', 'Test Collection / grpc', { status: st('review') }),
      req('r4', 'Publish', 'message', 'Async', 'Test Collection'),
    ] },
    { id: 'c:msg/flows', label: 'Flows', kind: 'flows', children: [{ id: 'fl', label: 'Onboard', kind: 'flow', type: 'flow', typeName: 'Flow', where: 'Test Collection' }] },
  ] },
  { id: 'environments', label: 'Environments', kind: 'environments', children: [{ id: 'e1', label: 'staging', kind: 'environment' }] },
]
const leafIds = (nodes: TreeNode[]): string[] => nodes.flatMap((n) => (n.children?.length ? leafIds(n.children) : n.kind === 'request' || n.kind === 'flow' || n.kind === 'environment' ? [n.id] : []))

describe('the search terms the filter popover writes', () => {
  // Before: only `tag:` and `status:` existed.
  it('reads type:, in: and has: next to tag: and status:', () => {
    const q = parseQuery('type:async in:face-match has:problems tag:x status:y word')

    expect(q).toEqual({ words: ['word'], tags: ['x'], statuses: ['y'], types: ['message'], places: ['face-match'], has: ['problems'] })
  })

  it('drops a type that is not one, or a flag nobody knows, so a half-typed term does not empty the list', () => {
    expect(parseQuery('type:gr has:nothing in:')).toEqual({ words: [], tags: [], statuses: [], types: [], places: [], has: [] })
  })
})

describe('filtering the tree by the new terms', () => {
  it('keeps the requests of a type, with the groups leading to them', () => {
    expect(leafIds(filterTree(roots, 'type:grpc'))).toEqual(['r3'])
    expect(leafIds(filterTree(roots, 'type:grpc type:async'))).toEqual(['r3', 'r4'])
    expect(leafIds(filterTree(roots, 'type:flow'))).toEqual(['fl'])
  })

  it('keeps what is in a collection or a folder, by its name written with dashes', () => {
    expect(leafIds(filterTree(roots, 'in:test-collection'))).toEqual(['r3', 'r4', 'fl'])
    expect(leafIds(filterTree(roots, 'in:face-match'))).toEqual(['r1'])
    expect(leafIds(filterTree(roots, 'in:hyper'))).toEqual(['r1', 'r2'])
  })

  it('keeps what the Problems panel lists when asked for has:problems', () => {
    const problems = new Set(['r2', 'fl'])

    expect(leafIds(filterTree(roots, 'has:problems', { problems }))).toEqual(['r2', 'fl'])
    expect(leafIds(filterTree(roots, 'has:problems'))).toEqual([])
  })

  it('combines kinds of term: all of them must hold, a status is any of several', () => {
    expect(leafIds(filterTree(roots, 'status:testing status:review type:http'))).toEqual(['r1'])
    expect(leafIds(filterTree(roots, 'in:hyperverge tag:auth'))).toEqual(['r2'])
  })

  it('does not take the group a term names as a hit of its own', () => {
    expect(leafIds(filterTree(roots, 'in:test-collection type:flow'))).toEqual(['fl'])
  })
})

describe('filterChoices', () => {
  it('lists the types of request in the order of the menu, with how many each has', () => {
    const c = filterChoices(roots)

    expect(c.types.map((t) => [t.term, t.name, t.count])).toEqual([['http', 'HTTP', 2], ['grpc', 'gRPC', 1], ['async', 'Async', 1], ['flow', 'Flow', 1]])
  })

  it('lists the collections with the terms that pick them, and the statuses and tags in use', () => {
    const c = filterChoices(roots)

    expect(c.collections).toEqual([{ name: 'Hyperverge', term: 'hyperverge', count: 2 }, { name: 'Test Collection', term: 'test-collection', count: 3 }])
    expect(c.statuses.map((s) => s.name)).toEqual(['testing', 'review'])
    expect(c.tags).toEqual([{ tag: 'auth', count: 1 }, { tag: 'face', count: 1 }])
  })
})

describe('the terms in a query', () => {
  it('finds, adds apart and clears filter terms and keeps the words', () => {
    const q = 'login type:grpc  in:shop #smoke'

    expect(filterTerms(q)).toEqual(['type:grpc', 'in:shop', '#smoke'])
    expect(withoutFilters(q)).toBe('login')
    expect(withoutPrefix(q, 'type:')).toBe('login in:shop #smoke')
    expect(hasTerm(q, 'IN:Shop')).toBe(true)
    expect(hasTerm(q, 'in:sho')).toBe(false)
  })

  it('says how many filters are on', () => {
    expect(filtersNote('plain words')).toBe('')
    expect(filtersNote('type:grpc')).toBe('1 filter on')
    expect(filtersNote('type:grpc has:problems')).toBe('2 filters on')
  })
})
