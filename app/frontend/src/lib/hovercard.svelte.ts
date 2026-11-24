// The card that shows when the pointer rests on a row of the sidebar (or the keyboard reaches it). One card for the
// whole sidebar: a row asks for it with `enter`, and it is fetched from Go (what the row is, what it talks to, what is
// wrong with it) once the pointer has stayed a moment, so a pass across the list opens nothing.
// Before: a row's hover text was its own name again.
import { WorkspaceService } from '../../bindings/github.com/githugt/flowi/app/internal/bridge'
import type { HoverCardView } from './types'

/** How long the pointer rests on a row before the card is asked for. */
export const HOVER_DELAY_MS = 450

class HoverState {
  card = $state<HoverCardView | null>(null)
  /** Where the card goes: left edge, and the top of the row it is about. */
  anchor = $state({ x: 0, top: 0 })
  #timer: ReturnType<typeof setTimeout> | undefined
  #row = ''

  /** The pointer or the keyboard reached `row`, the element of tree row `id`. */
  enter(row: HTMLElement, id: string) {
    this.leave()
    this.#row = id
    this.#timer = setTimeout(() => void this.#show(row, id), HOVER_DELAY_MS)
  }

  async #show(row: HTMLElement, id: string) {
    try {
      const card = (await WorkspaceService.Hover(id)) as unknown as HoverCardView
      if (this.#row !== id || !card.title) return // the pointer moved on while it was being fetched
      const r = row.getBoundingClientRect()
      const side = row.closest('aside')?.getBoundingClientRect()
      this.anchor = { x: (side?.right ?? r.right) + 6, top: r.top }
      this.card = card
    } catch {
      // no card is better than an error for a hint
    }
  }

  /** The pointer left, a click or a drag began, or the list scrolled: the card goes. */
  leave() {
    clearTimeout(this.#timer)
    this.#row = ''
    this.card = null
  }
}

export const hover = new HoverState()
