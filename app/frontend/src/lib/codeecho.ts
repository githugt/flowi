// Whether text that came back from the parent is already in the editor, without turning the document into a string.
// Before: each keystroke stringified the document for onchange, and the echo effect stringified it again to compare,
// so a 1 MB body was copied twice per key.

/**
 * True when `next` is what the editor already shows. `lastSeen` is the text the editor last reported (or was last set
 * to); when it equals `next` the document is the same and `current` is not called.
 */
export function alreadyShown(lastSeen: string | null, next: string, current: () => string): boolean {
  if (lastSeen !== null && lastSeen === next) return true
  return current() === next
}
