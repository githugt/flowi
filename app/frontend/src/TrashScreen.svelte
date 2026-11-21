<script lang="ts">
  import ScreenPage from './ScreenPage.svelte'
  // What was deleted from a collection, and drafts (try-out requests and flows of the Scratch) that sat unused, wait here;
  // each can be put back or deleted for good, with a question first.
  import EmptyState from './EmptyState.svelte'
  import { errorText } from './lib/errors'
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { TrashService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import type { TrashView } from './lib/types'
  import { agoText } from './lib/ago'
  import Button from './Button.svelte'

  let view = $state<TrashView>({ items: [], after: '' })

  onMount(() => live<TrashView>('trash', () => TrashService.State(), (v) => (view = v)))

  async function restore(id: string) {
    const deleted = view.items.find((it) => it.id === id)?.deleted
    try {
      const path = await TrashService.Restore(id)
      ui.toast(deleted ? `Put back as ${path}` : `Put back in the Drafts as ${path}`)
    } catch (e) {
      ui.toast(errorText(e), 'error')
    }
  }

  // Every item goes back; the count of the ones that could not is said once.
  async function restoreAll() {
    let failed = 0
    for (const it of [...view.items]) {
      try {
        await TrashService.Restore(it.id)
      } catch {
        failed++
      }
    }
    ui.toast(failed ? `${failed} could not be put back` : 'Put everything back', failed ? 'error' : 'info')
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

<ScreenPage>
  <p class="note">
    {#if view.after}A request or flow in Drafts that was not changed for {view.after} and is not open in a tab is moved here. The time is the setting <code>ui.drafts_trash_after</code>.{:else}Nothing is moved here by itself: the setting <code>ui.drafts_trash_after</code> is 0s.{/if}
    A request or flow you delete from the tree is moved here too, and goes back to its collection. What is here stays until you delete it.
  </p>
  <div class="bar"><strong>{view.items.length} {view.items.length === 1 ? 'item' : 'items'}</strong><span class="spacer"></span><Button small disabled={!view.items.length} onclick={restoreAll}>Put all back</Button> <Button small disabled={!view.items.length} onclick={empty}>Empty the trash…</Button></div>
  {#if view.error}<p class="err">{view.error}</p>{/if}
  {#each view.items as it (it.id)}
    <div class="row">
      <span class="kind">{it.kind}</span>
      <strong>{it.name || it.path}</strong>
      <span class="muted">{it.path}</span>
      <span class="muted at" title={it.at}>moved {agoText(it.at, new Date())}</span>
      <span class="spacer"></span>
      <Button small onclick={() => restore(it.id)}>Put back</Button>
      <Button small variant="danger" onclick={() => remove(it.id, it.name || it.path)}>Delete…</Button>
    </div>
  {:else}
    <EmptyState icon="trash" title="The trash is empty" hint="What you delete waits here until you remove it for good." compact />
  {/each}
</ScreenPage>

<style>
  .note, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 6px; }
  .bar { display: flex; align-items: center; margin: 10px 0 6px; }
  .spacer { flex: 1; }
  .err { color: var(--fail); }
  .row { display: flex; align-items: center; gap: 10px; padding: 6px 4px; border-bottom: 1px solid var(--line); font-size: var(--fs-sm); }
  .kind { font-size: var(--fs-xs); border: 1px solid var(--line); border-radius: var(--r-pill); padding: 0 8px; color: var(--muted); }
  .at { white-space: nowrap; }
  code { font: var(--fs-xs) var(--mono); }
</style>
