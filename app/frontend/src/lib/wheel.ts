// What a wheel event on the canvas means, whatever the device sent.
// Before: Ctrl+wheel always zoomed by exp(-dy/100), which is x0.37 or x2.7 for one notch of a Chromium mouse wheel and
// about 3% in Firefox's line mode; a plain mouse wheel could only pan up and down, and Shift+wheel did nothing.

export interface WheelLike {
  ctrlKey: boolean
  shiftKey: boolean
  deltaX: number
  deltaY: number
  /** 0 pixels, 1 lines, 2 pages. */
  deltaMode: number
}

export type WheelAction = { kind: 'zoom'; factor: number } | { kind: 'pan'; dx: number; dy: number }

/** The size of one line of a wheel in lines mode, in pixels. */
const LINE = 16
/** A zoom step for one notch of a mouse wheel. */
export const NOTCH = 1.1

/** A pinch on a trackpad comes as small, fractional deltas with Ctrl set; a mouse notch is large or in lines. */
const isNotch = (e: WheelLike) => e.deltaMode !== 0 || (Number.isInteger(e.deltaY) && Math.abs(e.deltaY) >= 50)

export function wheelAction(e: WheelLike): WheelAction {
  if (e.ctrlKey) {
    if (isNotch(e)) return { kind: 'zoom', factor: e.deltaY < 0 ? NOTCH : 1 / NOTCH }
    return { kind: 'zoom', factor: Math.exp(-e.deltaY * 0.01) }
  }
  const unit = e.deltaMode === 1 ? LINE : e.deltaMode === 2 ? 600 : 1
  let dx = e.deltaX * unit
  let dy = e.deltaY * unit
  // Shift turns a vertical wheel into a horizontal pan, as in most editors.
  if (e.shiftKey && dx === 0) [dx, dy] = [dy, 0]
  return { kind: 'pan', dx, dy }
}
