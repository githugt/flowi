<script lang="ts">
  import IconButton from './IconButton.svelte'
  import EmptyState from './EmptyState.svelte'
  import { bandColour } from './lib/envcolours'
  import { PanelsService, RequestService, SettingsService, WelcomeService, WorkspaceService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { app } from './lib/app.svelte'
  import { ui } from './lib/ui.svelte'
  import type { LocationView } from './lib/types'
  import { footerLabel } from './lib/footer'
  import { serviceChips } from './lib/services'
  import { errorText } from './lib/errors'
  import DetailsPane from './DetailsPane.svelte'
  import Button from './Button.svelte'
  import Splitter from './Splitter.svelte'
  import Tabs, { type TabItem } from './Tabs.svelte'
  import { runChip } from './lib/runstate'

  let appVersion = $state('')
  $effect(() => {
    WelcomeService.State().then((v) => (appVersion = (v as { version: string }).version)).catch(() => {})
  })

  type Tab = 'details' | 'log' | 'problems' | 'history'
  let open = $state<Tab | null>(null)
  let logEl = $state<HTMLDivElement>()

  // Where the tab in front lives: its collection, the git branch there and, on hover, the folder.
  let where = $state<LocationView>({ name: '', path: '', branch: '', scratch: false })
  // Before: it was read only when the tab in front changed, so after `git switch` in a terminal the footer named the
  // old branch. It is read again when the window gets the focus back (the usual way back from a terminal), and
  // every 15 s while it is visible.
  let refresh = $state(0)
  $effect(() => {
    const node = app.active
    void refresh
    WorkspaceService.Location(node).then((v) => {
      if (node === app.active) where = v
    })
  })
  $effect(() => {
    const again = () => {
      if (!document.hidden) refresh++
    }
    const timer = setInterval(again, 15_000)
    window.addEventListener('focus', again)
    document.addEventListener('visibilitychange', again)
    return () => {
      clearInterval(timer)
      window.removeEventListener('focus', again)
      document.removeEventListener('visibilitychange', again)
    }
  })

  // The open request's version and overlays, kept up to date by the request view the window already holds.
  const verLabel = $derived(footerLabel(app.requests[app.active]?.selected ?? '', app.requests[app.active]?.flowiOverlay ?? ''))

  const services = $derived(serviceChips(app.services))
  // Flows and sends in progress in any open tab.
  const running = $derived(runChip(Object.values(app.flows), Object.values(app.requests)))
  // The panel's height is the person's to set, and is kept (the handle does that).
  let height = $state(220)
  const band = $derived(bandColour(app.tree))
  const errors = $derived(app.panels.problems.filter((p) => p.severity === 'error').length)
  const items = $derived(app.panels.problems.length)
  const tabs = $derived<TabItem[]>([
    { id: 'details', label: 'Details' },
    { id: 'log', label: 'Run log' },
    { id: 'problems', label: 'Problems', count: items || undefined, tone: errors > 0 ? 'fail' : undefined },
    { id: 'history', label: 'Response history', count: app.panels.history.length || undefined },
  ])

  // What opening the workspace found shows once, unasked: the panel opens on it.
  let openedOnProblems = false
  $effect(() => {
    if (app.panels.opening && !openedOnProblems) {
      openedOnProblems = true
      open = 'problems'
    }
  })

  function pick(t: Tab) {
    open = open === t ? null : t
  }

  // New log lines scroll into view.
  $effect(() => {
    void app.panels.log
    if (open === 'log' && logEl) queueMicrotask(() => (logEl!.scrollTop = logEl!.scrollHeight))
  })

  function openNode(id?: string) {
    const n = id && app.find(id)
    if (n) app.open(n)
  }

  // An exchange of the history opens its request, showing that response.
  async function openExchange(id: number, node: string) {
    const n = app.find(node)
    if (!n) return
    await app.open(n)
    RequestService.ShowExchange(node, id)
  }
</script>

<footer style:--env={band}>
  <div class="row">
    <Tabs label="Panels" size="sm" bare items={tabs} active={open} onpick={(id) => pick(id as Tab)} />
    {#if open === 'problems'}
      <span class="sum">{app.panels.summary}</span>
      <Button variant="ghost" small onclick={() => PanelsService.RefreshProblems()}>Refresh</Button>
      <Button variant="ghost" small title="Forget what sends reported; problems in the files stay until they are fixed" onclick={() => PanelsService.ClearProblems()}>Clear</Button>
    {:else if open === 'log'}
      <span class="sum"></span>
      <Button variant="ghost" small onclick={() => PanelsService.ClearLog()}>Clear</Button>
    {:else if open === 'history'}
      <span class="sum"></span>
      <Button variant="ghost" small onclick={() => PanelsService.ClearHistory()}>Clear</Button>
    {/if}
    {#if services.length || running}
      <span class="svcs">
        {#if running}<span class="svc busy" title={running.title}><span class="runtext"><span class="dot" aria-hidden="true"></span>{running.text}</span></span>{/if}
        {#each services as s (s.id)}
          <span class="svc" class:busy={s.busy}><Button variant="ghost" small title={s.title} onclick={() => app.openScreen(s.screen, s.id === 'replayer' ? 'Replayer' : 'Schedules')}><span class="dot" aria-hidden="true"></span>{s.text}</Button></span>
        {/each}
      </span>
    {/if}
    {#if app.settings?.appearance.hideSensitive}<Button variant="ghost" small title="Secrets are blurred on screen. Click to show them again." onclick={() => SettingsService.Set('ui.hide_sensitive', 'false')}>Secrets hidden</Button>{/if}
    {#if appVersion}<button class="where-tab where-link" title="About Flowi: the version of this application" onclick={() => app.openScreen('screen:about', 'About')}>Flowi v{appVersion}</button>{/if}
    {#if verLabel}<span class="ver-tab" title="A request can keep several versions, different shapes of the same call. This is the one open here, and the overlays laid over it.">{verLabel}</span>{/if}
    {#if where.name}
      {#if where.scratch}
        <span class="where-tab" title="Drafts: not in a collection folder yet. Save one to keep it.">Drafts</span>
      {:else}
        <button class="where-tab where-link" title={`Show ${where.path} in the file manager`} onclick={() => WorkspaceService.RevealLocation(app.active).catch(ui.fail)}>
          {where.name}{#if where.branch} · <span class="branch">⎇ {where.branch}</span>{/if}
        </button>
      {/if}
    {/if}
  </div>
  {#if open}
    <Splitter name="footer-height" axis="y" min={120} max={600} bind:width={height} oncollapse={() => (open = null)} />
    <div class="body" style:height={`${height}px`} bind:this={logEl}>
      {#if open === 'details'}
        <DetailsPane />
      {:else if open === 'log'}
        {#if !app.panels.log.length}<p class="muted">Sent requests and runs are logged here.</p>{/if}
        {#each app.panels.log as sec}
          <details open>
            <summary>{sec.title || 'Earlier'}</summary>
            {#each sec.lines as l}
              <div class="line {l.tone}"><span class="t">{l.at}</span><span class="lvl {l.level}">{l.level}</span><span class="st" title={l.step}>{l.step}</span><span>{l.message}</span></div>
            {/each}
          </details>
        {/each}
      {:else if open === 'problems'}
        {#if app.panels.opening && items}<p class="opening" role="status">{app.panels.opening} Fix the files below; Refresh checks them again.</p>{/if}
        {#if !items}<EmptyState icon="checks" title="No problems" compact />{/if}
        {#each app.panels.problems as p}
          <button class="prob" onclick={() => openNode(p.node)} disabled={!p.node}>
            <span class="sev {p.severity}">{p.severity === 'error' ? '●' : '▲'}</span>
            <span class="code">{p.code}</span>
            <span class="msg">{p.message}</span>
            <span class="where">{p.where}</span>
          </button>
        {/each}
      {:else}
        {#if !app.panels.history.length}<p class="muted">Responses of this session appear here.</p>{/if}
        {#each app.panels.history as h}
          <div class="hrow">
            <button class="prob" onclick={() => openExchange(h.id, h.node)}><span class="where">{h.at}</span><span class="msg">{h.title}</span></button>
            <IconButton icon="close" size={12} danger label="Delete this response" hint="Delete this response from the history" onclick={() => ui.confirm('Delete response', `Forget "${h.title}" from this session's history?`, () => PanelsService.DeleteHistory(h.id))} />
          </div>
        {/each}
      {/if}
    </div>
  {/if}
</footer>

<style>
  footer { flex: none; background: var(--chrome); display: flex; flex-direction: column; border-top: 2px solid var(--env, transparent); }
  .row { display: flex; align-items: center; gap: 2px; padding: 0 8px; height: var(--ctl-h); }
  .svcs { display: inline-flex; margin-left: auto; }
  .svcs + .where-tab { margin-left: 8px; }
  .svc :global(.dot) { display: inline-block; width: 7px; height: 7px; margin-right: 5px; border-radius: 50%; background: var(--ok); }
  .svc.busy :global(.dot) { background: var(--warn); animation: svcblink 1s ease-in-out infinite; }
  @keyframes svcblink { 50% { opacity: 0.3; } }
  @media (prefers-reduced-motion: reduce) { .svc.busy :global(.dot) { animation: none; } }
  .ver-tab { margin-left: auto; color: var(--text); font-size: var(--fs-sm); padding: 4px 12px 2px 10px; margin-right: 2px; border-right: 1px solid var(--line); white-space: nowrap; }
  .ver-tab + .where-tab { margin-left: 8px; }
  .where-tab { margin-left: auto; color: var(--muted); font-size: var(--fs-sm); padding: 4px 4px 2px 0; white-space: nowrap; cursor: default; }
  .where-link { background: none; border: 0; font: inherit; font-size: var(--fs-sm); cursor: pointer; }
  .where-link:hover { color: var(--text); text-decoration: underline; }
  .branch { color: var(--text); }
  .sum { color: var(--muted); font-size: var(--fs-sm); margin-left: 12px; }
  .body { overflow: auto; padding: 6px 12px; border-top: 1px solid var(--line); background: var(--sunk); font: var(--fs-sm) var(--mono); -webkit-user-select: text; user-select: text; }
  .muted { color: var(--muted); font-family: inherit; }
  summary { color: var(--muted); cursor: default; padding: 3px 0; }
  .opening { margin: 0 0 var(--sp-2); color: var(--warn); }
  .line { display: grid; grid-template-columns: 64px 48px 64px 1fr; gap: 8px; padding: 1px 0; white-space: pre-wrap; }
  .t, .st { color: var(--muted); }
  .st { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .runtext { padding: 0 8px; color: var(--muted); font-size: var(--fs-sm); white-space: nowrap; }
  .lvl.ERROR { color: var(--fail); }
  .lvl.WARN { color: var(--warn); }
  .success { color: var(--ok); }
  .redirect { color: var(--info); }
  .client { color: var(--warn); }
  .server { color: var(--fail); }
  .pending { color: var(--pending); }
  .cancelled { color: var(--muted); }
  .prob { display: flex; gap: 10px; width: 100%; text-align: left; background: none; border: 0; color: var(--text); font: inherit; padding: 3px 0; }
  .prob:enabled:hover { background: var(--tint-hover); }
  .hrow { display: flex; align-items: center; }
  .sev.error { color: var(--fail); }
  .sev.warning { color: var(--warn); }
  .code { color: var(--muted); }
  .msg { flex: 1; }
  .where { color: var(--muted); }
</style>
