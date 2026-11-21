import { describe, expect, it } from 'vitest'
import { addToast, MAX_TOASTS, type Toast } from './toastlist'

describe('addToast', () => {
  it('adds a new message as it is', () => {
    const got = addToast([], 'Saved', 'info', 1)

    expect(got.list).toEqual([{ id: 1, level: 'info', text: 'Saved' }])
    expect(got.reused).toBe(false)
  })

  it('counts the same message again instead of stacking it', () => {
    const first = addToast([], 'Could not reach the server', 'error', 1).list

    const got = addToast(first, 'Could not reach the server', 'error', 2)

    expect(got.list).toHaveLength(1)
    expect(got.list[0].count).toBe(2)
    expect(got.id).toBe(1)
    expect(got.reused).toBe(true)
  })

  it('keeps the same text at another level as its own toast', () => {
    const first = addToast([], 'Done', 'info', 1).list

    expect(addToast(first, 'Done', 'error', 2).list).toHaveLength(2)
  })

  it('drops the oldest toasts past the cap and names them', () => {
    let list: Toast[] = []
    for (let i = 1; i <= MAX_TOASTS; i++) list = addToast(list, `m${i}`, 'info', i).list

    const got = addToast(list, 'newest', 'info', 99)

    expect(got.list.map((t) => t.text)).toEqual(['m2', 'm3', 'm4', 'newest'])
    expect(got.dropped).toEqual([1])
  })
})

describe('a toast with an action', () => {
  it('should never merge with another toast of the same text, so each Undo stays', () => {
    // Before: toasts had no button, and the same text twice was one toast with a count.
    const undo = { label: 'Undo', run: () => {} }
    const first = addToast([], 'Deleted Login', 'info', 1, undo)

    const second = addToast(first.list, 'Deleted Login', 'info', 2, undo)

    expect(second.reused).toBe(false)
    expect(second.list.map((t) => t.action?.label)).toEqual(['Undo', 'Undo'])
  })

  it('should not absorb a plain toast of the same text either', () => {
    const plain = addToast([], 'Deleted Login', 'info', 1)

    const withAction = addToast(plain.list, 'Deleted Login', 'info', 2, { label: 'Undo', run: () => {} })

    expect(withAction.list).toHaveLength(2)
    expect(plain.list[0].count).toBeUndefined()
  })
})
