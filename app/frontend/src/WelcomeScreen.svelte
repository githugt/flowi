<script lang="ts">
  import Button from './Button.svelte'
  // The first launch's screen: what to do next, worked out from the workspace, with the
  // changelog folded away below. Later launches open the home page; this stays reachable from it and the Help menu.
  import { onMount } from 'svelte'
  import { WelcomeService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { app } from './lib/app.svelte'
  import { live } from './lib/live'
  import { promptNewEnvironment } from './lib/menus'
  import type { TreeNode } from './lib/types'
  import Markdown from './Markdown.svelte'
  import Icon from './Icon.svelte'
  import Logo from './Logo.svelte'

  interface WelcomeView {
    version: string
    whatsNew: string
  }
  let view = $state<WelcomeView | null>(null)
  onMount(() => live<WelcomeView>('welcome', () => WelcomeService.State(), (v) => (view = v)))

  const collections = $derived((app.tree?.roots ?? []).filter((n) => n.kind === 'collection' && !n.scratch))
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
 <div class="col">
  <div class="hero">
    <div class="logo"><Logo size={56} /></div>
    <h1>Welcome to Flowi</h1>
    <p class="muted">Version {view?.version ?? ''} · your requests and flows stay on this machine</p>
  </div>

  <h3><span>Get started</span><span class="progress">{steps.filter((x) => x.done).length} of {steps.length}</span></h3>
  <ol class="steps">
    {#each steps as s, i}
      <li class:done={s.done}>
        <span class="mark" aria-hidden="true">{s.done ? '✓' : i + 1}</span>
        <div class="body">
          <strong>{s.title}</strong>
          <span class="detail">{s.detail}</span>
        </div>
        {#if i === 0}
          <Button variant={s.done ? 'secondary' : 'primary'} onclick={() => app.openScreen('screen:manager', 'Collections')}>{s.done ? 'Collections' : 'Open…'}</Button>
        {:else if i === 1 && !s.done}
          {#if environments.length}
            <select aria-label="Environment" value="" onchange={(e) => app.selectEnvironment(e.currentTarget.value)}>
              <option value="" disabled>Choose…</option>
              {#each environments as name}<option value={name}>{name}</option>{/each}
            </select>
          {:else}
            <Button variant="primary" disabled={!collections.length} onclick={promptNewEnvironment}>New environment…</Button>
          {/if}
        {:else if i === 2 && firstRequest}
          <Button variant={steps[0].done && steps[1].done ? 'primary' : 'secondary'} onclick={() => app.open(firstRequest!)}>Open {firstRequest.label}</Button>
        {/if}
      </li>
    {/each}
  </ol>

  <div class="cards">
    <button onclick={() => app.openScreen('screen:import', 'Import')}><span class="sq import"><Icon name="import" size={17} /></span><strong>Import an API</strong><span>OpenAPI, AsyncAPI, Postman, Bruno, Insomnia, Hoppscotch, HAR, curl…</span></button>
    <button onclick={() => app.openScreen('screen:telemetry', 'Telemetry')}><span class="sq usage"><Icon name="chart" size={17} /></span><strong>Usage reporting</strong><span>Off unless you choose otherwise.</span></button>
  </div>

  <div class="home"><Button variant="primary" onclick={() => app.openScreen('screen:home', 'Home')}>Go to Home</Button></div>

  {#if view?.whatsNew}
    <details class="news">
      <summary>What's new in this version</summary>
      <Markdown text={view.whatsNew} />
    </details>
  {/if}
 </div>
</div>

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: 48px 32px 48px; background: radial-gradient(ellipse 700px 420px at 50% 0%, var(--tint-accent), transparent 70%); }
  .col { max-width: 660px; margin: 0 auto; }
  .hero { display: flex; flex-direction: column; align-items: center; text-align: center; margin-bottom: var(--sp-6); }
  .logo { display: grid; filter: drop-shadow(0 8px 20px color-mix(in srgb, var(--brand-tile) 30%, transparent)); }
  h1 { margin: 16px 0 4px; font-size: var(--fs-xl); font-weight: 650; letter-spacing: -0.01em; }
  .muted { color: var(--muted); margin: 0; font-size: var(--fs-md); }
  h3 { margin: var(--sp-4) 0 var(--sp-2); font-size: var(--fs-xs); font-weight: 600; letter-spacing: 0.07em; text-transform: uppercase; color: var(--muted); display: flex; justify-content: space-between; }
  .progress { font-weight: 500; letter-spacing: 0; text-transform: none; }
  .steps { list-style: none; margin: 0 0 var(--sp-6); padding: 0; display: flex; flex-direction: column; gap: var(--sp-2); }
  .steps li { display: flex; align-items: center; gap: var(--sp-3); padding: 10px 14px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-xl); }
  .mark { flex: none; width: 22px; height: 22px; border-radius: 50%; display: grid; place-items: center; font-size: var(--fs-xs); font-weight: 600; border: 1px solid var(--line); color: var(--muted); }
  .done .mark { background: var(--ok); border-color: var(--ok); color: var(--on-accent); }
  .done strong { color: var(--muted); }
  .body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
  .detail { color: var(--muted); font-size: var(--fs-sm); }
  li select { flex: none; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 12px; font: inherit; }
  .cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: var(--sp-3); }
  .cards button { text-align: left; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-xl); padding: 14px; font: inherit; display: flex; flex-direction: column; gap: 4px; }
  .sq { width: 32px; height: 32px; border-radius: 9px; display: grid; place-items: center; margin-bottom: 6px; }
  .cards .sq.import { background: var(--tint-accent); color: var(--accent); }
  .cards .sq.usage { background: var(--data-t); color: var(--data); }
  .cards button:hover { background: var(--raised-3); }
  .cards span { color: var(--muted); font-size: var(--fs-sm); }
  .home { display: flex; justify-content: center; margin-top: var(--sp-6); }
  .news { margin-top: var(--sp-6); }
  .news summary { cursor: pointer; color: var(--muted); }
</style>
