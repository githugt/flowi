<script lang="ts">
  import { modalOpen } from './lib/modal'
  import { untrack } from 'svelte'
  import { nodeUi, setNodeUi } from './lib/nodeui'
  import { headerWord, sectionTitle } from './lib/sectiontitle'
  import { isLetter } from './lib/keymatch'
  import Select from './Select.svelte'
  import Toggle from './Toggle.svelte'
  import { headersHint } from './lib/headershint'
  import IconButton from './IconButton.svelte'
  import Segmented from './Segmented.svelte'
  import Menu, { type MenuItem } from './Menu.svelte'
  import { anchorAt, type MenuAt } from './lib/menuanchor'
  import { copy as copyOut } from './lib/clip'
  import { snippetCopied, copyCodeLabel } from './lib/snippetnote'
  import { COMMON_HEADERS, headerValues, implicitHeaders } from './lib/headernames'
  import { modeLoss } from './lib/modeloss'
  import { errorText } from './lib/errors'
  import LabelsForm from './LabelsForm.svelte'
  import { statusTitle } from './lib/devstatus'
  import { app } from './lib/app.svelte'
  import { ui } from './lib/ui.svelte'
  import { parseXml } from './lib/xml'
  import { jsonProblem } from './lib/jsonproblem'
  import { elapsedText } from './lib/elapsed'
  import Button from './Button.svelte'
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { DiskDiff } from './lib/diskdiff'
  import type { Extraction, RequestView } from './lib/types'
  import { extractChip } from './lib/extractchip'
  import TextInput from './TextInput.svelte'
  import Field from './Field.svelte'
  import TemplateField from './TemplateField.svelte'
  import KVRows from './KVRows.svelte'
  import ConnectionTest from './ConnectionTest.svelte'
  import ResponsePane from './ResponsePane.svelte'
  import Splitter from './Splitter.svelte'
  let leftWidth = $state(520)
  import FileBanner from './FileBanner.svelte'
  import RequestExtras from './RequestExtras.svelte'
  import MessageSections from './MessageSections.svelte'
  import AreaField from './AreaField.svelte'
  import Markdown from './Markdown.svelte'
  import AssertCards from './AssertCards.svelte'
  import OutcomesEditor from './OutcomesEditor.svelte'
  import { requestVocabulary } from './lib/vocabulary'
  import { sectionConfigured, sectionCounts } from './lib/tabcounts'
  import { startersFor } from './lib/assertstarters'
  import Dropdown from './Dropdown.svelte'
  import ExprHelp from './ExprHelp.svelte'
  import RequestMatrix from './RequestMatrix.svelte'
  import ReplayBar from './ReplayBar.svelte'
  import MissingBar from './MissingBar.svelte'
  import Problem from './Problem.svelte'
  import VersionDetails from './VersionDetails.svelte'
  import StoredResponses from './StoredResponses.svelte'
  import Verb from './Verb.svelte'
  import { isMac } from './lib/commands'
  import { hookBadge } from './lib/hookmode'
  import { onEvent } from './lib/events'
  import { isSendKey } from './lib/sendkey'
  import Icon from './Icon.svelte'
  import MethodPicker from './MethodPicker.svelte'
  import CodeArea from './CodeArea.svelte'
  import type { CodeLang } from './lib/highlight'
  import EmptyState from './EmptyState.svelte'
  import { protocolVerb } from './lib/verbs'
  import { sendNeedsConfirm } from './lib/protect'
  import { isFileDriver, moveServerPassword, serverHasPassword } from './lib/dataset'
  import { AuthsService, SecretsService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  const PASSWORD_NOTE = "The password is saved in this request's file, which is often committed. Put it in a secret and write {{secret.name}} in its place."
  import { recognise } from './lib/paste'
  import AuthFields from './AuthFields.svelte'
  import { authSummary } from './lib/authsummary'
  import type { DropOption } from './lib/types'
  import { WorkspaceService, ManagerService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { KeptSpec, SpecOrigin } from './lib/types'
  import OverlayDialog from './OverlayDialog.svelte'
  import RequestTypeBar from './RequestTypeBar.svelte'
  import PartsEditor from './PartsEditor.svelte'
  import DatasetSections from './DatasetSections.svelte'
  import SpecUpdateDialog from './SpecUpdateDialog.svelte'
  import FlowiOverlayDialog from './FlowiOverlayDialog.svelte'
  import FlowiOverlayEditor from './FlowiOverlayEditor.svelte'
  import FlowiOverlayWarning from './FlowiOverlayWarning.svelte'

  let { view }: { view: RequestView } = $props()
  const node = $derived(view.node)
  const statusMark = $derived(app.find(node)?.status)
  const v = $derived(view.current)
  const methods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS', 'QUERY']
  // The registered WebDAV, CalDAV and other methods are not listed: they go through Custom…, which keeps their registered spelling.
  const CUSTOM = '\u0000custom'
  const httpSections = ['Params', 'Headers', 'Body', 'Auth', 'Extract', 'Assert', 'Outcomes', 'Variables', 'Settings', 'Hooks', 'Schema', 'Docs', 'Code', 'Responses', 'Used by', 'Cookies']
  type Section = string
  // What a check or an extract on this kind of request can read, and the example its help gives.
  const checkContext = $derived(view.kind === 'http' ? 'request.check' : view.kind === 'dataset' ? 'dataset.check' : 'message.check')
  const checkExample = $derived(view.kind === 'http' ? 'status == 200' : view.kind === 'dataset' ? 'row_count >= 1' : 'received >= 1')
  // Databases that take a login in the connection URL; a file or an SQLite path has none.
  const urlDriver = $derived(['postgres', 'mysql', 'mariadb'].includes(view.dataset?.block.Driver ?? ''))
  const sections = $derived.by(() => {
    if (view.kind === 'dataset') return ['Query', 'Values', ...(urlDriver ? ['Credentials'] : []), 'Extract', 'Assert', 'Outcomes', 'Variables', 'Settings', 'Hooks', 'Docs', 'Code', 'Responses', 'Used by']
    if (view.kind === 'grpc') return ['Message', 'Headers', 'Receive', 'Auth', 'Proto', 'Variables', 'Extract', 'Assert', 'Outcomes', 'Settings', 'Hooks', 'Schema', 'Docs', 'Code', 'Responses', 'Used by']
    if (view.kind === 'async') {
      const p = view.async?.block.Protocol
      return ['Message', 'Headers', 'Receive', ...(view.async?.credentials.length ? ['Credentials'] : []), 'Binding', 'Variables', 'Extract', 'Assert', 'Outcomes', 'Settings', 'Hooks', 'Schema', 'Docs', 'Code', 'Responses', 'Used by'].filter(
        (s) => !(s === 'Receive' && p === 'sns') && !(s === 'Message' && p === 'sse' && !view.async?.block.Send),
      )
    }
    return httpSections
  })
  // The sections shown as tabs: what most sends need. The rest are under "More", in their usual order.
  const mainOf = $derived(
    view.kind === 'dataset'
      ? ['Query', 'Values', ...(urlDriver ? ['Credentials'] : []), 'Extract', 'Assert']
      : view.kind === 'grpc'
        ? ['Message', 'Headers', 'Receive', 'Auth', 'Extract', 'Assert']
        : view.kind === 'async'
          ? ['Message', 'Headers', 'Receive', 'Credentials', 'Extract', 'Assert']
          : ['Params', 'Headers', 'Body', 'Auth', 'Extract', 'Assert'],
  )
  const ICON_TAB_W = 37 // an icon tab and the gap after it
  const LABEL_W = 90 // room for the open tab's name
  const MORE_W = 84 // the More button before it is measured
  const NAV_PAD = 22 // the row's padding and More's margin
  // How many of n sections get a tab in a row navWidth wide; the rest need More beside them.
  function tabsThatFit(navWidth: number, moreW: number, n: number): number {
    const room = navWidth - NAV_PAD - LABEL_W
    if (room >= n * ICON_TAB_W) return n
    return Math.max(1, Math.floor((room - moreW) / ICON_TAB_W))
  }
  // As many sections as fit are icon tabs: what most sends need first, then the rest in their usual
  // order. The others are under More, at the end of the row. The open section always has a tab.
  const ordered = $derived([...mainOf.filter((s) => sections.includes(s)), ...sections.filter((s) => !mainOf.includes(s))])
  let navWidth = $state(0)
  let moreWidth = $state(0)
  const fitting = $derived(tabsThatFit(navWidth, moreWidth || MORE_W, ordered.length))
  const mainSecs = $derived.by(() => {
    const shown = ordered.slice(0, fitting)
    if (shown.includes(section) || !shown.length) return shown
    return [...shown.slice(0, -1), section]
  })
  const moreSecs = $derived(ordered.filter((s) => !mainSecs.includes(s)))
  // A gRPC request's message is its Body and what it waits for is a Stream.
  const titleOf = (s: string) => sectionTitle(view.kind, s)
  const iconOf = (s: string) =>
    ({ Params: 'params', Headers: 'headers', Body: 'body', Message: 'body', Query: 'body', Values: 'params', Auth: 'auth', Credentials: 'auth', Extract: 'extract', Assert: 'assert', Outcomes: 'checks', Variables: 'variables', Hooks: 'hooks', Docs: 'docs', Schema: 'schema', Receive: 'stream', Proto: 'proto', Binding: 'binding', Code: 'code', Cookies: 'cookie', Responses: 'clock', 'Used by': 'usedby', Settings: 'settings' } as Record<string, string>)[s] ?? 'list'
  // The green dot: the section has content.
  const configured = (s: string) => sectionConfigured(view, s)
  // The count badge: how many rows the section holds. Sections that are not lists keep the dot.
  const counts = $derived(sectionCounts(view))
  const moreOptions = $derived<DropOption[]>(moreSecs.map((s) => ({ value: s, label: titleOf(s), icon: iconOf(s), dot: configured(s), count: counts[s] })))
  // The section, the hook mode and the replay bar are kept per node, so a tab switch (which remounts this) leaves them.
  const kept = untrack(() => nodeUi(view.node))
  let chosen = $state<Section>(kept.section)
  let expectProblem = $state('') // what is wrong with the text typed in Expected status; '' when nothing is
  const section = $derived(sections.includes(chosen) ? chosen : sections[0])
  // The open menu: the Send button's (how to send) or the Save button's (versions).
  let menu = $state<'send' | 'save' | null>(null)
  let menuAt = $state<MenuAt>({ x: 0, y: 0 })
  function openMenu(which: 'send' | 'save', e: MouseEvent) {
    menuAt = anchorAt((e.currentTarget as HTMLElement).getBoundingClientRect(), { end: true })
    menu = menu === which ? null : which
  }
  // A click in the first 0.4 s after Send is the second half of a double-click: it does not cancel.
  // How long the send has run, counted in the window: it ticks while a send is on and starts again with the next.
  let sendMs = $state(0)
  $effect(() => {
    if (!view.sending) return
    const from = Date.now()
    sendMs = 0
    const tick = setInterval(() => (sendMs = Date.now() - from), 100)
    return () => clearInterval(tick)
  })
  let detailsOpen = $state(false)
  // The Docs section shows the notes as written (Markdown source) or as read.
  let docsPreview = $state(false)

  // The kept spec this request comes from, and which fields its overlays set. Asked again when the
  // version changes, after a save to the overlay ("spec-origin"), and when the workspace changes.
  let origin = $state<SpecOrigin | null>(null)
  let overlayOpen = $state(false)
  let updating = $state<KeptSpec | null>(null)
  function askOrigin(forNode: string) {
    RequestService.SpecOrigin(forNode).then((o) => {
      if (forNode === node) origin = o as unknown as SpecOrigin
    })
  }
  $effect(() => {
    void view.selected
    askOrigin(node)
  })
  $effect(() => {
    const forNode = node
    const offOrigin = onEvent('spec-origin', (data) => data === forNode && askOrigin(forNode))
    const offTree = onEvent('workspace', () => askOrigin(forNode))
    return () => {
      offOrigin()
      offTree()
    }
  })
  /** The rows an overlay sets in a section, by name: the tooltip names the overlay. Spec overlays and
   *  flowi overlays both tag rows; a header matches whatever case the row is written in. */
  function overlayMarks(section: 'query' | 'header', rows: { Key: string }[] | null = null): Record<string, string> {
    const marks: Record<string, string> = Object.fromEntries((origin?.fields ?? []).filter((f) => f.section === section).map((f) => [f.key, `Set by the overlay ${f.file}`]))
    for (const m of view.flowiOverlayMarks) {
      if (m.section !== section) continue
      const row = (rows ?? []).find((r) => r.Key.toLowerCase() === m.key.toLowerCase())
      marks[row?.Key ?? m.key] = `Set by the flowi overlay ${m.overlay}`
    }
    return marks
  }
  const fieldNames: Record<string, string> = { docs: 'docs', body: 'body', query: 'query', header: 'header' }
  const overlayFields = $derived((origin?.fields ?? []).map((f) => (f.key ? `${fieldNames[f.section]} ${f.key}` : fieldNames[f.section])))
  async function openUpdate() {
    if (!origin) return
    try {
      const specs = (await ManagerService.KeptSpecs(origin.collection)) as unknown as KeptSpec[]
      updating = specs.find((k) => k.name === origin?.name) ?? null
    } catch (e) {
      fail(e)
    }
  }
  let replayOpen = $state(kept.replay)
  let matrixOpen = $state(false)
  let hooks = $state(kept.hooks)
  $effect(() => setNodeUi(node, { section: chosen, hooks, replay: replayOpen }))

  const bodyKinds = ['none', 'text', 'form', 'file']
  const formats = ['json', 'xml', 'yaml', 'raw']
  const authKinds: [string, string][] = [['none', 'None'], ['bearer', 'Bearer token'], ['basic', 'Basic'], ['other', 'Other']]
  const bodyText = $derived(v.Body.JSON || v.Body.Text)
  // raw has no colouring of its own; its {{templates}} are still marked.
  // A body typed as XML is checked as it is written: the first problem and its line, or a quiet word that it reads.
  const xmlProblem = $derived.by(() => {
    if (view.bodyFormat !== 'xml' || !bodyText.trim()) return null
    const got = parseXml(bodyText)
    return 'error' in got ? got.error : null
  })
  // A JSON body is checked the same way, and Beautify indents it.
  const jsonBad = $derived(view.bodyFormat === 'json' && view.bodyKind === 'text' ? jsonProblem(bodyText) : null)
  async function beautifyBody() {
    const [pretty, ok] = (await RequestService.PrettyJSON(bodyText)) as unknown as [string, boolean]
    if (ok) RequestService.SetBodyText(node, pretty)
    else ui.toast('The body is not valid JSON, so it cannot be formatted. A template must be inside a string to count as JSON.', 'error')
  }
  const codeLangOf = (format: string): CodeLang => (format === 'json' ? 'json' : format === 'yaml' ? 'yaml' : format === 'xml' ? 'xml' : 'text')

  // A method the lists lack (typed through Custom…) shows as the current choice, so the button is never blank.
  const methodOptions = $derived<DropOption[]>([
    ...methods.map((m) => ({ value: m, label: m, tone: m })),
    ...(v.Method && !methods.includes(v.Method) ? [{ value: v.Method, label: v.Method, tone: v.Method }] : []),
    { value: '', label: '', separator: true },
    { value: CUSTOM, label: 'Custom…', detail: 'any method name' },
  ])
  function pickMethod(m: string) {
    if (m !== CUSTOM) return RequestService.SetMethod(node, m).catch(fail)
    ui.prompt({ title: 'Custom method', label: 'Method name (case matters: PURGE is not Purge)', initial: '', done: (name) => RequestService.SetMethod(node, name.trim()).catch(fail) })
  }
  const bodyKindOptions: DropOption[] = [['none', 'None'], ['text', 'Text'], ['form', 'Form'], ['file', 'File']].map(([value, label]) => ({ value, label }))
  const formatOptions: DropOption[] = formats.map((f) => ({ value: f, label: f.toUpperCase() }))
  const authOptions = $derived<DropOption[]>([...authKinds.map(([value, label]) => ({ value, label })), { value: 'use', label: 'Named authentication' }, ...view.auth.schemes.map((s) => ({ value: s.id, label: s.label }))])
  // The collection's named authentications, and the one in use even when it is gone, so the picker can say so.
  const namedOptions = $derived<DropOption[]>([...(view.auth.useMissing ? [{ value: view.auth.use, label: view.auth.use + ' (missing)' }] : []), ...view.auth.named.map((a) => ({ value: a.id, label: a.label }))])
  // Keeps the request's own scheme and values as a named authentication of its collection, and uses it from here.
  function keepAuth() {
    ui.prompt({
      title: 'Save as a named authentication',
      label: 'Name (other requests pick it by this name; it is kept in the collection\'s auths folder)',
      initial: '',
      done: (n) => AuthsService.FromRequest(node, n),
    })
  }
  const openAuths = () => app.openScreen('screen:auths', 'Authentications')
  const isScheme = $derived(view.auth.kind.startsWith('scheme:'))
  function pickAuth(k: string) {
    if (k === 'use') {
      // With none defined there is nothing to pick: the screen that makes one opens instead.
      if (view.auth.named.length) RequestService.SetAuthUse(node, view.auth.use || view.auth.named[0].id)
      else openAuths()
    } else if (k.startsWith('scheme:')) RequestService.SetAuthScheme(node, k.slice('scheme:'.length))
    else RequestService.SetAuth(node, k, view.auth.value)
  }
  const protocolOptions = $derived<DropOption[]>((view.async?.protocols ?? []).map((p) => ({ value: p.id, label: p.label, tone: protocolVerb(p.id) })))
  // Switching to a mode that has no room for what was written asks first: the other side is deleted.
  function chooseMode(m: string) {
    const lost = modeLoss(view.async?.block, m)
    if (!lost) return void RequestService.SetMode(node, m)
    ui.confirm('Switch the mode?', `Switching to ${modeLabels[m] ?? m} deletes ${lost}.`, () => RequestService.SetMode(node, m), 'Switch')
  }
  const modeLabels: Record<string, string> = { send: 'Send', receive: 'Receive', send_receive: 'Send, then receive' }
  // The button says what a message request will do, in its own colour: it said Send while set to receive.
  const actions: Record<string, { label: string; tone: string }> = {
    send: { label: 'Send', tone: 'send' },
    receive: { label: 'Receive', tone: 'receive' },
    send_receive: { label: 'Send & receive', tone: 'both' },
  }
  const action = $derived(actions[view.async?.mode ?? 'send'] ?? actions.send)
  const hookModes: [string, string][] = [['all', 'Run all hooks'], ['none', 'Run no hooks'], ['skip-pre', 'Skip the hooks before'], ['skip-post', 'Skip the hooks after']]

  const fail = (e: unknown) => ui.toast(errorText(e), 'error')
  // Writes the working copy over the saved version. With a flowi overlay laid over it, that includes the overlay.
  function saveVersion() {
    const server = view.dataset?.block.Server ?? ''
    if (view.kind === 'dataset' && !isFileDriver(view.dataset?.block.Driver ?? '') && serverHasPassword(server)) return askPasswordMove(server)
    askBreaking()
  }

  // An edit that breaks what uses the request (a flow that reads a value the request no longer extracts) is said
  // before it is saved. A request nothing uses, a new one and a check that cannot be made save without asking.
  async function askBreaking() {
    let lines: string[] = []
    if (view.dirty && !view.inScratch) lines = await RequestService.BreakingChanges(node).catch(() => [])
    if (!lines.length) return writeVersion()
    const shown = lines.slice(0, 6).join('\n') + (lines.length > 6 ? `\n… and ${lines.length - 6} more` : '')
    ui.unsaved(
      'This change breaks something that uses this request',
      `Saving it makes ${lines.length === 1 ? 'this' : 'these'} stop working:\n${shown}`,
      writeVersion,
      () => {},
      { save: 'Save anyway', discard: 'Do not save' },
    )
  }
  function writeVersion() {
    if (view.inScratch) saveTo()
    else RequestService.Save(node)
  }

  // A password written in a connection URL would be saved in the request's file, often committed: offer to keep it
  // in a secret of the selected environment instead. Not forced: "Save as is" saves it as it is.
  function askPasswordMove(server: string) {
    const envs = app.tree?.environments ?? []
    if (!envs.length) {
      ui.toast('The password is saved in the request file. To keep it in a secret, create an environment first (Environments in the sidebar), then save again.', 'error')
      return askBreaking()
    }
    const selected = envs.includes(app.tree?.selected ?? '') ? (app.tree?.selected ?? '') : envs[0]
    ui.unsaved(
      'Password in the connection string',
      `The password is saved in this request's file, which is often committed. Move it into a secret and write {{secret.name}} in its place?`,
      () =>
        ui.prompt({
          title: 'Keep the password in a secret',
          label: 'Name of the secret (its value is stored in the vault, never in a file)',
          initial: 'db_password',
          select: { label: 'For the environment', options: [selected, ...envs.filter((e) => e !== selected)].map((e) => ({ id: e, label: e })) },
          done: (name, _protocol, env) => void moveToSecret(env || selected, name.trim(), server),
        }),
      askBreaking,
      { save: 'Move to a secret…', discard: 'Save as is' },
    )
  }
  // The system's dialog picks the file or folder; the request keeps it relative to its collection when it lies inside.
  async function pickDataPath(folder: boolean) {
    try {
      const picked = await RequestService.PickDataPath(node, folder)
      const db = view.dataset?.block
      if (picked && db) RequestService.SetDataset(node, { ...db, Server: picked } as never)
    } catch (e) {
      fail(e)
    }
  }

  async function moveToSecret(env: string, name: string, server: string) {
    const moved = /^[A-Za-z_][A-Za-z0-9_]*$/.test(name) ? moveServerPassword(server, name) : null
    if (!moved) return ui.toast('A secret name is letters, digits and _, and starts with a letter or _.', 'error')
    const problem = await SecretsService.Declare(env, name, moved.password)
    if (problem) return ui.toast(problem, 'error')
    const block = structuredClone($state.snapshot(view.dataset!.block))
    block.Server = moved.server
    await RequestService.SetDataset(node, block as never)
    ui.toast(`The password is now the secret "${name}" of "${env}".`)
    askBreaking()
  }
  // Save (the button, ⌘S): with a flowi overlay open, the edit belongs to the overlay.
  function save() {
    if (view.flowiOverlay && view.dirty && !view.inScratch) saveOverlayOpen = true
    else saveVersion()
  }

  // Flowi overlays: laid over the open version beside the version picker. What does not fit shows once
  // for each overlay and version, as a popup; the bar below the header keeps the list.
  let saveOverlayOpen = $state(false)
  let overlaysOpen = $state(false)
  let warnedKey = $state('')
  let warningOpen = $state(false)
  const overlayKey = $derived(`${view.flowiOverlay}@${view.selected}:${view.flowiOverlayNotes.join('|')}`)
  $effect(() => {
    if (!view.flowiOverlay || !view.flowiOverlayNotes.length) {
      warnedKey = ''
      return
    }
    if (overlayKey !== warnedKey) {
      warnedKey = overlayKey
      warningOpen = true
    }
  })
  // Overlays can be stacked ("vip+slow"): they apply in the order named and a later one wins.
  const stack = $derived(view.flowiOverlay ? view.flowiOverlay.split('+') : [])
  function unstackOverlay(name: string) {
    RequestService.SelectFlowiOverlay(node, stack.filter((n) => n !== name).join('+'))
  }
  function newVersion() {
    ui.prompt({ title: 'New version', label: 'Label', initial: '', done: (label) => RequestService.NewVersion(node, label).catch(fail) })
  }
  async function deleteVersion(drop = false) {
    try {
      if (await RequestService.DeleteVersion(node, drop)) {
        ui.confirm('Comments will be lost', 'The request file has comments; rewriting it without this version removes them. Delete anyway?', () => deleteVersion(true), 'Delete')
      }
    } catch (e) {
      fail(e)
    }
  }
  const sendItems = $derived.by<MenuItem[]>(() => {
    const out: MenuItem[] = []
    const modes = view.async?.modes ?? []
    for (const m of modes) out.push({ label: modeLabels[m] ?? m, checked: view.async?.mode === m, onselect: () => chooseMode(m) })
    hookModes.forEach(([m, label], i) =>
      out.push({ label, checked: hooks === m, onselect: () => (hooks = m), heading: i === 0 ? 'When sending' : undefined, separated: i === 0 && modes.length > 0 }),
    )
    // Only an HTTP request has the replay bar: on any other kind the item toggled a tick and showed nothing.
    const blocked = view.inScratch || view.dirty
    if (view.kind === 'http') out.push({ label: 'Replay saved responses…', checked: replayOpen, onselect: () => (replayOpen = !replayOpen), separated: true })
    out.push({
      label: 'Run a matrix…',
      disabled: blocked,
      title: blocked ? 'Save the request first: a matrix sends the file on disk' : 'Send it once for every combination of environments, overlays and values',
      onselect: () => (matrixOpen = true),
      separated: view.kind !== 'http',
    })
    return out
  })
  const saveItems = $derived.by<MenuItem[]>(() => {
    const out: MenuItem[] = []
    if (view.flowiOverlay) {
      out.push({ label: `Save to flowi overlay ${view.flowiOverlay}…`, hint: '⌘S', disabled: !view.dirty, onselect: () => (saveOverlayOpen = true) })
      out.push({ label: 'Save into the version, with the overlay', title: "Writes the overlay's changes into the version itself", onselect: saveVersion })
    } else {
      out.push({ label: 'Save current version', hint: '⌘S', onselect: saveVersion })
    }
    out.push({ label: 'Save as new version…', onselect: newVersion })
    if (!view.inScratch) {
      out.push({ label: 'Save as new request…', title: 'Copies the request as saved, under a new name or in another place', onselect: saveAsNew })
      out.push({ label: 'Save as flowi overlay…', disabled: !view.dirty && !view.flowiOverlay, title: 'Keeps the changes apart from the version, to lay over it when sending', onselect: () => (saveOverlayOpen = true) })
    }
    if (origin?.spec) out.push({ label: "Save to the API spec's overlay file…", title: `Also write what the spec can say to an overlay on ${origin.spec}`, onselect: () => (overlayOpen = true) })
    out.push({ label: 'Set as default version', onselect: () => RequestService.SetDefault(node) })
    out.push({ label: 'Version title and description…', onselect: () => (detailsOpen = true) })
    out.push({
      label: 'Delete this version…',
      danger: true,
      separated: true,
      onselect: () => ui.confirm('Delete version?', `Version ${view.selected} is removed from the request file. This cannot be undone.`, () => deleteVersion(), 'Delete'),
    })
    if (!view.inScratch) out.push({ label: 'Delete this request…', danger: true, onselect: () => ui.confirm('Delete request', `Delete ${view.name}?`, () => WorkspaceService.Do('delete', [node], '', ''), 'Delete') })
    return out
  })

  function newExtraction(): Extraction {
    return { Name: '', From: 'body', Path: '', Header: '', Cookie: '', Regex: '' }
  }
  const selector = (e: Extraction) => ({ body: e.Path, header: e.Header, cookie: e.Cookie, status: '' })[e.From as 'body'] ?? ''
  function setSelector(rows: Extraction[], i: number, text: string) {
    const e = rows[i]
    if (e.From === 'body') e.Path = text
    else if (e.From === 'header') e.Header = text
    else if (e.From === 'cookie') e.Cookie = text
    RequestService.SetExtract(node, rows)
  }
  let extract = $state<Extraction[]>([])
  $effect(() => {
    extract = (v.Extract ?? []).map((e) => ({ ...e }))
  })

  // A try-out request is kept by choosing where; its name starts from its name or its URL.
  async function saveTo() {
    ui.place = {
      title: 'Save request',
      action: 'Save',
      tree: 'requests',
      name: { label: 'Name', initial: await RequestService.SuggestSavePath(node), required: true },
      done: (row, name) => RequestService.SaveTo(node, row, name),
    }
  }

  // A copy of a request that is kept somewhere: another name, another folder or another collection. The
  // original stays as it is.
  async function saveAsNew() {
    ui.place = {
      title: 'Save as new request',
      action: 'Save a copy',
      tree: 'requests',
      name: { label: 'Name', initial: await RequestService.SuggestCopyName(node), required: true },
      note: view.dirty ? 'Unsaved changes stay in this tab and are not in the copy.' : undefined,
      row: await RequestService.PlaceOf(node),
      done: (row, name) => RequestService.SaveAsNew(node, row, name),
    }
  }

  // A curl command, a .http request or an API description pasted into the address is offered for
  // filling this request or importing, instead of landing in the field as text.
  function onpaste(e: ClipboardEvent) {
    const text = e.clipboardData?.getData('text/plain') ?? ''
    const kind = recognise(text)
    if (!kind) return
    e.preventDefault()
    app.pasteOffer = { node, kind, text, http: view.kind === 'http', field: e.target instanceof HTMLElement ? e.target : null }
  }

  // A protected environment asks before a request that changes data goes out.
  // The click shows at once: Go takes a moment to report that it is sending, and a second click in that moment
  // sent the request twice. Before: the button waited for Go to answer before it changed.
  let pending = $state(false)
  function dispatch() {
    if (pending) return
    pending = true
    void Promise.resolve(RequestService.Send(node, hooks)).finally(() => (pending = false))
  }
  function send() {
    const env = app.tree?.selected ?? ''
    const shape = { kind: view.kind, method: view.current.Method, writes: !!view.dataset?.block.Writes }
    if (sendNeedsConfirm(env, app.tree?.protected ?? [], shape)) {
      const what = view.kind === 'http' ? `${view.current.Method.toUpperCase()} ${view.fullUrl}` : view.name
      ui.confirm(`Send to ${env}?`, `${env} is a protected environment. This request changes data: ${what}`, dispatch, 'Send')
    } else {
      dispatch()
    }
  }

  // The palette's Send command: only the editor that is showing sends.
  let seenSend = app.sendRequests
  $effect(() => {
    if (app.sendRequests === seenSend) return
    seenSend = app.sendRequests
    if (app.active === node) send()
  })

  function onkeydown(e: KeyboardEvent) {
    // A dialog is open: the keys belong to it. Before, Ctrl+Enter here still sent the request behind the dialog.
    if (modalOpen()) return
    if ((e.metaKey || e.ctrlKey) && isLetter(e, 's')) {
      e.preventDefault()
      save()
    }
    // Before: the Send button and the palette advertised Ctrl/Cmd+Enter but nothing handled it. A key that is
    // held down repeats, so a repeat is ignored: it must not send again.
    if (isSendKey(e)) {
      e.preventDefault()
      send()
    } else if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault() // the key that sends, held down or with Shift or Alt: not a send, and no new line either
    }
  }
</script>

<!-- The engine's own steps after the hooks (schema learning), which no hook names. -->
{#snippet builtIns()}
  {#each view.pipeline.filter((p) => p.kind === 'builtin') as b}
    <p class="muted builtin">Built in, after the hooks: <strong>{b.title}</strong> {b.state === 'not-run' ? `is off (${b.reason})` : `— ${b.detail}`}</p>
  {/each}
{/snippet}

<svelte:window {onkeydown} />

<div class="req">
  <header {onpaste}>
    <!-- The method and the address are one field: one outline and one focus ring around both. -->
    <div class="addr">
    {#if view.kind === 'http'}
      <Dropdown pill title="Method" options={methodOptions} value={v.Method} width="108px" onpick={pickMethod} />
      <div class="url">
        <TemplateField
          value={view.fullUrl}
          placeholder={'{{env.base_url}}/path'}
          oninput={(t) => RequestService.SetURL(node, t)}
          onenter={() => send()}
          spans={(t) => RequestService.URLSpans(node, t) as unknown as Promise<any>}
          suggest={(t, c) => RequestService.Suggest(node, t, c) as unknown as Promise<any>}
        />
      </div>
      {#if view.connection}<ConnectionTest {node} info={view.connection} kind="http" />{/if}
      {#if serverHasPassword(view.fullUrl)}
        <span class="pwwarn" role="note" title={PASSWORD_NOTE}>⚠ Password in the URL</span>
      {/if}
    {:else if view.async}
      {@const ab = view.async.block}
      <Dropdown pill title="Protocol" options={protocolOptions} value={ab.Protocol} width="110px" onpick={(p) => RequestService.SetProtocol(node, p)} />
      <div class="url2">
        <TemplateField aria="Server" value={ab.Server} placeholder={view.async.serverExample} oninput={(t) => RequestService.SetAsync(node, { ...ab, Server: t } as never)} onenter={() => send()}  spans={(t) => RequestService.URLSpans(node, t) as unknown as Promise<any>} suggest={(t, c) => RequestService.Suggest(node, t, c) as unknown as Promise<any>} />
        <TemplateField aria={view.async.channelLabel} value={ab.Channel} placeholder={view.async.channelHint} oninput={(t) => RequestService.SetAsync(node, { ...ab, Channel: t } as never)} onenter={() => send()}  spans={(t) => RequestService.TemplateSpans(node, t) as unknown as Promise<any>} suggest={(t, c) => RequestService.Suggest(node, t, c) as unknown as Promise<any>} />
      </div>
      {#if view.connection}<ConnectionTest {node} info={view.connection} />{/if}
    {:else if view.dataset}
      {@const db = view.dataset.block}
      {@const fileSource = isFileDriver(db.Driver)}
      <!-- A file source switches only between file types, and a database only between databases: they are different kinds of source. -->
      <Dropdown pill title={fileSource ? 'File type' : 'Database'} options={view.dataset.drivers.filter((x) => isFileDriver(x.id) === fileSource).map((x) => ({ value: x.id, label: x.label, tone: protocolVerb(x.id) }))} value={db.Driver} width="130px"
        onpick={(id) => RequestService.SetDataset(node, { ...db, Driver: id } as never)} />
      <div class="url">
        <TemplateField aria="Server" value={db.Server} placeholder={fileSource ? 'data/sales.csv (a file or a folder in the collection)' : db.Driver === 'sqlite' ? 'shop.db (a file in the collection)' : view.dataset.serverExample}
          oninput={(t) => RequestService.SetDataset(node, { ...db, Server: t } as never)} onenter={() => send()}  spans={(t) => RequestService.URLSpans(node, t) as unknown as Promise<any>} suggest={(t, c) => RequestService.Suggest(node, t, c) as unknown as Promise<any>} />
      </div>
      {#if fileSource || db.Driver === 'sqlite'}
        <span class="pick">
          <Button type="button" title={fileSource ? 'Choose a data file' : 'Choose the database file'} onclick={() => pickDataPath(false)}>File…</Button>
          {#if fileSource}<Button type="button" title="Choose a folder of data files" onclick={() => pickDataPath(true)}>Folder…</Button>{/if}
        </span>
      {/if}
      {#if view.connection}<ConnectionTest {node} info={view.connection} kind={fileSource ? 'file' : 'connect'} />{/if}
      {#if !fileSource && serverHasPassword(db.Server)}
        <span class="pwwarn" role="note" title={PASSWORD_NOTE}>⚠ Password in the file</span>
      {/if}
    {:else if view.grpc}
      {@const gb = view.grpc.block}
      <Verb verb="GRPC" />
      <div class="url2">
        <TemplateField aria="Server" value={gb.Server} placeholder={'grpc://{{env.host}}:50051'} oninput={(t) => RequestService.SetGRPC(node, { ...gb, Server: t } as never)} onenter={() => send()}  spans={(t) => RequestService.URLSpans(node, t) as unknown as Promise<any>} suggest={(t, c) => RequestService.Suggest(node, t, c) as unknown as Promise<any>} />
        <MethodPicker {node} info={view.grpc} />
      </div>
      {#if view.connection}<ConnectionTest {node} info={view.connection} kind="grpc" />{/if}
    {/if}
    {#if hookBadge(hooks)}
      {@const badge = hookBadge(hooks)!}
      <span class="hookbadge"><Button small title={badge.title} onclick={() => (hooks = 'all')}><Icon name="hooks" size={12} /> {badge.text}</Button></span>
    {/if}
    {#if view.snippet.text}
      <IconButton icon="code" label={copyCodeLabel(view.snippet.format)} onclick={() => { chosen = 'Code'; void copyOut(view.snippet.text, snippetCopied(!!view.snippet.secrets, view.snippet.exclude)) }} />
    {/if}
    <div class="send" class:cancel={view.sending} data-tone={view.sending ? 'send' : action.tone} style:--act={`var(--act-${action.tone})`}>
      {#if view.sending}
        <button class="act main" aria-label="Cancel the send, running for {elapsedText(sendMs)}" onclick={() => sendMs >= 400 && RequestService.Cancel(node)}><span class="stop" aria-hidden="true"></span>Cancel<span class="elapsed">{elapsedText(sendMs)}</span></button>
      {:else}
        <button class="act main" disabled={pending} aria-busy={pending} onclick={() => send()}>{action.label}<kbd>{isMac() ? '⌘↵' : 'Ctrl+↵'}</kbd></button>
      {/if}
      <button class="act more" aria-label="Send options" aria-haspopup="menu" aria-expanded={menu === 'send'} disabled={view.sending} onclick={(e) => openMenu('send', e)}><Icon name="chevd" size={14} /></button>
      {#if menu === 'send'}<Menu at={menuAt} minWidth={150} label="Send options" items={sendItems} onclose={() => (menu = null)} />{/if}
    </div>
    </div>
    {#if statusMark}{@const mark = statusMark}<IconButton icon={mark.icon} size={15} label="{statusTitle(mark)}. Change it in Docs." style="color: var(--st-{mark.color}); opacity: {mark.inherited ? 0.75 : 1}" onclick={() => (chosen = 'Docs')} />{/if}
  </header>
  <RequestTypeBar {view} {node} onmanage={() => (overlaysOpen = true)} onnew={newVersion}>
    <!-- The pill and Discard keep their room while the version is clean, so the header does not shift when an edit begins. -->
    <span class="edited" class:clean={!view.dirty} aria-hidden={!view.dirty} inert={!view.dirty}>
      <span class="pill" role="status">● Edited</span>
      <Button small variant="ghost" title="Throw away the unsaved changes" onclick={() => ui.confirm('Discard changes', 'Throw away the unsaved changes of this version?', () => RequestService.DiscardEdit(node), 'Discard')}>Discard</Button>
    </span>
    <!-- Saving as a new version, and the other version actions, sat at the end of the Send menu where
         nobody looked for them; they open from the Save button now. -->
    <div class="send savegrp">
      {#if view.inScratch}
        <button class="saveicon main pending" title="Save to a collection…" aria-label="Save to a collection" onclick={saveTo}><Icon name="save" size={15} /></button>
      {:else}
        <button class="saveicon main" class:pending={view.dirty} title={view.flowiOverlay ? `Save to the flowi overlay ${view.flowiOverlay} (⌘S)` : 'Save (⌘S)'} aria-label="Save" disabled={!view.dirty} onclick={save}><Icon name="save" size={15} /><span class="savetext">Save</span></button>
      {/if}
      <button class="more" aria-label="Save options" aria-haspopup="menu" aria-expanded={menu === 'save'} onclick={(e) => openMenu('save', e)}><Icon name="chevd" size={14} /></button>
      {#if menu === 'save'}<Menu at={menuAt} minWidth={150} label="Save options" items={saveItems} onclose={() => (menu = null)} />{/if}
    </div>
  </RequestTypeBar>
  {#if detailsOpen}
    <VersionDetails
      label={view.selected}
      name={v.Name ?? ''}
      docs={v.Docs ?? ''}
      onsave={(n, d) => { detailsOpen = false; RequestService.SetVersionDetails(node, n, d) }}
      oncancel={() => (detailsOpen = false)}
    />
  {/if}
  {#if overlayOpen}<OverlayDialog {node} onclose={() => (overlayOpen = false)} />{/if}
  {#if saveOverlayOpen}<FlowiOverlayDialog {node} onclose={() => (saveOverlayOpen = false)} />{/if}
  {#if overlaysOpen}<FlowiOverlayEditor {node} overlays={view.flowiOverlays} current={view.flowiOverlay} onclose={() => (overlaysOpen = false)} />{/if}
  {#if warningOpen && view.flowiOverlay}<FlowiOverlayWarning overlay={view.flowiOverlay} notes={view.flowiOverlayNotes} version={view.selected} onclose={() => (warningOpen = false)} />{/if}
  {#if updating && origin}<SpecUpdateDialog collection={origin.collection} spec={updating} onclose={() => (updating = null)} />{/if}
  <FileBanner dirty={view.dirty} onDisk={view.onDisk} needsDrop={view.needsDrop} revert={() => RequestService.Revert(node)} compare={() => RequestService.CompareDisk(node) as Promise<DiskDiff>} saveDropping={() => RequestService.SaveDroppingComments(node)} declineDrop={() => RequestService.DeclineDrop(node)} />
  {#if view.error}<p class="bar err"><Problem text={view.error} /></p>{/if}
  {#if view.flowiOverlay}
    {@const info = stack.length === 1 ? view.flowiOverlays.find((o) => o.name === view.flowiOverlay) : undefined}
    <p class="bar overlaybar">
      Flowi {stack.length > 1 ? 'overlays' : 'overlay'}
      {#each stack as n, i}{#if i}<span class="then">&nbsp;then&nbsp;</span>{/if}<strong>{n}</strong>{#if stack.length > 1}<IconButton icon="close" size={12} danger label={`Take ${n} off`} onclick={() => unstackOverlay(n)} />{/if}{/each}
      {stack.length > 1 ? 'are' : 'is'} laid over version {view.selected}{#if info} · {info.shared} shared {info.shared === 1 ? 'change' : 'changes'}{info.used ? `, ${info.own} for this request` : ''}{/if}.
      Sends use the request as shown; their responses are kept apart from the version's.
      {#if view.flowiOverlayNotes.length}<Button variant="link" onclick={() => (warningOpen = true)}>{view.flowiOverlayNotes.length} not applied in full</Button>{/if}
      · <Button variant="link" onclick={() => RequestService.SelectFlowiOverlay(node, '')}>Take it off</Button>
    </p>
  {/if}
  <MissingBar {view} />
  {#if origin?.spec}
    <p class="bar specbar">
      From the spec <code>{origin.spec}</code>
      {#if overlayFields.length}
        · <span title={origin.fields.map((f) => `${f.key ? `${f.section} ${f.key}` : f.section}: ${f.file}`).join('\n')}>{overlayFields.length} {overlayFields.length === 1 ? 'field' : 'fields'} set by overlays: {overlayFields.join(', ')}</span>
      {/if}
      · <Button variant="link" onclick={openUpdate}>Update from spec…</Button>
    </p>
  {/if}
  {#if matrixOpen}<RequestMatrix {node} onclose={() => (matrixOpen = false)} />{/if}
  {#if view.kind === 'http'}<ReplayBar path={view.replayPath} method={v.Method || 'GET'} open={replayOpen} onclose={() => (replayOpen = false)} />{/if}

  <div class="panels">
    <div class="left" style:width={`${leftWidth}px`}>
      {#if true}
        <div class="secnav" bind:clientWidth={navWidth}>
          <!-- Only the tabs scroll: a scrolling box would clip the More menu as well. -->
          <div class="sectabs" role="tablist" aria-label="Request sections" tabindex="-1">
          {#each mainSecs as sec}
            <!-- Icons keep six tabs and More in a narrow pane; the open tab also shows its name, the others say it on hover. -->
            <button role="tab" aria-selected={section === sec} aria-label={titleOf(sec)} title={titleOf(sec)} class="tab" class:on={section === sec} onclick={() => (chosen = sec)}>
              <Icon name={iconOf(sec)} size={15} />{#if section === sec}<span class="tname">{titleOf(sec)}</span>{/if}{#if counts[sec]}<span class="count tabcount" title={`${counts[sec]} ${counts[sec] === 1 ? 'entry' : 'entries'}`}>{counts[sec]}</span>{:else if configured(sec)}<i class="cdot" title="Has content"></i>{/if}
            </button>
          {/each}
          </div>
          {#if moreSecs.length}
            <span class="more-secs" class:on={moreSecs.includes(section)} bind:clientWidth={moreWidth}>
              <Dropdown icon title="More sections" label="More" options={moreOptions} value={moreSecs.includes(section) ? section : ''} onpick={(sec) => (chosen = sec)} />
            </span>
          {/if}
        </div>
        {#if section === 'Body'}
          <div class="typebar">
            <Dropdown title="Body type" options={bodyKindOptions} value={view.bodyKind} onpick={(k) => RequestService.SetBodyKind(node, k)} />
            {#if view.bodyKind === 'text'}
              <Dropdown title="Format" options={formatOptions} value={view.bodyFormat} onpick={(f) => RequestService.SetBodyFormat(node, f)} />
            {/if}
            {#if view.bodyKind === 'text' && view.bodyFormat === 'json'}
              {#if view.jsonrpc}
                <span class="rpc" title="This body is a JSON-RPC 2.0 call. Its parameters are edited above.">
                  <span class="muted">JSON-RPC</span>
                  <input class="mini" aria-label="JSON-RPC method" placeholder="method" value={view.jsonrpc.method} onchange={(e) => RequestService.SetJSONRPC(node, 'method', e.currentTarget.value)} />
                  <input class="mini id" aria-label="JSON-RPC id" placeholder="id" title="The call's id. Empty makes it a notification: no id, no answer." value={view.jsonrpc.id} onchange={(e) => RequestService.SetJSONRPC(node, 'id', e.currentTarget.value)} />
                </span>
              {:else}
                <Button variant="link" title="Wrap this body as the parameters of a JSON-RPC 2.0 call" onclick={() => RequestService.MakeJSONRPC(node)}>Make it a JSON-RPC call</Button>
              {/if}
            {/if}
            {#if view.bodyKind === 'form'}
              <Toggle checked={view.multipart} onchange={(on) => RequestService.SetMultipart(node, on)}>Multipart</Toggle>
            {/if}
          </div>
        {:else if section === 'Auth'}
          <div class="typebar"><Dropdown title="Auth type" options={authOptions} value={view.auth.kind} onpick={pickAuth} /></div>
        {/if}
        <div class="section">
          {#if view.kind === 'dataset' && section === 'Credentials'}
            <DatasetSections {view} {section} />
          {:else if ['Message', 'Receive', 'Credentials', 'Binding', 'Proto'].includes(section)}
            <MessageSections {view} {section} />
          {:else if ['Query', 'Values'].includes(section)}
            <DatasetSections {view} {section} />
          {:else if section === 'Params'}
            <p class="muted fieldnote">Write <code>{'{{name}}'}</code> for a variable. <ExprHelp context="request.fields" /></p>
            <KVRows spans={(t) => RequestService.TemplateSpans(node, t) as unknown as Promise<any>} suggest={(t, c) => RequestService.Suggest(node, t, c) as unknown as Promise<any>} rows={v.Query} describe toggle marks={overlayMarks('query', v.Query)} onchange={(r) => RequestService.SetRows(node, 'query', r)}
              empty={{ icon: 'params', title: 'No query parameters', hint: 'Add one here, or type ?name=value at the end of the address.', add: 'Add parameter' }} />
          {:else if section === 'Headers'}
            <p class="muted fieldnote">Write <code>{'{{name}}'}</code> for a variable. <ExprHelp context="request.fields" /></p>
            {@const sendHeaders = view.async && !view.async.headersAreConnection}
            <KVRows spans={(t) => RequestService.TemplateSpans(node, t) as unknown as Promise<any>} suggest={(t, c) => RequestService.Suggest(node, t, c) as unknown as Promise<any>} rows={sendHeaders ? (view.async?.block.Send?.Headers ?? null) : v.Headers} describe toggle={!sendHeaders} keyLabel="header" suggestKeys={view.kind === 'http' ? COMMON_HEADERS : []} valuesOf={headerValues} marks={sendHeaders ? {} : overlayMarks('header', v.Headers)} onchange={(r) => RequestService.SetRows(node, 'headers', r)}
              empty={{ icon: 'headers', title: `No ${headerWord(view.kind)}s`, hint: view.kind === 'grpc' ? 'Metadata travels with the call, the way headers do for HTTP. Credentials go in Auth.' : headersHint(view.async ? (sendHeaders ? 'send' : 'connect') : 'http'), add: `Add ${headerWord(view.kind)}` }} />
            {#if view.kind === 'http' && !view.async}
              {@const implicit = implicitHeaders({ body: v.Body, headers: (v.Headers ?? []).filter((h) => !h.Disabled).map((h) => h.Key), hasAuth: ['bearer', 'basic', 'header'].includes(view.auth.kind) })}
              {#each implicit as h (h.name)}
                <p class="muted implicit"><code>{h.name}: {h.value}</code> <span>Sent by Flowi, {h.source}. Add a header of the same name to replace it.</span></p>
              {/each}
            {/if}
          {:else if section === 'Body'}
            {@const bodyFrom = origin?.fields.find((f) => f.section === 'body')}
            {#if bodyFrom}<p class="muted overlaynote">The example comes from the overlay <code>{bodyFrom.file}</code>.</p>{/if}
            {#if view.bodyKind === 'text'}
              <CodeArea value={bodyText} lang={codeLangOf(view.bodyFormat)} oninput={(t) => RequestService.SetBodyText(node, t)}
                empty={{ icon: 'body', title: 'Empty body', hint: 'Click to write the body to send.' }} />
              {#if xmlProblem}<p class="xmlnote bad" role="status">Not well-formed XML: line {xmlProblem.line}, column {xmlProblem.column}: {xmlProblem.message}</p>
              {:else if view.bodyFormat === 'xml' && bodyText.trim()}<p class="xmlnote" role="status">Well-formed XML</p>{/if}
              {#if jsonBad}<p class="xmlnote bad" role="status">Not valid JSON: line {jsonBad.line}, column {jsonBad.column}: {jsonBad.message}</p>
              {:else if view.bodyFormat === 'json' && bodyText.trim()}<p class="xmlnote" role="status">Valid JSON <Button type="button" variant="ghost" small onclick={beautifyBody}>Beautify</Button></p>{/if}
            {:else if view.bodyKind === 'form' && !view.multipart}
              <KVRows spans={(t) => RequestService.TemplateSpans(node, t) as unknown as Promise<any>} suggest={(t, c) => RequestService.Suggest(node, t, c) as unknown as Promise<any>} rows={v.Body.Form} onchange={(r) => RequestService.SetFormRows(node, r)}
                empty={{ icon: 'list', title: 'No form fields', hint: 'Each field is sent as name=value.', add: 'Add field' }} />
            {:else if view.bodyKind === 'form'}
              <PartsEditor parts={view.parts} onchange={(p) => RequestService.SetParts(node, p as never)} />
            {:else if view.bodyKind === 'file'}
              <Field label="File path"><TextInput mono value={v.Body.File} ontext={(t) => RequestService.SetBodyFile(node, t)} /></Field>
            {:else}
              <EmptyState icon="body" title="No body" hint="Pick a body type above to send text, a form or a file." compact />
            {/if}
          {:else if section === 'Auth'}
            {#if view.auth.kind === 'none'}
              <EmptyState icon="auth" title="No auth" hint={'Pick a type above. Keep credentials out of the file: use {{secret.name}}.'} compact>
                <Button onclick={() => pickAuth('bearer')}>Bearer token</Button><Button onclick={() => pickAuth('basic')}>Basic</Button>
              </EmptyState>
            {:else if view.auth.kind === 'use'}
              <div class="pickrow"><Dropdown title="Named authentication" options={namedOptions} value={view.auth.use} onpick={(n) => RequestService.SetAuthUse(node, n)} /><Button onclick={openAuths}>Edit authentications…</Button></div>
              {#if view.auth.useMissing}
                <p class="muted">This collection has no authentication named {view.auth.use}. The request cannot be sent until you pick another or add it again.</p>
              {:else}
                <p class="muted">The request is sent with the collection's authentication {view.auth.use}. Its values are kept in one place: change them under Authentications.</p>
              {/if}
            {:else if isScheme}
              <AuthFields fields={view.auth.fields} onparam={(name, t) => RequestService.SetAuthParam(node, name, t)} />
              <div class="keepauth">
                <Button disabled={view.inScratch} title={view.inScratch ? 'Save the request into a collection first' : 'Keep this authentication under a name, so other requests can use it too. This request then uses it by name.'}
                  onclick={keepAuth}>Save as a named authentication…</Button>
              </div>
            {:else}
              <Field label={view.auth.kind === 'bearer' ? 'Token' : view.auth.kind === 'basic' ? 'Base64 credentials or a template' : 'Authorization header value'}><TextInput mono value={view.auth.value} ontext={(t) => RequestService.SetAuth(node, view.auth.kind, t)} /></Field>
              <p class="muted">Keep credentials out of the file: use {'{{secret.name}}'}.</p>
            {/if}
            {#if authSummary(view.auth.kind, view.auth.value, view.auth.fields)}
              <p class="sends">When sent, it {authSummary(view.auth.kind, view.auth.value, view.auth.fields).replace(/`([^`]+)`/g, '“$1”')}.</p>
            {/if}
          {:else if section === 'Extract' && !extract.length}
            <EmptyState icon="extract" title="Nothing extracted" hint="Take a value out of the response into a variable, for later requests and steps." compact>
              <Button onclick={() => extract.push(newExtraction())}>Add extraction</Button>
            </EmptyState>
          {:else if section === 'Extract'}
            <p class="muted">Take a value out of the response into a variable, for later requests and steps.{#if view.kind !== 'http'} <ExprHelp context={checkContext} />{/if}</p>
            {#each extract as e, i}
              <div class="row">
                <input class="k" placeholder="variable" bind:value={e.Name} oninput={() => RequestService.SetExtract(node, extract)} spellcheck="false" />
                <Select bind:value={e.From} onchange={() => RequestService.SetExtract(node, extract)}>
                  {#each ['body', 'header', 'cookie', 'status'] as f}<option value={f}>{f}</option>{/each}
                </Select>
                {#if e.From !== 'status'}
                  <input class="v" placeholder={e.From === 'body' ? '$.path.to.value' : 'name'} value={selector(e)} oninput={(ev) => setSelector(extract, i, ev.currentTarget.value)} spellcheck="false" />
                {/if}
                {#if extractChip(e.Name, view.response)}{@const chip = extractChip(e.Name, view.response)!}
                  <span class="xchip {chip.state}" title={chip.state === 'ok' ? 'What the last send took' : 'The last send found nothing at this path'}>{chip.text}</span>
                {/if}
                <IconButton icon="close" size={12} danger label="Remove" onclick={() => { extract.splice(i, 1); RequestService.SetExtract(node, extract) }} />
              </div>
            {/each}
            <Button variant="link" onclick={() => { extract.push(newExtraction()) }}>+ Add</Button>
          {:else if section === 'Assert'}
            {#if view.kind === 'http'}
              <label class="expect">
                <span>Expected status</span>
                <input class="mini" aria-label="Expected status" placeholder="any 2xx" value={v.Expect ?? ''} spellcheck="false"
                  title="A status (401), a range (400-499), a class (4xx) or a list (200, 404). The request passes on that status and fails on any other; the usual check for a 2xx stands aside."
                  onchange={async (e) => { expectProblem = await RequestService.SetExpect(node, e.currentTarget.value) }}
                  oninput={() => (expectProblem = '')} />
                {#if expectProblem}<span class="expect-problem" role="alert">{expectProblem}</span>{/if}
              </label>
            {/if}
            <p class="muted">One card per check, for example <code>{checkExample}</code>. A failing check fails the send. <ExprHelp context={checkContext} /></p>
            <AssertCards checks={v.Assert ?? []} outcome={view.response ?? undefined} example={checkExample} starters={view.kind === 'http' ? startersFor(view.response) : []}
              vocabulary={requestVocabulary(checkContext, node, view.response, view.kind)}
              evaluate={view.kind === 'http' ? (lines) => RequestService.EvalChecks(node, lines) : undefined}
              onchange={(c) => RequestService.SetAssert(node, c)} />
          {:else if section === 'Outcomes'}
            <OutcomesEditor outcomes={v.Outcomes} ruleKind={view.kind === 'dataset' ? 'dataset' : view.kind === 'http' ? 'http' : 'message'} onchange={(rows) => RequestService.SetOutcomes(node, rows)} />
          {:else if section === 'Variables'}
            {#if v.Variables?.length}<p class="muted">Default values for {'{{var.name}}'}; the environment's variable of the same name wins.</p>{/if}
            <KVRows rows={v.Variables} describe onchange={(r) => RequestService.SetRows(node, 'variables', r)}
              empty={{ icon: 'variables', title: 'No variables', hint: "Defaults for {{var.name}}; the environment's variable of the same name wins.", add: 'Add variable' }} />
          {:else if section === 'Settings'}
            <RequestExtras {view} which="settings" />
          {:else if section === 'Schema'}
            <RequestExtras {view} which="schema" />
          {:else if section === 'Hooks' && !view.hookChain.length}
            <EmptyState icon="hooks" title="No hooks" hint="Hooks run steps before the request is sent and after its response." compact>
              <Button onclick={() => app.openHooks(node, view.name)}>Add hooks…</Button>
            </EmptyState>
            {@render builtIns()}
          {:else if section === 'Hooks'}
            <p class="muted">Steps that run before the request is sent and after its response, from this request and the levels above it.</p>
            <ol class="chain">{#each view.hookChain as h}<li>{h}</li>{/each}</ol>
            <Button variant="link" onclick={() => app.openHooks(node, view.name)}>Edit this request's hooks…</Button>
            {@render builtIns()}
          {:else if section === 'Code'}
            <RequestExtras {view} which="snippet" />
          {:else if section === 'Responses'}
            <StoredResponses {view} />
          {:else if section === 'Used by'}
            <RequestExtras {view} which="usedby" />
          {:else if section === 'Cookies'}
            <RequestExtras {view} which="cookies" />
          {:else}
            <div class="labelsbox"><LabelsForm {node} /></div>
            <div class="docsbar"><Segmented size="xs" label="Notes view" items={[{ id: 'write', label: 'Write' }, { id: 'preview', label: 'Preview', disabled: !v.Docs }]} value={docsPreview ? 'preview' : 'write'} onpick={(id) => (docsPreview = id === 'preview')} /></div>
            {#if docsPreview && v.Docs}
              <div class="docsview"><Markdown text={v.Docs} /></div>
            {:else}
              <AreaField mono={false} value={v.Docs} oninput={(t) => RequestService.SetDocs(node, t)}
                empty={{ icon: 'docs', title: 'No notes', hint: 'Click to write about this request, in Markdown.' }} />
            {/if}
          {/if}
        </div>
      {/if}
    </div>
    <Splitter name="request-left" side="left" min={300} max={1000} bind:width={leftWidth} />
    <div class="right"><ResponsePane {view} onchecks={() => (chosen = 'Assert')} /></div>
  </div>
</div>

<style>
  .xmlnote { margin: 4px 0 0; font-size: var(--fs-xs); color: var(--muted); }
  .xmlnote.bad { color: var(--warn); }
  .req { position: absolute; inset: 0; display: flex; flex-direction: column; }
  header { display: flex; gap: 8px; padding: 12px 16px 4px; align-items: center; }
  .addr { flex: 1; min-width: 0; display: flex; align-items: center; height: var(--addr-h); box-sizing: border-box; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); transition: border-color var(--t-fast) var(--ease-out), box-shadow var(--t-fast) var(--ease-out); }
  .addr:focus-within { border-color: var(--accent); box-shadow: 0 0 0 3px var(--tint-accent); }
  /* The fields inside draw no edge of their own: the bar does. */
  .addr :global(input), .addr :global(.field), .addr :global(.box) { background: none; border: 0; box-shadow: none; min-height: 0; }
  .addr :global(.dd .btn) { background: none; border: 0; border-right: 1px solid var(--line); border-radius: var(--r-lg) 0 0 var(--r-lg); min-height: 36px; }
  .addr > .url2 { padding-right: 6px; }
  .url { flex: 1; min-width: 0; }
  .pick { display: flex; gap: 4px; padding-right: 6px; }
  .url2 { flex: 1; min-width: 0; display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
  .url2 :global(input) { width: 100%; box-sizing: border-box; }
  .url :global(input) { width: 100%; box-sizing: border-box; }
  button {
    background: var(--raised);
    color: var(--text);
    border: 0;
    border-radius: var(--r-md);
    font: inherit;
    /* The toolbar's controls share one height, so the method, the address and the buttons sit on one line. */
    padding: 0 12px; min-height: var(--addr-h); box-sizing: border-box; font-weight: 500;
  }
  header :global(input) { min-height: var(--addr-h-sm); box-sizing: border-box; }
  button:hover:not(:disabled) { background: var(--raised-3); }
  button:disabled { opacity: var(--op-disabled); }
  .send { position: relative; display: flex; gap: 2px; --act-fill: linear-gradient(135deg, color-mix(in srgb, var(--act) 78%, white), var(--act)); --act-glow: color-mix(in srgb, var(--act) 34%, transparent); --act-ink: var(--on-act); }
  .hookbadge { flex: none; }
  .hookbadge :global(button) { color: var(--warn); background: var(--warn-t); border-radius: var(--r-pill); font-size: var(--fs-xs); }
  .more { padding: 0 8px; min-height: var(--addr-h); }
  .bar { margin: 0; padding: 6px 12px; font-size: var(--fs-sm); }
  .specbar, .overlaybar { color: var(--muted); border-bottom: 1px solid var(--line); }
  .overlaybar { background: color-mix(in srgb, var(--accent) 7%, transparent); }
  .overlaybar strong { color: var(--text); font-weight: 600; }
  .overlaybar .then { color: var(--muted); }
  .specbar code, .overlaynote code { font-size: var(--fs-xs); }
  .rpc { display: inline-flex; align-items: center; gap: 6px; white-space: nowrap; min-width: 0; }
  .rpc .mini { width: 110px; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 2px 6px; font: inherit; font-size: var(--fs-sm); }
  .rpc .mini.id { width: 64px; }
  .expect { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; font-size: var(--fs-sm); color: var(--muted); }
  .expect .mini { width: 120px; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 2px 6px; font: inherit; font-size: var(--fs-sm); }
  .expect > span:first-child { white-space: nowrap; }
  .expect-problem { color: var(--fail); }
  .overlaynote { margin: 0 0 6px; font-size: var(--fs-xs); }
  .err { color: var(--fail); background: color-mix(in srgb, var(--fail) 12%, transparent); }
  .panels { flex: 1; min-height: 0; display: flex; }
  .left { flex: none; border-right: 1px solid var(--line); display: flex; flex-direction: column; min-height: 0; }
  .right { flex: 1; min-height: 0; min-width: 0; }
  .secnav { display: flex; align-items: flex-end; height: var(--tabbar-h); box-sizing: border-box; padding: 0 8px; border-bottom: 1px solid var(--line); flex: none; }
  .sectabs { display: flex; align-items: flex-end; gap: 2px; min-width: 0; overflow-x: auto; scrollbar-width: none; }
  .sectabs::-webkit-scrollbar { display: none; }
  .tab { position: relative; flex: none; display: inline-flex; align-items: center; gap: 6px; border: 0; border-bottom: 2px solid transparent; border-radius: 0; color: var(--muted); padding: 8px 10px 6px; white-space: nowrap; }
  .cdot { position: absolute; top: 6px; right: 5px; display: block; width: 5px; height: 5px; border-radius: 50%; background: var(--ok); }
  /* The badge sits on the icon's corner, so an icon-only tab keeps its width. */
  .tabcount { position: absolute; top: 1px; right: -2px; min-width: 14px; margin: 0; padding: 0 4px; font-size: var(--fs-xs); line-height: 14px; }
  .tab.on .tabcount { position: static; margin-left: 2px; }
  .tab:hover:not(:disabled) { color: var(--text); border-color: transparent; border-bottom-color: var(--line); }
  .tab.on, .tab.on:hover:not(:disabled) { color: var(--text); border-bottom-color: var(--accent); }
  .more-secs { flex: none; margin: 0 0 4px auto; padding-left: 6px; }
  .more-secs.on :global(.btn) { border-color: var(--accent); }
  .labelsbox { flex: none; padding: 2px 2px var(--sp-3); margin-bottom: var(--sp-3); border-bottom: 1px solid var(--line); }
  .docsbar { flex: none; display: flex; gap: 2px; padding: 0 0 6px; }
  .docsview { flex: 1; min-height: 0; overflow: auto; padding: 4px 2px; }
  .typebar { flex: none; display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; padding: 6px 10px; border-bottom: 1px solid var(--line); }
  .pickrow { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; }
  /* Every main action is a gradient with a soft glow: Send in the colour chosen in Settings, listening and the
     two-way modes in their own, so the mode still reads at a glance. */
  .send[data-tone='send'] { --act-fill: var(--send-fill); --act-glow: var(--send-glow); --act-ink: var(--on-send); }
  .act { background: var(--act-fill); color: var(--act-ink); border: 0; font-weight: 600; box-shadow: var(--send-edge), 0 2px 6px var(--act-glow); }
  .act:hover:not(:disabled) { background: var(--act-fill); filter: brightness(1.1); }
  .act:active:not(:disabled) { filter: brightness(0.95); }
  /* The floppy disk is the save everyone knows; the words stay as the tooltip and the name read aloud. */
  .saveicon { display: flex; align-items: center; gap: 5px; padding-inline: 9px; }
  .savetext { font-size: var(--fs-sm); }
  .edited { display: inline-flex; align-items: center; gap: var(--sp-1); flex: none; }
  .edited.clean { visibility: hidden; }
  .edited .pill { color: var(--dirty); font-size: var(--fs-xs); white-space: nowrap; }
  .send .main { border-radius: var(--r-md) 0 0 var(--r-md); white-space: nowrap; display: inline-flex; align-items: center; gap: 7px; }
  .send .main kbd { font: 500 var(--fs-xs) var(--font, system-ui); padding: 0 5px; border-radius: var(--r-sm); background: var(--shine); color: inherit; text-shadow: none; }
  /* Inside the address bar the buttons are shorter than the bar, so it frames them. */
  .send .more { border-radius: 0 var(--r-md) var(--r-md) 0; display: inline-flex; align-items: center; justify-content: center; }
  .send .act.more { border-left: 1px solid var(--shade); }
  .addr .send { margin-right: 4px; }
  .addr .send button { min-height: var(--ctl-h); }
  .send:not(.savegrp) { gap: 0; }
  /* While a send runs the button is a stop button: the danger colour, a square, and the time so far. */
  .send.cancel { --act-fill: var(--fail); --act-glow: transparent; --act-ink: var(--on-accent); }
  .send.cancel .more { opacity: 0.45; }
  .stop { width: 9px; height: 9px; border-radius: 2px; background: currentColor; }
  .elapsed { font: 500 var(--fs-xs) var(--mono); opacity: 0.85; }
  /* The button stays dark; the disk takes the accent colour while there is something to save. */
  .savegrp .pending { color: var(--accent); }
  .section { flex: 1; overflow: auto; padding: 14px 16px; display: flex; flex-direction: column; gap: 10px; }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .sends { color: var(--muted); font-size: var(--fs-sm); margin: 8px 0 0; }
  .row { display: flex; gap: 4px; }
  .row input {
    flex: 1; min-width: 0; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px;
    font: var(--fs-sm) var(--mono); -webkit-user-select: text; user-select: text;
  }
  .row .k { flex: 0 0 28%; }
  .xchip { flex: none; align-self: center; max-width: 30%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; padding: 1px 8px; border-radius: var(--r-pill); font: var(--fs-xs) var(--mono); cursor: default; }
  .xchip.ok { background: var(--ok-t); color: var(--ok); }
  .xchip.none { background: var(--fail-t); color: var(--fail); }
  .x { border: 0; color: var(--muted); }
  code { font-family: var(--mono); }
  .builtin { border-top: 1px solid var(--line); padding-top: 8px; }
  .chain { margin: 0; padding-left: 18px; font: var(--fs-sm) var(--mono); }
  .keepauth { margin-top: 8px; }
  .fieldnote { margin: 0 0 6px; }
  .implicit { margin: 4px 0 0; font-size: var(--fs-xs); }
  .implicit code { font-size: var(--fs-xs); }
  .pwwarn { flex: none; color: var(--warn); font-size: var(--fs-xs); white-space: nowrap; margin-right: 10px; }
</style>
