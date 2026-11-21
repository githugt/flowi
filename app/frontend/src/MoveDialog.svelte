<script lang="ts">
  import { listKeyTarget } from './lib/listkeys'
  import { contains } from './lib/fold'
  import { isEnter } from './lib/enterkey'
  import Modal from './Modal.svelte'
  import SearchField from './SearchField.svelte'
  import Button from './Button.svelte'
  import EmptyState from './EmptyState.svelte'
  // Move to…: where one or more requests go, any collection or request folder. The old "Rename / move"
  // asked only for a path, inside the request's own collection.
  import { WorkspaceService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import { moveRequests } from './lib/move'
  import type { MoveTarget } from './lib/types'

  let targets = $state<MoveTarget[]>([])
  let query = $state('')
  let chosen = $state('')
  let filter = $state<HTMLInputElement>()

  const ids = $derived(ui.moving)
  $effect(() => {
    if (!ids) return
    query = ''
    chosen = ''
    WorkspaceService.MoveTargets().then((t) => (targets = t as unknown as MoveTarget[]))
    queueMicrotask(() => filter?.focus())
  })

  const shown = $derived.by(() => {
    const q = query.trim()
    return q ? targets.filter((t) => contains(t.label, q)) : targets
  })
  const title = $derived(ids && ids.length > 1 ? `Move ${ids.length} requests` : 'Move request')

  function close() {
    ui.moving = null
  }
  async function move() {
    const go = ids
    close()
    if (go && chosen) await moveRequests(go, chosen)
  }
  function keys(e: KeyboardEvent) {
    if (e.key === 'Escape') close()
    else if (isEnter(e) && chosen) move()
    else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      const i = shown.findIndex((t) => t.row === chosen)
      const next = shown[listKeyTarget(e.key, i, shown.length)!]
      if (next) chosen = next.row
    } else return
    e.preventDefault()
  }
</script>

{#if ids}
  <Modal {title} onclose={close} onkeydown={keys}>
      <p class="muted">To a collection, or a folder of its requests. Each request keeps its name.</p>
      <SearchField bind:el={filter} bind:value={query} label="Filter places" />
      <div class="list" role="listbox" aria-label="Places">
        {#each shown as t (t.row)}
          <button type="button" role="option" aria-selected={chosen === t.row} class:on={chosen === t.row} class:top={t.depth === 0}
            style:padding-left={`${10 + (query ? 0 : t.depth * 16)}px`} onclick={() => (chosen = t.row)} ondblclick={() => { chosen = t.row; move() }}>
            {query ? t.label : t.label.split(' / ').pop()}{#if t.draft}<span class="tag">draft</span>{/if}
          </button>
        {:else}
          <EmptyState kind="filtered" compact {query} onclear={() => (query = '')} />
        {/each}
      </div>
    {#snippet actions()}
      <Button type="button" onclick={close}>Cancel</Button>
      <Button type="button" variant="primary" disabled={!chosen} onclick={move}>Move</Button>
    {/snippet}
  </Modal>
{/if}

<style>
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .list { flex: 1; min-height: 120px; max-height: 340px; overflow: auto; border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px; background: var(--bg); }
  .list button { width: 100%; display: flex; align-items: center; gap: 6px; text-align: left; background: none; border: 0; border-radius: var(--r-sm); color: var(--text); font: inherit; padding-block: 4px; padding-right: 10px; }
  .list button:hover { background: var(--tint-hover); }
  .list button.on { background: var(--tint-sel); }
  .list button.top { font-weight: 600; }
  .tag { margin-left: auto; color: var(--muted); font-size: var(--fs-xs); font-weight: 400; }
</style>
