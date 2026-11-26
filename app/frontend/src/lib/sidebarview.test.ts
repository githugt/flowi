import { describe, expect, it } from 'vitest'
import { countLeaves, leavesOf, loadTab, loadView, menuRowOf, saveTab, saveView, sectionRoots, sectionsFor, subtitle } from './sidebarview'
import type { TreeNode } from './types'

const req = (id: string, label: string, where = '', typeName = 'HTTP'): TreeNode => ({ id, label, kind: 'request', method: 'GET', type: 'http', typeName, where })
const flow = (id: string, label: string): TreeNode => ({ id, label, kind: 'flow', type: 'flow', typeName: 'Flow' })
const folder = (id: string, label: string, children: TreeNode[]): TreeNode => ({ id, label, kind: 'folder', tree: 'requests', children })
const collection = (id: string, label: string, requests: TreeNode[], flows: TreeNode[] = [], extra: Partial<TreeNode> = {}): TreeNode => ({
  id, label, kind: 'collection', ...extra,
  children: [
    { id: `${id}/requests`, label: 'Requests', kind: 'requests', tree: 'requests', children: requests },
    { id: `${id}/flows`, label: 'Flows', kind: 'flows', tree: 'flows', children: flows },
  ],
})

const roots: TreeNode[] = [
  collection('c:shop', 'Shop', [folder('f1', 'auth', [req('r1', 'Login')]), req('r2', 'Health')], [flow('fl1', 'Checkout')]),
  collection('c:bill', 'Billing', [req('r3', 'Invoice')]),
  collection('c:new', 'New one', []),
  collection('c:drafts', 'Drafts', [req('r4', 'poke')], [flow('fl2', 'untitled')], { draft: true, scratch: true }),
  { id: 'm:gone', label: '~/old (missing)', kind: 'missing' },
  { id: 'environments', label: 'Environments', kind: 'environments', children: [{ id: 'e:1', label: 'staging', kind: 'environment' }] },
]

const labels = (nodes: TreeNode[]) => nodes.map((n) => n.label)

describe('sectionRoots', () => {
  // Before: every collection repeated a Requests and a Flows row; a section's collection stands for its group's children.
  it('shows each collection once per section with the children of its own group, not the group row', () => {
    const requests = sectionRoots(roots, 'requests', [])

    expect(labels(requests)).toEqual(['Shop', 'Billing', 'New one', '~/old (missing)', 'Drafts'])
    expect(labels(requests[0].children ?? [])).toEqual(['auth', 'Health'])
    expect(requests[0].groupId).toBe('c:shop/requests')
  })

  it('leaves out of Flows the collections that have no flow', () => {
    expect(labels(sectionRoots(roots, 'flows', []))).toEqual(['Shop', 'Drafts'])
  })

  it('lists the environments themselves, with no Environments row', () => {
    expect(labels(sectionRoots(roots, 'environments', []))).toEqual(['staging'])
  })

  it('puts the starred collections first, in the order they were starred', () => {
    const found = sectionRoots(roots, 'requests', ['c:bill', 'c:shop'])

    expect(labels(found).slice(0, 3)).toEqual(['Billing', 'Shop', 'New one'])
  })

  it('keeps drafts last among the rest', () => {
    expect(labels(sectionRoots(roots, 'requests', ['c:bill'])).at(-1)).toBe('Drafts')
  })

  it('drops an empty collection while a search shows hits only', () => {
    expect(labels(sectionRoots(roots, 'requests', [], false))).toEqual(['Shop', 'Billing', 'Drafts'])
  })

  it('does not change the tree it is given', () => {
    const before = JSON.stringify(roots)

    sectionRoots(roots, 'requests', ['c:bill'])

    expect(JSON.stringify(roots)).toBe(before)
  })
})

describe('leavesOf, countLeaves and subtitle', () => {
  it('flattens a section to the rows that open something', () => {
    const shop = sectionRoots(roots, 'requests', [])[0]

    expect(labels(leavesOf([shop]))).toEqual(['Login', 'Health'])
    expect(countLeaves(sectionRoots(roots, 'requests', []))).toBe(4)
  })

  it('says where a row lives and what it is, as the list view shows it', () => {
    expect(subtitle(req('r', 'x', 'Test Collection / Messaging', 'Async'))).toBe('Test Collection / Messaging · Async')
    expect(subtitle({ where: '', typeName: 'HTTP' })).toBe('HTTP')
    expect(subtitle({})).toBe('')
  })
})

describe('sectionsFor', () => {
  it('shows all three sections, or only the part a rail icon stands for', () => {
    expect(sectionsFor('')).toEqual(['requests', 'flows', 'environments'])
    expect(sectionsFor('collections')).toEqual(['requests'])
    expect(sectionsFor('flows')).toEqual(['flows'])
    expect(sectionsFor('environments')).toEqual(['environments'])
  })
})

describe('what the sidebar keeps of its view', () => {
  const memory = (): { getItem(k: string): string | null; setItem(k: string, v: string): void } => {
    const m = new Map<string, string>()
    return { getItem: (k) => m.get(k) ?? null, setItem: (k, v) => void m.set(k, v) }
  }

  it('starts as a tree on the All tab and remembers the other choices', () => {
    const s = memory()
    expect([loadView(s), loadTab(s)]).toEqual(['tree', 'all'])

    saveView(s, 'list')
    saveTab(s, 'recent')

    expect([loadView(s), loadTab(s)]).toEqual(['list', 'recent'])
  })

  it('falls back to the defaults when storage is blocked or holds nonsense', () => {
    const blocked = { getItem: () => { throw new Error('blocked') }, setItem: () => { throw new Error('blocked') } }
    const odd = { getItem: () => 'sideways', setItem: () => {} }

    expect([loadView(blocked), loadTab(blocked), loadView(odd), loadTab(odd)]).toEqual(['tree', 'all', 'tree', 'all'])
    expect(() => saveView(blocked, 'list')).not.toThrow()
  })
})

describe('menuRowOf', () => {
  // Before: right-clicking a collection in Flows and choosing New folder made the folder under Requests.
  it('asks a collection in the Flows section for its Flows group menu, and every other row for its own', () => {
    const inFlows = sectionRoots(roots, 'flows', [])[0]
    const inRequests = sectionRoots(roots, 'requests', [])[0]

    expect(menuRowOf(inFlows)).toBe('c:shop/flows')
    expect(menuRowOf(inRequests)).toBe('c:shop')
    expect(menuRowOf(inFlows.children![0])).toBe('fl1')
  })
})
