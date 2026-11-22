// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest'
import { portal } from './portal'

describe('portal', () => {
  // Before: the hint of a field in a card's panel was clipped at that panel's right edge.
  it('Given an element inside a clipping panel When the action runs Then it is on body, and it is gone when destroyed', () => {
    const panel = document.createElement('div')
    panel.style.overflow = 'auto'
    const el = document.createElement('div')
    panel.appendChild(el)
    document.body.appendChild(panel)

    const action = portal(el)
    const moved = el.parentElement === document.body
    action.destroy()

    expect(moved).toBe(true)
    expect(document.body.contains(el)).toBe(false)
    panel.remove()
  })
})
