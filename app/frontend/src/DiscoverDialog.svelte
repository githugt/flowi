<script lang="ts">
  import { tick } from 'svelte'
  import Button from './Button.svelte'
  // Explore an API that has no description: list what would be asked, let the person tick the requests, send them,
  // then draft requests from what answered. Only GET is ever sent, and the token goes to that host only.
  import { ImportService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { errorText } from './lib/errors'
  import { ui } from './lib/ui.svelte'
  import type { Choice, DiscoveryView, ImportView, ProbeView } from './lib/types'

  const ask = $derived(ui.discover)
  let box = $state<HTMLDivElement>()
  let url = $state('')
  let header = $state('Authorization')
  let token = $state('')
  let collection = $state('')
  let probes = $state<ProbeView[]>([])
  let ticked = $state<string[]>([])
  let found = $state<DiscoveryView | null>(null)
  let saved = $state<DiscoveryView | null>(null)
  let busy = $state(false)
  let error = $state('')

  let collections = $state<Choice[]>([])

  // The dialog takes focus when it opens, so Escape reaches it; every open starts clean.
  $effect(() => {
    const a = ui.discover
    if (!a) return
    url = token = error = ''
    probes = []
    ticked = []
    found = saved = null
    collection = a.collection
    tick().then(() => box?.focus())
    // The collections the import wizard offers are the ones a request can be drafted into.
    ImportService.State().then((v) => {
      const view = v as unknown as ImportView
      collections = view.collections
      if (!collection) collection = view.collection || view.collections[0]?.id || ''
    })
  })

  async function run<T>(f: () => Promise<T>, set: (v: T) => void) {
    busy = true
    error = ''
    try {
      set(await f())
    } catch (e) {
      error = errorText(e)
    } finally {
      busy = false
    }
  }

  const plan = () =>
    run(() => ImportService.DiscoverPlan(url.trim()), (v) => {
      probes = v as unknown as ProbeView[]
      ticked = probes.map((p) => p.path)
      found = saved = null
    })
  const send = () => run(() => ImportService.Discover(url.trim(), header, token, ticked), (v) => (found = v as unknown as DiscoveryView))
  const save = () => run(() => ImportService.SaveDiscovery(collection), (v) => (saved = v as unknown as DiscoveryView))

  const drafts = $derived(found?.findings.filter((f) => f.kind === 'json' || f.kind === 'grpc').length ?? 0)
  const toggle = (path: string) => (ticked = ticked.includes(path) ? ticked.filter((p) => p !== path) : [...ticked, path])

  function close() {
    ui.discover = null
  }
  function keys(e: KeyboardEvent) {
    if (e.key !== 'Escape') return
    e.preventDefault()
    close()
  }
</script>

{#if ask}
  <div class="scrim" role="presentation" onpointerdown={close}>
    <div bind:this={box} class="dialog" role="dialog" aria-label="Discover an API" tabindex="-1" onpointerdown={(e) => e.stopPropagation()} onkeydown={keys}>
      <h3>Discover an API</h3>
      <p class="muted">For an API with no description. Flowi asks a few well-known paths and follows the paths their answers point to on the same host. It only sends GET (a gRPC server is only asked which services it offers: no method is called), says <code>User-Agent: flowi</code>, and sends nothing until you have ticked what to ask.</p>
      <label class="f"><span>Base URL, or a gRPC server (grpc://host:port)</span><input bind:value={url} placeholder="https://api.example.com" spellcheck="false" /></label>
      <div class="row">
        <label class="f"><span>Header (optional)</span><input bind:value={header} spellcheck="false" /></label>
        <label class="f"><span>Token (optional, not kept)</span><input type="password" bind:value={token} autocomplete="off" /></label>
      </div>
      <div class="actions"><Button type="button" disabled={busy || !url.trim()} onclick={plan}>List the requests</Button></div>

      {#if probes.length}
        <section aria-label="Requests to make">
          <h4>Requests to make</h4>
          {#each probes as p (p.path)}
            <label class="probe"><input type="checkbox" checked={ticked.includes(p.path)} onchange={() => toggle(p.path)} /> <code>{p.method === 'reflect' ? 'ask ' : 'GET '}{p.path}</code> <span class="muted">{p.why}</span></label>
          {/each}
          <div class="actions"><Button type="button" disabled={busy || !ticked.length} onclick={send}>{busy ? 'Asking…' : `Send ${ticked.length} request${ticked.length === 1 ? '' : 's'}`}</Button></div>
        </section>
      {/if}

      {#if found}
        <section aria-label="What answered">
          <h4>What answered</h4>
          {#each found.findings as f (f.path)}
            <div class="found"><span class="status">{f.status || '—'}</span> <code>{f.path}</code> <span class="muted">{f.kind === 'spec' ? 'an API description: import it instead' : f.kind === 'json' ? 'JSON' : f.kind === 'grpc' ? 'gRPC method' : 'nothing to draft from'}</span></div>
          {/each}
          {#if found.stopped}<p class="muted">{found.stopped}.</p>{/if}
          {#if drafts}
            <label class="f"><span>Draft {drafts} request{drafts === 1 ? '' : 's'} in</span>
              <select bind:value={collection}>{#each collections as c (c.id)}<option value={c.id}>{c.label}</option>{/each}</select>
            </label>
            <div class="actions"><Button type="button" disabled={busy || !collection} onclick={save}>Draft requests</Button></div>
          {:else}
            <p class="muted">Nothing answered with JSON or offered a method.</p>
          {/if}
        </section>
      {/if}

      {#if saved}<p class="ok">Drafted {saved.created} request{saved.created === 1 ? '' : 's'} under discovered/.{#if saved.skipped.length} Skipped: {saved.skipped.join('; ')}.{/if}</p>{/if}
      {#if error}<p class="err">{error}</p>{/if}
      <div class="actions"><Button type="button" onclick={close}>Close</Button></div>
    </div>
  </div>
{/if}

<style>
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 60; }
  .dialog { width: 600px; max-width: calc(100vw - 32px); max-height: 86vh; overflow: auto; display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); outline: none; }
  h3, h4 { margin: 0; }
  h4 { font-size: var(--fs-sm); margin-bottom: 4px; }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .ok { color: var(--ok, var(--text)); margin: 0; }
  .err { color: var(--fail, var(--text)); margin: 0; }
  .f { display: flex; flex-direction: column; gap: 3px; flex: 1; font-size: var(--fs-sm); }
  .f > span { color: var(--muted); }
  .row { display: flex; gap: 10px; }
  input:not([type='checkbox']), select { padding: 5px 8px; background: var(--input, var(--bg)); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-sm, 4px); font: inherit; }
  .probe { display: flex; gap: 6px; align-items: baseline; font-size: var(--fs-sm); padding: 2px 0; }
  .found { font-size: var(--fs-sm); padding: 2px 0; }
  .status { display: inline-block; min-width: 2.5em; font-family: var(--mono); color: var(--muted); }
  code { font-family: var(--mono); font-size: var(--fs-xs); }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
</style>
