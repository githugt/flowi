import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fake } from './fakewails'

vi.mock('@wailsio/runtime', async () => (await import('./fakewails')).fake.runtime())
vi.mock('../../bindings/github.com/githugt/flowi/app/internal/bridge', async () => (await import('./fakewails')).fake.bridge())

// The window's state, driven through a fake Go: the order answers and events arrive in is the test's to set.
// Before: app.svelte.ts held every race of the window (which click chooses the tab, which late event is dropped,
// whether a failed save closes a tab) and had no test; the ratchet only read its source.

const node = (id: string, kind: string, label = id) => ({ id, kind, label, children: [] })
const tree = (...nodes: ReturnType<typeof node>[]) => ({ roots: nodes, environments: [], selected: '' })

async function freshWindow() {
  vi.resetModules() // a new `app` and `ui`, so listeners and tabs of the last test are gone
  const { app } = await import('./app.svelte')
  const { ui } = await import('./ui.svelte')
  return { app, ui }
}

beforeEach(() => {
  fake.reset()
  vi.useFakeTimers()
})
afterEach(() => vi.useRealTimers())

/** The answers of a window that starts clean: nothing to restore, no welcome. */
function quietStart() {
  fake.answer('PanelsService.State', () => ({ log: [], problems: [], summary: '', history: [], opening: '' }))
  fake.answer('WorkspaceService.Tree', () => tree(node('r:A', 'request', 'Login'), node('f:B', 'flow', 'Checkout'), node('r:C', 'request', 'Logout')))
  fake.answer('SessionService.Restore', () => [])
  fake.answer('SessionService.ShowWelcome', () => false)
  fake.answer('SettingsService.State', () => new Promise(() => {})) // the settings screen is not under test
  fake.answer('TrashService.Sweep', () => undefined)
}

describe('app.open', () => {
  it('should let the newest click choose the tab when an older open answers last', async () => {
    // Arrange
    const { app } = await freshWindow()
    const slow = fake.deferred<unknown>()
    fake.answer('RequestService.Open', () => slow.promise)
    fake.answer('FlowService.Open', () => ({ node: 'f:B' }))

    // Act
    const first = app.open(node('r:A', 'request'))
    await app.open(node('f:B', 'flow'))
    slow.resolve({ node: 'r:A' })
    await first

    // Assert
    expect(app.active).toBe('f:B')
    expect(app.tabs.map((t) => t.node).sort()).toEqual(['f:B', 'r:A'])
  })

  it('should not open a second tab for a document that is open, and say why when Go refuses to open one', async () => {
    // Arrange
    const { app } = await freshWindow()
    fake.answer('RequestService.Open', (id: string) => (id === 'r:bad' ? Promise.reject(new Error('file is gone')) : { node: id }))

    // Act
    await app.open(node('r:A', 'request'))
    await app.open(node('r:A', 'request'))
    await app.open(node('r:bad', 'request'))

    // Assert
    expect(app.tabs.map((t) => t.node)).toEqual(['r:A'])
    expect(app.error).toBe('file is gone')
  })

  it('should open a group or a collection as nothing: they only expand', async () => {
    // Arrange
    const { app } = await freshWindow()

    // Act
    await app.open(node('c:1', 'collection'))

    // Assert
    expect(app.tabs).toEqual([])
    expect(fake.calls).toEqual([])
  })
})

describe('late events for a closed tab', () => {
  it('should drop a request event that arrives after its tab closed, and take events again once it is reopened', async () => {
    // Arrange
    const { app } = await freshWindow()
    quietStart()
    fake.answer('RequestService.Open', (id: string) => ({ node: id, dirty: false }))
    await app.init()
    await app.open(node('r:A', 'request'))

    // Act
    await app.close('r:A')
    fake.emit('request', { node: 'r:A', dirty: true })
    const afterClose = app.requests['r:A']
    await app.open(node('r:A', 'request'))
    fake.emit('request', { node: 'r:A', dirty: true })

    // Assert
    expect(afterClose).toBeUndefined()
    expect(app.requests['r:A']?.dirty).toBe(true)
    expect(fake.called('RequestService.Open', 'r:A')).toBe(true)
  })

  it('should drop a flow run that arrives for a flow that is not open', async () => {
    // Arrange
    const { app } = await freshWindow()
    quietStart()
    await app.init()
    fake.answer('FlowService.Open', () => ({ node: 'f:B', run: null }))
    await app.open(node('f:B', 'flow'))
    await app.close('f:B')

    // Act
    fake.emit('flowrun', { node: 'f:B', run: { status: 'passed' } })

    // Assert
    expect(app.flows['f:B']).toBeUndefined()
  })

  it('should keep the held tree when an event repeats it, so readers are not woken for nothing', async () => {
    // Arrange
    const { app } = await freshWindow()
    quietStart()
    await app.init()
    const held = app.tree

    // Act
    fake.emit('workspace', JSON.parse(JSON.stringify(tree(node('r:A', 'request', 'Login'), node('f:B', 'flow', 'Checkout'), node('r:C', 'request', 'Logout')))))
    const same = app.tree
    fake.emit('workspace', tree(node('r:A', 'request', 'Login')))

    // Assert
    expect(same).toBe(held)
    expect(app.tree?.roots).toHaveLength(1)
  })
})

describe('app.init', () => {
  it('should say why it could not start and leave the session unrestored when Go fails', async () => {
    // Arrange
    const { app } = await freshWindow()
    quietStart()
    fake.answer('WorkspaceService.Tree', () => Promise.reject(new Error('workspace locked')))

    // Act
    await app.init()

    // Assert
    expect(app.error).toBe('Could not start: workspace locked')
    expect(app.restored).toBe(false)
    expect(fake.called('SessionService.Restore')).toBe(false)
  })

  it('should bring back the tabs of the last session behind each other and choose the last, and sweep drafts once', async () => {
    // Arrange
    const { app } = await freshWindow()
    quietStart()
    fake.answer('SessionService.Restore', () => ['r:A', 'f:B', 'gone'])
    fake.answer('RequestService.Open', (id: string) => ({ node: id }))
    fake.answer('FlowService.Open', (id: string) => ({ node: id }))

    // Act
    await app.init()

    // Assert
    expect(app.tabs.map((t) => t.node)).toEqual(['r:A', 'f:B'])
    expect(app.active).toBe('f:B')
    expect(app.restored).toBe(true)
    expect(fake.callsTo('TrashService.Sweep')).toEqual([[['r:A', 'f:B']]])
  })

  it('should open the Welcome page on the first launch and Home after that', async () => {
    // Arrange
    const first = await freshWindow()
    quietStart()
    fake.answer('SessionService.ShowWelcome', () => true)

    // Act
    await first.app.init()
    const welcome = first.app.active

    // Assert
    expect(welcome).toBe('screen:welcome')

    // Arrange (second launch)
    fake.reset()
    const later = await freshWindow()
    quietStart()
    await later.app.init()
    expect(later.app.active).toBe('screen:home')
  })

  it('should run one draft sweep at a time: a slow pass is not overlapped by the next tick', async () => {
    // Arrange
    const { app } = await freshWindow()
    quietStart()
    const slow = fake.deferred<void>()
    fake.answer('TrashService.Sweep', () => slow.promise)

    // Act
    await app.init()
    await vi.advanceTimersByTimeAsync(30 * 60 * 1000)
    const during = fake.callsTo('TrashService.Sweep').length
    slow.resolve()
    await vi.advanceTimersByTimeAsync(30 * 60 * 1000)

    // Assert
    expect(during).toBe(1)
    expect(fake.callsTo('TrashService.Sweep').length).toBe(2)
  })
})

describe('closing a tab with unsaved edits', () => {
  async function openDirty() {
    const w = await freshWindow()
    quietStart()
    fake.answer('RequestService.Open', (id: string) => ({ node: id, dirty: true }))
    await w.app.init()
    w.app.tabs = [] // the Home tab init opens is not under test
    await w.app.open(node('r:A', 'request', 'Login'))
    return w
  }

  it('should ask first, and close at once when nothing is unsaved', async () => {
    // Arrange
    const { app, ui } = await openDirty()
    fake.answer('RequestService.Open', (id: string) => ({ node: id, dirty: false }))
    await app.open(node('r:C', 'request', 'Logout'))

    // Act
    await app.requestClose('r:C')
    const clean = app.tabs.map((t) => t.node)
    await app.requestClose('r:A')

    // Assert
    expect(clean).toEqual(['r:A'])
    expect(ui.dialog).toMatchObject({ kind: 'unsaved', text: 'Login has changes that are not saved.' })
    expect(app.tabs.map((t) => t.node)).toEqual(['r:A'])
  })

  it('should close only after the save has landed, which Go reports as an event', async () => {
    // Arrange
    const { app, ui } = await openDirty()
    await app.requestClose('r:A')
    const dialog = ui.take()
    if (dialog?.kind !== 'unsaved') throw new Error('no unsaved dialog')

    // Act
    const saving = dialog.save() as unknown as Promise<void>
    await vi.advanceTimersByTimeAsync(100) // the write has not finished: the tab must stay
    const waiting = app.tabs.length
    fake.emit('request', { node: 'r:A', dirty: false })
    await vi.advanceTimersByTimeAsync(100)
    await saving

    // Assert
    expect(fake.called('RequestService.Save', 'r:A')).toBe(true)
    expect(waiting).toBe(1)
    expect(app.tabs).toEqual([])
  })

  it('should keep the tab and say why when the save fails', async () => {
    // Arrange
    const { app, ui } = await openDirty()
    await app.requestClose('r:A')
    const dialog = ui.take()
    if (dialog?.kind !== 'unsaved') throw new Error('no unsaved dialog')

    // Act
    const saving = dialog.save() as unknown as Promise<void>
    await vi.advanceTimersByTimeAsync(10)
    fake.emit('request', { node: 'r:A', dirty: true, error: 'disk full' })
    await vi.advanceTimersByTimeAsync(100)
    await saving

    // Assert
    expect(app.tabs.map((t) => t.node)).toEqual(['r:A'])
    expect(ui.toasts.map((t) => [t.text, t.level])).toEqual([['Not saved: disk full. The tab stays open.', 'error']])
  })

  it('should give up waiting for a save that never reports and keep the tab', async () => {
    // Arrange
    const { app, ui } = await openDirty()
    await app.requestClose('r:A')
    const dialog = ui.take()
    if (dialog?.kind !== 'unsaved') throw new Error('no unsaved dialog')

    // Act
    const saving = dialog.save() as unknown as Promise<void>
    await vi.advanceTimersByTimeAsync(6000)
    await saving

    // Assert
    expect(app.tabs.map((t) => t.node)).toEqual(['r:A'])
    expect(ui.toasts[0]?.text).toBe('Not saved. The tab stays open so the edits are not lost.')
  })
})

describe('tabs by menu', () => {
  async function threeTabs() {
    const w = await freshWindow()
    fake.answer('RequestService.Open', (id: string) => ({ node: id }))
    fake.answer('FlowService.Open', (id: string) => ({ node: id }))
    fake.answer('WorkspaceService.Tree', () => tree(node('r:A', 'request'), node('f:B', 'flow'), node('r:C', 'request')))
    fake.answer('PanelsService.State', () => ({ log: [], problems: [], summary: '', history: [], opening: '' }))
    fake.answer('SessionService.Restore', () => [])
    fake.answer('SessionService.ShowWelcome', () => false)
    fake.answer('SettingsService.State', () => new Promise(() => {}))
    await w.app.init()
    w.app.tabs = []
    for (const [id, kind] of [['r:A', 'request'], ['f:B', 'flow'], ['r:C', 'request']]) await w.app.open(node(id, kind))
    return w
  }

  it('should wrap round at both ends with next and previous, and jump with tab-N, clamped to the last', async () => {
    // Arrange
    const { app } = await threeTabs()

    // Act
    app.onMenu('next-tab')
    const wrapped = app.active
    app.onMenu('previous-tab')
    const back = app.active
    app.onMenu('tab-2')
    const second = app.active
    app.onMenu('tab-9')

    // Assert
    expect(wrapped).toBe('r:A')
    expect(back).toBe('r:C')
    expect(second).toBe('f:B')
    expect(app.active).toBe('r:C')
  })

  it('should reopen the tab closed last, skipping one that is already open again, and none when the tree lost it', async () => {
    // Arrange
    const { app } = await threeTabs()
    await app.close('f:B')
    await app.close('r:A')

    // Act
    await app.open(node('r:A', 'request')) // reopened by hand: Ctrl+Shift+T must not open it twice
    await app.reopenClosedTab()
    const brought = app.tabs.map((t) => t.node)
    await app.reopenClosedTab() // nothing left to bring back

    // Assert
    expect(brought).toEqual(['r:C', 'r:A', 'f:B'])
    expect(app.tabs).toHaveLength(3)
  })
})

describe('Go events that reach the user', () => {
  it('should offer Undo for a delete, run it against the deleted ids and show a problem Go reports', async () => {
    // Arrange
    const { app, ui } = await freshWindow()
    quietStart()
    await app.init()
    fake.answer('TrashService.Undo', () => 'name taken')

    // Act
    fake.emit('deleted', { text: 'Deleted Login', ids: ['r:A', 'r:C'] })
    const toast = ui.toasts[0]
    toast.action?.run()
    await vi.advanceTimersByTimeAsync(0)

    // Assert
    expect(toast.text).toBe('Deleted Login')
    expect(fake.callsTo('TrashService.Undo')).toEqual([[['r:A', 'r:C']]])
    expect(ui.toasts.map((t) => [t.text, t.level])).toContainEqual(['name taken', 'error'])
  })

  it('should keep the Undo toast eight seconds', async () => {
    // Arrange
    const { app, ui } = await freshWindow()
    quietStart()
    await app.init()
    fake.emit('deleted', { text: 'Deleted Login', ids: ['r:A'] })

    // Act
    await vi.advanceTimersByTimeAsync(7999)
    const before = ui.toasts.length
    await vi.advanceTimersByTimeAsync(1)

    // Assert
    expect(before).toBe(1)
    expect(ui.toasts).toEqual([])
  })

  it('should show a notice with its level', async () => {
    // Arrange
    const { app, ui } = await freshWindow()
    quietStart()
    await app.init()

    // Act
    fake.emit('notice', { text: 'Imported 3 requests', level: 'info' })

    // Assert
    expect(ui.toasts.map((t) => [t.text, t.level])).toEqual([['Imported 3 requests', 'info']])
  })
})

describe('the vault passphrase question', () => {
  it('should not open the dialog again while it shows, since Go repeats the question every two seconds', async () => {
    // Arrange
    const { app, ui } = await freshWindow()
    quietStart()
    await app.init()

    // Act
    fake.emit('passphrase', 'unlock')
    const shown = ui.dialog
    fake.emit('passphrase', 'unlock')

    // Assert
    expect(shown).toMatchObject({ kind: 'prompt', key: 'passphrase', title: 'Unlock the vault' })
    expect(ui.dialog).toBe(shown)
  })

  it('should tell Go the question was cancelled and say the secrets stay locked', async () => {
    // Arrange
    const { app, ui } = await freshWindow()
    quietStart()
    await app.init()
    fake.emit('passphrase', 'unlock')

    // Act
    ui.dismiss()

    // Assert
    expect(fake.called('PassphraseService.Cancel')).toBe(true)
    expect(ui.toasts[0]?.text).toContain('Secrets stay locked')
  })

  it('should ask for a new passphrase twice and ask again, with the reason, when the two differ', async () => {
    // Arrange
    const { app, ui } = await freshWindow()
    quietStart()
    await app.init()
    fake.emit('passphrase', 'create')
    const first = ui.take()
    if (first?.kind !== 'prompt') throw new Error('no prompt')
    first.done('alpha', '', '')
    const second = ui.take()
    if (second?.kind !== 'prompt') throw new Error('no second prompt')

    // Act
    second.done('beta', '', '')

    // Assert
    expect(second.title).toBe('Repeat the passphrase')
    expect(fake.called('PassphraseService.Answer')).toBe(false)
    expect(ui.dialog).toMatchObject({ title: 'Create the vault passphrase' })
    expect((ui.dialog as { label: string }).label).toContain('The two passphrases were not the same')
  })

  it('should hand a matching new passphrase to Go', async () => {
    // Arrange
    const { app, ui } = await freshWindow()
    quietStart()
    await app.init()
    fake.emit('passphrase', 'create')
    const first = ui.take()
    if (first?.kind !== 'prompt') throw new Error('no prompt')
    first.done('alpha', '', '')
    const second = ui.take()
    if (second?.kind !== 'prompt') throw new Error('no second prompt')

    // Act
    second.done('alpha', '', '')

    // Assert
    expect(fake.callsTo('PassphraseService.Answer')).toEqual([['alpha']])
  })
})

describe('quitting with unsaved work', () => {
  it('should tell Go this page is alive and offer to keep the work as a draft', async () => {
    // Arrange
    const { app, ui } = await freshWindow()
    quietStart()
    await app.init()

    // Act
    fake.emit('quit-requested', { environments: 1 })
    const dialog = ui.dialog
    if (dialog?.kind !== 'unsaved') throw new Error('no unsaved dialog')
    dialog.save()
    dialog.discard()

    // Assert
    expect(fake.emitted).toEqual([{ name: 'quit-ack', data: undefined }])
    expect(dialog.text).toContain('Changes to environments cannot be kept')
    expect(fake.called('SystemService.QuitKeeping')).toBe(true)
    expect(fake.called('SystemService.Quit')).toBe(true)
  })
})
