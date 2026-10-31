<script lang="ts">
  // Where it goes: one chooser for New request, New message request, New gRPC request, New flow, Save and
  // the importer. A searchable tree of collections and the folders in them (the Move dialog's list); a
  // collection or a folder can be made here, so nobody has to leave the dialog to have somewhere to put it.
  import { WorkspaceService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import Dropdown from './Dropdown.svelte'
  import { protocolVerb } from './lib/verbs'
  import { app } from './lib/app.svelte'
  import { ui } from './lib/ui.svelte'
  import type { MoveTarget } from './lib/types'

  let all = $state<MoveTarget[]>([])
  let query = $state('')
  let chosen = $state('')
  let name = $state('')
  let protocol = $state('')
  let making = $state<'collection' | 'folder' | null>(null)
  let madeName = $state('')
  let nameBox = $state<HTMLInputElement>()
  let filter = $state<HTMLInputElement>()
  let madeBox = $state<HTMLInputElement>()

  const d = $derived(ui.place)
  $effect(() => {
    if (!d) return
    query = ''
    chosen = ''
    making = null
    name = d.name?.initial ?? ''
    protocol = d.protocols?.[0]?.id ?? ''
    load(d.row ?? app.tree?.selected ?? '')
    queueMicrotask(() => (nameBox ?? filter)?.select())
  })

  const targets = $derived(d?.collectionsOnly ? all.filter((t) => t.depth === 0) : all)
  const shown = $derived.by(() => {
    const q = query.trim().toLowerCase()
    return q ? targets.filter((t) => t.label.toLowerCase().includes(q)) : targets
  })
  const place = $derived(targets.find((t) => t.row === chosen))
  const ready = $derived(!!place && (!d?.name?.required || name.trim() !== ''))

  async function load(pick: string) {
    const list = (await WorkspaceService.PlaceTargets(d?.tree ?? 'requests')) as unknown as MoveTarget[]
    all = list
    const there = (row: string) => list.some((t) => t.row === row)
    chosen = there(pick) ? pick : there(chosen) ? chosen : (list[0]?.row ?? '')
  }

  function close() {
    ui.place = null
  }
  function accept() {
    if (!d || !ready) return
    const done = d.done
    const args = [chosen, name, protocol] as const
    close()
    done(...args)
  }

  async function startMaking(what: 'collection' | 'folder') {
    making = what
    madeName = ''
    queueMicrotask(() => madeBox?.focus())
  }
  async function make() {
    if (!making || madeName.trim() === '') return
    const row = making === 'collection' ? await WorkspaceService.NewPlaceCollection(madeName) : await WorkspaceService.NewPlaceFolder(chosen, d?.tree ?? 'requests', madeName)
    making = null
    if (row) await load(row)
  }

  function keys(e: KeyboardEvent) {
    const typing = e.target instanceof HTMLInputElement
    if (e.key === 'Escape') {
      if (making) making = null
      else close()
    } else if (e.key === 'Enter' && making) make()
    else if (e.key === 'Enter' && (!typing || e.target !== filter)) accept()
    else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      const i = shown.findIndex((t) => t.row === chosen)
      const next = shown[Math.min(Math.max(i + (e.key === 'ArrowDown' ? 1 : -1), 0), shown.length - 1)]
      if (next) chosen = next.row
    } else return
    e.preventDefault()
  }
</script>

{#if d}
  <div class="scrim" role="presentation" onpointerdown={close}>
    <div class="dialog" role="dialog" aria-label={d.title} tabindex="-1" onpointerdown={(e) => e.stopPropagation()} onkeydown={keys}>
      <h3>{d.title}</h3>
      {#if d.note}<p class="muted">{d.note}</p>{/if}
      {#if d.name}
        <label class="f"><span>{d.name.label}</span>
          <input bind:this={nameBox} bind:value={name} spellcheck="false" autocomplete="off" />
        </label>
      {/if}
      {#if d.protocols}
        <label class="f"><span>{d.protocolsLabel ?? 'Protocol'}</span>
          <Dropdown pill options={d.protocols.map((p) => ({ value: p.id, label: p.label, tone: protocolVerb(p.id) }))} value={protocol} onpick={(p) => (protocol = p)} />
        </label>
      {/if}
      <div class="where">
        <span class="label">{d.collectionsOnly ? 'Collection' : 'Collection or folder'}</span>
        <span class="make">
          <button type="button" onclick={() => startMaking('collection')}>New collection</button>
          {#if !d.collectionsOnly}<button type="button" disabled={!chosen} onclick={() => startMaking('folder')} title={chosen ? 'Inside the chosen place' : 'Choose a collection first'}>New folder</button>{/if}
        </span>
      </div>
      {#if making}
        <div class="making">
          <input bind:this={madeBox} bind:value={madeName} placeholder={making === 'collection' ? 'Collection name' : `Folder name, inside ${place?.label.split(' / ').pop() ?? ''}`} spellcheck="false" aria-label={making === 'collection' ? 'New collection name' : 'New folder name'} />
          <button type="button" class="primary" disabled={madeName.trim() === ''} onclick={make}>Create</button>
          <button type="button" onclick={() => (making = null)}>Cancel</button>
        </div>
      {/if}
      <input bind:this={filter} type="search" placeholder="Search" bind:value={query} spellcheck="false" aria-label="Search places" />
      <div class="list" role="listbox" aria-label="Places">
        {#each shown as t (t.row)}
          <button type="button" role="option" aria-selected={chosen === t.row} class:on={chosen === t.row} class:top={t.depth === 0}
            style:padding-left={`${10 + (query ? 0 : t.depth * 16)}px`} onclick={() => (chosen = t.row)} ondblclick={() => { chosen = t.row; accept() }}>
            {query ? t.label : t.label.split(' / ').pop()}{#if t.draft}<span class="tag">draft</span>{/if}
          </button>
        {:else}
          <p class="muted empty">{all.length ? 'No place matches.' : 'No collection yet. Use New collection above.'}</p>
        {/each}
      </div>
      {#if place && d.name}<p class="muted at">Goes to <strong>{place.label}</strong> as <code>{name.trim() || 'untitled'}</code></p>{/if}
      <div class="actions">
        <button type="button" onclick={close}>Cancel</button>
        <button type="button" class="primary" disabled={!ready} onclick={accept}>{d.action}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 65; }
  .dialog { width: 440px; max-width: calc(100vw - 32px); max-height: 86vh; display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); outline: none; }
  h3 { margin: 0; }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .at code { font: var(--fs-xs) var(--mono); }
  .f { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  input { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: inherit; -webkit-user-select: text; user-select: text; }
  input:focus { border-color: var(--accent); }
  .where { display: flex; align-items: center; justify-content: space-between; gap: 8px; color: var(--muted); font-size: var(--fs-xs); }
  .make { display: flex; gap: 6px; }
  .make button, .making button, .actions button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 10px; font: inherit; white-space: nowrap; }
  .make button { color: var(--accent); font-size: var(--fs-sm); }
  .making { display: flex; gap: 6px; }
  .making input { flex: 1; min-width: 0; }
  .list { flex: 1; min-height: 120px; max-height: 280px; overflow: auto; border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px; background: var(--bg); }
  .list button { width: 100%; display: flex; align-items: center; gap: 6px; text-align: left; background: none; border: 0; border-radius: var(--r-sm); color: var(--text); font: inherit; padding-block: 4px; padding-right: 10px; }
  .list button:hover { background: color-mix(in srgb, var(--accent) 10%, transparent); }
  .list button.on { background: color-mix(in srgb, var(--accent) 24%, transparent); }
  .list button.top { font-weight: 600; }
  .tag { margin-left: auto; color: var(--muted); font-size: var(--fs-xs); font-weight: 400; }
  .empty { padding: 8px; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
  .primary { background: var(--accent) !important; color: var(--on-accent) !important; border-color: var(--accent) !important; }
  button:disabled { opacity: 0.4; }
</style>
