<script lang="ts">
  import ScreenPage from './ScreenPage.svelte'
  import TextInput from './TextInput.svelte'
  import { onDestroy, onMount } from 'svelte'
  import { debounced, type Debounced } from './lib/debounce'
  import { live } from './lib/live'
  import { ManagerService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import type { KeptSpec, ManagerView } from './lib/types'
  import PathField from './PathField.svelte'
  import SpecUpdateDialog from './SpecUpdateDialog.svelte'
  import KVRows from './KVRows.svelte'
  import { errorText } from './lib/errors'
  import { app } from './lib/app.svelte'
  import { reported } from './lib/reported'
  import { dependentsOf, tabsIn, unmountNote } from './lib/unmount'
  import { collectionRoot } from './lib/treewalk'
  import Hint from './Hint.svelte'
  import Button from './Button.svelte'
  import { glossary } from './lib/glossary'
  import type { KV } from './lib/types'

  let view = $state<ManagerView>({ collections: [] })
  let mountDir = $state('')
  let bundleFile = $state('')
  let arazzoRef = $state('')
  let newName = $state('')
  let newLocation = $state('')
  let relocate = $state<Record<string, string>>({})
  // Each action keeps what was typed until it worked, and says why when it did not. Before: the fields cleared at
  // once, and a refusal was an error line at the top of the page, far from the form.
  let busy = $state('')
  async function attempt(kind: string, call: Promise<unknown>, ok: string, clear: () => void) {
    busy = kind
    try {
      if (await reported(call, ui.toast, ok)) clear()
    } finally {
      busy = ''
    }
  }
  // The example's environment holds the address of the demo server the app started for it (lib/example_api.go).
  const EXAMPLE_ENVIRONMENT = 'demo'
  async function tryExample() {
    busy = 'example'
    try {
      if (await reported(ManagerService.OpenExample(), ui.toast, 'Example opened')) await app.selectEnvironment(EXAMPLE_ENVIRONMENT)
    } finally {
      busy = ''
    }
  }
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

  // What to do about the generated files of a collection in a git repository, read once per collection.
  let advice = $state<Record<string, string[]>>({})
  $effect(() => {
    for (const c of view.collections) {
      if (c.missing || c.draft || advice[c.id]) continue
      advice[c.id] = [] // set first, so the effect does not ask again while the answer is on its way
      ManagerService.RepoAdvice(c.id).then((a) => (advice[c.id] = a)).catch(() => (advice[c.id] = []))
    }
  })

  // A save follows a pause in typing, not each keystroke: one per collection, flushed when the screen closes.
  const saves: Record<string, Debounced<[KV[]]>> = {}
  function saveLater(id: string, rows: KV[]) {
    variables[id] = rows // what is typed stays on screen while the save waits
    ;(saves[id] ??= debounced((r: KV[]) => saveVariables(id, r), 700))(rows)
  }
  onDestroy(() => Object.values(saves).forEach((d) => d.flush()))

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
<ScreenPage>
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
          <Button onclick={() => ui.confirm('Delete collection', 'Delete this collection and everything in it? This cannot be undone.', () => ManagerService.Discard(c.id))}>Delete…</Button>
        {:else}
          <Button onclick={() => ui.confirm(`Unmount ${c.name || c.id}?`, unmountNote(c.name || c.id, tabsIn(collectionRoot(app.tree?.roots, c.id), app.tabs), dependentsOf(c.name, view.collections)), () => ManagerService.Unmount(c.id), 'Unmount')}>Unmount…</Button>
        {/if}
      </div>
      <code class="path">{c.path || '(kept in Flowi’s own folder)'}</code>
      {#each specs[c.id] ?? [] as k (k.name)}
        <div class="line spec">
          <span>Spec <code>{k.spec}</code>{#if k.overlays.length}<span class="muted">{` · overlays: ${k.overlays.join(', ')}`}</span>{/if}</span>
          <span class="spacer"></span>
          <Button onclick={() => (updating = { collection: c.id, spec: k })}>Update from spec…</Button>
        </div>
      {/each}
      {#if !c.missing}
        <details class="vars">
          <summary>Variables {#if (variables[c.id] ?? []).length}<span class="muted">({variables[c.id].length})</span>{/if}</summary>
          <p class="muted">Values every request and flow of this collection can read as <code>{'{{collection_var.name}}'}</code>. An environment variable of the same name overrides one.</p>
          <KVRows rows={variables[c.id] ?? []} keyLabel="name" onchange={(rows) => saveLater(c.id, rows)}
            empty={{ icon: 'list', title: 'No collection variables', hint: 'A base URL or a region that belongs to this collection, not to an environment.', add: 'Add variable' }} />
          {#if variableError[c.id]}<p class="err">{variableError[c.id]}</p>{/if}
        </details>
      {/if}
      {#if advice[c.id]?.length}
        <div class="advice" role="note">
          <strong>Keep your repo clean</strong>
          {#each advice[c.id] as a}<p>{a}</p>{/each}
        </div>
      {/if}
      {#each c.deps as d}
        <div class="dep"><span class:ok={d.mounted} class:bad={!d.mounted}>{d.mounted ? '✓' : '✗'}</span> depends on {d.name} <span class="muted">{d.repo}</span></div>
      {/each}
      {#if c.missing}
        <div class="line">
          <PathField bind:value={() => relocate[c.id] ?? '', (v) => (relocate[c.id] = v)} label="New location" />
          <Button disabled={!relocate[c.id] || busy === 'relocate'} onclick={() => attempt('relocate', ManagerService.Relocate(c.id, relocate[c.id]), 'Relocated', () => delete relocate[c.id])}>Relocate</Button>
        </div>
      {/if}
      {#if c.draft}
        <div class="line">
          <PathField bind:value={() => draftDir[c.id] ?? '', (v) => (draftDir[c.id] = v)} label="Move it into this empty folder" />
          <Button disabled={!draftDir[c.id] || busy === 'save'} onclick={() => attempt('save', ManagerService.SaveDraft(c.id, draftDir[c.id]), 'Saved', () => delete draftDir[c.id])}>Save here</Button>
        </div>
      {/if}
    </div>
  {/each}

  <h3>Try the example</h3>
  <p class="muted">A small collection with a demo server of its own, started inside Flowi on a free port and gone when you quit. It opens with the <b>demo</b> environment chosen, so its flows (log in, create a user, check it) run at once with nothing to set up.</p>
  <div class="line"><Button variant="primary" loading={busy === 'example'} onclick={tryExample}>Open the example</Button></div>

  <h3>Mount an existing collection</h3>
  <div class="line">
    <PathField bind:value={mountDir} label="Folder with a flowi.yaml" />
    <Button disabled={!mountDir || busy === 'mount'} onclick={() => attempt('mount', ManagerService.Mount(mountDir), 'Mounted', () => (mountDir = ''))}>Mount</Button>
  </div>

  <h3>Open an Arazzo document<Hint>{glossary.arazzo}</Hint></h3>
  <p class="muted">An Arazzo file (or an address) with the OpenAPI description it names. It opens as a collection of its workflows, made in memory and gone when you quit; nothing is mounted or written. Press Run on a flow with <b>No environment</b> chosen to use the server the description names, or choose an environment that has a <code>base_url</code>.</p>
  <div class="line">
    <PathField bind:value={arazzoRef} label="An .arazzo.yaml file or an https:// address" kind="file" />
    <Button disabled={!arazzoRef || busy === 'arazzo'} onclick={() => attempt('arazzo', ManagerService.OpenArazzo(arazzoRef), 'Opened', () => (arazzoRef = ''))}>Open</Button>
  </div>

  <h3>Open a run bundle</h3>
  <p class="muted">A .flowi-run file is one run of a flow: the steps, the seed, the inputs and every call. It opens as a collection; press Run on its flow to see the run again, without sending anything.</p>
  <div class="line">
    <PathField bind:value={bundleFile} label="A .flowi-run file" kind="file" />
    <Button disabled={!bundleFile || busy === 'bundle'} onclick={() => attempt('bundle', ManagerService.OpenBundle(bundleFile), 'Opened', () => (bundleFile = ''))}>Open</Button>
  </div>

  <h3>New collection</h3>
  <label class="f"><span>Name</span><TextInput bind:value={newName} label="Name" /></label>
  <div class="line">
    <PathField bind:value={newLocation} label="Folder (empty: Flowi’s own folder)" />
    <Button disabled={!newName || busy === 'new'} onclick={() => attempt('new', ManagerService.NewCollection(newName, newLocation), 'Created', () => ((newName = ''), (newLocation = '')))}>Create</Button>
  </div>
</ScreenPage>

<style>
  .advice { margin: 6px 0; padding: 8px 12px; border: 1px solid var(--line); border-left: 3px solid var(--warn); border-radius: var(--r-md); background: var(--warn-t); font-size: var(--fs-sm); }
  .advice p { margin: 4px 0 0; }
  h3 { margin: 24px 0 8px; font-size: var(--fs-lg); font-weight: 600; }
  h3:first-of-type { margin-top: 0; }
  .err { color: var(--fail); }
  .card { border: 1px solid var(--line); border-radius: var(--r-xl); background: var(--panel); padding: 10px 12px; margin: 8px 0; display: flex; flex-direction: column; gap: 6px; }
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
</style>
