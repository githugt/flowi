import { mainItems } from './mainmenu'

/** The canvas gestures, which have no menu item. Before, only a toast that vanished explained them. */
export const CANVAS_GESTURES = [
  'Drag a step onto a line or between steps: move it there',
  'Drag a step into empty space: place it freely',
  'Drag from a port to a step: add a connection',
  'Shift-click: select several steps',
  'Delete: remove the selected steps',
  'Double-click a strip: fold or unfold it',
  'Pinch or Ctrl+scroll: zoom',
]

/** The text of the shortcut sheet: the menu shortcuts, tab keys, then the canvas gestures. */
export function shortcutSheet(mac: boolean): string {
  const key = (s: string) => (mac ? s.replace('Ctrl+Shift+', '⇧⌘').replace('Ctrl+', '⌘') : s)
  const lines = mainItems.filter((i) => i.shortcut).map((i) => `${key(i.shortcut!)}   ${i.label}`)
  lines.push(`${mac ? '⌘' : 'Ctrl+'}Enter   Send the request`)
  lines.push(`${mac ? '⌘' : 'Ctrl+'}1 … 9   Go to the first to ninth tab`)
  lines.push('Ctrl+Tab, Ctrl+Shift+Tab   Next and previous tab')
  lines.push(`${mac ? '⌥⌘' : 'Ctrl+Alt+'}S   Save every open document`)
  lines.push(`${mac ? '⌘' : 'Ctrl+'}B   Fold or unfold the sidebar`)
  lines.push(`${mac ? '⌘' : 'Ctrl+'}+ , − , 0   Zoom the window in, out, back to normal`)
  lines.push('?   This list')
  return [lines.join('\n'), 'On the flow canvas', CANVAS_GESTURES.map((g) => '· ' + g).join('\n')].join('\n\n')
}

/** What a key press means for the tabs and the sheet, or '' when it is none of them. `typing` is true in a text field. */
export function windowKeyAction(e: { key: string; code?: string; ctrlKey: boolean; metaKey: boolean; altKey: boolean; shiftKey: boolean }, typing: boolean): string {
  // Alt changes the letter a key types on a Mac, so Save all is matched by the key's place.
  if ((e.ctrlKey || e.metaKey) && e.altKey && !e.shiftKey && (e.code === 'KeyS' || e.key.toLowerCase() === 's')) return 'save-all'
  // Before: tabs cycled only with Ctrl+Shift+] and [, which need AltGr on many keyboard layouts; Ctrl+Tab and Ctrl+1..9 did nothing.
  if (e.ctrlKey && !e.altKey && !e.metaKey && e.key === 'Tab') return e.shiftKey ? 'previous-tab' : 'next-tab'
  if ((e.ctrlKey || e.metaKey) && !e.altKey && !e.shiftKey && /^[1-9]$/.test(e.key)) return 'tab-' + e.key
  if ((e.ctrlKey || e.metaKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === 'b') return 'toggle-sidebar'
  if (!typing && !e.ctrlKey && !e.metaKey && !e.altKey && e.key === '?') return 'shortcuts'
  return ''
}
