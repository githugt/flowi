// Two small helpers that were written out by hand in a dozen places, each slightly different.
// Before: 12 inline Math.min(Math.max(…)) copies, and three popovers that each kept themselves inside the window
// with their own arithmetic.

/** n held between lo and hi. When hi is below lo (an empty list) the result is lo. */
export function clamp(n: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(n, hi))
}

/**
 * Where a popup of `size` starts so that it stays inside a window of `room`, `margin` from the edges, when it was
 * asked to start at `at`. A popup bigger than the room starts at the margin.
 */
export function keepInside(at: number, size: number, room: number, margin = 8): number {
  return clamp(at, margin, room - size - margin)
}
