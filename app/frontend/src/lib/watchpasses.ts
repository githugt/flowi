import type { WatchPass } from './types'

/** How the latest pass went, in a word: what the row says next to the sparkline. */
export function lastPassText(history: WatchPass[]): { text: string; tone: 'none' | 'ok' | 'changed' | 'bad' } {
  const last = history[history.length - 1]
  if (!last) return { text: 'No pass yet', tone: 'none' }
  if (last.status !== 'passed') return { text: `Run ${last.status}`, tone: 'bad' }
  if (last.changes > 0) return { text: last.changes === 1 ? '1 change' : `${last.changes} changes`, tone: 'changed' }
  return { text: 'No change', tone: 'ok' }
}

/** The points of a sparkline of the calls' total latency: x spreads the passes over the width, y is the height from the
 *  bottom, scaled so the slowest pass touches the top. A single pass is a flat line in the middle. */
export function sparkPoints(history: WatchPass[], width: number, height: number): { x: number; y: number; changed: boolean }[] {
  if (!history.length) return []
  const slowest = Math.max(...history.map((p) => p.ms), 1)
  return history.map((p, i) => ({
    x: history.length === 1 ? width / 2 : (i / (history.length - 1)) * width,
    y: history.length === 1 ? height / 2 : height - (p.ms / slowest) * height,
    changed: p.status === 'passed' && p.changes > 0,
  }))
}
