import { describe, expect, it } from 'vitest'
import { isDangerLabel } from './menudanger'

describe('isDangerLabel', () => {
  it('marks items that destroy', () => {
    for (const l of ['Delete…', 'Remove', 'Move to trash', 'Discard changes', 'Clear all']) expect(isDangerLabel(l), l).toBe(true)
  })
  it('leaves the rest alone', () => {
    for (const l of ['Rename…', 'Duplicate', 'Run', 'Deleted items', 'Reveal in folder']) expect(isDangerLabel(l), l).toBe(false)
  })
})
