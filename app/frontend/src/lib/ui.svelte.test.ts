import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ui, type ConfirmDialog, type PromptDialog } from './ui.svelte'

// The window-level UI state holds the races: a dialog that fails and comes back while another one took its place,
// and toast timers that must not fire after a toast is closed. Before: none of it had a test, so it was covered only
// by reading the source. Time is faked, so nothing waits.
beforeEach(() => {
  vi.useFakeTimers()
  ui.toasts.forEach((t) => ui.dismissToast(t.id))
  ui.dialog = null
  ui.menu = null
})
afterEach(() => vi.useRealTimers())

const prompt = (over: Partial<PromptDialog> = {}): PromptDialog => ({ kind: 'prompt', title: 'Rename', label: 'Name', initial: 'old', done: () => {}, ...over })

describe('ui menu', () => {
  it('should open with its items and close, and ignore a menu with none', () => {
    // Arrange
    const items = [{ label: 'Open', run: () => {} }]

    // Act
    ui.openMenu(5, 6, [])
    const none = ui.menu
    ui.openMenu(5, 6, items)
    const open = ui.menu
    ui.closeMenu()

    // Assert
    expect(none).toBeNull()
    expect(open).toEqual({ x: 5, y: 6, items })
    expect(ui.menu).toBeNull()
  })
})

describe('ui dialogs', () => {
  it('should tell a prompt that another dialog replaced it, and one that is dismissed', () => {
    // Arrange
    const cancelled: string[] = []

    // Act
    ui.prompt({ title: 'A', label: 'Name', initial: '', cancel: () => cancelled.push('A'), done: () => {} })
    ui.confirm('B', 'sure?', () => {})
    ui.prompt({ title: 'C', label: 'Name', initial: '', cancel: () => cancelled.push('C'), done: () => {} })
    ui.dismiss()

    // Assert
    expect(cancelled).toEqual(['A', 'C'])
    expect(ui.dialog).toBeNull()
  })

  it('should not tell a prompt that was answered it was cancelled', () => {
    // Arrange
    let cancelled = false
    ui.prompt({ title: 'A', label: 'Name', initial: '', cancel: () => (cancelled = true), done: () => {} })

    // Act
    const taken = ui.take()
    ui.dismiss()

    // Assert
    expect(taken?.kind).toBe('prompt')
    expect(cancelled).toBe(false)
  })

  it('should know which keyed prompt shows', () => {
    // Act
    ui.prompt({ title: 'Vault', label: 'Passphrase', initial: '', key: 'vault', done: () => {} })

    // Assert
    expect(ui.showing('vault')).toBe(true)
    expect(ui.showing('other')).toBe(false)
    ui.confirm('X', 'y', () => {})
    expect(ui.showing('vault')).toBe(false)
  })
})

describe('ui answer', () => {
  it('should bring the prompt back with the error and the typed text when the answer rejects', async () => {
    // Arrange
    const d = prompt()

    // Act
    ui.answer(d, 'typed', () => Promise.reject(new Error('name taken')))
    await vi.advanceTimersByTimeAsync(0)

    // Assert
    expect(ui.dialog).toMatchObject({ kind: 'prompt', initial: 'typed', error: 'name taken' })
  })

  it('should bring a prompt back when the answer throws at once', () => {
    // Arrange
    const d = prompt()

    // Act
    ui.answer(d, 'typed', () => {
      throw new Error('bad path')
    })

    // Assert
    expect(ui.dialog).toMatchObject({ initial: 'typed', error: 'bad path' })
  })

  it('should show the error as a toast, and leave the newer dialog, when another dialog took its place', async () => {
    // Arrange
    const d = prompt()
    const newer = prompt({ title: 'Newer' })
    ui.answer(d, 'typed', () => {
      ui.dialog = newer
      return Promise.reject(new Error('late failure'))
    })

    // Act
    await vi.advanceTimersByTimeAsync(0)

    // Assert
    expect(ui.dialog).toBe(newer)
    expect(ui.toasts.map((t) => [t.text, t.level])).toEqual([['late failure', 'error']])
  })

  it('should stay quiet when the call was cancelled by the window', async () => {
    // Arrange
    const cancelled = Object.assign(new Error('cancelled'), { name: 'CancelError' })

    // Act
    ui.answer(prompt(), 'x', () => Promise.reject(cancelled))
    await vi.advanceTimersByTimeAsync(0)

    // Assert
    expect(ui.dialog).toBeNull()
    expect(ui.toasts).toEqual([])
  })

  it('should bring a confirm back with the error and leave a successful answer closed', async () => {
    // Arrange
    const confirm: ConfirmDialog = { kind: 'confirm', title: 'Delete', text: 'sure?', action: 'Delete', done: () => {} }

    // Act
    ui.answer(confirm, '', () => Promise.resolve())
    await vi.advanceTimersByTimeAsync(0)
    const afterOk = ui.dialog
    ui.answer(confirm, '', () => Promise.reject(new Error('locked')))
    await vi.advanceTimersByTimeAsync(0)

    // Assert
    expect(afterOk).toBeNull()
    expect(ui.dialog).toMatchObject({ kind: 'confirm', error: 'locked' })
  })
})

describe('ui toasts', () => {
  it('should leave after its time and not before', () => {
    // Arrange
    ui.toast('Saved')

    // Act
    vi.advanceTimersByTime(3999)
    const before = ui.toasts.length
    vi.advanceTimersByTime(1)

    // Assert
    expect(before).toBe(1)
    expect(ui.toasts).toEqual([])
  })

  it('should count the same message again and start its time over', () => {
    // Arrange
    ui.toast('Copied')
    vi.advanceTimersByTime(3000)

    // Act
    ui.toast('Copied')
    vi.advanceTimersByTime(3000)
    const stillThere = ui.toasts.map((t) => t.count)
    vi.advanceTimersByTime(1000)

    // Assert
    expect(stillThere).toEqual([2])
    expect(ui.toasts).toEqual([])
  })

  it('should hold a toast while it is pointed at and let it go a little after, at most four seconds', () => {
    // Arrange
    ui.toast('Held', 'info', { life: 10000 })
    const id = ui.toasts[0].id

    // Act
    ui.pause(id)
    vi.advanceTimersByTime(60000)
    const held = ui.toasts.length
    ui.resume(id)
    vi.advanceTimersByTime(3999)
    const resumed = ui.toasts.length
    vi.advanceTimersByTime(1)

    // Assert
    expect(held).toBe(1)
    expect(resumed).toBe(1)
    expect(ui.toasts).toEqual([])
  })

  it('should not fire the timer of a toast that was closed', () => {
    // Arrange
    ui.toast('First')
    const first = ui.toasts[0].id
    ui.dismissToast(first)
    ui.toast('Second', 'info', { life: 9000 })

    // Act
    vi.advanceTimersByTime(5000)

    // Assert
    expect(ui.toasts.map((t) => t.text)).toEqual(['Second'])
  })

  it('should keep at most four and clear the timers of the ones pushed out', () => {
    // Act
    for (const n of ['a', 'b', 'c', 'd', 'e']) ui.toast(n, 'info', { life: 50000 })

    // Assert
    expect(ui.toasts.map((t) => t.text)).toEqual(['b', 'c', 'd', 'e'])
    expect(vi.getTimerCount()).toBe(4)
  })

  it('should keep a toast with a button apart from one with the same text', () => {
    // Act
    ui.toast('Deleted x', 'info', { action: { label: 'Undo', run: () => {} } })
    ui.toast('Deleted x', 'info', { action: { label: 'Undo', run: () => {} } })

    // Assert
    expect(ui.toasts).toHaveLength(2)
  })

  it('should show a failed call as an error toast through fail, and not a cancelled one', () => {
    // Act
    ui.fail(new Error('boom'))
    ui.fail(Object.assign(new Error('x'), { name: 'CancelledRejectionError' }))

    // Assert
    expect(ui.toasts.map((t) => [t.text, t.level])).toEqual([['boom', 'error']])
  })
})
