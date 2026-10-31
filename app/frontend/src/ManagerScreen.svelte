<script lang="ts">
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { ManagerService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import type { KeptSpec, ManagerView } from './lib/types'
  import PathField from './PathField.svelte'
  import SpecUpdateDialog from './SpecUpdateDialog.svelte'
  import KVRows from './KVRows.svelte'
  import { errorText } from './lib/errors'
  import type { KV } from './lib/types'

  let view = $state<ManagerView>({ collections: [] })
  let mountDir = $state('')
  let bundleFile = $state('')
  let arazzoRef = $state('')
  let newName = $state('')
  let newLocation = $state('')
  let relocate = $state<Record<string, string>>({})
  let draftDir = $state<Record<string, string>>({})
  // A collection's path field starts empty: binding the missing entry (undefined) to PathField, whose
  // value has a default, threw (props_invalid_value) and the screen showed no collections at all.

  onMount(() => live<ManagerView>('manager', () => ManagerService.State(), (v) => (view = v)))

  // The variables of each collection: read in templates as {{collection_var.name}}; the selected
  // environment overrides one of the same name.
  let variables = $state<Record<string, KV[]>>({})
  let variableError = $state<Record<string, string>>({})
  $effect(() => {
    for (const c of view.collections) {
      if (c.missing || variables[c.id]) continue
      ManagerService.Variables(c.id)
        .then((rows) => (variables[c.id] = rows as unknown as KV[]))
        .catch(() => (variables[c.id] = []))
    }
  })
  async function saveVariables(id: string, rows: KV[], drop = false) {
    variableError[id] = ''
    try {
      const needsConfirm = await ManagerService.SetVariables(id, rows.filter((r) => r.Key) as never, drop)
      if (needsConfirm) {
        ui.confirm('Comments will be lost', 'The collection file has comments; saving rewrites it without them. Save anyway?', () => saveVariables(id, rows, true), 'Save')
        return
      }
      variables[id] = rows
    } catch (e) {
      variableError[id] = errorText(e)
    }
  }

  // The specs each collection keeps (imported OpenAPI documents), for Update from spec.
  let specs = $state<Record<string, KeptSpec[]>>({})
  let updating = $state<{ collection: string; spec: KeptSpec } | null>(null)
  $effect(() => {
    for (const c of view.collections) {
      if (c.missing) continue
      ManagerService.KeptSpecs(c.id)
        .then((k) => (specs[c.id] = k as unknown as KeptSpec[]))
        .catch(() => (specs[c.id] = [])) // a folder that cannot be read offers no update
    }
  })
</script>

{#if updating}<SpecUpdateDialog collection={updating.collection} spec={updating.spec} onclose={() => (updating = null)} />{/if}
<div class="page">
  {#if view.error}<p class="err">{view.error}</p>{/if}
  <h3>Collections</h3>
  {#each view.collections as c (c.id)}
    <div class="card">
      <div class="top">
        <strong>{c.name || c.id}</strong>
        {#if c.missing}<span class="tag bad">missing</span>{/if}
        {#if c.draft}<span class="tag">draft</span>{/if}
        <span class="spacer"></span>
        {#if c.draft}
          <button onclick={() => ui.confirm('Delete collection', 'Delete this collection and everything in it? This cannot be undone.', () => ManagerService.Discard(c.id))}>Delete…</button>
        {:else}
          <button onclick={() => ManagerService.Unmount(c.id)}>Unmount</button>
        {/if}
      </div>
      <code class="path">{c.path || '(kept in Flowi’s own folder)'}</code>
      {#each specs[c.id] ?? [] as k (k.name)}
        <div class="line spec">
          <span>Spec <code>{k.spec}</code>{#if k.overlays.length}<span class="muted">{` · overlays: ${k.overlays.join(', ')}`}</span>{/if}</span>
          <span class="spacer"></span>
          <button onclick={() => (updating = { collection: c.id, spec: k })}>Update from spec…</button>
        </div>
      {/each}
      {#if !c.missing}
        <details class="vars">
          <summary>Variables {#if (variables[c.id] ?? []).length}<span class="muted">({variables[c.id].length})</span>{/if}</summary>
          <p class="muted">Values every request and flow of this collection can read as <code>{'{{collection_var.name}}'}</code>. An environment variable of the same name overrides one.</p>
          <KVRows rows={variables[c.id] ?? []} keyLabel="name" onchange={(rows) => saveVariables(c.id, rows)}
            empty={{ icon: 'list', title: 'No collection variables', hint: 'A base URL or a region that belongs to this collection, not to an environment.', add: 'Add variable' }} />
          {#if variableError[c.id]}<p class="err">{variableError[c.id]}</p>{/if}
        </details>
      {/if}
      {#each c.deps as d}
        <div class="dep"><span class:ok={d.mounted} class:bad={!d.mounted}>{d.mounted ? '✓' : '✗'}</span> depends on {d.name} <span class="muted">{d.repo}</span></div>
      {/each}
      {#if c.missing}
        <div class="line">
          <PathField bind:value={() => relocate[c.id] ?? '', (v) => (relocate[c.id] = v)} label="New location" />
          <button disabled={!relocate[c.id]} onclick={() => ManagerService.Relocate(c.id, relocate[c.id])}>Relocate</button>
        </div>
      {/if}
      {#if c.draft}
        <div class="line">
          <PathField bind:value={() => draftDir[c.id] ?? '', (v) => (draftDir[c.id] = v)} label="Move it into this empty folder" />
          <button disabled={!draftDir[c.id]} onclick={() => ManagerService.SaveDraft(c.id, draftDir[c.id])}>Save here</button>
        </div>
      {/if}
    </div>
  {/each}

  <h3>Mount an existing collection</h3>
  <div class="line">
    <PathField bind:value={mountDir} label="Folder with a flowi.yaml" />
    <button disabled={!mountDir} onclick={() => { ManagerService.Mount(mountDir); mountDir = '' }}>Mount</button>
  </div>

  <h3>Open an Arazzo document</h3>
  <p class="muted">An Arazzo file (or an address) with the OpenAPI description it names. It opens as a collection of its workflows, made in memory and gone when you quit; nothing is mounted or written. Press Run on a flow with <b>No environment</b> chosen to use the server the description names, or choose an environment that has a <code>base_url</code>.</p>
  <div class="line">
    <PathField bind:value={arazzoRef} label="An .arazzo.yaml file or an https:// address" kind="file" />
    <button disabled={!arazzoRef} onclick={() => { ManagerService.OpenArazzo(arazzoRef); arazzoRef = '' }}>Open</button>
  </div>

  <h3>Open a run bundle</h3>
  <p class="muted">A .flowi-run file is one run of a flow: the steps, the seed, the inputs and every call. It opens as a collection; press Run on its flow to see the run again, without sending anything.</p>
  <div class="line">
    <PathField bind:value={bundleFile} label="A .flowi-run file" kind="file" />
    <button disabled={!bundleFile} onclick={() => { ManagerService.OpenBundle(bundleFile); bundleFile = '' }}>Open</button>
  </div>

  <h3>New collection</h3>
  <label class="f"><span>Name</span><input bind:value={newName} spellcheck="false" /></label>
  <div class="line">
    <PathField bind:value={newLocation} label="Folder (empty: Flowi’s own folder)" />
    <button disabled={!newName} onclick={() => { ManagerService.NewCollection(newName, newLocation); newName = ''; newLocation = '' }}>Create</button>
  </div>
</div>

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: 16px 24px 40px; max-width: 820px; }
  h3 { margin: 24px 0 8px; font-size: var(--fs-md); border-bottom: 1px solid var(--line); padding-bottom: 4px; }
  h3:first-of-type { margin-top: 0; }
  .err { color: var(--fail); }
  .card { border: 1px solid var(--line); border-radius: var(--r-lg); padding: 10px 12px; margin: 8px 0; display: flex; flex-direction: column; gap: 6px; }
  .top { display: flex; align-items: center; gap: 8px; }
  .spacer { flex: 1; }
  .path { color: var(--muted); font: var(--fs-sm) var(--mono); }
  .tag { font-size: var(--fs-xs); border: 1px solid var(--line); border-radius: var(--r-pill); padding: 0 8px; color: var(--muted); }
  .tag.bad, .bad { color: var(--fail); }
  .ok { color: var(--ok); }
  .muted { color: var(--muted); }
  .dep { font-size: var(--fs-sm); }
  .vars summary { cursor: default; font-size: var(--fs-sm); }
  .vars p { margin: 4px 0; font-size: var(--fs-sm); }
  .line { display: flex; gap: 8px; align-items: flex-end; }
  .line :global(.f) { flex: 1; }
  .line.spec { align-items: center; font-size: var(--fs-sm); }
  .line.spec code { font-size: var(--fs-xs); }
  .f { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); margin-bottom: 8px; }
  input { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: inherit; -webkit-user-select: text; user-select: text; }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 12px; font: inherit; }
  button:disabled { opacity: 0.4; }
</style>
