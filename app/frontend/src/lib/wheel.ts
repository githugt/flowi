// What a wheel event on the canvas means, whatever the device sent.
// Before: Ctrl+wheel always zoomed by exp(-dy/100), which is x0.37 or x2.7 for one notch of a Chromium mouse wheel and
// about 3% in Firefox's line mode; a plain mouse wheel could only pan up and down, and Shift+wheel did nothing.
// Now a plain mouse wheel zooms by one step per notch and Ctrl+wheel pans.

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

/** Two events this close are one gesture (a spin of the wheel, a flick of the trackpad), in milliseconds. */
const BURST = 120

export interface WheelEventLike extends WheelLike {
  timeStamp: number
}

/**
 * Reads wheel events for the canvas: a mouse wheel zooms and Ctrl+wheel scrolls, as in most diagram editors. A trackpad
 * keeps its own gestures: a pinch (Ctrl+wheel with small deltas) zooms and a two-finger scroll pans.
 *
 * A notch of a mouse wheel is told from a fast flick of a trackpad (which can also be large whole numbers) by the
 * events around it: a wheel sends the same amount every time, a trackpad a ramp that varies. The amount itself differs
 * by system (53, 100, 120), so it is not matched to a number.
 */
export function wheelReader(): (e: WheelEventLike) => WheelAction {
  let last: { dy: number; at: number } | null = null
  return (e) => {
    const same = last !== null && e.timeStamp - last.at <= BURST && Math.abs(last.dy) === Math.abs(e.deltaY)
    const fresh = last === null || e.timeStamp - last.at > BURST
    last = { dy: e.deltaY, at: e.timeStamp }
    const wheel = e.deltaMode !== 0 || (e.deltaX === 0 && isNotch(e) && (fresh || same))
    if (wheel && !e.ctrlKey && !e.shiftKey) return { kind: 'zoom', factor: e.deltaY < 0 ? NOTCH : 1 / NOTCH }
    if (e.ctrlKey && !wheel) return { kind: 'zoom', factor: Math.exp(-e.deltaY * 0.01) }
    const unit = e.deltaMode === 1 ? LINE : e.deltaMode === 2 ? 600 : 1
    let dx = e.deltaX * unit
    let dy = e.deltaY * unit
    // Shift turns a vertical wheel into a horizontal pan, as in most editors.
    if (e.shiftKey && dx === 0) [dx, dy] = [dy, 0]
    return { kind: 'pan', dx, dy }
  }
}
