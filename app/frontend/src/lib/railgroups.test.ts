import { describe, expect, it } from 'vitest'
import { rootsForGroup } from './railgroups'
import type { TreeNode } from './types'

const node = (id: string, kind: string, children: TreeNode[] = []): TreeNode => ({ id, label: id, kind, children })
const roots = [
  node('shop', 'collection', [node('reqs', 'requests'), node('flows', 'flows', [node('f1', 'flow')])]),
  node('empty', 'collection', [node('reqs2', 'requests')]),
  node('envs', 'environments', [node('local', 'environment')]),
]

describe('rootsForGroup', () => {
  it('shows everything when no group is chosen', () => {
    expect(rootsForGroup(roots, '')).toHaveLength(3)
  })

  it('keeps the collections whole and leaves the environments out', () => {
    expect(rootsForGroup(roots, 'collections').map((r) => r.id)).toEqual(['shop', 'empty'])
  })

  it('shows the environments alone', () => {
    expect(rootsForGroup(roots, 'environments').map((r) => r.id)).toEqual(['envs'])
  })

  it('shows only the flows of each collection and drops a collection without any', () => {
    const flows = rootsForGroup(roots, 'flows')

    expect(flows.map((r) => r.id)).toEqual(['shop'])
    expect(flows[0].children?.map((k) => k.id)).toEqual(['flows'])
  })
})
