import { describe, expect, it } from 'vitest'
import { rowTitle } from './rowtitle'
import { filterTree } from './treefilter'
import item from '../TreeItem.svelte?raw'

describe('tree row title', () => {
  it('gives the whole name with the verb or the kind', () => {
    expect(rowTitle({ kind: 'request', label: 'Create an order', method: 'POST' })).toBe('POST Create an order')
    expect(rowTitle({ kind: 'request', label: 'Ping' })).toBe('Ping')
    expect(rowTitle({ kind: 'flow', label: 'Checkout' })).toBe('Checkout (flow)')
    expect(rowTitle({ kind: 'group', label: 'Payments' })).toBe('Payments')
  })

  it('draws a compact verb pill on requests and flows only, and the folder icon on folders only (Drafts has its own)', () => {
    // Before: the verb was bare text, and a folder had no mark of its own, so a row of any kind could be
    // taken for any other; the hover text was the name again, where it is a card now (HoverCard).
    expect(item).toContain('<Verb verb={node.method} short compact />')
    expect(item).toContain('<Verb verb="FLOW" compact />')
    expect(item).toContain("const isFolder = $derived(node.kind === 'folder')")
    expect(item).toContain('<Icon name="folder"')
    expect(item).toContain("node.kind === 'collection' && node.scratch")
    expect(item).not.toContain('title={rowTitle(node)}')
    expect(item).toContain('hover.enter(')
  })

  it('keeps a file that cannot be read in the tree, in red, with the reason, and opens nothing for it', () => {
    // Before: a request with merge conflict markers vanished from the tree as if it had been deleted.
    expect(rowTitle({ kind: 'broken', label: 'refresh', detail: 'unresolved git merge conflict (line 14)' })).toBe('refresh: cannot load, unresolved git merge conflict (line 14)')
    expect(rowTitle({ kind: 'broken', label: 'refresh' })).toBe('refresh: cannot load, the file is not valid')
    expect(item).toContain("node.kind === 'broken'")
    expect(item).toContain('class:broken=')
    expect(item).toContain('unmerged')
    expect(filterTree([{ id: 'r', label: 'Requests', kind: 'requests', children: [{ id: 'x', label: 'refresh', kind: 'broken' }] }], 'refresh')[0].children?.[0].id).toBe('x')
  })
})
