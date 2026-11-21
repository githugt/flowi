import { describe, expect, it } from 'vitest'
import { blockedUnits, hintOf } from './wiretargets'
import canvas from '../Canvas.svelte?raw'
import editor from '../FlowEditor.svelte?raw'

const units = [
  { id: 'u1', nodes: ['a'] },
  { id: 'u2', nodes: ['b', 'c'] },
  { id: 'u3', nodes: ['x'] },
]

describe('wire targets', () => {
  it('blocks the units whose first step Go refused, and never the one the wire starts from', () => {
    const targets = [
      { id: 'a', ok: false, reason: 'a cannot wait for itself' },
      { id: 'b', ok: true, reason: '' },
      { id: 'c', ok: false, reason: 'c is not what the unit is judged by' },
      { id: 'x', ok: false, reason: 'a and x are in different lists' },
    ]
    expect(blockedUnits(units, targets, 'u1')).toEqual({ u3: 'a and x are in different lists' })
  })

  it('gives a reason even when Go gave none', () => {
    expect(blockedUnits(units, [{ id: 'x', ok: false, reason: '' }], 'u1').u3).toBe('This step cannot take the link')
  })

  it('shortens a reason to the part before the colon', () => {
    expect(hintOf('b comes after a, so a cannot wait for it: start the link from the earlier step')).toBe('b comes after a, so a cannot wait for it')
    expect(hintOf('no colon here')).toBe('no colon here')
  })

  it('is asked when a drag starts, dims what is refused, and the refusal reaches the person as a toast', () => {
    expect(canvas).toContain('wireTargets?:')
    expect(canvas).toContain('class:blocked=')
    expect(editor).toContain('FlowService.WireTargets(node, from, kind)')
    expect(editor).toContain('FlowService.Connect(node, from, to).catch(ui.fail)')
  })
})
