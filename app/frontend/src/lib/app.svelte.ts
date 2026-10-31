// The window's state. Go owns the documents; this holds the latest view of
// each one, pushed by events, plus what only the window knows (tabs).
import { Clipboard, Events } from '@wailsio/runtime'
import { ui } from './ui.svelte'
import { live } from './live'
import { moved } from './reorder'
import { screens } from './screens'
import type { PastedKind } from './paste'
import { EnvironmentService, FlowService, HooksService, PanelsService, PassphraseService, RequestService, SessionService, SettingsService, SystemService, TrashService, WorkspaceService } from '../../bindings/github.com/githugt/flowi/app/internal/bridge'
import type { HooksView, SettingsView, Panels, EnvView, FlowView, HistoryRow, RequestView, RunView, Tree, TreeNode } from './types'

export interface Tab {
  node: string
  label: string
  kind: string
}

class AppState {
  tree = $state<Tree | null>(null)
  tabs = $state<Tab[]>([])
  active = $state('')
  flows = $state<Record<string, FlowView>>({})
  history = $state<Record<string, HistoryRow[]>>({})
  requests = $state<Record<string, RequestView>>({})
  envs = $state<Record<string, EnvView>>({})
  hooks = $state<Record<string, HooksView>>({})
  settings = $state<SettingsView | null>(null)
  panels = $state<Panels>({ log: [], problems: [], summary: '', history: [], opening: '' })
  error = $state('')
  marked = $state<string[]>([])
  /** The row a dragged row would be put before or after (reordering the tree). */
  /** into: the dragged requests go inside the row (a collection or request folder), not beside it. */
  treeDrop = $state<{ id: string; after: boolean; into?: boolean } | null>(null)
  /** Where a Shift-click range of marked rows starts: the row last clicked. */
  markAnchor = $state('')
  /** Each flow tab's canvas pan and zoom, so switching tabs does not reset it. Not reactive: only the canvas reads it, on mount. */
  canvasViews: Record<string, { x: number; y: number; k: number }> = {}
  /** True once the last session's tabs are back: before that, the tab list is not worth keeping. */
  restored = $state(false)

  // Go repeats the question until it is answered, and the window also asks when it starts: a repeat
  // must not open the dialog again, which would cancel the one showing.
  /** Text pasted into a request's address that PasteOffer offers to fill the request from or import. */
  pasteOffer = $state<{ node: string; kind: PastedKind; text: string; http: boolean; field: HTMLElement | null } | null>(null)

  askPassphrase() {
    if (ui.showing('passphrase')) return
    ui.prompt({
      key: 'passphrase',
      title: 'Unlock the vault',
      label: 'Vault passphrase (secrets stay locked without it)',
      initial: '',
      secret: true,
      done: (text) => PassphraseService.Answer(text),
      cancel: () => {
        PassphraseService.Cancel()
        ui.toast('Secrets stay locked. Open an environment or the Secrets screen to unlock the vault.')
      },
    })
  }

  async init() {
    Events.On('workspace', (e: any) => (this.tree = e.data as Tree))
    Events.On('flow', (e: any) => {
      const v = e.data as unknown as FlowView
      this.flows[v.node] = v
    })
    Events.On('open', (e: any) => {
      const n = this.find(String(e.data))
      if (n) this.open(n)
    })
    Events.On('authprompt', (e: any) => (ui.signIn = e.data))
    Events.On('notice', (e: any) => ui.toast(e.data.text, e.data.level))
    Events.On('clipboard', (e: any) => {
      Clipboard.SetText(String(e.data))
      ui.toast('Copied')
    })
    Events.On('request', (e: any) => {
      const v = e.data as RequestView
      this.requests[v.node] = v
    })
    Events.On('hooks', (e: any) => {
      const v = e.data as HooksView
      this.hooks[v.request] = v
    })
    Events.On('close', (e: any) => this.close(String(e.data)))
    Events.On('menu', (e: any) => this.onMenu(String(e.data)))
    Events.On('reorder-needs-confirm', (e: any) => {
      const ask = e.data as { parent: string; names: string[] }
      ui.confirm('Rewrite the folder file?', 'The folder file has comments. Keeping the new order rewrites it without them.', () => WorkspaceService.Reorder(ask.parent, ask.names, true), 'Rewrite')
    })
    Events.On('quit-requested', (e: any) => {
      const envs = (e.data as { environments?: number } | null)?.environments ?? 0
      const note = envs ? ' Changes to environments cannot be kept: they are lost either way.' : ''
      ui.unsaved(
        'Unsaved changes',
        `Quit keeps the unsaved changes as a draft: they come back when the flow is opened again.${note}`,
        () => SystemService.QuitKeeping().catch((err: unknown) => ui.toast(String(err), 'error')),
        () => SystemService.Quit(),
        { save: 'Quit', discard: 'Quit without saving' },
      )
    })
    Events.On('panels', (e: any) => (this.panels = e.data as Panels))
    // The vault needs its passphrase (no OS keychain holds its key); Go waits for the answer.
    // A question asked before this window listened (a reload) is still open: ask it now.
    Events.On('passphrase', () => this.askPassphrase())
    if (await PassphraseService.Pending()) this.askPassphrase()
    Events.On('environment', (e: any) => {
      const v = e.data as EnvView
      this.envs[v.node] = v
    })
    Events.On('flowrun', (e: any) => {
      const { node, run } = e.data as { node: string; run: RunView }
      if (this.flows[node]) this.flows[node].run = run
    })
    Events.On('flowhistory', (e: any) => {
      const { node, runs } = e.data as { node: string; runs: HistoryRow[] }
      this.history[node] = runs
    })
    live<SettingsView>('settings', () => SettingsService.State(), (v) => this.applySettings(v))
    this.panels = (await PanelsService.State()) as unknown as Panels
    this.tree = (await WorkspaceService.Tree()) as unknown as Tree
    await this.restoreSession()
    this.sweepDrafts()
  }

  /** Drafts that sat unused move to the trash: once at the start, then every half hour. Open tabs are never moved. */
  private sweepDrafts() {
    const sweep = () => TrashService.Sweep(this.tabRows)
    sweep()
    setInterval(sweep, 30 * 60 * 1000)
  }

  /** What a menu item (or its shortcut) does. */
  onMenu(action: string) {
    const screen = screens.find((s) => s.menu === action)
    if (screen) return this.openScreen(screen.id, screen.title)
    if (action === 'new-request') return void WorkspaceService.NewScratchRequest()
    if (action === 'new-flow') return void WorkspaceService.NewScratchFlow()
    const i = this.tabs.findIndex((t) => t.node === this.active)
    if (action === 'close-tab' && i >= 0) this.requestClose(this.active)
    if (action === 'next-tab' && this.tabs.length) this.active = this.tabs[(i + 1) % this.tabs.length].node
    if (action === 'previous-tab' && this.tabs.length) this.active = this.tabs[(i - 1 + this.tabs.length) % this.tabs.length].node
  }

  /** Opens the tabs of the last session. */
  async restoreSession() {
    try {
      for (const id of (await SessionService.Restore()) as string[]) {
        const n = this.find(id)
        if (n) await this.open(n)
      }
    } finally {
      this.restored = true
    }
    if (!this.tabs.length) this.openScreen('screen:welcome', 'Welcome')
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
    if (a.font) root.style.setProperty('--font', a.font)
    else root.style.removeProperty('--font')
    root.style.setProperty('--font-size', `${a.fontSize || 13}px`)
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
    this.paletteProblem = a.theme === 'custom' ? (a.customProblem ?? '') : ''
    if (a.theme === 'custom') {
      if (!a.custom) return
      for (const name of this.customVars) root.style.removeProperty(name)
      this.customVars = Object.keys(a.custom.vars)
      for (const [name, value] of Object.entries(a.custom.vars)) root.style.setProperty(name, value)
      root.dataset.theme = a.custom.base
      this.rainbow = a.custom.rainbow
      return
    }
    for (const name of this.customVars) root.style.removeProperty(name)
    this.customVars = []
    this.rainbow = []
    if (a.theme && a.theme !== 'system') root.dataset.theme = a.theme
    else delete root.dataset.theme
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

  async open(n: TreeNode) {
    this.error = ''
    if (n.kind === 'flow') {
      try {
        this.flows[n.id] = (await FlowService.Open(n.id)) as unknown as FlowView
      } catch (e) {
        this.error = String(e)
        return
      }
    } else if (n.kind === 'request') {
      try {
        this.requests[n.id] = (await RequestService.Open(n.id)) as unknown as RequestView
      } catch (e) {
        this.error = String(e)
        return
      }
    } else if (n.kind === 'environment') {
      try {
        await PassphraseService.Unlock() // the user turns to secrets: a cancelled passphrase may be asked again
        this.envs[n.id] = (await EnvironmentService.Open(n.id)) as unknown as EnvView
      } catch (e) {
        this.error = String(e)
        return
      }
    } else {
      return // groups and collections only expand
    }
    if (!this.tabs.some((t) => t.node === n.id)) this.tabs.push({ node: n.id, label: n.label, kind: n.kind })
    this.active = n.id
  }

  /** Opens the hooks editor of an open request as a tab of its own. */
  async openHooks(request: string, label: string) {
    try {
      const v = (await HooksService.Open(request)) as unknown as HooksView
      this.hooks[request] = v
      for (const row of [v.pre, v.post]) this.flows[row] = (await FlowService.Open(row)) as unknown as FlowView
    } catch (e) {
      ui.toast(String(e), 'error')
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
        this.close(node)
      },
      () => this.close(node),
    )
  }

  /** Saves the document of a tab. */
  async save(node: string) {
    if (this.flows[node]) await FlowService.Save(node)
    else if (this.requests[node]) await RequestService.Save(node)
    else if (this.envs[node]) await EnvironmentService.Save(node)
    else if (node.startsWith('hooks:')) await HooksService.Save(node.slice(6))
  }

  async close(node: string) {
    const i = this.tabs.findIndex((t) => t.node === node)
    if (i < 0) return
    this.tabs.splice(i, 1)
    if (node.startsWith('hooks:')) {
      const request = node.slice('hooks:'.length)
      const h = this.hooks[request]
      await HooksService.Close(request)
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
}

export const app = new AppState()
