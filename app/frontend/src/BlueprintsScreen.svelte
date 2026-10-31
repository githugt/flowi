<script lang="ts">
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { BlueprintsService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import type { BlueprintsView } from './lib/types'

  let view = $state<BlueprintsView>({ blueprints: [], collections: [] })

  onMount(() => live<BlueprintsView>('blueprints', () => BlueprintsService.State(), (v) => (view = v)))

  const kindLabel = { flow: 'Flow', 'post-hooks': 'Post-request hooks', 'pre-hooks': 'Pre-request hooks' }
</script>

<div class="page">
  <p class="note">Blueprints are flow templates and reusable hooks kept in a collection's blueprints folder. Using one makes an ordinary flow; nothing links back.</p>
  <p class="note">The built-in hooks are what new requests start with, unless the setting <code>requests.seed_blueprint</code> names other blueprints. They cannot be deleted.</p>
  {#if !view.blueprints.some((b) => !b.builtIn)}<p class="note">No collection has a blueprint of its own yet. Save a flow as a blueprint to start one.</p>{/if}
  {#each view.blueprints as b (b.key)}
    <div class="card">
      <div class="top"><strong>{b.name}</strong><span class="tag">{kindLabel[b.kind]}</span>{#if b.builtIn}<span class="tag builtin">Built in</span>{:else}<span class="muted">{b.collectionName}</span>{/if}<span class="spacer"></span>
        {#if b.builtIn}
          <!-- Shipped with Flowi: nothing to delete, and new requests use it unless told otherwise. -->
        {:else if b.kind === 'flow'}<button onclick={() => (ui.blueprint = { collection: b.collection, folder: '', blueprint: b.key })}>New flow from this…</button>
        {:else}<button onclick={() => BlueprintsService.UseAsDefault(b.key)}>Use for new requests</button>{/if}
        {#if !b.builtIn}<button onclick={() => ui.confirm('Delete blueprint', `Delete the blueprint ${b.name}? Flows made from it are not affected.`, () => BlueprintsService.Delete(b.key))}>Delete…</button>{/if}
      </div>
      {#if b.description}<p class="muted">{b.description}</p>{/if}
      {#if b.builtIn}
        <div class="muted">{b.nodes} {b.nodes === 1 ? 'step' : 'steps'}</div>
      {:else}
        <div class="muted">{b.params.length} parameter(s) · {b.nodes} step(s) · <code>{b.path}</code></div>
      {/if}
    </div>
  {/each}

</div>

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: 16px 24px; max-width: 820px; }
  .note, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 4px; }
  .card { border: 1px solid var(--line); border-radius: var(--r-lg); padding: 8px 12px; margin: 8px 0; }
  .top { display: flex; align-items: center; gap: 10px; }
  .spacer { flex: 1; }
  .tag { font-size: var(--fs-xs); border: 1px solid var(--line); border-radius: var(--r-pill); padding: 0 8px; color: var(--muted); }
  .builtin { color: var(--accent); border-color: color-mix(in srgb, var(--accent) 45%, transparent); }
  code { font: var(--fs-xs) var(--mono); }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 12px; font: inherit; }
</style>
