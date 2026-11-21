<script lang="ts">
  import ScreenPage from './ScreenPage.svelte'
  import Field from './Field.svelte'
  import Select from './Select.svelte'
  import TextInput from './TextInput.svelte'
  import Button from './Button.svelte'
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { ImportService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { ImportView } from './lib/types'
  import PathField from './PathField.svelte'
  import FileTree from './FileTree.svelte'
  import Icon from './Icon.svelte'
  import { buildFileTree } from './lib/filetree'
  import { ui } from './lib/ui.svelte'
  import { app } from './lib/app.svelte'
  import { DEFAULT_EXISTING, EXISTING_CHOICES, groupKinds } from './lib/importkinds'
  import { collectionRoot, firstOfKind } from './lib/treewalk'

  let view = $state<ImportView | null>(null)
  const fileTree = $derived(buildFileTree(view?.files ?? []))
  let file = $state('')
  let overlay = $state('')
  let existing = $state(DEFAULT_EXISTING)
  let showItems = $state(true)
  // Where to go next once an import is done: the first request of the collection it went into.
  const firstRequest = $derived(view?.done ? firstOfKind('request', collectionRoot(app.tree?.roots, view.collection)?.children ?? []) : undefined)

  onMount(() =>
    live<ImportView>('import', () => ImportService.State(), (v) => {
      if (!view) file = v.file
      view = v
    }),
  )

  // The same chooser the new-request and Save dialogs use; an import fills a whole collection, so no folder.
  function chooseCollection() {
    ui.place = {
      title: 'Import into',
      action: 'Choose',
      tree: 'requests',
      collectionsOnly: true,
      row: view ? `c:${view.collection}` : undefined,
      done: (row) => ImportService.SetCollection(row.replace(/^c:/, '')),
    }
  }

  // Back to choosing a file: the pasted text is let go.
  function usePath() {
    file = ''
    ImportService.SetPaste('')
    ImportService.SetFile('')
  }

  const collectionLabel = $derived(view?.collections.find((c) => c.id === view?.collection)?.label)
  const kind = $derived(view?.kinds.find((k) => k.id === view?.kind))
  const hasSource = $derived(!!(view?.pasted || file))

  // A path chosen in the dialog or typed goes to Go once typing pauses.
  let timer: ReturnType<typeof setTimeout>
  $effect(() => {
    const f = file
    clearTimeout(timer)
    timer = setTimeout(() => f !== view?.file && ImportService.SetFile(f), 250)
  })
</script>

{#if view}
  <ScreenPage width={860} column>
    <p class="note">Import an API description or a collection from another tool. Nothing is written until you have seen the preview.</p>
    <div class="grid">
      <Field label="Kind">
        <Select value={view.kind} onchange={(e) => ImportService.SetKind(e.currentTarget.value)}>
          {#each groupKinds(view.kinds) as g (g.title)}
            <optgroup label={g.title}>{#each g.kinds as k (k.id)}<option value={k.id}>{k.label}</option>{/each}</optgroup>
          {/each}
        </Select>
      </Field>
      <div class="f"><span>Into collection</span>
        <button type="button" class="choose" onclick={chooseCollection} title="Choose a collection, or make a new one">
          {collectionLabel ?? 'Choose a collection…'}<span class="chev"><Icon name="chevd" size={12} /></span>
        </button>
      </div>
    </div>

    {#if view.detected}<p class="note" role="status">Recognised {view.detected}: the kind is set to match. Change it above if that is wrong.</p>{/if}
    {#if view.pasted}
      <div class="pasted">
        <span>Importing the text you pasted.</span>
        <Button variant="link" onclick={usePath}>Choose a file instead</Button>
      </div>
    {:else}
      <PathField bind:value={file} kind={kind?.source === 'folder' ? 'folder' : 'file'} label={kind?.source === 'folder' ? 'Folder to import' : 'File to import, or an http(s) URL'} title="Choose what to import" />
    {/if}
    {#if kind?.source === 'paste'}
      <Field label="…or paste it here (curl commands, a .http file, a Mermaid flowchart)">
        <textarea value={view.paste} oninput={(e) => ImportService.SetPaste(e.currentTarget.value)} spellcheck="false"></textarea>
      </Field>
    {/if}
    {#if kind?.overlay}
      <Field label="Overlay (fixes applied before the import): file(s) or URL(s), separated by ;">
        <TextInput bind:value={overlay} label="Overlay" oninput={() => ImportService.SetOverlay(overlay)} />
      </Field>
    {/if}

    <div class="actions">
      <Button variant="primary" disabled={!hasSource} onclick={() => ImportService.Preview()}>Preview</Button>
    </div>
    {#if view.error}<p class="err">{view.error}</p>{/if}

    {#if view.previewed}
      <h3>Preview <span class="muted">{view.summary}</span></h3>
      {#if view.lossy.length}
        <h4>What does not carry over</h4>
        <ul>{#each view.lossy as l}<li>{l}</li>{/each}</ul>
      {/if}
      {#if view.items.length}
        <h4><button class="link" onclick={() => (showItems = !showItems)}><Icon name={showItems ? 'chevd' : 'chevr'} size={12} /> Report ({view.items.length})</button></h4>
        {#if showItems}
          {#each view.items as it}
            <div class="item" class:skipped={it.skipped}>
              <strong>{it.name}</strong>{#if it.skipped}<span class="tag">cannot be imported</span>{/if}
              {#each it.problems as p}
                <div class="prob {p.severity}">{p.text}{#if p.where} <span class="muted">({p.where})</span>{/if}{#if p.hint}<div class="muted">{p.hint}</div>{/if}
                  {#if p.excerpt?.length}<pre>{p.excerpt.join('\n')}</pre>{/if}</div>
              {/each}
            </div>
          {/each}
        {/if}
      {/if}
      <h4>Files</h4>
      <div class="files"><FileTree nodes={fileTree} /></div>
      {#if view.existing}
        <Field label={`${view.existing} file(s) already exist`}>
          <Select bind:value={existing}>
            {#each EXISTING_CHOICES as c (c.id)}<option value={c.id}>{c.label}</option>{/each}
          </Select>
        </Field>
      {/if}
      <div class="actions">
        <Button variant="primary" disabled={!view.canImport} onclick={() => ImportService.Import(existing, view!.skipped > 0)}>
          {view.skipped > 0 ? 'Import the rest' : 'Import'}
        </Button>
      </div>
    {/if}
    {#if view.done}
      <p class="ok">{view.done}</p>
      <div class="actions">
        {#if firstRequest}<Button variant="primary" onclick={() => app.open(firstRequest!)}>Open {firstRequest.label}</Button>{/if}
        <Button onclick={() => app.openScreen('screen:manager', 'Collections')}>Collections</Button>
      </div>
    {/if}
  </ScreenPage>
{/if}

<style>
  .note { color: var(--muted); margin: 0; }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
  .f { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  .choose { min-height: var(--ctl-h); display: flex; justify-content: space-between; align-items: center; gap: 8px; text-align: left; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: inherit; }
  .choose:hover { border-color: var(--accent); }
  .chev { display: inline-flex; color: var(--muted); }
  textarea { min-height: 90px; font: var(--fs-sm) var(--mono); resize: vertical; }
  .actions { display: flex; gap: 8px; }
  .link { display: inline-flex; align-items: center; gap: var(--sp-1); background: none; border: 0; color: var(--text); padding: 0; font: inherit; font-weight: 600; }
  h3 { margin: 12px 0 0; }
  h4 { margin: 8px 0 2px; font-size: var(--fs-sm); }
  .muted { color: var(--muted); font-weight: 400; font-size: var(--fs-sm); }
  .err { color: var(--fail); margin: 0; }
  .ok { color: var(--ok); margin: 0; }
  ul { margin: 0; padding-left: 18px; color: var(--muted); font-size: var(--fs-sm); }
  .item { border-left: 2px solid var(--line); padding: 2px 10px; margin: 4px 0; font-size: var(--fs-sm); }
  .item.skipped { border-color: var(--fail); }
  .tag { margin-left: 8px; color: var(--fail); font-size: var(--fs-xs); }
  .prob.error { color: var(--fail); }
  .prob.warning { color: var(--warn); }
  pre { margin: 2px 0; font: var(--fs-xs) var(--mono); white-space: pre-wrap; }
  .files { max-height: 320px; overflow: auto; border: 1px solid var(--line); border-radius: var(--r-md); padding: 6px 10px; }
</style>
