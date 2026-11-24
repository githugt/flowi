// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fake } from './fakewails'

vi.mock('@wailsio/runtime', async () => (await import('./fakewails')).fake.runtime())
vi.mock('../../bindings/github.com/githugt/flowi/app/internal/bridge', async () => (await import('./fakewails')).fake.bridge())

// The card of a sidebar row, asked of Go once the pointer has rested.
// Before: the hover text was the row's own name.

const card = (title: string) => ({ title, kind: 'HTTP request', facts: [], problems: [] })
const row = (): HTMLElement => {
  const aside = document.createElement('aside')
  const el = document.createElement('div')
  aside.appendChild(el)
  document.body.appendChild(aside)
  return el
}

async function freshHover() {
  vi.resetModules()
  const { hover, HOVER_DELAY_MS } = await import('./hovercard.svelte')
  return { hover, HOVER_DELAY_MS }
}

beforeEach(() => {
  fake.reset()
  vi.useFakeTimers()
})
afterEach(() => {
  vi.useRealTimers()
  document.body.innerHTML = ''
})

describe('hover card', () => {
  it('asks Go for the card only after the pointer has rested on the row', async () => {
    const { hover, HOVER_DELAY_MS } = await freshHover()
    fake.answer('WorkspaceService.Hover', (id: string) => card(id))

    hover.enter(row(), 'r:A')
    await vi.advanceTimersByTimeAsync(HOVER_DELAY_MS - 1)
    const early = fake.callsTo('WorkspaceService.Hover').length
    await vi.advanceTimersByTimeAsync(1)

    expect(early).toBe(0)
    expect(fake.callsTo('WorkspaceService.Hover')).toEqual([['r:A']])
    expect(hover.card?.title).toBe('r:A')
  })

  it('asks for nothing when the pointer passes over the row', async () => {
    const { hover, HOVER_DELAY_MS } = await freshHover()

    hover.enter(row(), 'r:A')
    hover.leave()
    await vi.advanceTimersByTimeAsync(HOVER_DELAY_MS * 2)

    expect(fake.callsTo('WorkspaceService.Hover')).toEqual([])
    expect(hover.card).toBeNull()
  })

  it('drops a card that arrives after the pointer has moved to another row', async () => {
    const { hover, HOVER_DELAY_MS } = await freshHover()
    const slow = fake.deferred<ReturnType<typeof card>>()
    fake.answer('WorkspaceService.Hover', (id: string) => (id === 'r:A' ? slow.promise : card(id)))

    hover.enter(row(), 'r:A')
    await vi.advanceTimersByTimeAsync(HOVER_DELAY_MS)
    hover.enter(row(), 'r:B')
    await vi.advanceTimersByTimeAsync(HOVER_DELAY_MS)
    slow.resolve(card('r:A'))
    await vi.advanceTimersByTimeAsync(0)

    expect(hover.card?.title).toBe('r:B')
  })

  it('shows no card, and no error, when Go has none for the row', async () => {
    const { hover, HOVER_DELAY_MS } = await freshHover()
    fake.answer('WorkspaceService.Hover', () => card(''))

    hover.enter(row(), 'gone')
    await vi.advanceTimersByTimeAsync(HOVER_DELAY_MS)

    expect(hover.card).toBeNull()
  })

  it('takes the card away on leave', async () => {
    const { hover, HOVER_DELAY_MS } = await freshHover()
    fake.answer('WorkspaceService.Hover', (id: string) => card(id))
    hover.enter(row(), 'r:A')
    await vi.advanceTimersByTimeAsync(HOVER_DELAY_MS)

    hover.leave()

    expect(hover.card).toBeNull()
  })
})
