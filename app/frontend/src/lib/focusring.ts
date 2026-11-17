// The focus ring is for keyboard users. When the window comes back from another app, the browser gives focus
// back to whatever had it and draws the ring as if it had been reached by keyboard: a tab that was clicked
// showed a ring after Alt+Tab. So a control that was reached with the pointer lets go of focus when the window
// loses it. A text field keeps it (the caret should be where it was), and so does anything reached by keyboard.

const MODIFIERS = ['Alt', 'Control', 'Meta', 'Shift']
const TEXT = 'input, textarea, select, [contenteditable="true"], .cm-content'

export function dropPointerFocusOnBlur(win: Window = window) {
  let byPointer = false
  win.addEventListener('pointerdown', () => (byPointer = true), true)
  // Alt on its way to Alt+Tab is not keyboard navigation.
  win.addEventListener('keydown', (e) => !MODIFIERS.includes(e.key) && (byPointer = false), true)
  win.addEventListener('blur', () => {
    const el = win.document.activeElement
    if (byPointer && el && 'blur' in el && !el.matches(TEXT)) (el as HTMLElement).blur()
  })
}
