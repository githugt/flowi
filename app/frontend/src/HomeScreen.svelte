<script lang="ts">
  import IconButton from './IconButton.svelte'
  // The page the window opens with after the first launch: what is set up here, and the features not used yet.
  // The welcome page, with the get-started steps, stays a click away.
  import { onMount } from 'svelte'
  import Button from './Button.svelte'
  import Logo from './Logo.svelte'
  import Markdown from './Markdown.svelte'
  import { WelcomeService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { app } from './lib/app.svelte'
  import { live } from './lib/live'
  import { promptNewEnvironment } from './lib/menus'
  import { screens } from './lib/screens'
  import { breakdownText, hiddenCount, visibleIdeas, type Home } from './lib/home'
  import type { TreeNode } from './lib/types'

  let version = $state('')
  // Release notes since the last version that showed them; empty on the first launch and when nothing changed.
  let upgraded = $state('')
  onMount(() =>
    live<{ version: string; upgraded: string }>('welcome', () => WelcomeService.State(), (v) => {
      version = v.version
      upgraded = v.upgraded
    }),
  )

  // What the workspace holds. It is counted again whenever the workspace changes.
  let home = $state<Home | null>(null)
  $effect(() => {
    app.tree
    WelcomeService.Home().then((h) => (home = h as Home))
  })

  // Ideas the person has waved away stay away: a card they do not want must not come back every time.
  const dismissKey = 'flowi.home.dismissed'
  function readDismissed(): Set<string> {
    try {
      return new Set(JSON.parse(localStorage.getItem(dismissKey) ?? '[]'))
    } catch {
      return new Set() // storage can be blocked or hold something else; the cards just all show
    }
  }
  let dismissed = $state(readDismissed())
  function dismiss(id: string) {
    dismissed = new Set([...dismissed, id])
    try {
      localStorage.setItem(dismissKey, JSON.stringify([...dismissed]))
    } catch {
      /* not kept; the card is gone until the next visit */
    }
  }
  function restoreDismissed() {
    dismissed = new Set()
    try {
      localStorage.removeItem(dismissKey)
    } catch {
      /* nothing was kept to remove */
    }
  }
  const hidden = $derived(hiddenCount(home?.ideas ?? [], dismissed))
  const ideas = $derived(visibleIdeas(home?.ideas ?? [], dismissed, 4))
  let allIdeas = $state(false)
  const shownIdeas = $derived(allIdeas ? visibleIdeas(home?.ideas ?? [], dismissed, 99).shown : ideas.shown)

  function runIdea(action: string) {
    if (action.startsWith('screen:')) return app.openScreen(action, screens.find((s) => s.id === action)?.title ?? '')
    if (action === 'new-environment') return promptNewEnvironment()
    if (action === 'open-request') return void (firstRequest && app.open(firstRequest))
    app.onMenu(action) // new-request, new-flow
  }

  // Unsaved collections count too: the requests in them are as real as any.
  const collections = $derived((app.tree?.roots ?? []).filter((n) => n.kind === 'collection'))
  function firstOf(kind: string, nodes: TreeNode[]): TreeNode | undefined {
    for (const n of nodes) {
      if (n.kind === kind) return n
      const inner = firstOf(kind, n.children ?? [])
      if (inner) return inner
    }
  }
  const firstRequest = $derived(firstOf('request', collections))
  const tiles = $derived<{ n: number; label: string; note?: string; screen?: string; node?: TreeNode }[]>(
    home
      ? [
          { n: home.collections, label: 'Collections', screen: 'screen:manager' },
          { n: home.requests, label: 'Requests', note: breakdownText(home), node: firstRequest },
          { n: home.flows, label: 'Flows', node: firstOf('flow', collections) },
          // Before: this tile opened the Secrets screen, which is not the list of environments it counts.
          { n: home.environments, label: 'Environments', note: home.secretVars ? `${home.secretVars} secret ${home.secretVars === 1 ? 'value' : 'values'}` : '', node: firstOf('environment', app.tree?.roots ?? []) ?? firstOf('environment', collections), screen: undefined as string | undefined },
          { n: home.schedules, label: 'Schedules', screen: 'screen:schedules' },
          { n: home.blueprints, label: 'Blueprints', screen: 'screen:blueprints' },
        ]
      : [],
  )

  const open = (id: string) => app.openScreen(id, screens.find((s) => s.id === id)?.title ?? '')
</script>

<div class="page">
 <div class="col">
  <div class="hero">
    <div class="logo"><Logo size={56} /></div>
    <h1>{app.tree?.name ?? 'Home'}</h1>
    <p class="muted"><Button variant="link" title="About Flowi" onclick={() => open('screen:about')}>Version {version}</Button> · your requests and flows stay on this machine</p>
  </div>

  {#if upgraded}
    <div class="upgraded" role="status">
      <div class="head"><strong>Updated to {version}</strong><Button small onclick={() => WelcomeService.DismissUpgrade()}>Got it</Button></div>
      <Markdown text={upgraded} />
    </div>
  {/if}

  {#if home}
    {#if home.collections === 0}
      <div class="empty">
        <strong>Nothing is set up yet.</strong>
        <span>Open a collection or import an API to get going.</span>
        <div class="row"><Button variant="primary" onclick={() => open('screen:manager')}>Collections</Button><Button onclick={() => open('screen:import')}>Import an API</Button></div>
      </div>
    {:else}
      <h3><span>Your setup</span></h3>
      <div class="tiles">
        {#each tiles as t}
          {#if t.screen || t.node}
            <button class="tile" onclick={() => (t.screen ? open(t.screen) : app.open(t.node!))}>
              <strong class:zero={t.n === 0}>{t.n}</strong><span>{t.label}</span>{#if t.note}<em>{t.note}</em>{/if}
            </button>
          {:else}
            <div class="tile static"><strong class:zero={t.n === 0}>{t.n}</strong><span>{t.label}</span>{#if t.note}<em>{t.note}</em>{/if}</div>
          {/if}
        {/each}
      </div>

      {#if shownIdeas.length}
        <h3><span>Worth trying</span>{#if ideas.more && !allIdeas}<Button variant="link" onclick={() => (allIdeas = true)}>{ideas.more} more</Button>{/if}</h3>
        <div class="ideas">
          {#each shownIdeas as i (i.id)}
            <div class="idea">
              <IconButton icon="close" size={12} label="Dismiss {i.title}" hint="Don't show this again" onclick={() => dismiss(i.id)} />
              <strong>{i.title}</strong>
              <span>{i.why}</span>
              <Button small onclick={() => runIdea(i.action)}>{i.label}</Button>
            </div>
          {/each}
        </div>
      {/if}
    {/if}
  {/if}

  <div class="foot">
    <Button variant="link" onclick={() => open('screen:welcome')}>Welcome page</Button>
    <Button variant="link" onclick={() => open('screen:import')}>Import an API</Button>
    <Button variant="link" onclick={() => open('screen:telemetry')}>Usage reporting</Button>
    {#if hidden}<Button variant="link" onclick={restoreDismissed}>Show {hidden} hidden {hidden === 1 ? 'idea' : 'ideas'}</Button>{/if}
  </div>
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
  .upgraded { border: 1px solid var(--line); border-radius: var(--r-xl); padding: 12px 16px; margin-bottom: var(--sp-6); max-height: 240px; overflow: auto; background: var(--tint-accent); }
  .upgraded .head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 6px; }
  .empty { display: flex; flex-direction: column; align-items: flex-start; gap: 6px; padding: 16px; border: 1px dashed var(--line); border-radius: var(--r-xl); color: var(--muted); }
  .empty strong { color: var(--text); }
  .row { display: flex; gap: 8px; margin-top: 6px; }
  .tiles { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: var(--sp-2); margin-bottom: var(--sp-6); }
  .tile { text-align: left; display: flex; flex-direction: column; gap: 2px; padding: 12px 14px; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-xl); font: inherit; }
  button.tile:hover { background: var(--raised-3); }
  .tile strong { font-size: var(--fs-xl); font-weight: 650; line-height: 1.1; }
  .tile strong.zero { color: var(--muted); }
  .tile span { color: var(--muted); font-size: var(--fs-sm); }
  .tile em { font-style: normal; color: var(--muted); font-size: var(--fs-xs); }
  .ideas { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: var(--sp-3); margin-bottom: var(--sp-6); }
  .idea { position: relative; display: flex; flex-direction: column; align-items: flex-start; gap: 6px; padding: 14px 34px 14px 14px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-xl); }
  .idea span { color: var(--muted); font-size: var(--fs-sm); line-height: 1.45; flex: 1; }
  .idea :global(.iconbtn) { position: absolute; top: 4px; right: 6px; }
  .foot { display: flex; gap: 16px; justify-content: center; margin-top: var(--sp-6); font-size: var(--fs-sm); }
</style>
