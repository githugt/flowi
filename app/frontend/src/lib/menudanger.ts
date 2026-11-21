// A menu item that destroys something: its label starts with one of these words.
// Before: "Delete…" looked the same as "Rename…" in every context menu.
const DANGER = /^(delete|remove|discard|empty trash|move to trash|clear all|reset)\b/i

export function isDangerLabel(label: string): boolean {
  return DANGER.test(label.trim())
}
