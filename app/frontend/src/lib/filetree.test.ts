import { describe, expect, it } from 'vitest'
import { buildFileTree } from './filetree'

const f = (rel: string, exists = false, remove = false) => ({ rel, exists, remove })

describe('buildFileTree', () => {
  it('groups paths by folder, folders before files, each sorted by name', () => {
    const tree = buildFileTree([f('workspace.yaml'), f('shop/requests/b.yaml'), f('shop/requests/a.yaml'), f('shop/_collection.yaml')])

    expect(tree.map((n) => n.name)).toEqual(['shop', 'workspace.yaml'])
    expect(tree[0].children.map((n) => n.name)).toEqual(['requests', '_collection.yaml'])
    expect(tree[0].children[0].children.map((n) => n.name)).toEqual(['a.yaml', 'b.yaml'])
  })

  it('counts the files under each folder and how many of them already exist', () => {
    const tree = buildFileTree([f('shop/requests/a.yaml', true), f('shop/requests/b.yaml'), f('shop/flows/c.yaml')])

    expect(tree[0]).toMatchObject({ name: 'shop', files: 3, existing: 1 })
    expect(tree[0].children.map((n) => [n.name, n.files, n.existing])).toEqual([['flows', 1, 0], ['requests', 2, 1]])
  })

  it('keeps a file marked for removal and the path to it', () => {
    const tree = buildFileTree([f('old/x.yaml', false, true)])

    expect(tree[0].children[0]).toMatchObject({ name: 'x.yaml', path: 'old/x.yaml', remove: true, files: 1 })
  })

  it('is empty for no files', () => {
    expect(buildFileTree([])).toEqual([])
  })

  it('counts the requests and flows below a folder, by the requests or flows folder a file sits in', () => {
    const tree = buildFileTree([f('shop/requests/a.yaml'), f('shop/requests/auth/b.yaml'), f('shop/flows/c.yaml'), f('shop/_collection.yaml'), f('shop/requests/a.schemas/200.json')])

    // The kept response schemas beside a request are files, not requests.
    expect(tree[0]).toMatchObject({ files: 5, requests: 2, flows: 1 })
    expect(tree[0].children.find((n) => n.name === 'requests')).toMatchObject({ requests: 2, flows: 0 })
    expect(tree[0].children.find((n) => n.name === '_collection.yaml')).toMatchObject({ requests: 0, flows: 0 })
  })
})
