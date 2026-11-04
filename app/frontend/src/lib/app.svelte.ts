// The window's state. Go owns the documents; this holds the latest view of
// each one, pushed by events, plus what only the window knows (tabs).
import { Clipboard, Events } from '@wailsio/runtime'
import { ui } from './ui.svelte'
import { live } from './live'
import { moved } from './reorder'
import { screens } from './screens'
import { closeNode, closePane, decodeLayout, encodeLayout, leaf, nodesOf, panesOf, resizeSplit, splitForDrop, splitPane, withNode, type Layout, type Side } from './layout'
import { readLast, readNamed, writeLast, writeNamed } from './layoutstore'
import type { PastedKind } from './paste'
import { EnvironmentService, FlowService, HooksService, PanelsService, PassphraseService, RequestService, SessionService, SettingsService, SystemService, TrashService, WorkspaceService } from '../../bindings/github.com/githugt/flowi/app/internal/bridge'
import type { HooksView, SettingsView, Panels, EnvView, FlowView, HistoryRow, RequestView, RunView, Tree, TreeNode } from './types'
import { themeMode } from './theme'
import type { PaletteContext } from './commands'
import { tracing } from './trace.svelte'

export interface Tab {
  node: string
  label: string
  kind: string
}

class AppState {
  tree = $state<Tree | null>(null)
  tabs = $state<Tab[]>([])
  /** How the editor is divided: each pane shows one tab. One pane is the usual case. */
  layout = $state<Layout>(leaf('p1', ''))
  /** The pane last pressed: the tab strip, Save and Send act on the tab it shows. */
  focus = $state('p1')
  private paneSeq = 1
  /** The flow whose selected request step a pane follows ("" for none), and the pane that does. */
  follow = $state('')
  followPane = $state('')
  /** The layouts the person named for this workspace, by name. */
  named = $state<Record<string, unknown>>({})

  /** The tab in the pane in front. Setting it shows that tab: in the pane that already shows it, else in the focused pane. */
  get active(): string {
    return panesOf(this.layout).find((p) => p.id === this.focus)?.node ?? ''
  }
  set active(node: string) {
    const showing = panesOf(this.layout).find((p) => p.node === node && node !== '')
    if (showing) this.focus = showing.id
    else this.layout = withNode(this.layout, this.focus, node)
  }

  private newPaneId(): string {
    return `p${++this.paneSeq}`
  }

  /** Tabs the split opened by itself, which it closes again when it moves on, unless they were edited. */
  private beside = new Set<string>()
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
    if (action === 'jump') return void this.jumpRequests++
    if (action === 'send') return void this.sendRequests++
    if (action === 'palette') return void (this.paletteOpen = !this.paletteOpen)
    if (action === 'new-request') return void WorkspaceService.NewScratchRequest()
    if (action === 'new-flow') return void WorkspaceService.NewScratchFlow()
    if (action === 'discover') return void (ui.discover = { collection: '' })
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
    if (a.sendButton && a.sendButton !== 'orange') root.dataset.send = a.sendButton
    else delete root.dataset.send
    if (a.density === 'compact') root.dataset.density = 'compact'
    else delete root.dataset.density
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

  async open(n: TreeNode) {
    if (await this.ensureOpen(n)) this.active = n.id
  }

  /** Loads the document of a tree row and gives it a tab, without showing it. False when it cannot be opened. */
  async ensureOpen(n: TreeNode): Promise<boolean> {
    this.error = ''
    if (n.kind === 'flow') {
      try {
        this.flows[n.id] = (await FlowService.Open(n.id)) as unknown as FlowView
      } catch (e) {
        this.error = String(e)
        return false
      }
    } else if (n.kind === 'request') {
      try {
        this.requests[n.id] = (await RequestService.Open(n.id)) as unknown as RequestView
      } catch (e) {
        this.error = String(e)
        return false
      }
    } else if (n.kind === 'environment') {
      try {
        await PassphraseService.Unlock() // the user turns to secrets: a cancelled passphrase may be asked again
        this.envs[n.id] = (await EnvironmentService.Open(n.id)) as unknown as EnvView
      } catch (e) {
        this.error = String(e)
        return false
      }
    } else {
      return false // groups and collections only expand
    }
    if (!this.tabs.some((t) => t.node === n.id)) this.tabs.push({ node: n.id, label: n.label, kind: n.kind })
    return true
  }

  /** Opens a tree row beside the tab in front, as a pane to its right. */
  async openBeside(id: string) {
    const n = this.find(id)
    if (!n || !this.active || this.active === id) return
    if (!(await this.ensureOpen(n))) return
    this.splitFocused(id, 'right')
  }

  /** Splits the focused pane on a side so that the new pane shows `node`, and focuses it. */
  private splitFocused(node: string, side: Side) {
    const id = this.newPaneId()
    this.layout = splitPane(this.layout, this.focus, node, side, id)
    this.focus = id
    this.follow = ''
  }

  /** Shows the request a step of an open flow calls beside the flow, and keeps following the flow's selection. */
  async showStepBeside(flow: string, step: string) {
    let id: string
    try {
      id = await FlowService.RequestNode(flow, step)
    } catch {
      return // a step that calls no request has nothing to show beside it
    }
    const n = this.find(id)
    if (!n || id === flow) return
    const wasOpen = this.tabs.some((t) => t.node === id)
    if (!(await this.ensureOpen(n))) return
    if (!wasOpen) this.beside.add(id)
    const flowPane = panesOf(this.layout).find((p) => p.node === flow)
    if (!flowPane) return // the flow is no longer in front anywhere: there is nothing to follow it from
    const followed = panesOf(this.layout).find((p) => p.id === this.followPane)
    const previous = followed?.node
    if (followed) this.layout = withNode(this.layout, followed.id, id)
    else {
      this.followPane = this.newPaneId()
      this.layout = splitPane(this.layout, flowPane.id, id, 'right', this.followPane)
    }
    this.follow = flow
    this.focus = flowPane.id
    if (previous && previous !== id && this.beside.delete(previous) && !this.isDirty(previous) && !this.shown(previous)) await this.close(previous)
  }

  /** Whether a pane shows the tab. */
  private shown(node: string): boolean {
    return panesOf(this.layout).some((p) => p.node === node)
  }

  /** Splits the pane `paneId` with an open tab dropped on one of its edges (see splitForDrop); focuses the new pane. */
  splitWith(node: string, paneId: string, side: Side) {
    const made = splitForDrop(this.layout, this.tabs.map((t) => t.node), paneId, node, side, this.newPaneId())
    if (!made) return
    this.layout = made.layout
    this.focus = made.pane
    this.follow = ''
    this.followPane = ''
  }

  /** Closes a pane: its tab stays open, and the panes beside it share its room. */
  closePaneById(id: string) {
    this.layout = closePane(this.layout, id)
    this.settleFocus()
  }

  /** Moves a divider of the layout: see resizeSplit. */
  resizeDivider(path: number[], boundary: number, position: number) {
    this.layout = resizeSplit(this.layout, path, boundary, position)
  }

  /** Ends every split: the tab in front stays, the others stay open in the strip. */
  closeSplit() {
    this.layout = leaf(this.newPaneId(), this.active)
    this.focus = panesOf(this.layout)[0].id
    this.follow = ''
    this.followPane = ''
  }

  /** After panes went away: the focus and the follower must be panes that are still there. */
  private settleFocus() {
    const panes = panesOf(this.layout)
    if (!panes.some((p) => p.id === this.focus)) this.focus = panes[0].id
    if (!panes.some((p) => p.id === this.followPane)) {
      this.followPane = ''
      this.follow = ''
    }
  }

  /** Remembers the arrangement of the panes as a named layout, replacing one of that name. */
  saveLayout(name: string) {
    const workspace = this.tree?.name ?? ''
    this.named = { ...this.named, [name]: encodeLayout(this.layout) }
    writeNamed(workspace, this.named)
  }

  /** Puts a named layout back: its tabs are opened if they are not, and a tab that cannot be opened is left out. */
  async applyLayout(name: string) {
    const raw = decodeLayout(this.named[name], () => true, () => '')
    if (!raw) return ui.toast(`The layout ${name} is empty.`, 'error')
    for (const node of nodesOf(raw)) await this.openForLayout(node)
    const layout = decodeLayout(this.named[name], (node) => this.tabs.some((t) => t.node === node), () => this.newPaneId())
    if (!layout) return ui.toast(`None of the tabs of ${name} could be opened.`, 'error')
    this.layout = layout
    this.focus = panesOf(layout)[0].id
    this.follow = ''
    this.followPane = ''
  }

  /** Makes sure the tab of a saved pane exists: a row of the tree is opened, a screen too. */
  private async openForLayout(node: string) {
    if (this.tabs.some((t) => t.node === node)) return
    if (node.startsWith('screen:')) {
      this.tabs.push({ node, label: screens.find((s) => s.id === node)?.title ?? node, kind: 'screen' })
      return
    }
    const n = this.find(node)
    if (n) await this.ensureOpen(n)
  }

  deleteLayout(name: string) {
    const { [name]: _gone, ...rest } = this.named
    this.named = rest
    writeNamed(this.tree?.name ?? '', this.named)
  }

  private lastRead = $state(false)

  /** Reads the layout the window was left in, and the named layouts, once the tabs of the last session are back. */
  readLayouts() {
    if (this.lastRead || !this.restored || !this.tree) return
    this.lastRead = true
    const workspace = this.tree.name
    this.named = readNamed(workspace)
    const saved = decodeLayout(readLast(workspace), (node) => this.tabs.some((t) => t.node === node), () => this.newPaneId())
    if (!saved || panesOf(saved).length < 2) return
    this.layout = saved
    this.focus = panesOf(saved)[0].id
  }

  /** Keeps the layout the window is in, for the next session; one pane needs no remembering. Not before it was read. */
  keepLayout() {
    // The layout is read before the guard: an effect that returns early depends on nothing it did not read, and would
    // never run again when the layout changes.
    const encoded = panesOf(this.layout).length > 1 ? encodeLayout(this.layout) : null
    if (!this.lastRead || !this.tree) return
    writeLast(this.tree.name, encoded)
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
    if (node === this.follow) this.follow = ''
    this.beside.delete(node)
    // The panes that showed it close; the last one shows the tab next to it in the strip, or nothing.
    this.layout = closeNode(this.layout, node, this.tabs[Math.max(0, i - 1)]?.node ?? '')
    this.settleFocus()
  }

  selectEnvironment(name: string) {
    return WorkspaceService.SelectEnvironment(name)
  }

  /** The steps of the flow in the active tab, as the palette lists them: id and what the card says it is. */
  private openFlowSteps(): { id: string; label: string }[] {
    const view = this.flows[this.active]
    if (!view) return []
    // The diagram's units are in flow order; the cards are keyed by id, which a map does not keep in order.
    return view.diagram.units.flatMap((u) => u.nodes).filter((id) => view.cards[id]).map((id) => ({ id, label: view.cards[id].title }))
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
      steps: this.openFlowSteps(),
      variables: this.flows[this.active]?.vars.set ?? [],
      selectStep: (id) => void FlowService.Select(this.active, id),
      traceValue: (name) => void tracing.start(this.active, name),
      compareEnvironments: (a, b) => (ui.envCompare = { a, b }),
      selectEnvironment: (n) => void this.selectEnvironment(n),
      setTheme: (n) => void SettingsService.Set('appearance.theme', n),
    }
  }
}

export const app = new AppState()
