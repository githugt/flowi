<script lang="ts">
  // "New flow from blueprint": pick a blueprint, where the flow goes, and its
  // parameters. Opened from the New menu, from a folder's menu and from the
  // Blueprints screen.
  import { BlueprintsService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import type { BlueprintsView } from './lib/types'

  let view = $state<BlueprintsView>({ blueprints: [], collections: [] })
  let key = $state('')
  let collection = $state('')
  let path = $state('')
  let values = $state<Record<string, string>>({})

  const open = $derived(ui.blueprint)
  const flows = $derived(view.blueprints.filter((b) => b.kind === 'flow'))
  const chosen = $derived(flows.find((b) => b.key === key))

  $effect(() => {
    const ask = ui.blueprint
    if (!ask) return
    BlueprintsService.State().then((v) => {
      view = v as BlueprintsView
      const first = flows.find((b) => b.key === ask.blueprint) ?? flows[0]
      collection = ask.collection || first?.collection || view.collections[0]?.id || ''
      pick(first?.key ?? '', ask.folder)
    })
  })

  // Choosing a blueprint fills in its parameters and a flow path inside the folder.
  function pick(k: string, folder = ui.blueprint?.folder ?? '') {
    key = k
    const b = flows.find((x) => x.key === k)
    values = Object.fromEntries((b?.params ?? []).map((p) => [p.name, p.default]))
    const name = b ? (b.path.split('/').pop() ?? b.path) : ''
    path = folder ? `${folder}/${name}` : name
  }

  function create() {
    const b = chosen
    if (!b || !path.trim()) return
    ui.blueprint = null
    BlueprintsService.CreateFlow(b.key, collection, path.trim(), $state.snapshot(values))
  }
</script>

{#if open}
  <div class="scrim" role="presentation" onpointerdown={() => (ui.blueprint = null)}>
    <form class="dialog" onpointerdown={(e) => e.stopPropagation()} onsubmit={(e) => { e.preventDefault(); create() }}>
      <h3>New flow from blueprint</h3>
      {#if !flows.length}
        <p class="muted">No collection has a flow blueprint yet. Open a flow's ⋯ menu and choose "Save as blueprint…" to make one.</p>
      {:else}
        <label><span>Blueprint</span>
          <select value={key} onchange={(e) => pick(e.currentTarget.value)}>
            {#each flows as b}<option value={b.key}>{b.name}{view.collections.length > 1 ? ` · ${b.collectionName}` : ''}</option>{/each}
          </select>
        </label>
        {#if chosen?.description}<p class="muted">{chosen.description}</p>{/if}
        <label><span>Collection</span>
          <select bind:value={collection}>{#each view.collections as c}<option value={c.id}>{c.label}</option>{/each}</select>
        </label>
        <label><span>Flow path</span><input bind:value={path} spellcheck="false" /></label>
        {#each Object.keys(values) as name}
          <label><span>{name}</span><input bind:value={values[name]} spellcheck="false" /></label>
        {/each}
      {/if}
      <div class="actions">
        <button type="button" onclick={() => (ui.blueprint = null)}>Cancel</button>
        <button class="primary" type="submit" disabled={!chosen}>Create</button>
      </div>
    </form>
  </div>
{/if}

<style>
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 60; }
  .dialog { width: 420px; max-height: 80vh; overflow: auto; display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); }
  .dialog h3 { margin: 0; }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  label { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  input, select { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: inherit; -webkit-user-select: text; user-select: text; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 12px; font: inherit; }
  .primary { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
  button:disabled { opacity: 0.4; }
</style>
