<script lang="ts">
  // Move to…: where one or more requests go, any collection or request folder. The old "Rename / move"
  // asked only for a path, inside the request's own collection.
  import { WorkspaceService } from '../bindings/github.com/githugt/flowi/internal/bridge'
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
    const q = query.trim().toLowerCase()
    return q ? targets.filter((t) => t.label.toLowerCase().includes(q)) : targets
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
    else if (e.key === 'Enter' && chosen) move()
    else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      const i = shown.findIndex((t) => t.row === chosen)
      const next = shown[Math.min(Math.max(i + (e.key === 'ArrowDown' ? 1 : -1), 0), shown.length - 1)]
      if (next) chosen = next.row
    } else return
    e.preventDefault()
  }
</script>

{#if ids}
  <div class="scrim" role="presentation" onpointerdown={close}>
    <div class="dialog" role="dialog" aria-label={title} tabindex="-1" onpointerdown={(e) => e.stopPropagation()} onkeydown={keys}>
      <h3>{title}</h3>
      <p class="muted">To a collection, or a folder of its requests. Each request keeps its name.</p>
      <input bind:this={filter} type="search" placeholder="Filter" bind:value={query} spellcheck="false" aria-label="Filter places" />
      <div class="list" role="listbox" aria-label="Places">
        {#each shown as t (t.row)}
          <button type="button" role="option" aria-selected={chosen === t.row} class:on={chosen === t.row} class:top={t.depth === 0}
            style:padding-left={`${10 + (query ? 0 : t.depth * 16)}px`} onclick={() => (chosen = t.row)} ondblclick={() => { chosen = t.row; move() }}>
            {query ? t.label : t.label.split(' / ').pop()}{#if t.draft}<span class="tag">draft</span>{/if}
          </button>
        {:else}
          <p class="muted empty">No place matches.</p>
        {/each}
      </div>
      <div class="actions">
        <button type="button" onclick={close}>Cancel</button>
        <button type="button" class="primary" disabled={!chosen} onclick={move}>Move</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 60; }
  .dialog { width: 420px; max-height: 80vh; display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); outline: none; }
  h3 { margin: 0; }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  input { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: inherit; -webkit-user-select: text; user-select: text; }
  .list { flex: 1; min-height: 120px; max-height: 340px; overflow: auto; border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px; background: var(--bg); }
  .list button { width: 100%; display: flex; align-items: center; gap: 6px; text-align: left; background: none; border: 0; border-radius: var(--r-sm); color: var(--text); font: inherit; padding-block: 4px; padding-right: 10px; }
  .list button:hover { background: color-mix(in srgb, var(--accent) 10%, transparent); }
  .list button.on { background: color-mix(in srgb, var(--accent) 24%, transparent); }
  .list button.top { font-weight: 600; }
  .tag { margin-left: auto; color: var(--muted); font-size: var(--fs-xs); font-weight: 400; }
  .empty { padding: 8px; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
  .actions button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 12px; font: inherit; }
  .actions .primary { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
  .actions button:disabled { opacity: 0.4; }
</style>
