// The arrow keys in a list of choices (the palette, pickers, move and place dialogs), one rule for all of them.
// Before: the palette and the Move and Place dialogs stopped at the ends while the method picker wrapped round.

/**
 * The index an arrow key moves to, or null when the key is not Up or Down. Both wrap round the ends; from nothing
 * chosen (-1) Down goes to the first row and Up to the last. An empty list stays at 0. Home and End are left to the
 * search field these lists sit under: there they move the caret.
 */
export function listKeyTarget(key: string, index: number, length: number): number | null {
  if (key !== 'ArrowDown' && key !== 'ArrowUp') return null
  if (length <= 0) return 0
  if (key === 'ArrowDown') return index < 0 ? 0 : (index + 1) % length
  return index < 0 ? length - 1 : (index - 1 + length) % length
}
