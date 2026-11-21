// Whether the window is waiting on Go, and for how long: the pure part of the busy bar (Busy.svelte, busy.svelte.ts).
// Before: one slow call on Go's serial queue left every click waiting with nothing on screen to say so.

/** A call that has not come back yet. */
export interface Pending {
  id: number
  /** What the window asked: "FlowService.Open" when the runtime names it, else "method 1238036098". */
  label: string
  /** Milliseconds on the window's clock when it was sent. */
  since: number
}

/** After this long a thin activity bar shows. A call that answers at once never flickers one. */
export const BAR_AFTER_MS = 400
/** After this long the window says it is still waiting and offers a diagnostics copy. */
export const STUCK_AFTER_MS = 5000

export type BusyLevel = 'idle' | 'working' | 'stuck'

export interface BusyState {
  level: BusyLevel
  /** How many calls are waiting. */
  count: number
  /** How long the oldest has waited, in whole seconds. */
  seconds: number
  /** The oldest call's label, or ''. */
  oldest: string
}

export function busyState(pending: readonly Pending[], now: number): BusyState {
  let oldest: Pending | undefined
  for (const p of pending) if (!oldest || p.since < oldest.since) oldest = p
  const age = oldest ? Math.max(0, now - oldest.since) : 0
  const level: BusyLevel = age >= STUCK_AFTER_MS ? 'stuck' : age >= BAR_AFTER_MS ? 'working' : 'idle'
  return { level, count: pending.length, seconds: Math.floor(age / 1000), oldest: oldest?.label ?? '' }
}

/** The calls that wait, oldest first. */
export class PendingCalls {
  private next = 1
  private calls = new Map<number, Pending>()

  /** Notes a call as sent; the function it returns notes it as answered (once). */
  begin(label: string, now: number): () => void {
    const id = this.next++
    this.calls.set(id, { id, label, since: now })
    return () => void this.calls.delete(id)
  }

  list(): Pending[] {
    return [...this.calls.values()].sort((a, b) => a.since - b.since)
  }
}

/**
 * What a runtime request is, when it is a call to a bound Go method: its label. Anything else (an event, the
 * clipboard, a call being cancelled) is null, because only a call can wait on Go's queue.
 *
 * The body is {"object":0,"method":0,"args":{"call-id":…,"methodID":123,"args":[…]}} and only its start is read, so a
 * large request body is not parsed to be counted.
 */
export function callLabel(body: unknown): string | null {
  if (typeof body !== 'string') return null
  const head = body.slice(0, 300)
  if (!/^\{"object":0,"method":0[,}]/.test(head)) return null
  const name = /"methodName":"([^"]+)"/.exec(head)
  if (name) return name[1]
  const id = /"methodID":(\d+)/.exec(head)
  return id ? `method ${id[1]}` : 'a call'
}

/** The text "Copy diagnostics" puts on the clipboard: what the window can say when Go cannot be asked. */
export function diagnosticsText(pending: readonly Pending[], now: number, extra = ''): string {
  const s = busyState(pending, now)
  const lines = [
    `Flowi window is waiting on Go: ${s.count} call${s.count === 1 ? '' : 's'}, the oldest ${s.seconds} s (${s.oldest || 'none'}).`,
    ...[...pending].sort((a, b) => a.since - b.since).slice(0, 20).map((p) => `  ${p.label}: waiting ${Math.max(0, Math.round((now - p.since) / 1000))} s`),
  ]
  if (extra) lines.push('', extra)
  return lines.join('\n')
}

type FetchFn = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>

/**
 * `real` with every Go call counted while it waits. The runtime sends a call as a POST to /wails/runtime with the
 * global fetch, so wrapping that is the one place all calls pass, and the generated bindings stay as they are.
 */
export function countingFetch(real: FetchFn, begin: (label: string) => () => void): FetchFn {
  return (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const label = init?.method === 'POST' && url.endsWith('/wails/runtime') ? callLabel(init.body) : null
    if (label === null) return real(input, init)
    const end = begin(label)
    return real(input, init).finally(end)
  }
}
