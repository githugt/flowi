// How big a port is to hit. The ports are 12px circles and the + beside them 16px: under the 24px a pointer target
// should have. An invisible circle of this radius sits behind each, so the target is 24px with the look unchanged.
// Before: a port had to be hit to the pixel.

/** The radius of the invisible target behind a port or a +: 24px across. Ports are 24 apart, so neighbours just touch. */
export const PORT_HIT = 12

/** True for the ports a link can start from: done, fail and a named outcome. */
export function startsLink(portId: string): boolean {
  return portId === 'done' || portId === 'fail' || portId.startsWith('outcome-')
}
