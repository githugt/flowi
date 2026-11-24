import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fake } from './fakewails'

vi.mock('@wailsio/runtime', async () => (await import('./fakewails')).fake.runtime())
vi.mock('../../bindings/github.com/githugt/flowi/app/internal/bridge', async () => (await import('./fakewails')).fake.bridge())

// What the window does with the sidebar's memory (stars, recent): load it, follow Go's event, and tell Go what was opened.
// Before: there was no Recent tab and no stars.

const node = (id: string, kind: string, label = id) => ({ id, kind, label, children: [] })

async function freshWindow() {
  vi.resetModules()
  const { app } = await import('./app.svelte')
  return { app }
}

beforeEach(() => {
  fake.reset()
  vi.useFakeTimers()
  fake.answer('PanelsService.State', () => ({ log: [], problems: [], summary: '', history: [], opening: '' }))
  fake.answer('WorkspaceService.Tree', () => ({ roots: [node('r:A', 'request', 'Login'), node('f:B', 'flow', 'Checkout'), node('e:1', 'environment', 'staging')], environments: [], selected: '' }))
  fake.answer('WorkspaceService.Sidebar', () => ({ favourites: ['c:X'], recent: [{ id: 'r:A', label: 'Login', kind: 'request' }] }))
  fake.answer('SessionService.Restore', () => ['r:A'])
  fake.answer('SessionService.ShowWelcome', () => false)
  fake.answer('SettingsService.State', () => new Promise(() => {}))
  fake.answer('TrashService.Sweep', () => undefined)
  fake.answer('RequestService.Open', (id: string) => ({ node: id }))
  fake.answer('FlowService.Open', (id: string) => ({ node: id }))
  fake.answer('PassphraseService.Unlock', () => undefined)
  fake.answer('EnvironmentService.Open', (id: string) => ({ node: id }))
})
afterEach(() => vi.useRealTimers())

describe('the sidebar memory in the window', () => {
  it('loads the stars and the recent list at the start, and follows the sidebar event', async () => {
    const { app } = await freshWindow()

    await app.init()
    const loaded = JSON.stringify(app.sidebar)
    fake.emit('sidebar', { favourites: [], recent: [] })

    expect(loaded).toContain('c:X')
    expect(app.sidebar.favourites).toEqual([])
  })

  it('tells Go about a request or flow the person opens, and not about the tabs a restored session brings back', async () => {
    const { app } = await freshWindow()
    await app.init()
    fake.calls = [] // what the restore did is not under test

    await app.open(app.find('f:B')!)

    expect(fake.callsTo('WorkspaceService.NoteOpened')).toEqual([['f:B']])
  })

  it('does not count a restored tab as opened', async () => {
    const { app } = await freshWindow()

    await app.init()

    expect(fake.callsTo('WorkspaceService.NoteOpened')).toEqual([])
  })

  it('does not count an environment as opened', async () => {
    const { app } = await freshWindow()
    await app.init()

    await app.open(app.find('e:1')!)

    expect(fake.callsTo('WorkspaceService.NoteOpened')).toEqual([])
  })
})
