// A rectangle that can be hit: a unit on the canvas.
export interface Box {
  x: number
  y: number
  w: number
  h: number
}

// innermost returns the smallest box that holds the point. Boxes nest (a step sits inside the lane of a loop, route
// or group), so the first match in list order is often the container.
// Before: the canvas took the first unit in list order, so a wire released over a step inside a lane connected to the
// container.
export function innermost<T extends Box>(boxes: readonly T[], x: number, y: number): T | undefined {
  let best: T | undefined
  for (const b of boxes) {
    if (x < b.x || x > b.x + b.w || y < b.y || y > b.y + b.h) continue
    if (!best || b.w * b.h < best.w * best.h) best = b
  }
  return best
}
