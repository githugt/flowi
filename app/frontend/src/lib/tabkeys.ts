// The arrow keys of a horizontal tab bar. Left and Right move (wrapping round), Home and End jump to the ends; Up and
// Down are left to the page, which scrolls with them.
import { radioKeyTarget } from './radiogroup'

/** The index a key moves to among `length` tabs, or null when the key is not for the bar. */
export function tabKeyTarget(key: string, index: number, length: number): number | null {
  if (index < 0 || key === 'ArrowUp' || key === 'ArrowDown') return null
  return radioKeyTarget(key, index, length)
}
