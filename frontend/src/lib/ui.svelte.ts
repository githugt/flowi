// Window-level UI state that has no document: the context menu, the modal
// dialog and the toasts.
import type { MenuEntry } from './types'

export interface MenuItem {
  label: string
  run: () => void
  disabled?: boolean
  separator?: boolean
}

export interface PromptDialog {
  kind: 'prompt'
  title: string
  label: string
  initial: string
  protocols?: { id: string; label: string }[]
  /** An extra choice asked with the text (a collection), answered as the third argument. */
  select?: { label: string; options: { id: string; label: string }[] }
  /** The text is a password: typed out of sight. */
  secret?: boolean
  /** Names a question that may be asked again while it shows (the vault's passphrase): see showing. */
  key?: string
  /** Runs when the dialog goes without an answer (Cancel, Escape, a click outside, another dialog). */
  cancel?: () => void
  done: (text: string, protocol: string, chosen: string) => void
}

export interface ConfirmDialog {
  kind: 'confirm'
  title: string
  text: string
  action: string
  done: () => void
}

export interface UnsavedDialog {
  kind: 'unsaved'
  title: string
  text: string
  save: () => void
  discard: () => void
  /** The buttons' words, "Save" and "Don't save" unless set. */
  saveLabel?: string
  discardLabel?: string
}

class UIState {
  menu = $state<{ x: number; y: number; items: MenuItem[] } | null>(null)
  dialog = $state<PromptDialog | ConfirmDialog | UnsavedDialog | null>(null)
  /** The "New flow from blueprint" dialog: where it was asked from, and a blueprint to start with. */
  blueprint = $state<{ collection: string; folder: string; blueprint?: string } | null>(null)
  /** The Move dialog: the request rows to move. */
  moving = $state<string[] | null>(null)
  toasts = $state<{ id: number; level: string; text: string }[]>([])
  private next = 1

  openMenu(x: number, y: number, items: MenuItem[]) {
    if (items.length) this.menu = { x, y, items }
  }

  closeMenu() {
    this.menu = null
  }

  prompt(d: Omit<PromptDialog, 'kind'>) {
    this.show({ kind: 'prompt', ...d })
  }

  confirm(title: string, text: string, done: () => void, action = 'Delete') {
    this.show({ kind: 'confirm', title, text, action, done })
  }

  /** Save, discard or cancel: for closing something with unsaved edits. */
  unsaved(title: string, text: string, save: () => void, discard: () => void, labels: { save?: string; discard?: string } = {}) {
    this.show({ kind: 'unsaved', title, text, save, discard, saveLabel: labels.save, discardLabel: labels.discard })
  }

  /** Whether the prompt with this key shows now. */
  showing(key: string) {
    return this.dialog?.kind === 'prompt' && this.dialog.key === key
  }

  /** Closes the dialog without an answer. One that waits on its answer (the vault's passphrase) hears so. */
  dismiss() {
    const d = this.dialog
    this.dialog = null
    if (d?.kind === 'prompt') d.cancel?.()
  }

  /** Closes the dialog with its answer: the caller acts on it, so cancel does not run. */
  take() {
    const d = this.dialog
    this.dialog = null
    return d
  }

  private show(d: PromptDialog | ConfirmDialog | UnsavedDialog) {
    this.dismiss()
    this.dialog = d
  }

  toast(text: string, level = 'info') {
    const id = this.next++
    this.toasts.push({ id, level, text })
    setTimeout(() => (this.toasts = this.toasts.filter((t) => t.id !== id)), level === 'error' ? 8000 : 4000)
  }
}

export const ui = new UIState()

export type { MenuEntry }
