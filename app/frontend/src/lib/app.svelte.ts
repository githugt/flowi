// The window's state. Go owns the documents; this holds the latest view of
// each one, pushed by events, plus what only the window knows (tabs).
import { Events } from '@wailsio/runtime'
import { noServices, type ServiceState } from './services'
import { onEvent } from './events'
import { forgetNodeUi } from './nodeui'
import { ui } from './ui.svelte'
import { latest } from './latest'
import { errorText } from './errors'
import { live } from './live'
import { moved } from './reorder'
import { screens } from './screens'
import type { PastedKind } from './paste'
import { EnvironmentService, FlowService, HooksService, PanelsService, PassphraseService, RequestService, SessionService, SettingsService, SystemService, TrashService, WorkspaceService } from '../../bindings/github.com/githugt/flowi/app/internal/bridge'
import type { HooksView, SettingsView, Panels, EnvView, FlowView, HistoryRow, RequestView, RunView, Tree, TreeNode } from './types'
import { themeMode } from './theme'
import { rememberTheme } from './themecache'
import { rememberClosed, takeReopen, type ClosedTab } from './closedtabs'
import { settle } from './settle'
import { checkNewPassphrase, newPassphraseProblem } from './passphrase'
import { inkForChosenAccent } from './contrast'
import { ClosedSet } from './closedset'
import { copy } from './clip'
import { sameData } from './same'
import { helpUrl } from './helplinks'
import { openExternal } from './openurl'
import { quoteFamily } from './fontfamily'
import { isMac, type PaletteContext } from './commands'
import { shortcutSheet } from './shortcuts'
import { planSaveAll, saveAllMessage } from './saveall'
import { SIDEBAR_MODES, loadMode, saveMode, type SidebarMode } from './sidebar'
import { activeToRestore, loadActive, rememberable, saveActive } from './activetab'

export interface Tab {
  node: string
  label: string
  kind: string
}

/** How long Undo stays on a toast after a delete. */
const UNDO_MS = 8000

class AppState {
  tree = $state<Tree | null>(null)
  /** The open request uses {{env.…}} values and no environment is selected: set by MissingBar, which the chip reads to turn amber. */
  envWanted = $state(false)
  tabs = $state<Tab[]>([])
  active = $state('')
  flows = $state<Record<string, FlowView>>({})
  history = $state<Record<string, HistoryRow[]>>({})
  /** The latest runs of each flow as a line, and the slow-step note: from the same event as history. */
  trends = $state<Record<string, { trend: string; slowNote: string }>>({})
  requests = $state<Record<string, RequestView>>({})
  envs = $state<Record<string, EnvView>>({})
  hooks = $state<Record<string, HooksView>>({})
  settings = $state<SettingsView | null>(null)
  // Whether the replayer and the schedule runner are working, kept from their events so the footer can say so after
  // their screen is closed. Both start stopped, and only the person starts them.
  services = $state<ServiceState>(structuredClone(noServices))
  panels = $state<Panels>({ log: [], problems: [], summary: '', history: [], opening: '' })
  error = $state('')
  marked = $state<string[]>([])
  /** The row a dragged row would be put before or after (reordering the tree). */
  /** into: the dragged requests go inside the row (a collection or request folder), not beside it. */
  treeDrop = $state<{ id: string; after: boolean; into?: boolean } | null>(null)
  /** Where a Shift-click range of marked rows starts: the row last clicked. */
  markAnchor = $state('')
  /** Counts the requests to jump to a row (Cmd/Ctrl+K): the open sidebar, or the panel floating from the rail, focuses its Jump to field. */
  jumpRequests = $state(0)
  /** Bumped by the palette's Send command; the open request's editor sends when it changes. */
  sendRequests = $state(0)
  /** The command palette (Cmd/Ctrl+Shift+P) is open. */
  paletteOpen = $state(false)
  /** Each flow tab's canvas pan and zoom, so switching tabs does not reset it. Not reactive: only the canvas reads it, on mount. */
  canvasViews: Record<string, { x: number; y: number; k: number }> = {}
  /** True once the last session's tabs are back: before that, the tab list is not worth keeping. */
  restored = $state(false)

  // Go repeats the question until it is answered, and the window also asks when it starts: a repeat
  // must not open the dialog again, which would cancel the one showing.
  /** Text pasted into a request's address that PasteOffer offers to fill the request from or import. */
  /** Counts the requests to fold or unfold the sidebar (Ctrl+B): the window that owns the sidebar answers them. */
  sidebarToggles = $state(0)
  /** When the sidebar folds by itself: kept in the browser's storage. */
  sidebarMode = $state<SidebarMode>(loadMode(safeStorage()))
  setSidebarMode(mode: SidebarMode) {
    this.sidebarMode = mode
    saveMode(safeStorage(), mode)
    ui.toast(SIDEBAR_MODES.find((m) => m.id === mode)?.title ?? mode)
  }
  pasteOffer = $state<{ node: string; kind: PastedKind; text: string; http: boolean; field: HTMLElement | null } | null>(null)

  /** Asks for the vault passphrase. "create" is a vault being made: the person enters it twice and is told it cannot be recovered. */
  askPassphrase(mode: string = 'unlock', problem = '') {
    if (ui.showing('passphrase') && !problem) return
    const cancel = () => {
      PassphraseService.Cancel()
      ui.toast('Secrets stay locked. Open an environment or the Secrets screen to unlock the vault.')
    }
    if (mode !== 'create') {
      ui.prompt({
        key: 'passphrase',
        title: 'Unlock the vault',
        label: 'Vault passphrase (secrets stay locked without it)',
        initial: '',
        secret: true,
        done: (text) => PassphraseService.Answer(text),
        cancel,
      })
      return
    }
    ui.prompt({
      key: 'passphrase',
      title: 'Create the vault passphrase',
      label: `${problem ? problem + ' ' : ''}Choose a passphrase for your secrets. It cannot be recovered: if you forget it, the saved secrets are lost.`,
      initial: '',
      secret: true,
      done: (first) =>
        ui.prompt({
          key: 'passphrase',
          title: 'Repeat the passphrase',
          label: 'Enter the same passphrase again.',
          initial: '',
          secret: true,
          done: (second) => {
            const result = checkNewPassphrase(first, second)
            if (result === 'ok') PassphraseService.Answer(first)
            else this.askPassphrase('create', newPassphraseProblem(result))
          },
          cancel,
        }),
      cancel,
    })
  }

  /** Documents whose tab was closed: late events for them are dropped. */
  #closed = new ClosedSet()

  async init() {
    // An event that repeats what is held is dropped: replacing a deep $state with an equal copy wakes every reader.
    onEvent('workspace', (data) => {
      if (!sameData(this.tree, data)) this.tree = data as Tree
    })
    onEvent('flow', (data) => {
      const v = data as unknown as FlowView
      if (this.#closed.accepts(v.node) && !sameData(this.flows[v.node], v)) this.flows[v.node] = v
    })
    onEvent('open', (data) => {
      const n = this.find(String(data))
      if (n) this.open(n)
    })
    onEvent('authprompt', (data) => (ui.signIn = data))
    onEvent('notice', (data) => ui.toast(data.text, data.level))
    // A delete went to the trash: Undo stays on offer for a while.
    onEvent('deleted', (data) => {
      const { text, ids } = data as { text: string; ids: string[] }
      ui.toast(text, 'info', {
        life: UNDO_MS,
        action: { label: 'Undo', run: () => void TrashService.Undo(ids).then((problem) => problem && ui.toast(String(problem), 'error'), ui.fail) },
      })
    })
    onEvent('clipboard', (data) => {
      void copy(String(data))
    })
    onEvent('request', (data) => {
      const v = data as RequestView
      if (this.#closed.accepts(v.node)) this.requests[v.node] = v
    })
    onEvent('hooks', (data) => {
      const v = data as HooksView
      this.hooks[v.request] = v
    })
    onEvent('close', (data) => this.close(String(data)))
    onEvent('menu', (data) => this.onMenu(String(data)))
    onEvent('reorder-needs-confirm', (data) => {
      const ask = data as { parent: string; names: string[] }
      ui.confirm('Rewrite the folder file?', 'The folder file has comments. Keeping the new order rewrites it without them.', () => WorkspaceService.Reorder(ask.parent, ask.names, true), 'Rewrite')
    })
    onEvent('quit-requested', (data) => {
      Events.Emit('quit-ack') // tells Go this page is alive: without it a second close quits at once
      const envs = (data as { environments?: number } | null)?.environments ?? 0
      const note = envs ? ' Changes to environments cannot be kept: they are lost either way.' : ''
      ui.unsaved(
        'Unsaved changes',
        `Quit keeps the unsaved changes as a draft: they come back when the flow is opened again.${note}`,
        () => SystemService.QuitKeeping().catch(ui.fail),
        () => SystemService.Quit(),
        { save: 'Quit', discard: 'Quit without saving' },
      )
    })
    onEvent('replayer', (data) => {
      const v = data as { state?: string; addr?: string }
      this.services.replayer = { state: v.state ?? 'stopped', addr: v.addr ?? '' }
    })
    onEvent('schedules', (data) => {
      const r = (data as { runner?: { state?: string; triggerAddr?: string; upcoming?: unknown[] } }).runner
      this.services.schedules = { state: r?.state ?? 'stopped', triggerAddr: r?.triggerAddr ?? '', upcoming: r?.upcoming?.length ?? 0 }
    })
    onEvent('panels', (data) => {
      if (!sameData(this.panels, data)) this.panels = data as Panels
    })
    // The vault needs its passphrase (no OS keychain holds its key); Go waits for the answer.
    // A question asked before this window listened (a reload) is still open: ask it now.
    // Go repeats the question every two seconds until it is answered, so a window that was not yet listening (a
    // reload) hears it again, with its mode ("create" or "unlock").
    onEvent('passphrase', (data) => this.askPassphrase(data === 'create' ? 'create' : 'unlock'))
    onEvent('environment', (data) => {
      const v = data as EnvView
      if (this.#closed.accepts(v.node) && !sameData(this.envs[v.node], v)) this.envs[v.node] = v
    })
    onEvent('flowrun', (data) => {
      const { node, run } = data as { node: string; run: RunView }
      if (this.#closed.accepts(node) && this.flows[node]) this.flows[node].run = run
    })
    onEvent('flowhistory', (data) => {
      const { node, runs, trend, slowNote } = data as { node: string; runs: HistoryRow[]; trend: string; slowNote: string }
      if (this.#closed.accepts(node)) {
        if (!sameData(this.history[node], runs)) this.history[node] = runs
        if (!sameData(this.trends[node], { trend, slowNote })) this.trends[node] = { trend, slowNote }
      }
    })
    live<SettingsView>('settings', () => SettingsService.State(), (v) => this.applySettings(v), (e) => ui.toast(`Could not load the settings: ${errorText(e)}`, 'error'))
    // The two first calls do not depend on each other. A failure of either is shown, and the session is not
    // restored (so `restored` stays false and the tabs of the last session are not overwritten by an empty window).
    // Before: they ran one after the other, and a rejection ended init() with nothing but a console error.
    try {
      const [panels, tree] = await Promise.all([PanelsService.State(), WorkspaceService.Tree()])
      this.panels = panels as unknown as Panels
      this.tree = tree as unknown as Tree
      await this.restoreSession()
      this.sweepDrafts()
    } catch (e) {
      this.error = `Could not start: ${errorText(e)}`
    }
  }

  /** Drafts that sat unused move to the trash: once at the start, then every half hour. Open tabs are never moved. */
  private sweepDrafts() {
    // Before: the call was neither awaited nor caught (a failure was an unhandled rejection) and a slow pass could
    // overlap the next one. One pass at a time; a failed pass is tried again at the next tick.
    let running = false
    const sweep = async () => {
      if (running) return
      running = true
      try {
        await TrashService.Sweep(this.tabRows)
      } catch {
        // housekeeping in the background: nothing to tell the person, the next pass tries again
      } finally {
        running = false
      }
    }
    void sweep()
    setInterval(sweep, 30 * 60 * 1000)
  }

  /** What a menu item (or its shortcut) does. */
  onMenu(action: string) {
    const screen = screens.find((s) => s.menu === action)
    if (screen) return this.openScreen(screen.id, screen.title)
    if (action === 'jump') return void this.jumpRequests++
    if (action === 'send') return void this.sendRequests++
    if (action === 'save-all') return void this.saveAll()
    if (action === 'toggle-sidebar') return void this.sidebarToggles++
    if (action.startsWith('sidebar-mode:')) {
      const mode = action.slice('sidebar-mode:'.length)
      if (mode === 'auto' || mode === 'open' || mode === 'manual') this.setSidebarMode(mode)
      return
    }
    if (action === 'palette') return void (this.paletteOpen = !this.paletteOpen)
    const link = helpUrl(action)
    if (link) return void openExternal(link)
    if (action === 'shortcuts') return void ui.confirm('Keyboard shortcuts', shortcutSheet(isMac()), () => {}, 'Close')
    const tabNumber = /^tab-([1-9])$/.exec(action)
    if (tabNumber && this.tabs.length) return void (this.active = this.tabs[Math.min(Number(tabNumber[1]), this.tabs.length) - 1].node)
    if (action === 'new-request') return void WorkspaceService.NewScratchRequest()
    if (action === 'new-flow') return void WorkspaceService.NewScratchFlow()
    const i = this.tabs.findIndex((t) => t.node === this.active)
    if (action === 'close-tab' && i >= 0) this.requestClose(this.active)
    if (action === 'reopen-tab') return void this.reopenClosedTab()
    if (action === 'next-tab' && this.tabs.length) this.active = this.tabs[(i + 1) % this.tabs.length].node
    if (action === 'previous-tab' && this.tabs.length) this.active = this.tabs[(i - 1 + this.tabs.length) % this.tabs.length].node
  }

  /** Opens the tabs of the last session. */
  async restoreSession() {
    try {
      const opened: string[] = []
      for (const id of (await SessionService.Restore()) as string[]) {
        const n = this.find(id)
        if (n) {
          await this.open(n, false) // quietly: the tab in front is chosen once, below
          opened.push(n.id)
        }
      }
      const front = activeToRestore(this.tabRows, loadActive())
      if (front && opened.length) this.active = front
    } finally {
      this.restored = true
    }
    if (!this.tabs.length) {
      // The welcome page is for the first launch only; after that the window opens on Home.
      const first = (await SessionService.ShowWelcome()) as boolean
      this.openScreen(first ? 'screen:welcome' : 'screen:home', first ? 'Welcome' : 'Home')
    }
  }

  /** The rows of the tabs worth restoring: documents, not screens or editors made from them. */
  get tabRows(): string[] {
    return this.tabs.filter((t) => !t.node.startsWith('screen:') && !t.node.startsWith('hooks:')).map((t) => t.node)
  }

  /** Keeps the settings and puts the appearance on the window. */
  applySettings(v: SettingsView) {
    this.settings = v
    const a = v.appearance
    const root = document.documentElement
    this.applyTheme(a)
    if (/^#[0-9a-fA-F]{6}$/.test(a.accent)) root.style.setProperty('--accent', a.accent)
    else if (!(a.theme === 'custom' && a.custom?.vars['--accent'])) root.style.removeProperty('--accent')
    // The ink on the accent follows an accent the person chose, so a light accent does not get white text.
    const palette = a.theme === 'custom' ? a.custom?.vars : undefined
    const ink = inkForChosenAccent(a.accent, palette?.['--accent'])
    if (ink) root.style.setProperty('--on-accent', ink)
    else if (!palette?.['--on-accent']) root.style.removeProperty('--on-accent')
    if (a.sendButton && a.sendButton !== 'orange') root.dataset.send = a.sendButton
    else delete root.dataset.send
    if (a.density === 'compact') root.dataset.density = 'compact'
    else delete root.dataset.density
    const family = quoteFamily(a.font ?? '')
    if (family) root.style.setProperty('--font', family)
    else root.style.removeProperty('--font')
    const mono = quoteFamily(a.monoFont ?? '')
    if (mono) root.style.setProperty('--mono', `${mono}, ui-monospace, "SF Mono", Menlo, Consolas, monospace`)
    else root.style.removeProperty('--mono')
    root.style.setProperty('--font-size', `${a.fontSize || 13}px`)
    if (a.hideSensitive) root.dataset.hideSensitive = 'true'
    else delete root.dataset.hideSensitive
  }

  /** CSS variables the custom palette set last, to take away when another theme is chosen. */
  private customVars: string[] = []
  /** Why the custom theme's palette could not be used, "" when it can or is not chosen. */
  paletteProblem = $state('')
  /** The tints of the collection rows (the palette's rainbow), empty for the other themes. */
  rainbow = $state<string[]>([])

  /** Puts the theme on the root. A custom palette that does not read changes nothing: the window keeps the theme it has. */
  private applyTheme(a: SettingsView['appearance']) {
    const root = document.documentElement
    rememberTheme(typeof localStorage === 'undefined' ? undefined : localStorage, a.theme === 'custom' ? (a.custom?.base ?? '') : a.theme)
    this.paletteProblem = a.theme === 'custom' ? (a.customProblem ?? '') : ''
    if (a.theme === 'custom') {
      if (!a.custom) return
      for (const name of this.customVars) root.style.removeProperty(name)
      this.customVars = Object.keys(a.custom.vars)
      for (const [name, value] of Object.entries(a.custom.vars)) root.style.setProperty(name, value)
      root.dataset.theme = a.custom.base
      root.dataset.mode = themeMode(a.custom.base)
      this.rainbow = a.custom.rainbow
      return
    }
    for (const name of this.customVars) root.style.removeProperty(name)
    this.customVars = []
    this.rainbow = []
    if (a.theme && a.theme !== 'system') {
      root.dataset.theme = a.theme
      root.dataset.mode = themeMode(a.theme)
    } else {
      delete root.dataset.theme
      delete root.dataset.mode
    }
  }

  /** Opens a screen that is not a tree node (settings, import, ...) as a tab. */
  openScreen(id: string, label: string) {
    if (!this.tabs.some((t) => t.node === id)) this.tabs.push({ node: id, label, kind: 'screen' })
    this.active = id
  }

  /** The tree node of the environment called name. */
  environmentNode(name: string): TreeNode | undefined {
    return this.tree?.roots.find((r) => r.kind === 'environments')?.children?.find((c) => c.label === name)
  }

  /** Opens environment `name` with a row (not yet saved) for each of vars it lacks, so only the values are left to type. */
  async addEnvironmentRows(name: string, vars: { name: string; secret: boolean }[]) {
    const n = this.environmentNode(name)
    if (!n) return
    const v = (await EnvironmentService.Open(n.id)) as unknown as EnvView
    const have = new Set(v.rows.map((r) => r.name))
    const added = vars
      .filter((x) => x.name && !have.has(x.name))
      .map((x) => ({ name: x.name, value: '', secret: x.secret, secretSet: false, override: false, kind: '' as const }))
    // Awaited: calls can reach Go in any order, and an Open answered before the
    // new rows would show the environment without them.
    if (added.length) await EnvironmentService.SetRows(n.id, [...v.rows, ...added])
    await this.open(n)
  }

  /** The tree node with this id, wherever it is in the tree. */
  find(id: string, nodes: TreeNode[] = this.tree?.roots ?? []): TreeNode | undefined {
    for (const n of nodes) {
      if (n.id === id) return n
      const inner = n.children && this.find(id, n.children)
      if (inner) return inner
    }
  }

  isDirty(id: string): boolean {
    return !!(this.flows[id]?.dirty || this.requests[id]?.dirty || this.envs[id]?.dirty || (id.startsWith('hooks:') && this.hooks[id.slice(6)]?.dirty))
  }

  /** Closes the tabs of these rows, for an action that moves or deletes their files. */
  async closeAll(ids: string[]) {
    for (const id of ids) if (this.tabs.some((t) => t.node === id)) await this.close(id)
  }

  // Each click that opens a row starts an "open"; only the newest may choose the tab to show.
  // Before: the last answer won, not the last click, so a slow request opened after a fast flow took the focus back.
  #opens = latest()

  /** Opens a document in a tab; `activate` false opens it behind the one in front (the restore of a session). */
  async open(n: TreeNode, activate = true) {
    const current = this.#opens()
    this.error = ''
    this.#closed.reopen(n.id)
    if (n.kind === 'flow') {
      try {
        this.flows[n.id] = (await FlowService.Open(n.id)) as unknown as FlowView
      } catch (e) {
        this.error = errorText(e)
        return
      }
    } else if (n.kind === 'request') {
      try {
        this.requests[n.id] = (await RequestService.Open(n.id)) as unknown as RequestView
      } catch (e) {
        this.error = errorText(e)
        return
      }
    } else if (n.kind === 'environment') {
      try {
        await PassphraseService.Unlock() // the user turns to secrets: a cancelled passphrase may be asked again
        this.envs[n.id] = (await EnvironmentService.Open(n.id)) as unknown as EnvView
      } catch (e) {
        this.error = errorText(e)
        return
      }
    } else {
      return // groups and collections only expand
    }
    if (!this.tabs.some((t) => t.node === n.id)) this.tabs.push({ node: n.id, label: n.label, kind: n.kind })
    if (activate && current()) this.active = n.id
  }

  /** Opens the hooks editor of an open request as a tab of its own. */
  async openHooks(request: string, label: string) {
    try {
      const v = (await HooksService.Open(request)) as unknown as HooksView
      this.hooks[request] = v
      for (const row of [v.pre, v.post]) {
        this.#closed.reopen(row)
        this.flows[row] = (await FlowService.Open(row)) as unknown as FlowView
      }
    } catch (e) {
      ui.fail(e)
      return
    }
    const id = 'hooks:' + request
    if (!this.tabs.some((t) => t.node === id)) this.tabs.push({ node: id, label: `Hooks · ${label}`, kind: 'hooks' })
    this.active = id
  }

  /** Moves a tab to where another one is (dragging tabs to reorder them). */
  /** Moves a tab to index `to` of the tab list. */
  moveTabTo(node: string, to: number) {
    const from = this.tabs.findIndex((t) => t.node === node)
    const next = moved(this.tabs, from, to)
    if (next !== this.tabs) this.tabs = next
  }

  /** Closes a tab, asking first when its document has unsaved edits. */
  async requestClose(node: string) {
    if (!this.isDirty(node)) return this.close(node)
    const label = this.tabs.find((t) => t.node === node)?.label ?? 'this tab'
    ui.unsaved(
      'Unsaved changes',
      `${label} has changes that are not saved.`,
      async () => {
        await this.save(node)
        // Before: the tab closed as soon as Save returned, but the write finishes later, so a failed save lost the edits.
        // Wait for the answer, and close only when the edits are saved.
        // The views are replaced as events arrive, so look the view up again each time.
        const view = () => (this.flows[node] ?? this.requests[node] ?? this.envs[node] ?? (node.startsWith('hooks:') ? this.hooks[node.slice(6)] : undefined)) as { error?: string; needsDrop?: boolean } | undefined
        await settle(() => !this.isDirty(node) || !!view()?.error || !!view()?.needsDrop)
        if (!this.isDirty(node)) return this.close(node)
        ui.toast(view()?.error ? `Not saved: ${view()?.error}. The tab stays open.` : 'Not saved. The tab stays open so the edits are not lost.', 'error')
      },
      () => this.close(node),
    )
  }

  /** Saves every open document with unsaved edits; a draft without a place is left for its own Save. */
  async saveAll() {
    const plan = planSaveAll(
      this.tabs.map((t) => t.node),
      (n) => this.isDirty(n),
      (n) => !!this.requests[n]?.inScratch,
    )
    for (const node of plan.save) await this.save(node)
    ui.toast(saveAllMessage(plan))
  }

  /** Saves the document of a tab. */
  async save(node: string) {
    if (this.flows[node]) await FlowService.Save(node)
    else if (this.requests[node]) await RequestService.Save(node)
    else if (this.envs[node]) await EnvironmentService.Save(node)
    else if (node.startsWith('hooks:')) await HooksService.Save(node.slice(6))
  }

  /** The documents whose tab was closed, newest first (Ctrl+Shift+T). */
  #reopenable: ClosedTab[] = []

  /** Brings back the tab closed last that is still in the tree and not open. */
  async reopenClosedTab() {
    const got = takeReopen(this.#reopenable, (id) => !this.tabs.some((t) => t.node === id) && !!this.find(id))
    if (!got) return
    this.#reopenable = got.rest
    const n = this.find(got.tab.node)
    if (n) await this.open(n)
  }

  async close(node: string) {
    const i = this.tabs.findIndex((t) => t.node === node)
    if (i < 0) return
    this.#reopenable = rememberClosed(this.#reopenable, { node, kind: this.tabs[i].kind })
    this.tabs.splice(i, 1)
    this.#closed.close(node)
    forgetNodeUi(node)
    if (node.startsWith('hooks:')) {
      const request = node.slice('hooks:'.length)
      const h = this.hooks[request]
      await HooksService.Close(request)
      if (h) this.#closed.close(h.pre, h.post)
      if (h) for (const row of [h.pre, h.post]) delete this.flows[row]
      delete this.hooks[request]
    }
    if (this.flows[node]) {
      await FlowService.Close(node)
      delete this.flows[node]
    }
    if (this.envs[node]) {
      await EnvironmentService.Close(node)
      delete this.envs[node]
    }
    if (this.requests[node]) {
      await RequestService.Close(node)
      delete this.requests[node]
    }
    if (this.active === node) this.active = this.tabs[Math.max(0, i - 1)]?.node ?? ''
  }

  selectEnvironment(name: string) {
    return WorkspaceService.SelectEnvironment(name)
  }

  /** What the palette needs to know to offer environments, themes and Send. */
  paletteContext(): PaletteContext {
    const themeField = this.settings?.groups.flatMap((g) => g.fields).find((f) => f.name === 'appearance.theme')
    return {
      environments: this.tree?.environments ?? [],
      selected: this.tree?.selected ?? '',
      themes: themeField?.enum ?? [],
      theme: String(themeField?.value ?? ''),
      canSend: !!this.requests[this.active],
      selectEnvironment: (n) => void this.selectEnvironment(n),
      setTheme: (n) => void SettingsService.Set('appearance.theme', n),
      hideSensitive: !!this.settings?.appearance.hideSensitive,
      setHideSensitive: (on) => void SettingsService.Set('ui.hide_sensitive', String(on)),
    }
  }
}

export const app = new AppState()

/** The browser's storage, or one that keeps nothing when it is blocked. */
function safeStorage(): { getItem(k: string): string | null; setItem(k: string, v: string): void } {
  try {
    return localStorage
  } catch {
    return { getItem: () => null, setItem: () => {} }
  }
}
