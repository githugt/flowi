<script lang="ts">
  import { PanelsService, RequestService, WorkspaceService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { app } from './lib/app.svelte'
  import { ui } from './lib/ui.svelte'
  import type { LocationView } from './lib/types'

  type Tab = 'log' | 'problems' | 'history'
  let open = $state<Tab | null>(null)
  let logEl = $state<HTMLDivElement>()

  // Where the tab in front lives: its collection, the git branch there and, on hover, the folder.
  let where = $state<LocationView>({ name: '', path: '', branch: '', scratch: false })
  $effect(() => {
    const node = app.active
    WorkspaceService.Location(node).then((v) => {
      if (node === app.active) where = v
    })
  })

  const errors = $derived(app.panels.problems.filter((p) => p.severity === 'error').length)
  const items = $derived(app.panels.problems.length)

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

<footer>
  <div class="tabs">
    <button class:on={open === 'log'} onclick={() => pick('log')}>Run log</button>
    <button class:on={open === 'problems'} onclick={() => pick('problems')}>
      Problems{#if items}<span class="badge" class:err={errors > 0}>{items}</span>{/if}
    </button>
    <button class:on={open === 'history'} onclick={() => pick('history')}>
      Response history{#if app.panels.history.length}<span class="badge">{app.panels.history.length}</span>{/if}
    </button>
    {#if open === 'problems'}
      <span class="sum">{app.panels.summary}</span>
      <button class="link" onclick={() => PanelsService.RefreshProblems()}>Refresh</button>
      <button class="link" title="Forget what sends reported; problems in the files stay until they are fixed" onclick={() => PanelsService.ClearProblems()}>Clear</button>
    {:else if open === 'log'}
      <span class="sum"></span>
      <button class="link" onclick={() => PanelsService.ClearLog()}>Clear</button>
    {:else if open === 'history'}
      <span class="sum"></span>
      <button class="link" onclick={() => PanelsService.ClearHistory()}>Clear</button>
    {/if}
    {#if where.name}
      <span class="where-tab" title={where.scratch ? 'Scratch: drafts that are not saved to a folder yet' : where.path}>
        {#if where.scratch}Scratch{:else}{where.name}{#if where.branch} · <span class="branch">⎇ {where.branch}</span>{/if}{/if}
      </span>
    {/if}
  </div>
  {#if open}
    <div class="body" bind:this={logEl}>
      {#if open === 'log'}
        {#if !app.panels.log.length}<p class="muted">Sent requests and runs are logged here.</p>{/if}
        {#each app.panels.log as sec}
          <details open>
            <summary>{sec.title || 'Earlier'}</summary>
            {#each sec.lines as l}
              <div class="line {l.tone}"><span class="t">{l.at}</span><span class="lvl {l.level}">{l.level}</span><span class="st">{l.step}</span><span>{l.message}</span></div>
            {/each}
          </details>
        {/each}
      {:else if open === 'problems'}
        {#if app.panels.opening && items}<p class="opening" role="status">{app.panels.opening} Fix the files below; Refresh checks them again.</p>{/if}
        {#if !items}<p class="muted">No problems.</p>{/if}
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
            <button class="hdel" aria-label="Delete this response" title="Delete this response from the history" onclick={() => ui.confirm('Delete response', `Forget "${h.title}" from this session's history?`, () => PanelsService.DeleteHistory(h.id))}>×</button>
          </div>
        {/each}
      {/if}
    </div>
  {/if}
</footer>

<style>
  footer { flex: none; border-top: 1px solid var(--line); background: var(--panel); display: flex; flex-direction: column; }
  .tabs { display: flex; align-items: center; gap: 2px; padding: 0 8px; height: 28px; }
  .tabs button { background: none; border: 0; color: var(--muted); font: inherit; font-size: var(--fs-sm); padding: 3px 10px; border-radius: var(--r-md); }
  .tabs button.on { color: var(--text); background: color-mix(in srgb, var(--accent) 18%, transparent); }
  .tabs button:hover { color: var(--text); }
  .badge { margin-left: 6px; background: var(--line); color: var(--text); border-radius: var(--r-pill); padding: 0 6px; font-size: var(--fs-xs); }
  .badge.err { background: var(--fail); color: var(--on-accent); }
  .where-tab { margin-left: auto; color: var(--muted); font-size: var(--fs-sm); padding-right: 4px; white-space: nowrap; cursor: default; }
  .branch { color: var(--text); }
  .sum { color: var(--muted); font-size: var(--fs-sm); margin-left: 12px; }
  .link { color: var(--accent) !important; }
  .body { height: 220px; overflow: auto; padding: 6px 12px; border-top: 1px solid var(--line); font: var(--fs-sm) var(--mono); -webkit-user-select: text; user-select: text; }
  .muted { color: var(--muted); font-family: inherit; }
  summary { color: var(--muted); cursor: default; padding: 3px 0; }
  .opening { margin: 0 0 var(--sp-2); color: var(--warn); }
  .line { display: grid; grid-template-columns: 64px 48px 64px 1fr; gap: 8px; padding: 1px 0; white-space: pre-wrap; }
  .t, .st { color: var(--muted); }
  .lvl.ERROR { color: var(--fail); }
  .lvl.WARN { color: var(--warn); }
  .success { color: var(--ok); }
  .redirect { color: var(--info); }
  .client { color: var(--warn); }
  .server { color: var(--fail); }
  .pending { color: var(--pending); }
  .cancelled { color: var(--muted); }
  .prob { display: flex; gap: 10px; width: 100%; text-align: left; background: none; border: 0; color: var(--text); font: inherit; padding: 3px 0; }
  .prob:enabled:hover { background: color-mix(in srgb, var(--accent) 10%, transparent); }
  .hrow { display: flex; align-items: center; }
  .hdel { background: none; border: 0; color: var(--muted); font: inherit; padding: 0 8px; cursor: pointer; }
  .hdel:hover { color: var(--fail); }
  .sev.error { color: var(--fail); }
  .sev.warning { color: var(--warn); }
  .code { color: var(--muted); }
  .msg { flex: 1; }
  .where { color: var(--muted); }
</style>
