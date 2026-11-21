// The right-click menu of a JSON tree row: copy its value, copy its path.
// Before: a node of a response or message tree could be picked but not copied, so a value was copied by selecting text.

/** The text "Copy value" puts on the clipboard: a string as it is, anything else as indented JSON. */
export function valueText(value: unknown): string {
  return typeof value === 'string' ? value : (JSON.stringify(value, null, 2) ?? String(value))
}

/** The toast name for a copied value: "the value of $.a.b", shortened for a long path. */
export function valueName(path: string): string {
  return path.length > 40 ? `the value at …${path.slice(-30)}` : `the value of ${path}`
}
