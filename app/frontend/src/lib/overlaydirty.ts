// Whether the overlay in the editor holds edits that are not saved: its text differs from what was read or saved, or
// a new overlay has been given a name.
// Before: opening another overlay (or "+ New overlay") threw away the text typed, with no word and no mark.
export function overlayDirty(editing: boolean, isNew: boolean, name: string, text: string, saved: string): boolean {
  if (!editing) return false
  return text !== saved || (isNew && name.trim() !== '')
}
