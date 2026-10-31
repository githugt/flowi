<script lang="ts">
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { ImportService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { ImportView } from './lib/types'
  import PathField from './PathField.svelte'
  import { ui } from './lib/ui.svelte'

  let view = $state<ImportView | null>(null)
  let file = $state('')
  let overlay = $state('')
  let existing = $state('refuse')
  let showItems = $state(true)

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
  <div class="page">
    <p class="note">Import an API description or a collection from another tool. Nothing is written until you have seen the preview.</p>
    <div class="grid">
      <label class="f"><span>Kind</span>
        <select value={view.kind} onchange={(e) => ImportService.SetKind(e.currentTarget.value)}>
          {#each view.kinds as k}<option value={k.id}>{k.label}</option>{/each}
        </select>
      </label>
      <div class="f"><span>Into collection</span>
        <button type="button" class="choose" onclick={chooseCollection} title="Choose a collection, or make a new one">
          {collectionLabel ?? 'Choose a collection…'}<span class="chev">▾</span>
        </button>
      </div>
    </div>

    {#if view.pasted}
      <div class="pasted">
        <span>Importing the text you pasted.</span>
        <button type="button" class="link" onclick={usePath}>Choose a file instead</button>
      </div>
    {:else}
      <PathField bind:value={file} kind={kind?.source === 'folder' ? 'folder' : 'file'} label={kind?.source === 'folder' ? 'Folder to import' : 'File to import, or an http(s) URL'} title="Choose what to import" />
    {/if}
    {#if kind?.source === 'paste'}
      <label class="f"><span>…or paste it here (curl commands, a .http file, a Mermaid flowchart)</span>
        <textarea value={view.paste} oninput={(e) => ImportService.SetPaste(e.currentTarget.value)} spellcheck="false"></textarea>
      </label>
    {/if}
    {#if kind?.overlay}
      <label class="f"><span>Overlay (fixes applied before the import): file(s) or URL(s), separated by ;</span>
        <input bind:value={overlay} oninput={() => ImportService.SetOverlay(overlay)} spellcheck="false" />
      </label>
    {/if}

    <div class="actions">
      <button class="primary" disabled={!hasSource} onclick={() => ImportService.Preview()}>Preview</button>
    </div>
    {#if view.error}<p class="err">{view.error}</p>{/if}

    {#if view.previewed}
      <h3>Preview <span class="muted">{view.summary}</span></h3>
      {#if view.lossy.length}
        <h4>What does not carry over</h4>
        <ul>{#each view.lossy as l}<li>{l}</li>{/each}</ul>
      {/if}
      {#if view.items.length}
        <h4><button class="link" onclick={() => (showItems = !showItems)}>{showItems ? '▾' : '▸'} Report ({view.items.length})</button></h4>
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
      <div class="files">
        {#each view.files as f}<div class:exists={f.exists}>{f.remove ? '− ' : '+ '}{f.rel}{f.exists ? ' (exists)' : ''}</div>{/each}
      </div>
      {#if view.existing}
        <label class="f"><span>{view.existing} file(s) already exist</span>
          <select bind:value={existing}>
            <option value="refuse">Stop if any exist</option>
            <option value="overwrite">Overwrite them</option>
            <option value="skip">Skip them, add only new files</option>
          </select>
        </label>
      {/if}
      <div class="actions">
        <button class="primary" disabled={!view.canImport} onclick={() => ImportService.Import(existing, view!.skipped > 0)}>
          {view.skipped > 0 ? 'Import the rest' : 'Import'}
        </button>
      </div>
    {/if}
    {#if view.done}<p class="ok">{view.done}</p>{/if}
  </div>
{/if}

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: 16px 24px 48px; max-width: 860px; display: flex; flex-direction: column; gap: 10px; }
  .note { color: var(--muted); margin: 0; }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
  .f { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  .choose { display: flex; justify-content: space-between; align-items: center; gap: 8px; text-align: left; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: inherit; }
  .choose:hover { border-color: var(--accent); }
  .chev { color: var(--muted); }
  select, input, textarea { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: inherit; -webkit-user-select: text; user-select: text; }
  textarea { min-height: 90px; font: var(--fs-sm) var(--mono); resize: vertical; }
  .actions { display: flex; gap: 8px; }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 16px; font: inherit; }
  button:disabled { opacity: 0.4; }
  .primary:not(:disabled) { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
  .link { border: 0; color: var(--text); padding: 0; font-weight: 600; }
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
  .files { font: var(--fs-sm) var(--mono); max-height: 220px; overflow: auto; border: 1px solid var(--line); border-radius: var(--r-md); padding: 6px 10px; }
  .exists { color: var(--warn); }
</style>
