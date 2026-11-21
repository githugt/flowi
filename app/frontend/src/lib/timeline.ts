import type { TimelineBar } from './types'

/** One bar of the run timeline, placed as percentages of the run's length so it needs no pixel maths. */
export interface TimelineRow {
  step: string
  status: string
  left: number
  width: number
  /** "120 ms" or "1.4 s". */
  duration: string
  /** True when this step started before the one above it ended: it ran side by side with it. */
  overlaps: boolean
}

/** A readable length: milliseconds below a second, then seconds with one decimal, then minutes. */
export function formatSpan(ms: number): string {
  if (ms < 1) return '<1 ms'
  if (ms < 1000) return `${Math.round(ms)} ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(ms < 10_000 ? 1 : 0)} s`
  const m = Math.floor(ms / 60_000)
  return `${m} min ${Math.round((ms % 60_000) / 1000)} s`
}

/** The run's length: when the last step ended. */
export function runLength(bars: TimelineBar[]): number {
  return bars.reduce((end, b) => Math.max(end, b.endMs), 0)
}

/** The rows to draw, in the order given (the bridge sends them by start). A bar never gets thinner than half a percent. */
export function timelineRows(bars: TimelineBar[]): TimelineRow[] {
  const total = runLength(bars)
  if (total <= 0) return bars.map((b) => ({ step: b.step, status: b.status, left: 0, width: 100, duration: formatSpan(b.endMs - b.startMs), overlaps: false }))
  let lastEnd = -Infinity
  return bars.map((b) => {
    const overlaps = b.startMs < lastEnd
    lastEnd = Math.max(lastEnd, b.endMs)
    const left = (b.startMs / total) * 100
    const width = Math.max(0.5, ((b.endMs - b.startMs) / total) * 100)
    return { step: b.step, status: b.status, left: Math.min(left, 100 - 0.5), width, duration: formatSpan(b.endMs - b.startMs), overlaps }
  })
}

/** Where the time axis gets a tick: a round step (1, 2, 5 × a power of ten) that gives about four to six. */
export function axisTicks(totalMs: number): { at: number; label: string }[] {
  if (totalMs <= 0) return []
  const rough = totalMs / 5
  const pow = 10 ** Math.floor(Math.log10(rough))
  const step = [1, 2, 5, 10].map((m) => m * pow).find((s) => s >= rough) ?? rough
  const ticks: { at: number; label: string }[] = []
  for (let t = 0; t <= totalMs + 1e-9; t += step) ticks.push({ at: (t / totalMs) * 100, label: t === 0 ? '0' : formatSpan(t) })
  return ticks
}

/** The slowest step, as a share of the run: "fetch took 80% of the run". Null when nothing stands out. */
export function slowestStep(bars: TimelineBar[]): { step: string; share: number } | null {
  const total = runLength(bars)
  if (bars.length < 2 || total <= 0) return null
  const top = bars.reduce((a, b) => (b.endMs - b.startMs > a.endMs - a.startMs ? b : a))
  const share = Math.round(((top.endMs - top.startMs) / total) * 100)
  return share >= 50 ? { step: top.step, share } : null
}
