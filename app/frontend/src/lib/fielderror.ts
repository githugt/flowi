// What a field says about a value that cannot be used: the message sits next to the field, is named by the field
// (aria-describedby) and is announced once (role="alert"), and the colour is not the only sign.
// Before: a field turned its border red and nothing said why, to a screen reader or to anyone who cannot tell red.

let next = 0

/** A fresh id for the message of one field. */
export function errorId(): string {
  next++
  return `field-error-${next}`
}

/** The aria-describedby of a field: the ids it already had and the id of its error message, when there is one. */
export function describedBy(existing: string | undefined, error: string | undefined, id: string): string | undefined {
  const ids = [existing, error ? id : ''].filter(Boolean).join(' ')
  return ids || undefined
}
