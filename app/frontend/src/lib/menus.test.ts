import { describe, expect, it, vi } from 'vitest'

const requestNode = { id: 'r1', kind: 'request', label: 'Create' }

vi.mock('../../bindings/github.com/githugt/flowi/app/internal/bridge', () => ({
  WorkspaceService: { Menu: async () => [{ label: 'New flow', action: 'newFlow' }, { label: 'Rename', action: 'rename' }] },
  BlueprintsService: { Where: async () => ({ collection: 'c', folder: '' }) },
}))
vi.mock('./app.svelte', () => ({
  app: { marked: [] as string[], active: 'r2', find: () => requestNode, openBeside: async () => {} },
}))

import { rowMenu } from './menus'

describe('rowMenu', () => {
  it('rowMenu_Given_request_row_When_built_Then_extra_items_do_not_break_the_blueprint_lookup', async () => {
    // Arrange: two extra items ("Open to the side", "Negative variants…") are appended to the entries from Go.

    // Act
    const labels = (await rowMenu('r1')).map((i) => i.label)

    // Assert: the lookup of the entry that offers a flow ran over the Go entries only, so the menu was built at all.
    expect(labels).toEqual(['New flow', 'New flow from blueprint…', 'Suggest flows…', 'Discover an API…', 'Rename', 'Open to the side', 'Negative variants…'])
  })
})
