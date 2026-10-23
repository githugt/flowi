<script lang="ts">
  // The first screen: what to do next, worked out from the workspace, with the
  // changelog folded away below for those who want it.
  import { onMount } from 'svelte'
  import { WelcomeService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { app } from './lib/app.svelte'
  import { live } from './lib/live'
  import { promptNewEnvironment } from './lib/menus'
  import type { TreeNode } from './lib/types'
  import Markdown from './Markdown.svelte'

  interface WelcomeView {
    version: string
    whatsNew: string
  }
  let view = $state<WelcomeView | null>(null)
  onMount(() => live<WelcomeView>('welcome', () => WelcomeService.State(), (v) => (view = v)))

  const collections = $derived((app.tree?.roots ?? []).filter((n) => n.kind === 'collection' && !n.draft))
  const environments = $derived(app.tree?.environments ?? [])
  const selected = $derived(app.tree?.selected ?? '')

  function firstOf(kind: string, nodes: TreeNode[]): TreeNode | undefined {
    for (const n of nodes) {
      if (n.kind === kind) return n
      const inner = firstOf(kind, n.children ?? [])
      if (inner) return inner
    }
  }
  const firstRequest = $derived(firstOf('request', collections))
  const firstFlow = $derived(firstOf('flow', collections))

  const steps = $derived([
    { done: collections.length > 0, title: 'Open a collection', detail: collections.length ? collections.map((c) => c.label).join(', ') : 'Mount a folder of requests and flows, or start a new one.' },
    { done: !!selected, title: 'Choose an environment', detail: selected ? `Using ${selected}.` : environments.length ? 'Requests read {{env.…}} values from it.' : 'Create one for values like base_url.' },
    { done: false, title: 'Send a request or run a flow', detail: firstRequest ? `For example ${firstRequest.label}${firstFlow ? ` or the ${firstFlow.label} flow` : ''}.` : 'Once a collection is open.' },
  ])
</script>

<div class="page">
  <h1>Welcome to Flow-I</h1>
  <p class="muted">Version {view?.version ?? ''}</p>

  <h3>Get started</h3>
  <ol class="steps">
    {#each steps as s, i}
      <li class:done={s.done}>
        <span class="mark" aria-hidden="true">{s.done ? '✓' : i + 1}</span>
        <div class="body">
          <strong>{s.title}</strong>
          <span class="detail">{s.detail}</span>
        </div>
        {#if i === 0}
          <button class:primary={!s.done} onclick={() => app.openScreen('screen:manager', 'Collections')}>{s.done ? 'Collections' : 'Open…'}</button>
        {:else if i === 1 && !s.done}
          {#if environments.length}
            <select aria-label="Environment" value="" onchange={(e) => app.selectEnvironment(e.currentTarget.value)}>
              <option value="" disabled>Choose…</option>
              {#each environments as name}<option value={name}>{name}</option>{/each}
            </select>
          {:else}
            <button class="primary" disabled={!collections.length} onclick={promptNewEnvironment}>New environment…</button>
          {/if}
        {:else if i === 2 && firstRequest}
          <button class:primary={steps[0].done && steps[1].done} onclick={() => app.open(firstRequest!)}>Open {firstRequest.label}</button>
        {/if}
      </li>
    {/each}
  </ol>

  <div class="cards">
    <button onclick={() => app.openScreen('screen:import', 'Import')}><strong>Import an API</strong><span>OpenAPI, AsyncAPI, Postman, Bruno, HAR, curl…</span></button>
    <button onclick={() => app.openScreen('screen:telemetry', 'Telemetry')}><strong>Usage reporting</strong><span>Off unless you choose otherwise.</span></button>
  </div>

  {#if view?.whatsNew}
    <details class="news">
      <summary>What's new in this version</summary>
      <Markdown text={view.whatsNew} />
    </details>
  {/if}
</div>

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: var(--sp-6) 32px 48px; max-width: 760px; }
  h1 { margin: 0 0 4px; font-size: var(--fs-xl); }
  .muted { color: var(--muted); margin: 0 0 var(--sp-4); }
  h3 { margin: var(--sp-4) 0 var(--sp-2); font-size: var(--fs-md); }
  .steps { list-style: none; margin: 0 0 var(--sp-6); padding: 0; display: flex; flex-direction: column; gap: var(--sp-2); }
  .steps li { display: flex; align-items: center; gap: var(--sp-3); padding: 10px 14px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); }
  .mark { flex: none; width: 22px; height: 22px; border-radius: 50%; display: grid; place-items: center; font-size: var(--fs-xs); font-weight: 600; border: 1px solid var(--line); color: var(--muted); }
  .done .mark { background: var(--ok); border-color: var(--ok); color: var(--on-accent); }
  .done strong { color: var(--muted); }
  .body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
  .detail { color: var(--muted); font-size: var(--fs-sm); }
  li button, li select { flex: none; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 12px; font: inherit; }
  li button:hover:not(:disabled) { border-color: var(--accent); }
  li button:disabled { opacity: 0.4; }
  li .primary { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
  .cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: var(--sp-3); }
  .cards button { text-align: left; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-lg); padding: var(--sp-3) 14px; font: inherit; display: flex; flex-direction: column; gap: 4px; }
  .cards button:hover { border-color: var(--accent); }
  .cards span { color: var(--muted); font-size: var(--fs-sm); }
  .news { margin-top: var(--sp-6); }
  .news summary { cursor: pointer; color: var(--muted); }
</style>
