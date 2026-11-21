/** The window title: what is open, then the product. A tab with unsaved edits starts with a dot. Before it was always "Flowi". */
export function windowTitle(label: string, dirty: boolean): string {
  const name = label.trim()
  if (!name) return 'Flowi'
  return `${dirty ? '● ' : ''}${name} — Flowi`
}
