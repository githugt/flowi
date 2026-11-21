import { clamp as hold } from './clamp'

/** Window zoom: Ctrl or Cmd with + (or =), - and 0. */
export const ZOOM_MIN = 0.7
export const ZOOM_MAX = 1.6
const STEP = 0.1
const KEY = 'flowi.zoom'

const clamp = (z: number) => hold(Math.round(z * 100) / 100, ZOOM_MIN, ZOOM_MAX)

/** The zoom after a key press, or null when the key is not a zoom key. */
export function zoomForKey(e: { key: string; ctrlKey: boolean; metaKey: boolean; altKey?: boolean }, current: number): number | null {
  if ((!e.ctrlKey && !e.metaKey) || e.altKey) return null
  switch (e.key) {
    case '+':
    case '=':
      return clamp(current + STEP)
    case '-':
    case '_':
      return clamp(current - STEP)
    case '0':
      return 1
    default:
      return null
  }
}

/** The zoom kept from the last session, 1 when none. */
export function loadZoom(): number {
  try {
    const z = Number(localStorage.getItem(KEY))
    return Number.isFinite(z) && z >= ZOOM_MIN && z <= ZOOM_MAX ? z : 1
  } catch {
    return 1
  }
}

export function saveZoom(z: number) {
  try {
    localStorage.setItem(KEY, String(z))
  } catch {
    // storage may be blocked: the zoom then lasts for this session only
  }
}
