// The window's one view of "is Go busy": it counts the calls that wait and re-reads the clock while any does.
import { untrack } from 'svelte'
import { PendingCalls, busyState, countingFetch, diagnosticsText, type BusyState } from './busy'

const IDLE: BusyState = { level: 'idle', count: 0, seconds: 0, oldest: '' }

class Busy {
  state = $state<BusyState>(IDLE)
  private calls = new PendingCalls()
  private timer: ReturnType<typeof setInterval> | undefined

  /** Notes a call as sent; the returned function notes it as answered. */
  begin = (label: string): (() => void) => {
    const end = this.calls.begin(label, performance.now())
    this.refresh()
    this.timer ??= setInterval(() => this.refresh(), 250)
    return () => {
      end()
      this.refresh()
    }
  }

  // Untracked: noting a call reads and writes `state`, and many screens ask Go from inside an $effect. Tracked, that
  // effect would depend on `state`, which the call it makes changes, so it ran again, asked again, and Svelte stopped
  // it with effect_update_depth_exceeded.
  private refresh() {
    untrack(() => this.update())
  }

  private update() {
    const list = this.calls.list()
    if (list.length === 0 && this.timer) {
      clearInterval(this.timer)
      this.timer = undefined
    }
    const next = busyState(list, performance.now())
    const s = this.state
    if (next.level !== s.level || next.count !== s.count || next.seconds !== s.seconds || next.oldest !== s.oldest) this.state = next
  }

  /** What the window knows about its waiting calls, for a bug report. */
  diagnostics(extra = ''): string {
    return diagnosticsText(this.calls.list(), performance.now(), extra)
  }

  /** Counts every call to Go made through `win.fetch` from now on. Once. */
  install(win: { fetch: typeof fetch }) {
    win.fetch = countingFetch(win.fetch.bind(win), this.begin)
  }
}

export const busy = new Busy()
