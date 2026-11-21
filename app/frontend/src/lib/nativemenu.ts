/**
 * Whether a right-click on `target` should open the platform's own menu: in a field (Cut, Copy, Paste) and on selected
 * text (Copy). Anywhere else the app has its own context menu or none, and the web view's (Reload, Inspect Element,
 * and on Linux a GTK warning about its scrollbar) has no place in it.
 */
export function keepsNativeMenu(target: EventTarget | null, selection: string = window.getSelection?.()?.toString() ?? ''): boolean {
  if (selection.trim() !== '') return true
  return target instanceof Element && target.closest('input, textarea, select, [contenteditable]:not([contenteditable="false"])') !== null
}
