<script lang="ts">
  // Drafts (try-out requests and flows of the Scratch) that sat unused are moved here;
  // each can be put back or deleted for good, with a question first.
  import { errorText } from './lib/errors'
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { TrashService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import type { TrashView } from './lib/types'

  let view = $state<TrashView>({ items: [], after: '' })

  onMount(() => live<TrashView>('trash', () => TrashService.State(), (v) => (view = v)))

  async function restore(id: string) {
    try {
      ui.toast(`Put back in the Drafts as ${await TrashService.Restore(id)}`)
    } catch (e) {
      ui.toast(errorText(e), 'error')
    }
  }

  function remove(id: string, name: string) {
    ui.confirm('Delete for good', `Delete "${name}" for good? This cannot be undone.`, async () => {
      const problem = (await TrashService.Delete(id)) as string
      if (problem) ui.toast(problem, 'error')
    })
  }

  function empty() {
    ui.confirm('Empty the trash', `Delete all ${view.items.length} items for good? This cannot be undone.`, async () => {
      const problem = (await TrashService.Empty()) as string
      if (problem) ui.toast(problem, 'error')
    }, 'Empty')
  }
</script>

<div class="page">
  <p class="note">
    {#if view.after}A try-out request or flow in the Drafts that was not changed for {view.after} and is not open in a tab is moved here. The time is the setting <code>ui.drafts_trash_after</code>.{:else}Nothing is moved here by itself: the setting <code>ui.drafts_trash_after</code> is 0s.{/if}
    What is here stays until you delete it.
  </p>
  <div class="bar"><strong>{view.items.length} {view.items.length === 1 ? 'item' : 'items'}</strong><span class="spacer"></span><button disabled={!view.items.length} onclick={empty}>Empty the trash…</button></div>
  {#if view.error}<p class="err">{view.error}</p>{/if}
  {#each view.items as it (it.id)}
    <div class="row">
      <span class="kind">{it.kind}</span>
      <strong>{it.name || it.path}</strong>
      <span class="muted">{it.path}</span>
      <span class="muted at">moved {it.at}</span>
      <span class="spacer"></span>
      <button onclick={() => restore(it.id)}>Put back</button>
      <button class="del" onclick={() => remove(it.id, it.name || it.path)}>Delete…</button>
    </div>
  {:else}
    <p class="muted">The trash is empty.</p>
  {/each}
</div>

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: 16px 24px 48px; max-width: 820px; }
  .note, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 6px; }
  .bar { display: flex; align-items: center; margin: 10px 0 6px; }
  .spacer { flex: 1; }
  .err { color: var(--fail); }
  .row { display: flex; align-items: center; gap: 10px; padding: 6px 4px; border-bottom: 1px solid var(--line); font-size: var(--fs-sm); }
  .kind { font-size: var(--fs-xs); border: 1px solid var(--line); border-radius: var(--r-pill); padding: 0 8px; color: var(--muted); }
  .at { white-space: nowrap; }
  code { font: var(--fs-xs) var(--mono); }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 2px 10px; font: inherit; cursor: pointer; }
  button:disabled { opacity: 0.4; }
  .del:hover { color: var(--fail); border-color: var(--fail); }
</style>
