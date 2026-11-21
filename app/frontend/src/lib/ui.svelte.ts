// Window-level UI state that has no document: the context menu, the modal
// dialog and the toasts.
import type { MenuEntry } from './types'
import { toastLife } from './toastlife'
import { errorText, isCancelled, reportUnhandled } from './errors'
import { reopened } from './dialogretry'
import { addToast, type Toast, type ToastAction } from './toastlist'

export interface MenuItem {
  label: string
  run: () => void
  disabled?: boolean
  separator?: boolean
  /** Shown with a check: the current choice in a list of options. */
  checked?: boolean
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
  /** What the last try said when it failed; the dialog comes back with it, and the text typed. */
  error?: string
  /** A promise it returns that rejects brings the dialog back with the error and the typed text. */
  done: (text: string, protocol: string, chosen: string) => void | Promise<unknown>
}

export interface ConfirmDialog {
  kind: 'confirm'
  title: string
  text: string
  action: string
  /** What the last try said when it failed. */
  error?: string
  /** A promise it returns that rejects brings the dialog back with the error. */
  done: () => void | Promise<unknown>
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
  toasts = $state<Toast[]>([])
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

  /**
   * Runs the answer of a dialog the person confirmed. When it returns a promise that rejects, the dialog comes
   * back, with the error under the field and the text typed, unless another dialog took its place meanwhile.
   * Before: the dialog closed first, so a failed action lost what had been typed and only a toast said why.
   */
  answer(d: PromptDialog | ConfirmDialog, text = '', run: () => void | Promise<unknown>) {
    let result: void | Promise<unknown>
    try {
      result = run()
    } catch (e) {
      if (this.dialog === null) this.dialog = reopened(d, text, errorText(e))
      return
    }
    if (result && typeof (result as Promise<unknown>).then === 'function') {
      ;(result as Promise<unknown>).catch((e) => {
        if (isCancelled(e)) return
        if (this.dialog === null) this.dialog = reopened(d, text, errorText(e))
        else this.fail(e)
      })
    }
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

  // Each toast leaves after a time that depends on its length. Pointing at it holds it, and a click on × closes it.
  private timers = new Map<number, ReturnType<typeof setTimeout>>()
  private lives = new Map<number, number>()

  /** `action` adds a button (Undo); `life` keeps the toast that many milliseconds instead of the usual time. */
  toast(text: string, level = 'info', options: { action?: ToastAction; life?: number } = {}) {
    const added = addToast(this.toasts, text, level, this.next, options.action)
    if (!added.reused) this.next++
    this.toasts = added.list
    for (const gone of added.dropped) this.forget(gone)
    // The same message again shows the toast once more for its whole time.
    this.pause(added.id)
    const life = options.life ?? toastLife(text, level)
    this.lives.set(added.id, life)
    this.timers.set(added.id, setTimeout(() => this.dismissToast(added.id), life))
  }

  /**
   * Shows the error of a failed call as an error toast; a call the window cancelled stays quiet. Pass it to `.catch`.
   * Before: each call site spelled `.catch((e) => ui.toast(errorText(e)))` itself, some without the error level.
   */
  fail = (e: unknown) => reportUnhandled(e, (text, level) => this.toast(text, level))

  private forget(id: number) {
    this.pause(id)
    this.lives.delete(id)
  }

  /** Closes a toast now. */
  dismissToast(id: number) {
    this.pause(id)
    this.lives.delete(id)
    this.toasts = this.toasts.filter((t) => t.id !== id)
  }

  /** Holds a toast while the pointer or the focus is on it. */
  pause(id: number) {
    const t = this.timers.get(id)
    if (t !== undefined) clearTimeout(t)
    this.timers.delete(id)
  }

  /** Lets a held toast go on: it stays a little longer after the pointer leaves. */
  resume(id: number) {
    if (!this.lives.has(id)) return
    this.pause(id)
    this.timers.set(id, setTimeout(() => this.dismissToast(id), Math.min(this.lives.get(id)!, 4000)))
  }
}

export const ui = new UIState()

export type { MenuEntry }
