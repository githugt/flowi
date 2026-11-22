import { describe, expect, it } from 'vitest'
import { forgetNodeUi, nodeUi, setNodeUi } from './nodeui'

describe('nodeUi', () => {
  it('starts every node at the first section with all hooks', () => {
    expect(nodeUi('r:new')).toEqual({ section: '', stepSection: '', hooks: 'all', replay: false })
  })
  it('keeps what was set for a node, apart from other nodes', () => {
    // Before: the editor remounted on a tab switch and every one of these went back to its first value.
    setNodeUi('r:a', { hooks: 'none' })
    setNodeUi('r:a', { section: 'Headers' })

    expect(nodeUi('r:a')).toEqual({ section: 'Headers', stepSection: '', hooks: 'none', replay: false })
    expect(nodeUi('r:b').hooks).toBe('all')
  })
  it('hands out a copy', () => {
    setNodeUi('r:c', { replay: true })
    const copy = nodeUi('r:c')
    copy.replay = false

    expect(nodeUi('r:c').replay).toBe(true)
  })
  it('forgets a closed node', () => {
    setNodeUi('r:d', { hooks: 'skip-pre' })
    forgetNodeUi('r:d')

    expect(nodeUi('r:d').hooks).toBe('all')
  })
})
