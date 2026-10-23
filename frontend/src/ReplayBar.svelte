<script lang="ts">
  // The replayer, from the request screen: start and stop it, choose its port,
  // and see the address a client calls to get this request's saved answer.
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { ReplayerService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import type { ReplayerView } from './lib/types'

  // Hidden unless asked for (open) or the replayer is running: most sends never need it.
  let { path, method, open = false, onclose }: { path: string; method: string; open?: boolean; onclose?: () => void } = $props()

  let view = $state<ReplayerView | null>(null)
  let port = $state('')
  let editedPort = false

  onMount(() => live<ReplayerView>('replayer', () => ReplayerService.State(), (v) => {
    view = v
    if (!editedPort || v.state !== 'stopped') port = portOf(v.state === 'stopped' ? v.form.addr : v.addr)
  }))

  const portOf = (addr: string) => addr.slice(addr.lastIndexOf(':') + 1)
  const hostOf = (addr: string) => addr.slice(0, addr.lastIndexOf(':')) || '127.0.0.1'
  const running = $derived(view?.state === 'running' || view?.state === 'stopping')
  const busy = $derived(view?.state === 'starting' || view?.state === 'stopping')
  const addr = $derived(view ? (running ? view.addr : view.form.addr) : '')
  // 0.0.0.0 and :: are where it listens, not where a client calls.
  const host = $derived(['0.0.0.0', '::', '[::]'].includes(hostOf(addr)) ? '127.0.0.1' : hostOf(addr))
  const url = $derived(`http://${host}:${running ? portOf(addr) : port || portOf(addr)}${path}`)

  function toggle() {
    if (!view) return
    if (running) return void ReplayerService.Stop()
    ReplayerService.Start({ ...view.form, addr: `${hostOf(view.form.addr)}:${port || portOf(view.form.addr)}` })
  }

  async function copy() {
    await navigator.clipboard.writeText(url)
    ui.toast('Copied ' + url)
  }
</script>

{#if view && (open || running || busy)}
  <div class="replay" class:on={running}>
    <button class="toggle" disabled={busy} onclick={toggle} title={running ? 'Stop answering calls' : 'Answer calls to this address with the saved responses'}>
      {#if running}■ Stop replayer{:else if view.state === 'starting'}Starting…{:else}▶ Replay{/if}
    </button>
    <label class="port" title="The port the replayer listens on">
      <span>port</span>
      <input inputmode="numeric" spellcheck="false" bind:value={port} disabled={running || busy} oninput={() => (editedPort = true)} />
    </label>
    <span class="muted">{method}</span>
    <input class="url" readonly value={url} aria-label="Address of the replayer for this request" onfocus={(e) => e.currentTarget.select()} />
    <button onclick={copy} title="Copy the address">Copy</button>
    {#if running}<span class="state">● {view.form.record ? 'recording from ' + view.form.record : 'listening'}</span>{:else}<span class="muted">stopped</span>{/if}
    {#if view.error}<span class="err">{view.error}</span>{/if}
    {#if !running && !busy && onclose}<button class="close" aria-label="Hide the replay bar" onclick={onclose}>×</button>{/if}
  </div>
{/if}

<style>
  .replay { display: flex; align-items: center; gap: var(--sp-2); padding: 4px 12px; border-bottom: 1px solid var(--line); font-size: var(--fs-sm); }
  .close { border: 0; color: var(--muted); padding: 0 4px; }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 2px 10px; font: inherit; white-space: nowrap; }
  button:disabled { opacity: 0.5; }
  .on .toggle { border-color: var(--fail); color: var(--fail); }
  .toggle:not(:disabled):hover { border-color: var(--accent); }
  .port { display: flex; align-items: center; gap: 4px; color: var(--muted); }
  .port input { width: 52px; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 2px 6px; font: var(--fs-sm) var(--mono); }
  .url { flex: 1; min-width: 0; background: color-mix(in srgb, var(--bg) 70%, transparent); color: var(--muted); border: 1px dashed var(--line); border-radius: var(--r-md); padding: 2px 8px; font: var(--fs-sm) var(--mono); -webkit-user-select: text; user-select: text; cursor: text; }
  .muted { color: var(--muted); }
  .state { color: var(--ok); white-space: nowrap; }
  .err { color: var(--fail); }
</style>
