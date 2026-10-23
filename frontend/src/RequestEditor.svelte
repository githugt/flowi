<script lang="ts">
  import { errorText } from './lib/errors'
  import { app } from './lib/app.svelte'
  import { ui } from './lib/ui.svelte'
  import { RequestService } from '../bindings/github.com/githugt/flowi/internal/bridge'
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
  import { WorkspaceService, ManagerService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { Events } from '@wailsio/runtime'
  import type { KeptSpec, SpecOrigin } from './lib/types'
  import OverlayDialog from './OverlayDialog.svelte'
  import SpecUpdateDialog from './SpecUpdateDialog.svelte'

  let { view }: { view: RequestView } = $props()
  const node = $derived(view.node)
  const v = $derived(view.current)
  const methods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']
  const httpSections = ['Params', 'Headers', 'Body', 'Auth', 'Extract', 'Assert', 'Variables', 'Settings', 'Hooks', 'Schema', 'Docs', 'Code', 'Responses', 'Used by', 'Cookies']
  type Section = string
  const sections = $derived.by(() => {
    if (view.kind === 'grpc') return ['Message', 'Headers', 'Receive', 'Auth', 'Proto', 'Variables', 'Extract', 'Assert', 'Settings', 'Hooks', 'Schema', 'Docs', 'Responses', 'Used by']
    if (view.kind === 'async') {
      const p = view.async?.block.Protocol
      return ['Message', 'Headers', 'Receive', ...(view.async?.credentials.length ? ['Credentials'] : []), 'Binding', 'Variables', 'Extract', 'Assert', 'Settings', 'Hooks', 'Schema', 'Docs', 'Responses', 'Used by'].filter(
        (s) => !(s === 'Receive' && p === 'sns') && !(s === 'Message' && p === 'sse' && !view.async?.block.Send),
      )
    }
    return httpSections
  })
  // The sections shown as tabs: what most sends need. The rest are under "More", in their usual order.
  const mainOf = $derived(
    view.kind === 'grpc'
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
    ({ Params: 'params', Headers: 'headers', Body: 'body', Message: 'body', Auth: 'auth', Credentials: 'auth', Extract: 'extract', Assert: 'assert', Variables: 'variables', Hooks: 'hooks', Docs: 'docs', Schema: 'schema', Receive: 'stream', Proto: 'proto', Binding: 'binding', Code: 'code', Cookies: 'cookie', Responses: 'clock', 'Used by': 'usedby', Settings: 'settings' } as Record<string, string>)[s] ?? 'list'
  // The green dot: the section has content.
  const configured = (s: string) =>
    ({
      Params: !!v.Query?.length,
      Headers: !!(view.async && !view.async.headersAreConnection ? view.async.block.Send?.Headers?.length : v.Headers?.length),
      Body: view.bodyKind !== 'none',
      Message: !!(view.async?.block.Send || (view.grpc?.block.Body.JSON || view.grpc?.block.Body.Text)),
      Auth: view.auth.kind !== 'none',
      Hooks: view.hookChain.length > 0,
      Docs: !!v.Docs,
      Variables: !!v.Variables?.length,
      Extract: !!v.Extract?.length,
      Assert: !!v.Assert?.length,
    })[s] ?? false
  const moreOptions = $derived<DropOption[]>(moreSecs.map((s) => ({ value: s, label: titleOf(s), icon: iconOf(s), dot: configured(s) })))
  let chosen = $state<Section>('')
  const section = $derived(sections.includes(chosen) ? chosen : sections[0])
  // The open menu: the Send button's (how to send) or the Save button's (versions).
  let menu = $state<'send' | 'save' | null>(null)
  let detailsOpen = $state(false)

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
  /** The rows an overlay sets in a section, by name: the tooltip names the overlay. */
  function overlayMarks(section: 'query' | 'header'): Record<string, string> {
    return Object.fromEntries((origin?.fields ?? []).filter((f) => f.section === section).map((f) => [f.key, `Set by the overlay ${f.file}`]))
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
  // xml and raw have no colouring of their own; their {{templates}} are still marked.
  const codeLangOf = (format: string): CodeLang => (format === 'json' ? 'json' : format === 'yaml' ? 'yaml' : 'text')

  const methodOptions: DropOption[] = methods.map((m) => ({ value: m, label: m, tone: m }))
  const bodyKindOptions: DropOption[] = [['none', 'None'], ['text', 'Text'], ['form', 'Form'], ['file', 'File']].map(([value, label]) => ({ value, label }))
  const formatOptions: DropOption[] = formats.map((f) => ({ value: f, label: f.toUpperCase() }))
  const authOptions: DropOption[] = authKinds.map(([value, label]) => ({ value, label }))
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
  function save() {
    if (view.inScratch) saveTo()
    else RequestService.Save(node)
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

  // A try-out request is kept by choosing where; its path starts from its name or its URL.
  async function saveTo() {
    const collections = (app.tree?.roots ?? []).filter((n) => n.kind === 'collection').map((n) => ({ id: n.id, label: n.label }))
    ui.prompt({
      title: 'Save request',
      label: 'Path',
      initial: await RequestService.SuggestSavePath(node),
      select: { label: 'Save to', options: collections },
      done: (path, _p, collection) => RequestService.SaveTo(node, collection, path),
    })
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
      if (view.inScratch) saveTo()
      else RequestService.Save(node)
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
      <Dropdown pill title="Method" options={methodOptions} value={v.Method} width="108px" onpick={(m) => RequestService.SetMethod(node, m)} />
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
    {:else if view.grpc}
      {@const gb = view.grpc.block}
      <Verb verb="GRPC" />
      <div class="url2">
        <TextField mono value={gb.Server} placeholder={'grpc://{{env.host}}:50051'} oninput={(t) => RequestService.SetGRPC(node, { ...gb, Server: t } as never)} />
        <MethodPicker {node} info={view.grpc} />
      </div>
    {/if}
    <div class="ver">
      <span class="muted">Ver.</span>
      <select value={view.selected} onchange={(e) => RequestService.SelectVersion(node, e.currentTarget.value)} aria-label="Version">
        {#each view.versions as l, i}<option value={l}>{l}{view.versionNames[i] ? ' · ' + view.versionNames[i] : ''}{l === view.default ? ' (default)' : ''}</option>{/each}
      </select>
    </div>
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
        <button class="primary saveicon main" title="Save to a collection…" aria-label="Save to a collection" onclick={saveTo}><Icon name="save" size={15} /></button>
      {:else}
        <button class="saveicon main" title="Save (⌘S)" aria-label="Save" disabled={!view.dirty} onclick={() => RequestService.Save(node)}><Icon name="save" size={15} /></button>
      {/if}
      <button class="more" class:primary={view.inScratch} aria-label="Save options" aria-expanded={menu === 'save'} onpointerdown={(e) => e.stopPropagation()} onclick={() => (menu = menu === 'save' ? null : 'save')}>▾</button>
      {#if menu === 'save'}
        <div class="menu" role="menu" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
          <button role="menuitem" onclick={() => menuAction(save)}><span class="tick"></span>Save current version<kbd>⌘S</kbd></button>
          <button role="menuitem" onclick={() => menuAction(newVersion)}><span class="tick"></span>Save as new version…</button>
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
  {#if updating && origin}<SpecUpdateDialog collection={origin.collection} spec={updating} onclose={() => (updating = null)} />{/if}
  <FileBanner onDisk={view.onDisk} needsDrop={view.needsDrop} revert={() => RequestService.Revert(node)} saveDropping={() => RequestService.SaveDroppingComments(node)} declineDrop={() => RequestService.DeclineDrop(node)} />
  {#if view.error}<p class="bar err"><Problem text={view.error} /></p>{/if}
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
              <Icon name={iconOf(sec)} size={15} />{#if section === sec}<span class="tname">{titleOf(sec)}</span>{/if}{#if configured(sec)}<i class="cdot" title="Has content"></i>{/if}
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
          {:else if section === 'Params'}
            <KVRows rows={v.Query} describe marks={overlayMarks('query')} onchange={(r) => RequestService.SetRows(node, 'query', r)}
              empty={{ icon: 'params', title: 'No query parameters', hint: 'Add one here, or type ?name=value at the end of the address.', add: 'Add parameter' }} />
          {:else if section === 'Headers'}
            {@const sendHeaders = view.async && !view.async.headersAreConnection}
            <KVRows rows={sendHeaders ? (view.async?.block.Send?.Headers ?? null) : v.Headers} describe keyLabel="header" marks={sendHeaders ? {} : overlayMarks('header')} onchange={(r) => RequestService.SetRows(node, 'headers', r)}
              empty={{ icon: 'headers', title: 'No headers', hint: sendHeaders ? 'Headers travel with each message sent.' : 'Content-Type, Accept and the like. Credentials go in Auth.', add: 'Add header' }} />
          {:else if section === 'Body'}
            {@const bodyFrom = origin?.fields.find((f) => f.section === 'body')}
            {#if bodyFrom}<p class="muted overlaynote">The example comes from the overlay <code>{bodyFrom.file}</code>.</p>{/if}
            {#if view.bodyKind === 'text'}
              <CodeArea value={bodyText} lang={codeLangOf(view.bodyFormat)} oninput={(t) => RequestService.SetBodyText(node, t)}
                empty={{ icon: 'body', title: 'Empty body', hint: 'Click to write the body to send.' }} />
            {:else if view.bodyKind === 'form' && !view.multipart}
              <KVRows rows={v.Body.Form} onchange={(r) => RequestService.SetFormRows(node, r)}
                empty={{ icon: 'list', title: 'No form fields', hint: 'Each field is sent as name=value.', add: 'Add field' }} />
            {:else if view.bodyKind === 'form'}
              <p class="muted">Multipart fields (including files) are edited in the file for now.</p>
            {:else if view.bodyKind === 'file'}
              <TextField label="File path" mono value={v.Body.File} oninput={(t) => RequestService.SetBodyFile(node, t)} />
            {:else}
              <EmptyState icon="body" title="No body" hint="Pick a body type below to send text, a form or a file." compact />
            {/if}
          {:else if section === 'Auth'}
            {#if view.auth.kind === 'none'}
              <EmptyState icon="auth" title="No auth" hint={'Pick a type below. Keep credentials out of the file: use {{secret.name}}.'} compact />
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
            <p class="muted">Take a value out of the response into a variable, for later requests and steps.{#if view.kind !== 'http'} <ExprHelp context="message.check" />{/if}</p>
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
            <p class="muted">One check per line, for example <code>{view.kind === 'http' ? 'status == 200' : 'received >= 1'}</code>. A failing check fails the send. <ExprHelp context={view.kind === 'http' ? 'request.check' : 'message.check'} /></p>
            <AreaField value={(v.Assert ?? []).join('\n')} oninput={(t) => RequestService.SetAssert(node, t.split('\n').filter((l) => l.trim() !== ''))}
              empty={{ icon: 'assert', title: 'No checks', hint: 'Click to write one per line.' }} />
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
            <AreaField mono={false} value={v.Docs} oninput={(t) => RequestService.SetDocs(node, t)}
              empty={{ icon: 'docs', title: 'No notes', hint: 'Click to write about this request, in Markdown.' }} />
          {/if}
        </div>
      {/if}
        {#if section === 'Body'}
          <div class="typebar">
            <Dropdown title="Body type" options={bodyKindOptions} value={view.bodyKind} onpick={(k) => RequestService.SetBodyKind(node, k)} />
            {#if view.bodyKind === 'text'}
              <Dropdown title="Format" options={formatOptions} value={view.bodyFormat} onpick={(f) => RequestService.SetBodyFormat(node, f)} />
            {/if}
            {#if view.bodyKind === 'form'}
              <label class="radio"><input type="checkbox" checked={view.multipart} onchange={(e) => RequestService.SetMultipart(node, e.currentTarget.checked)} />Multipart</label>
            {/if}
          </div>
        {:else if section === 'Auth'}
          <div class="typebar"><Dropdown title="Auth type" options={authOptions} value={view.auth.kind} onpick={(k) => RequestService.SetAuth(node, k, view.auth.value)} /></div>
        {/if}
    </div>
    <Splitter name="request-left" side="left" min={300} max={1000} bind:width={leftWidth} />
    <div class="right"><ResponsePane {view} /></div>
  </div>
</div>

<style>
  .req { position: absolute; inset: 0; display: flex; flex-direction: column; }
  header { display: flex; gap: 8px; padding: 8px 12px; border-bottom: 1px solid var(--line); align-items: center; }
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
  select, button { padding: 5px 10px; }
  button { background: transparent; }
  button:hover:not(:disabled) { border-color: var(--accent); }
  button:disabled { opacity: 0.4; }
  .primary { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
  .send { position: relative; display: flex; gap: 2px; }
  .more { padding: 5px 8px; }
  .menu {
    position: absolute; top: 100%; right: 0; margin-top: 4px; min-width: 150px; display: flex; flex-direction: column; padding: 4px;
    background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); box-shadow: var(--shadow); z-index: 20;
  }
  .menu button { border: 0; text-align: left; }
  .hd { color: var(--muted); font-size: var(--fs-xs); padding: 2px 10px; }
  .bar { margin: 0; padding: 6px 12px; font-size: var(--fs-sm); }
  .specbar { color: var(--muted); border-bottom: 1px solid var(--line); }
  .specbar code, .overlaynote code { font-size: var(--fs-xs); }
  .specbar .link { background: none; border: 0; padding: 0; color: var(--accent); font: inherit; }
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
  .tab:hover:not(:disabled) { color: var(--text); border-color: transparent; border-bottom-color: var(--line); }
  .tab.on, .tab.on:hover:not(:disabled) { color: var(--text); border-bottom-color: var(--accent); }
  .cdot { display: block; width: 5px; height: 5px; border-radius: 50%; background: var(--ok); }
  .more-secs { flex: none; margin: 0 0 4px auto; padding-left: 6px; }
  .more-secs.on :global(.btn) { border-color: var(--accent); }
  .typebar { flex: none; display: flex; align-items: center; gap: 10px; padding: 6px 10px; border-top: 1px solid var(--line); }
  .ver { display: flex; align-items: center; gap: 4px; }
  .act { background: var(--act); color: var(--on-act); border-color: var(--act); font-weight: 600; }
  .act:hover:not(:disabled) { border-color: var(--act); background: color-mix(in srgb, var(--act) 88%, white); }
  /* The floppy disk is the save everyone knows; the words stay as the tooltip and the name read aloud. */
  .saveicon { display: flex; align-items: center; padding-inline: 9px; }
  .send .main { border-radius: 6px 0 0 6px; white-space: nowrap; }
  .send .more { border-radius: 0 6px 6px 0; border-left-color: color-mix(in srgb, var(--on-act) 35%, transparent); }
  .savegrp .more:not(.primary) { border-left-color: var(--line); }
  .savegrp .more.primary { border-left-color: color-mix(in srgb, var(--on-accent) 35%, transparent); }
  .menu .tick { width: 14px; display: inline-block; }
  .menu hr { border: 0; border-top: 1px solid var(--line); margin: 4px 0; }
  .menu kbd { margin-left: auto; padding-left: 16px; color: var(--muted); font: inherit; font-size: var(--fs-xs); }
  .menu button { display: flex; align-items: center; white-space: nowrap; }
  .section { flex: 1; overflow: auto; padding: 12px; display: flex; flex-direction: column; gap: 8px; }
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
