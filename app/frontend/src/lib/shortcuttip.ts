// A tooltip that names its shortcut: "Undo: delete 3 steps (Ctrl+Z)". The shortcut is written for the platform.
// Before: Undo and Redo were text buttons with no icon and no hint of the keys.

/** `keys` as written for Windows and Linux ("Ctrl+Shift+Z"); on a Mac the modifiers become ⇧ and ⌘. */
export function withShortcut(label: string, keys: string, mac: boolean): string {
  const shown = mac ? keys.replace('Ctrl+Shift+', '⇧⌘').replace('Ctrl+', '⌘') : keys
  return `${label} (${shown})`
}
