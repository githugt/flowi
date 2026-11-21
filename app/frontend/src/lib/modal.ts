// What makes a dialog a dialog for a keyboard and a screen reader: it says so (role and aria-modal, named by its
// heading), focus goes into it and stays there, and focus returns to where it was when the dialog goes. While one is
// open the rest of the window ignores the keyboard.
// Before: dialogs had no role, Tab walked out of them into the page behind, closing one left focus nowhere, and the
// canvas's Delete key still worked behind an open dialog.

/** The attribute the action sets on the layer it manages, so the rest of the window can tell a dialog is open. */
export const OPEN = 'data-modal'

/** Whether a dialog is open anywhere in the window. Global key handlers return early when it is. */
export function modalOpen(root: Pick<ParentNode, 'querySelector'> = document): boolean {
  return root.querySelector(`[${OPEN}]`) !== null
}

/** The index Tab or Shift+Tab goes to among `count` tabbable elements, wrapping round; null when there are none. */
export function nextTab(index: number, count: number, shift: boolean): number | null {
  if (count <= 0) return null
  if (index < 0) return shift ? count - 1 : 0
  return shift ? (index - 1 + count) % count : (index + 1) % count
}

const TABBABLE = 'a[href], button:not(:disabled), input:not(:disabled):not([type="hidden"]), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])'

const tabbables = (root: HTMLElement) =>
  Array.from(root.querySelectorAll<HTMLElement>(TABBABLE)).filter((el) => el.tabIndex >= 0 && el.getClientRects().length > 0)

let counter = 0

/**
 * Svelte action for a dialog. Put it on the layer behind the dialog (the scrim) when the dialog is inside it, or on the
 * dialog itself when it stands alone. It names the dialog, traps Tab inside it, moves focus in when nothing inside has
 * it already, and gives focus back on the way out.
 */
export function modal(node: HTMLElement) {
  const box = node.matches('[role="dialog"], [role="alertdialog"], .dialog') ? node : (node.querySelector<HTMLElement>('[role="dialog"], [role="alertdialog"], .dialog') ?? node)
  const before = document.activeElement instanceof HTMLElement ? document.activeElement : null
  node.setAttribute(OPEN, '')
  if (!box.getAttribute('role')) box.setAttribute('role', 'dialog')
  box.setAttribute('aria-modal', 'true')
  const heading = box.querySelector<HTMLElement>('h1, h2, h3')
  if (heading && !box.hasAttribute('aria-label') && !box.hasAttribute('aria-labelledby')) {
    heading.id ||= `modal-title-${++counter}`
    box.setAttribute('aria-labelledby', heading.id)
  }
  // Once the dialog has drawn (and any field it focuses itself has), move in if focus is still outside.
  const timer = setTimeout(() => {
    if (box.contains(document.activeElement)) return
    if (!box.hasAttribute('tabindex')) box.setAttribute('tabindex', '-1')
    ;(tabbables(box)[0] ?? box).focus()
  }, 0)
  const keys = (e: KeyboardEvent) => {
    if (e.key !== 'Tab' || e.defaultPrevented) return
    const list = tabbables(box)
    const to = nextTab(list.indexOf(document.activeElement as HTMLElement), list.length, e.shiftKey)
    if (to === null) {
      e.preventDefault()
      box.focus()
      return
    }
    const at = list.indexOf(document.activeElement as HTMLElement)
    // Only take over at the ends (or when focus is outside), so the browser's own order is used inside.
    if (at < 0 || (!e.shiftKey && at === list.length - 1) || (e.shiftKey && at === 0)) {
      e.preventDefault()
      list[to].focus()
    }
  }
  node.addEventListener('keydown', keys)
  return {
    destroy() {
      clearTimeout(timer)
      node.removeEventListener('keydown', keys)
      node.removeAttribute(OPEN)
      // Back to where the person was, unless something else has taken focus since.
      const active = document.activeElement
      if (before?.isConnected && (active === document.body || active === null || !active.isConnected || box.contains(active))) before.focus()
    },
  }
}
