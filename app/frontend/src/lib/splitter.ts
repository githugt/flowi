// The keyboard half of a splitter: arrows move the handle, Shift moves it faster, Home and End go to the limits.
// Before: the handle could only be dragged, so a pane could not be resized without a pointer.

export const STEP = 16
export const BIG_STEP = 64

/**
 * The width after a key, or null when the key is not one the splitter takes. `side` is which side of the handle the
 * resized pane is on: for a pane on the left, the Right arrow makes it wider; for a pane on the right, the Left arrow does.
 * With `axis` y the handle is horizontal and the pane under it: Up makes it taller.
 */
export function widthAfterKey(width: number, key: string, shift: boolean, side: 'left' | 'right', min: number, max: number, axis: 'x' | 'y' = 'x'): number | null {
  const step = shift ? BIG_STEP : STEP
  // A pane below its handle (axis y) grows when the handle goes up.
  const grow = axis === 'y' ? 'ArrowUp' : side === 'left' ? 'ArrowRight' : 'ArrowLeft'
  const shrink = axis === 'y' ? 'ArrowDown' : side === 'left' ? 'ArrowLeft' : 'ArrowRight'
  let next: number
  if (key === grow) next = width + step
  else if (key === shrink) next = width - step
  else if (key === 'Home') next = min
  else if (key === 'End') next = max
  else return null
  return Math.min(max, Math.max(min, next))
}

/** How far past the minimum a drag has to go before it means "fold the pane" (px). */
export const SNAP = 60

/** True when a drag has gone far enough below the minimum that letting go should fold the pane. */
export function wantsCollapse(raw: number, min: number, snap = SNAP): boolean {
  return raw < min - snap
}
