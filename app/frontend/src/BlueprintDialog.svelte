<script lang="ts">
  import Modal from './Modal.svelte'
  import Select from './Select.svelte'
  import TextInput from './TextInput.svelte'
  import Button from './Button.svelte'
  import EmptyState from './EmptyState.svelte'
  // "New flow from blueprint": pick a blueprint, where the flow goes, and its
  // parameters. Opened from the New menu, from a folder's menu and from the
  // Blueprints screen.
  import { BlueprintsService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import { missingParams } from './lib/blueprintform'
  import type { BlueprintsView } from './lib/types'

  let view = $state<BlueprintsView>({ blueprints: [], collections: [] })
  let key = $state('')
  let collection = $state('')
  let path = $state('')
  let values = $state<Record<string, string>>({})

  const open = $derived(ui.blueprint)
  const flows = $derived(view.blueprints.filter((b) => b.kind === 'flow'))
  const chosen = $derived(flows.find((b) => b.key === key))
  const missing = $derived(chosen ? missingParams(chosen.params, values) : [])

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
    if (!b || !path.trim() || missing.length) return
    ui.blueprint = null
    BlueprintsService.CreateFlow(b.key, collection, path.trim(), $state.snapshot(values))
  }
</script>

{#if open}
  <Modal title="New flow from blueprint" scroll onclose={() => (ui.blueprint = null)} onsubmit={() => create()}>
      {#if !flows.length}
        <EmptyState compact icon="flow" title="No flow blueprints yet" hint={'Open a flow\'s ⋯ menu and choose "Save as blueprint…" to make one.'} />
      {:else}
        <label><span>Blueprint</span>
          <Select value={key} onchange={(e) => pick(e.currentTarget.value)}>
            {#each flows as b}<option value={b.key}>{b.name}{view.collections.length > 1 ? ` · ${b.collectionName}` : ''}</option>{/each}
          </Select>
        </label>
        {#if chosen?.description}<p class="muted">{chosen.description}</p>{/if}
        <label><span>Collection</span>
          <Select bind:value={collection}>{#each view.collections as c}<option value={c.id}>{c.label}</option>{/each}</Select>
        </label>
        <label><span>Flow path</span><TextInput bind:value={path} label="Flow path" /></label>
        {#each Object.keys(values) as name}
          {@const spec = chosen?.params.find((p) => p.name === name)}
          <label><span>{name}{spec && spec.default === '' ? ' *' : ''}</span><TextInput bind:value={values[name]} label={name} /></label>
          {#if spec?.description}<p class="muted">{spec.description}</p>{/if}
        {/each}
        {#if chosen?.steps.length}
          <p class="muted">Makes {chosen.steps.length} {chosen.steps.length === 1 ? 'step' : 'steps'}: {chosen.steps.map((x) => x.name).join(' → ')}</p>
        {/if}
        {#if missing.length}<p class="muted" role="note">Fill in {missing.join(', ')} first.</p>{/if}
      {/if}
    {#snippet actions()}
      <Button type="button" onclick={() => (ui.blueprint = null)}>Cancel</Button>
      <Button variant="primary" type="submit" disabled={!chosen || missing.length > 0}>Create</Button>
    {/snippet}
  </Modal>
{/if}

<style>
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  label { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
</style>
