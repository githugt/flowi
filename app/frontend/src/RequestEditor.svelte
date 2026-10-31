<script lang="ts">
  import { errorText } from './lib/errors'
  import { app } from './lib/app.svelte'
  import { ui } from './lib/ui.svelte'
  import { parseXml } from './lib/xml'
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { Extraction, RequestView } from './lib/types'
  import TextField from './TextField.svelte'
  import TemplateField from './TemplateField.svelte'
  import KVRows from './KVRows.svelte'
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
  import { sectionCounts } from './lib/tabcounts'
  import Dropdown from './Dropdown.svelte'
  import ExprHelp from './ExprHelp.svelte'
  import ReplayBar from './ReplayBar.svelte'
  import MissingBar from './MissingBar.svelte'
  import Problem from './Problem.svelte'
  import VersionDetails from './VersionDetails.svelte'
  import StoredResponses from './StoredResponses.svelte'
  import Verb from './Verb.svelte'
  import Icon from './Icon.svelte'
  import MethodPicker from './MethodPicker.svelte'
  import CodeArea from './CodeArea.svelte'
  import type { CodeLang } from './lib/highlight'
  import EmptyState from './EmptyState.svelte'
  import { protocolVerb } from './lib/verbs'
  import { recognise } from './lib/paste'
  import type { DropOption } from './lib/types'
  import { WorkspaceService, ManagerService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { Events } from '@wailsio/runtime'
  import type { KeptSpec, SpecOrigin } from './lib/types'
  import OverlayDialog from './OverlayDialog.svelte'
  import VersionPicker from './VersionPicker.svelte'
  import PartsEditor from './PartsEditor.svelte'
  import DatasetSections from './DatasetSections.svelte'
  import SpecUpdateDialog from './SpecUpdateDialog.svelte'
  import FlowiOverlayDialog from './FlowiOverlayDialog.svelte'
  import FlowiOverlayEditor from './FlowiOverlayEditor.svelte'
  import FlowiOverlayWarning from './FlowiOverlayWarning.svelte'

  let { view }: { view: RequestView } = $props()
  const node = $derived(view.node)
  const v = $derived(view.current)
  const methods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS', 'QUERY']
  // The registered WebDAV, CalDAV and other methods are not listed: they go through Custom…, which keeps their registered spelling.
  const CUSTOM = '\u0000custom'
  const httpSections = ['Params', 'Headers', 'Body', 'Auth', 'Extract', 'Assert', 'Outcomes', 'Variables', 'Settings', 'Hooks', 'Schema', 'Docs', 'Code', 'Responses', 'Used by', 'Cookies']
  type Section = string
  // What a check or an extract on this kind of request can read, and the example its help gives.
  const checkContext = $derived(view.kind === 'http' ? 'request.check' : view.kind === 'dataset' ? 'dataset.check' : 'message.check')
  const checkExample = $derived(view.kind === 'http' ? 'status == 200' : view.kind === 'dataset' ? 'row_count >= 1' : 'received >= 1')
  const sections = $derived.by(() => {
    if (view.kind === 'dataset') return ['Query', 'Values', 'Extract', 'Assert', 'Outcomes', 'Variables', 'Settings', 'Hooks', 'Docs', 'Responses', 'Used by']
    if (view.kind === 'grpc') return ['Message', 'Headers', 'Receive', 'Auth', 'Proto', 'Variables', 'Extract', 'Assert', 'Outcomes', 'Settings', 'Hooks', 'Schema', 'Docs', 'Responses', 'Used by']
    if (view.kind === 'async') {
      const p = view.async?.block.Protocol
      return ['Message', 'Headers', 'Receive', ...(view.async?.credentials.length ? ['Credentials'] : []), 'Binding', 'Variables', 'Extract', 'Assert', 'Outcomes', 'Settings', 'Hooks', 'Schema', 'Docs', 'Responses', 'Used by'].filter(
        (s) => !(s === 'Receive' && p === 'sns') && !(s === 'Message' && p === 'sse' && !view.async?.block.Send),
      )
    }
    return httpSections
  })
  // The sections shown as tabs: what most sends need. The rest are under "More", in their usual order.
  const mainOf = $derived(
    view.kind === 'dataset'
      ? ['Query', 'Values', 'Extract', 'Assert']
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
  const titleOf = (s: string) => (view.kind === 'grpc' ? ({ Message: 'Body', Receive: 'Stream' } as Record<string, string>)[s] ?? s : s)
  const iconOf = (s: string) =>
    ({ Params: 'params', Headers: 'headers', Body: 'body', Message: 'body', Query: 'body', Values: 'params', Auth: 'auth', Credentials: 'auth', Extract: 'extract', Assert: 'assert', Outcomes: 'checks', Variables: 'variables', Hooks: 'hooks', Docs: 'docs', Schema: 'schema', Receive: 'stream', Proto: 'proto', Binding: 'binding', Code: 'code', Cookies: 'cookie', Responses: 'clock', 'Used by': 'usedby', Settings: 'settings' } as Record<string, string>)[s] ?? 'list'
  // The green dot: the section has content.
  const configured = (s: string) =>
    ({
      Params: !!v.Query?.length,
      Headers: !!(view.async && !view.async.headersAreConnection ? view.async.block.Send?.Headers?.length : v.Headers?.length),
      Body: view.bodyKind !== 'none',
      Query: !!view.dataset?.block.Query.trim(),
      Values: !!view.dataset?.block.Params?.length,
      Message: !!(view.async?.block.Send || (view.grpc?.block.Body.JSON || view.grpc?.block.Body.Text)),
      Auth: view.auth.kind !== 'none',
      Hooks: view.hookChain.length > 0,
      Docs: !!v.Docs,
      Variables: !!v.Variables?.length,
      Extract: !!v.Extract?.length,
      Assert: !!v.Assert?.length,
      Outcomes: !!v.Outcomes?.length,
    })[s] ?? false
  // The count badge: how many rows the section holds. Sections that are not lists keep the dot.
  const counts = $derived(sectionCounts(view))
  const moreOptions = $derived<DropOption[]>(moreSecs.map((s) => ({ value: s, label: titleOf(s), icon: iconOf(s), dot: configured(s), count: counts[s] })))
  let chosen = $state<Section>('')
  const section = $derived(sections.includes(chosen) ? chosen : sections[0])
  // The open menu: the Send button's (how to send) or the Save button's (versions).
  let menu = $state<'send' | 'save' | null>(null)
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
    const offOrigin = Events.On('spec-origin', (e: any) => e.data === forNode && askOrigin(forNode))
    const offTree = Events.On('workspace', () => askOrigin(forNode))
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
  let replayOpen = $state(false)
  let hooks = $state('all')

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
  const authOptions = $derived<DropOption[]>([...authKinds.map(([value, label]) => ({ value, label })), ...view.auth.schemes.map((s) => ({ value: s.id, label: s.label }))])
  const isScheme = $derived(view.auth.kind.startsWith('scheme:'))
  function pickAuth(k: string) {
    if (k.startsWith('scheme:')) RequestService.SetAuthScheme(node, k.slice('scheme:'.length))
    else RequestService.SetAuth(node, k, view.auth.value)
  }
  const protocolOptions = $derived<DropOption[]>((view.async?.protocols ?? []).map((p) => ({ value: p.id, label: p.label, tone: protocolVerb(p.id) })))
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
    if (view.inScratch) saveTo()
    else RequestService.Save(node)
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
  function menuAction(run: () => void) {
    menu = null
    run()
  }

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

  function onkeydown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
      e.preventDefault()
      save()
    }
  }
</script>

<!-- The engine's own steps after the hooks (schema learning), which no hook names. -->
{#snippet builtIns()}
  {#each view.pipeline.filter((p) => p.kind === 'builtin') as b}
    <p class="muted builtin">Built in, after the hooks: <strong>{b.title}</strong> {b.state === 'not-run' ? `is off (${b.reason})` : `— ${b.detail}`}</p>
  {/each}
{/snippet}

<svelte:window {onkeydown} onpointerdown={() => (menu = null)} />

<div class="req">
  <header {onpaste}>
    {#if view.kind === 'http'}
      <Dropdown pill title="Method" options={methodOptions} value={v.Method} width="108px" onpick={pickMethod} />
      <div class="url">
        <TemplateField
          value={view.fullUrl}
          placeholder={'{{env.base_url}}/path'}
          oninput={(t) => RequestService.SetURL(node, t)}
          onenter={() => RequestService.Send(node, hooks)}
          spans={(t) => RequestService.URLSpans(node, t) as unknown as Promise<any>}
          suggest={(t, c) => RequestService.Suggest(node, t, c) as unknown as Promise<any>}
        />
      </div>
    {:else if view.async}
      {@const ab = view.async.block}
      <Dropdown pill title="Protocol" options={protocolOptions} value={ab.Protocol} width="110px" onpick={(p) => RequestService.SetProtocol(node, p)} />
      <div class="url2">
        <TextField mono value={ab.Server} placeholder={'{{env.broker}}'} oninput={(t) => RequestService.SetAsync(node, { ...ab, Server: t } as never)} />
        <TextField mono value={ab.Channel} placeholder="topic, subject, queue…" oninput={(t) => RequestService.SetAsync(node, { ...ab, Channel: t } as never)} />
      </div>
    {:else if view.dataset}
      {@const db = view.dataset.block}
      <Dropdown pill title="Database" options={view.dataset.drivers.map((x) => ({ value: x.id, label: x.label, tone: protocolVerb(x.id) }))} value={db.Driver} width="130px"
        onpick={(id) => RequestService.SetDataset(node, { ...db, Driver: id } as never)} />
      <div class="url">
        <TextField mono value={db.Server} placeholder={db.Driver === 'sqlite' ? 'shop.db (a file in the collection)' : '{{env.database_url}}'}
          oninput={(t) => RequestService.SetDataset(node, { ...db, Server: t } as never)} onenter={() => RequestService.Send(node, hooks)} />
      </div>
    {:else if view.grpc}
      {@const gb = view.grpc.block}
      <Verb verb="GRPC" />
      <div class="url2">
        <TextField mono value={gb.Server} placeholder={'grpc://{{env.host}}:50051'} oninput={(t) => RequestService.SetGRPC(node, { ...gb, Server: t } as never)} />
        <MethodPicker {node} info={view.grpc} />
      </div>
    {/if}
    <VersionPicker {view} {node} onmanage={() => (overlaysOpen = true)} />
    <div class="send" style:--act={`var(--act-${action.tone})`}>
      {#if view.sending}
        <button class="act main" onclick={() => RequestService.Cancel(node)}>Cancel</button>
      {:else}
        <button class="act main" onclick={() => RequestService.Send(node, hooks)}>{action.label}</button>
      {/if}
      <button class="act more" aria-label="Send options" onpointerdown={(e) => e.stopPropagation()} onclick={() => (menu = menu === 'send' ? null : 'send')}>▾</button>
      {#if menu === 'send'}
        <div class="menu" role="menu" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
          {#if view.async && view.async.modes.length}
            {#each view.async.modes as m}
              <button role="menuitemradio" aria-checked={view.async.mode === m} onclick={() => menuAction(() => RequestService.SetMode(node, m))}><span class="tick">{view.async.mode === m ? '✓' : ''}</span>{modeLabels[m] ?? m}</button>
            {/each}
            <hr />
          {/if}
          <div class="hd">When sending</div>
          {#each hookModes as [m, label]}
            <button role="menuitemradio" aria-checked={hooks === m} onclick={() => menuAction(() => (hooks = m))}><span class="tick">{hooks === m ? '✓' : ''}</span>{label}</button>
          {/each}
          <hr />
          <button role="menuitem" onclick={() => menuAction(() => (replayOpen = !replayOpen))}><span class="tick">{replayOpen ? '✓' : ''}</span>Replay saved responses…</button>
        </div>
      {/if}
    </div>
    {#if view.dirty}
      <button title="Throw away the unsaved changes" onclick={() => ui.confirm('Discard changes', 'Throw away the unsaved changes of this version?', () => RequestService.DiscardEdit(node), 'Discard')}>Discard</button>
    {/if}
    <!-- Saving as a new version, and the other version actions, sat at the end of the Send menu where
         nobody looked for them; they open from the Save button now. -->
    <div class="send savegrp">
      {#if view.inScratch}
        <button class="saveicon main pending" title="Save to a collection…" aria-label="Save to a collection" onclick={saveTo}><Icon name="save" size={15} /></button>
      {:else}
        <button class="saveicon main" class:pending={view.dirty} title={view.flowiOverlay ? `Save to the flowi overlay ${view.flowiOverlay} (⌘S)` : 'Save (⌘S)'} aria-label="Save" disabled={!view.dirty} onclick={save}><Icon name="save" size={15} /></button>
      {/if}
      <button class="more" aria-label="Save options" aria-expanded={menu === 'save'} onpointerdown={(e) => e.stopPropagation()} onclick={() => (menu = menu === 'save' ? null : 'save')}>▾</button>
      {#if menu === 'save'}
        <div class="menu" role="menu" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
          {#if view.flowiOverlay}
            <button role="menuitem" disabled={!view.dirty} onclick={() => menuAction(() => (saveOverlayOpen = true))}><span class="tick"></span>Save to flowi overlay {view.flowiOverlay}…<kbd>⌘S</kbd></button>
            <button role="menuitem" title="Writes the overlay's changes into the version itself" onclick={() => menuAction(saveVersion)}><span class="tick"></span>Save into the version, with the overlay</button>
          {:else}
            <button role="menuitem" onclick={() => menuAction(saveVersion)}><span class="tick"></span>Save current version<kbd>⌘S</kbd></button>
          {/if}
          <button role="menuitem" onclick={() => menuAction(newVersion)}><span class="tick"></span>Save as new version…</button>
          {#if !view.inScratch}
            <button role="menuitem" title="Copies the request as saved, under a new name or in another place" onclick={() => menuAction(saveAsNew)}><span class="tick"></span>Save as new request…</button>
            <button role="menuitem" disabled={!view.dirty && !view.flowiOverlay} title="Keeps the changes apart from the version, to lay over it when sending" onclick={() => menuAction(() => (saveOverlayOpen = true))}><span class="tick"></span>Save as flowi overlay…</button>
          {/if}
          {#if origin?.spec}
            <button role="menuitem" title={`Also write what the spec can say to an overlay on ${origin.spec}`} onclick={() => menuAction(() => (overlayOpen = true))}><span class="tick"></span>Save to spec overlay…</button>
          {/if}
          <button role="menuitem" onclick={() => menuAction(() => RequestService.SetDefault(node))}><span class="tick"></span>Set as default version</button>
          <button role="menuitem" onclick={() => menuAction(() => (detailsOpen = true))}><span class="tick"></span>Version title and description…</button>
          <hr />
          <button role="menuitem" onclick={() => menuAction(() => ui.confirm('Delete version?', `Version ${view.selected} is removed from the request file. This cannot be undone.`, () => deleteVersion(), 'Delete'))}><span class="tick"></span>Delete this version…</button>
          {#if !view.inScratch}
            <button role="menuitem" onclick={() => menuAction(() => ui.confirm('Delete request', `Delete ${view.name}?`, () => WorkspaceService.Do('delete', [node], '', ''), 'Delete'))}><span class="tick"></span>Delete this request…</button>
          {/if}
        </div>
      {/if}
    </div>
  </header>
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
  <FileBanner onDisk={view.onDisk} needsDrop={view.needsDrop} revert={() => RequestService.Revert(node)} saveDropping={() => RequestService.SaveDroppingComments(node)} declineDrop={() => RequestService.DeclineDrop(node)} />
  {#if view.error}<p class="bar err"><Problem text={view.error} /></p>{/if}
  {#if view.flowiOverlay}
    {@const info = stack.length === 1 ? view.flowiOverlays.find((o) => o.name === view.flowiOverlay) : undefined}
    <p class="bar overlaybar">
      Flowi {stack.length > 1 ? 'overlays' : 'overlay'}
      {#each stack as n, i}{#if i}<span class="then">&nbsp;then&nbsp;</span>{/if}<strong>{n}</strong>{#if stack.length > 1}<button class="link" title={`Take ${n} off`} aria-label={`Take ${n} off`} onclick={() => unstackOverlay(n)}>×</button>{/if}{/each}
      {stack.length > 1 ? 'are' : 'is'} laid over version {view.selected}{#if info} · {info.shared} shared {info.shared === 1 ? 'change' : 'changes'}{info.used ? `, ${info.own} for this request` : ''}{/if}.
      Sends use the request as shown; their responses are kept apart from the version's.
      {#if view.flowiOverlayNotes.length}<button class="link" onclick={() => (warningOpen = true)}>{view.flowiOverlayNotes.length} not applied in full</button>{/if}
      · <button class="link" onclick={() => RequestService.SelectFlowiOverlay(node, '')}>Take it off</button>
    </p>
  {/if}
  <MissingBar {view} />
  {#if origin?.spec}
    <p class="bar specbar">
      From the spec <code>{origin.spec}</code>
      {#if overlayFields.length}
        · <span title={origin.fields.map((f) => `${f.key ? `${f.section} ${f.key}` : f.section}: ${f.file}`).join('\n')}>{overlayFields.length} {overlayFields.length === 1 ? 'field' : 'fields'} set by overlays: {overlayFields.join(', ')}</span>
      {/if}
      · <button class="link" onclick={openUpdate}>Update from spec…</button>
    </p>
  {/if}
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
        <div class="section">
          {#if ['Message', 'Receive', 'Credentials', 'Binding', 'Proto'].includes(section)}
            <MessageSections {view} {section} />
          {:else if ['Query', 'Values'].includes(section)}
            <DatasetSections {view} {section} />
          {:else if section === 'Params'}
            <KVRows rows={v.Query} describe marks={overlayMarks('query', v.Query)} onchange={(r) => RequestService.SetRows(node, 'query', r)}
              empty={{ icon: 'params', title: 'No query parameters', hint: 'Add one here, or type ?name=value at the end of the address.', add: 'Add parameter' }} />
          {:else if section === 'Headers'}
            {@const sendHeaders = view.async && !view.async.headersAreConnection}
            <KVRows rows={sendHeaders ? (view.async?.block.Send?.Headers ?? null) : v.Headers} describe keyLabel="header" marks={sendHeaders ? {} : overlayMarks('header', v.Headers)} onchange={(r) => RequestService.SetRows(node, 'headers', r)}
              empty={{ icon: 'headers', title: 'No headers', hint: sendHeaders ? 'Headers travel with each message sent.' : 'Content-Type, Accept and the like. Credentials go in Auth.', add: 'Add header' }} />
          {:else if section === 'Body'}
            {@const bodyFrom = origin?.fields.find((f) => f.section === 'body')}
            {#if bodyFrom}<p class="muted overlaynote">The example comes from the overlay <code>{bodyFrom.file}</code>.</p>{/if}
            {#if view.bodyKind === 'text'}
              <CodeArea value={bodyText} lang={codeLangOf(view.bodyFormat)} oninput={(t) => RequestService.SetBodyText(node, t)}
                empty={{ icon: 'body', title: 'Empty body', hint: 'Click to write the body to send.' }} />
              {#if xmlProblem}<p class="xmlnote bad" role="status">Not well-formed XML: line {xmlProblem.line}, column {xmlProblem.column}: {xmlProblem.message}</p>
              {:else if view.bodyFormat === 'xml' && bodyText.trim()}<p class="xmlnote" role="status">Well-formed XML</p>{/if}
            {:else if view.bodyKind === 'form' && !view.multipart}
              <KVRows rows={v.Body.Form} onchange={(r) => RequestService.SetFormRows(node, r)}
                empty={{ icon: 'list', title: 'No form fields', hint: 'Each field is sent as name=value.', add: 'Add field' }} />
            {:else if view.bodyKind === 'form'}
              <PartsEditor parts={view.parts} onchange={(p) => RequestService.SetParts(node, p as never)} />
            {:else if view.bodyKind === 'file'}
              <TextField label="File path" mono value={v.Body.File} oninput={(t) => RequestService.SetBodyFile(node, t)} />
            {:else}
              <EmptyState icon="body" title="No body" hint="Pick a body type below to send text, a form or a file." compact />
            {/if}
          {:else if section === 'Auth'}
            {#if view.auth.kind === 'none'}
              <EmptyState icon="auth" title="No auth" hint={'Pick a type below. Keep credentials out of the file: use {{secret.name}}.'} compact />
            {:else if isScheme}
              {#each view.auth.fields as f (f.name)}
                <TextField label={f.name + (f.known ? '' : ' (not a parameter of this scheme)')} mono placeholder={f.secret ? '{{secret.' + f.name + '}}' : ''} value={f.value} oninput={(t) => RequestService.SetAuthParam(node, f.name, t)} />
              {/each}
              <p class="muted">Every value is a template. Keep credentials out of the file: {'{{secret.name}}'} reads the vault. The request is signed or authenticated as it is sent.</p>
            {:else}
              <TextField label={view.auth.kind === 'bearer' ? 'Token' : view.auth.kind === 'basic' ? 'Base64 credentials or a template' : 'Authorization header value'} mono
                value={view.auth.value} oninput={(t) => RequestService.SetAuth(node, view.auth.kind, t)} />
              <p class="muted">Keep credentials out of the file: use {'{{secret.name}}'}.</p>
            {/if}
          {:else if section === 'Extract' && !extract.length}
            <EmptyState icon="extract" title="Nothing extracted" hint="Take a value out of the response into a variable, for later requests and steps." compact>
              <button onclick={() => extract.push(newExtraction())}>Add extraction</button>
            </EmptyState>
          {:else if section === 'Extract'}
            <p class="muted">Take a value out of the response into a variable, for later requests and steps.{#if view.kind !== 'http'} <ExprHelp context={checkContext} />{/if}</p>
            {#each extract as e, i}
              <div class="row">
                <input class="k" placeholder="variable" bind:value={e.Name} oninput={() => RequestService.SetExtract(node, extract)} spellcheck="false" />
                <select bind:value={e.From} onchange={() => RequestService.SetExtract(node, extract)}>
                  {#each ['body', 'header', 'cookie', 'status'] as f}<option value={f}>{f}</option>{/each}
                </select>
                {#if e.From !== 'status'}
                  <input class="v" placeholder={e.From === 'body' ? '$.path.to.value' : 'name'} value={selector(e)} oninput={(ev) => setSelector(extract, i, ev.currentTarget.value)} spellcheck="false" />
                {/if}
                <button class="x" aria-label="Remove" onclick={() => { extract.splice(i, 1); RequestService.SetExtract(node, extract) }}>×</button>
              </div>
            {/each}
            <button class="add" onclick={() => { extract.push(newExtraction()) }}>+ Add</button>
          {:else if section === 'Assert'}
            <p class="muted">One card per check, for example <code>{checkExample}</code>. A failing check fails the send. <ExprHelp context={checkContext} /></p>
            <AssertCards checks={v.Assert ?? []} outcome={view.response ?? undefined} example={checkExample}
              vocabulary={requestVocabulary(checkContext, node, view.response, view.kind)}
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
              <button onclick={() => app.openHooks(node, view.name)}>Add hooks…</button>
            </EmptyState>
            {@render builtIns()}
          {:else if section === 'Hooks'}
            <p class="muted">Steps that run before the request is sent and after its response, from this request and the levels above it.</p>
            <ol class="chain">{#each view.hookChain as h}<li>{h}</li>{/each}</ol>
            <button class="add" onclick={() => app.openHooks(node, view.name)}>Edit this request's hooks…</button>
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
            <div class="docsbar" role="group" aria-label="Notes view">
              <button class:on={!docsPreview} onclick={() => (docsPreview = false)}>Write</button>
              <button class:on={docsPreview} onclick={() => (docsPreview = true)} disabled={!v.Docs}>Preview</button>
            </div>
            {#if docsPreview && v.Docs}
              <div class="docsview"><Markdown text={v.Docs} /></div>
            {:else}
              <AreaField mono={false} value={v.Docs} oninput={(t) => RequestService.SetDocs(node, t)}
                empty={{ icon: 'docs', title: 'No notes', hint: 'Click to write about this request, in Markdown.' }} />
            {/if}
          {/if}
        </div>
      {/if}
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
                <button class="link" title="Wrap this body as the parameters of a JSON-RPC 2.0 call" onclick={() => RequestService.MakeJSONRPC(node)}>Make it a JSON-RPC call</button>
              {/if}
            {/if}
            {#if view.bodyKind === 'form'}
              <label class="radio"><input type="checkbox" checked={view.multipart} onchange={(e) => RequestService.SetMultipart(node, e.currentTarget.checked)} />Multipart</label>
            {/if}
          </div>
        {:else if section === 'Auth'}
          <div class="typebar"><Dropdown title="Auth type" options={authOptions} value={view.auth.kind} onpick={pickAuth} /></div>
        {/if}
    </div>
    <Splitter name="request-left" side="left" min={300} max={1000} bind:width={leftWidth} />
    <div class="right"><ResponsePane {view} /></div>
  </div>
</div>

<style>
  .xmlnote { margin: 4px 0 0; font-size: var(--fs-xs); color: var(--muted); }
  .xmlnote.bad { color: var(--warn); }
  .req { position: absolute; inset: 0; display: flex; flex-direction: column; }
  header { display: flex; gap: 8px; padding: 10px 12px; border-bottom: 1px solid var(--line); align-items: center; }
  .url { flex: 1; min-width: 0; }
  .url2 { flex: 1; min-width: 0; display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
  .url2 :global(input) { width: 100%; box-sizing: border-box; }
  .url :global(input) { width: 100%; box-sizing: border-box; }
  select, button {
    background: var(--bg);
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    font: inherit;
  }
  /* The toolbar's controls share one height, so the method, the address and the buttons sit on one line. */
  select, button { padding: 5px 10px; min-height: 32px; box-sizing: border-box; }
  header :global(input) { min-height: 32px; box-sizing: border-box; }
  button { background: transparent; }
  button:hover:not(:disabled) { border-color: var(--accent); }
  button:disabled { opacity: 0.4; }
  .send { position: relative; display: flex; gap: 2px; }
  .more { padding: 5px 8px; min-height: 32px; }
  .menu {
    position: absolute; top: 100%; right: 0; margin-top: 4px; min-width: 150px; display: flex; flex-direction: column; padding: 4px;
    background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); box-shadow: var(--shadow); z-index: 20;
  }
  .menu button { border: 0; text-align: left; }
  .hd { color: var(--muted); font-size: var(--fs-xs); padding: 2px 10px; }
  .bar { margin: 0; padding: 6px 12px; font-size: var(--fs-sm); }
  .specbar, .overlaybar { color: var(--muted); border-bottom: 1px solid var(--line); }
  .overlaybar { background: color-mix(in srgb, var(--accent) 7%, transparent); }
  .overlaybar strong { color: var(--text); font-weight: 600; }
  .overlaybar .then { color: var(--muted); }
  .overlaybar strong + .link { margin-left: 3px; font-size: 1.1em; line-height: 1; }
  .overlaybar .link { background: none; border: 0; padding: 0; color: var(--accent); font: inherit; }
  .specbar code, .overlaynote code { font-size: var(--fs-xs); }
  .specbar .link { background: none; border: 0; padding: 0; color: var(--accent); font: inherit; }
  .rpc { display: inline-flex; align-items: center; gap: 6px; white-space: nowrap; min-width: 0; }
  .rpc .mini { width: 110px; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 2px 6px; font: inherit; font-size: var(--fs-sm); }
  .rpc .mini.id { width: 64px; }
  .typebar .link { background: none; border: 0; padding: 0; color: var(--accent); font: inherit; font-size: var(--fs-sm); }
  .overlaynote { margin: 0 0 6px; font-size: var(--fs-xs); }
  .err { color: var(--fail); background: color-mix(in srgb, var(--fail) 12%, transparent); }
  .panels { flex: 1; min-height: 0; display: flex; }
  .left { flex: none; border-right: 1px solid var(--line); display: flex; flex-direction: column; min-height: 0; }
  .right { flex: 1; min-height: 0; min-width: 0; }
  .secnav { display: flex; align-items: flex-end; height: var(--tabbar-h); box-sizing: border-box; padding: 0 8px; border-bottom: 1px solid var(--line); flex: none; }
  .sectabs { display: flex; align-items: flex-end; gap: 2px; min-width: 0; overflow-x: auto; scrollbar-width: none; }
  .sectabs::-webkit-scrollbar { display: none; }
  .tab { position: relative; flex: none; display: inline-flex; align-items: center; gap: 6px; border: 0; border-bottom: 2px solid transparent; border-radius: 0; color: var(--muted); padding: 8px 10px 6px; white-space: nowrap; }
  .cdot { position: absolute; top: 6px; right: 5px; }
  /* The badge sits on the icon's corner, so an icon-only tab keeps its width. */
  .tabcount { position: absolute; top: 1px; right: -2px; min-width: 14px; margin: 0; padding: 0 4px; font-size: 10px; line-height: 14px; }
  .tab.on .tabcount { position: static; margin-left: 2px; }
  .tab:hover:not(:disabled) { color: var(--text); border-color: transparent; border-bottom-color: var(--line); }
  .tab.on, .tab.on:hover:not(:disabled) { color: var(--text); border-bottom-color: var(--accent); }
  .cdot { display: block; width: 5px; height: 5px; border-radius: 50%; background: var(--ok); }
  .more-secs { flex: none; margin: 0 0 4px auto; padding-left: 6px; }
  .more-secs.on :global(.btn) { border-color: var(--accent); }
  .docsbar { flex: none; display: flex; gap: 2px; padding: 0 0 6px; }
  .docsbar button { background: transparent; color: var(--muted); border: 1px solid var(--line); border-radius: var(--r-md); padding: 1px 10px; font: inherit; font-size: var(--fs-xs); }
  .docsbar button.on { color: var(--text); background: var(--panel); }
  .docsbar button:disabled { opacity: 0.5; }
  .docsview { flex: 1; min-height: 0; overflow: auto; padding: 4px 2px; }
  .typebar { flex: none; display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; padding: 6px 10px; border-top: 1px solid var(--line); }
  .act { background: var(--act); color: var(--on-act); border-color: var(--act); font-weight: 600; box-shadow: 0 2px 10px color-mix(in srgb, var(--act) 32%, transparent); }
  .act:hover:not(:disabled) { border-color: var(--act); background: color-mix(in srgb, var(--act) 88%, white); }
  /* The floppy disk is the save everyone knows; the words stay as the tooltip and the name read aloud. */
  .saveicon { display: flex; align-items: center; padding-inline: 9px; }
  .send .main { border-radius: 6px 0 0 6px; white-space: nowrap; }
  .send .more { border-radius: 0 6px 6px 0; border-left-color: color-mix(in srgb, var(--on-act) 35%, transparent); }
  .savegrp .more:not(.primary) { border-left-color: var(--line); }
  /* The button stays dark; the disk takes the accent colour while there is something to save. */
  .savegrp .pending { color: var(--accent); }
  .menu .tick { width: 14px; display: inline-block; }
  .menu hr { border: 0; border-top: 1px solid var(--line); margin: 4px 0; }
  .menu kbd { margin-left: auto; padding-left: 16px; color: var(--muted); font: inherit; font-size: var(--fs-xs); }
  .menu button { display: flex; align-items: center; white-space: nowrap; }
  .section { flex: 1; overflow: auto; padding: 14px 16px; display: flex; flex-direction: column; gap: 10px; }
  .radio { display: flex; gap: 4px; align-items: center; }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .row { display: flex; gap: 4px; }
  .row input {
    flex: 1; min-width: 0; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px;
    font: var(--fs-sm) var(--mono); -webkit-user-select: text; user-select: text;
  }
  .row .k { flex: 0 0 28%; }
  .add { align-self: flex-start; border: 0; color: var(--accent); padding: 2px 0; }
  .x { border: 0; color: var(--muted); }
  code { font-family: var(--mono); }
  .builtin { border-top: 1px solid var(--line); padding-top: 8px; }
  .chain { margin: 0; padding-left: 18px; font: var(--fs-sm) var(--mono); }
</style>
