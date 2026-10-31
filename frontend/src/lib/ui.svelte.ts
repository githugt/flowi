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
  /** What the choice is called ('Protocol' when absent): a dataset query asks for the Database. */
  protocolsLabel?: string
  /** An extra choice asked with the text (a collection), answered as the third argument. */
  select?: { label: string; options: { id: string; label: string }[] }
  /** A caution shown under the field (what the answer commits or reveals); "" or absent for none. */
  warning?: string
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

/** The place chooser: a searchable tree of collections and folders, with a name, and New collection / New folder made inside it. */
export interface PlaceDialog {
  title: string
  /** The confirming button: "Create", "Save", "Choose". */
  action: string
  /** Which folders are listed: those of the requests or of the flows. */
  tree: 'requests' | 'flows'
  /** A name or path box under the tree; absent when only a place is asked for. */
  name?: { label: string; initial: string; /** An empty name is refused. */ required?: boolean }
  protocols?: { id: string; label: string }[]
  protocolsLabel?: string
  /** Only whole collections are offered (no folders, no New folder). */
  collectionsOnly?: boolean
  /** A row to start on; the selected row of the tree when absent. */
  row?: string
  /** A line under the title. */
  note?: string
  done: (row: string, name: string, protocol: string) => void
}

class UIState {
  menu = $state<{ x: number; y: number; items: MenuItem[] } | null>(null)
  dialog = $state<PromptDialog | ConfirmDialog | UnsavedDialog | null>(null)
  /** The "New flow from blueprint" dialog: where it was asked from, and a blueprint to start with. */
  blueprint = $state<{ collection: string; folder: string; blueprint?: string } | null>(null)
  /** The place chooser (New request, Save…, Import). */
  place = $state<PlaceDialog | null>(null)
  /** The Move dialog: the request rows to move. */
  moving = $state<string[] | null>(null)
  toasts = $state<{ id: number; level: string; text: string }[]>([])
  /** A request is waiting for the person to sign in (an OAuth 2.0 grant that needs a browser or a device code). */
  signIn = $state<{ kind: 'open' | 'device'; url: string; code?: string; expires?: string } | null>(null)
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
