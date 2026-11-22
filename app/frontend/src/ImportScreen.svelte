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
  import Verb from './Verb.svelte'
  import FileTree from './FileTree.svelte'
  import Icon from './Icon.svelte'
  import { buildFileTree } from './lib/filetree'
  import { ui } from './lib/ui.svelte'
  import { app } from './lib/app.svelte'
  import { DEFAULT_EXISTING, EXISTING_CHOICES, groupKinds } from './lib/importkinds'
  import { groupFiles, importLabel, previewState, supportingFiles, type ExistingMode } from './lib/importpreview'
  import { nodeById, collectionRoot, firstOfKind } from './lib/treewalk'

  // The preview is of the source chosen or pasted: it appears by itself a moment after the source, the kind, the
  // target or the overlays change (Go does the waiting), and "Refresh" reads it again.
  let view = $state<ImportView | null>(null)
  const fileTree = $derived(buildFileTree(view?.files ?? []))
  let file = $state('')
  let overlay = $state('')
  let existing = $state(DEFAULT_EXISTING)
  let showItems = $state(true)
  let pasteBox = $state<HTMLTextAreaElement>()
  let pasteHint = $state('')
  const shown = $derived(view ? previewState(view) : 'waiting')
  const groups = $derived(groupFiles(view?.files ?? [], existing as ExistingMode))
  const supporting = $derived(supportingFiles(view?.files ?? []))
  // Where to go next once an import is done: the first request of the place it went into.
  const firstRequest = $derived.by(() => {
    if (!view?.done || !view.wrote) return undefined
    const place = nodeById(app.tree?.roots, view.targetRow) ?? collectionRoot(app.tree?.roots, view.collection)
    return firstOfKind('request', place?.children ?? [])
  })

  onMount(() =>
    live<ImportView>('import', () => ImportService.State(), (v) => {
      if (!view) file = v.file
      view = v
    }),
  )

  // The chooser the new-request and Save dialogs use: a collection, or a folder in it at any depth; it can make both.
  function chooseTarget() {
    ui.place = {
      title: 'Import into',
      action: 'Choose',
      tree: 'requests',
      note: 'A collection, or a folder in it. New folder makes one inside the place you chose, so folders can be nested.',
      row: view?.targetRow || undefined,
      done: (row) => ImportService.SetTarget(row),
    }
  }

  // Back to choosing a file: the pasted text is let go.
  function usePath() {
    file = ''
    ImportService.SetPaste('')
    ImportService.SetFile('')
  }

  function startOver() {
    file = ''
    overlay = ''
    pasteHint = ''
    ImportService.StartOver()
  }

  // The system clipboard, when the window may read it; otherwise the box takes a paste with the keyboard.
  async function pasteFromClipboard() {
    pasteHint = ''
    try {
      const text = await navigator.clipboard.readText()
      if (text) return void ImportService.SetPaste(text)
      pasteHint = 'The clipboard is empty.'
    } catch {
      pasteHint = 'This window may not read the clipboard: click in the box and paste with the keyboard.'
      pasteBox?.focus()
    }
  }

  const kind = $derived(view?.kinds.find((k) => k.id === view?.kind))
  const kindName = $derived(view?.detected ?? kind?.label ?? '')

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
    <p class="note">Import an API description or a collection from another tool. Choose or paste it and the preview shows what would be imported; nothing is written until you press Import.</p>
    <div class="grid">
      <Field label="Kind">
        <Select value={view.kind} onchange={(e) => ImportService.SetKind(e.currentTarget.value)}>
          {#each groupKinds(view.kinds) as g (g.title)}
            <optgroup label={g.title}>{#each g.kinds as k (k.id)}<option value={k.id}>{k.label}</option>{/each}</optgroup>
          {/each}
        </Select>
      </Field>
      <div class="f"><span>Into</span>
        <button type="button" class="choose" onclick={chooseTarget} title="Choose a collection or a folder in it, or make a new one" aria-label={`Import into: ${view.target || 'choose a place'}`}>
          {view.target || 'Choose a collection…'}<span class="chev"><Icon name="chevd" size={12} /></span>
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
    {#if kind?.source !== 'folder'}
      <Field label="…or paste the text here (a document, a curl command, a .http file, a Mermaid flowchart)">
        <textarea bind:this={pasteBox} value={view.paste} oninput={(e) => ImportService.SetPaste(e.currentTarget.value)} spellcheck="false" aria-label="Pasted text to import"></textarea>
      </Field>
      <div class="actions">
        <Button small onclick={pasteFromClipboard}>Paste from clipboard</Button>
        {#if view.paste}<Button small variant="ghost" onclick={usePath}>Clear the text</Button>{/if}
        {#if pasteHint}<span class="muted" role="status">{pasteHint}</span>{/if}
      </div>
    {/if}
    {#if kind?.overlay}
      <Field label="Overlay (fixes applied before the import): file(s) or URL(s), separated by ;">
        <TextInput bind:value={overlay} label="Overlay" oninput={() => ImportService.SetOverlay(overlay)} />
      </Field>
    {/if}

    {#if shown === 'waiting'}
      <p class="muted" role="status" data-testid="preview-waiting">{view.waiting || 'Reading the source in a moment…'}</p>
    {:else if shown === 'reading'}
      <p class="muted" role="status" data-testid="preview-reading">Reading the source…</p>
    {:else if shown === 'failed'}
      <div class="callout fail" role="alert" data-testid="preview-failed">
        <div>
          <strong>No preview: this cannot be imported as {kindName || 'this kind'}.</strong>
          <div>{view.error}</div>
          <div class="muted">Check that the kind above matches the source, then refresh.</div>
        </div>
        <Button small onclick={() => ImportService.Preview()} disabled={view.previewing || !view.canPreview}>Refresh</Button>
      </div>
    {:else if shown === 'ready'}
      <div class="head">
        <h3>Preview <span class="muted">{view.summary}</span></h3>
        <Button small onclick={() => ImportService.Preview()} disabled={view.previewing} title="Read the source again, for a file that changed">Refresh</Button>
      </div>
      <p class="note" data-testid="preview-into">{kindName} will be imported into <strong>{view.target}</strong>. Nothing has been written yet.</p>
      {#if view.lossy.length}
        <h4>What does not carry over</h4>
        <ul>{#each view.lossy as l}<li>{l}</li>{/each}</ul>
      {/if}
      {#each groups as g (g.kind)}
        <h4>{g.title} ({g.entries.length})</h4>
        <ul class="entries" aria-label={g.title}>
          {#each g.entries as e (e.path)}
            <li>{#if e.verb}<Verb verb={e.verb} />{/if}<span class="ename">{e.name}</span><span class="epath">{e.path}</span><span class="enote {e.tone}">{e.note}</span></li>
          {/each}
        </ul>
      {/each}
      {#if groups.length === 0}<p class="muted">The source makes no requests or flows.</p>{/if}
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
      <details class="all">
        <summary>All {view.files.length} files{supporting.length ? `, including ${supporting.length} that support the import` : ''}</summary>
        <div class="files"><FileTree nodes={fileTree} /></div>
      </details>
      {#if view.existing}
        <Field label={`${view.existing} file(s) already exist`}>
          <Select bind:value={existing}>
            {#each EXISTING_CHOICES as c (c.id)}<option value={c.id}>{c.label}</option>{/each}
          </Select>
        </Field>
      {/if}
      <div class="actions">
        <Button variant="primary" disabled={!view.canImport} onclick={() => ImportService.Import(existing, view!.skipped > 0)}>
          {importLabel(view.files, existing as ExistingMode, view.skipped)}
        </Button>
      </div>
    {:else}
      <div class="callout" class:ok={view.wrote > 0} class:warn={view.wrote === 0} role="status" data-testid="import-result">
        <div>
          <strong>{view.done}</strong>
          {#if view.notes.length}<ul class="notes">{#each view.notes as n}<li>{n}</li>{/each}</ul>{/if}
          {#if view.details.length}<ul class="details">{#each view.details as d}<li>{d}</li>{/each}</ul>{/if}
        </div>
      </div>
      <div class="actions">
        {#if firstRequest}<Button variant="primary" onclick={() => app.open(firstRequest!)}>Open {firstRequest.label}</Button>{/if}
        <Button onclick={() => app.openScreen('screen:manager', 'Collections')}>Collections</Button>
        <Button onclick={startOver}>Import something else</Button>
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
  .pasted { display: flex; align-items: center; gap: 8px; }
  textarea { min-height: 90px; font: var(--fs-sm) var(--mono); resize: vertical; }
  .actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
  .head { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-top: 12px; }
  .link { display: inline-flex; align-items: center; gap: var(--sp-1); background: none; border: 0; color: var(--text); padding: 0; font: inherit; font-weight: 600; }
  h3 { margin: 0; }
  h4 { margin: 8px 0 2px; font-size: var(--fs-sm); }
  .muted { color: var(--muted); font-weight: 400; font-size: var(--fs-sm); }
  .callout { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; padding: 8px 12px; border-radius: var(--r-md); border: 1px solid var(--line); font-size: var(--fs-sm); }
  .callout.fail { background: var(--fail-t); border-color: var(--fail); }
  .callout.ok { background: var(--ok-t); border-color: var(--ok); }
  .callout.warn { background: var(--warn-t); border-color: var(--warn); }
  .notes { margin: 4px 0 0; padding-left: 18px; color: var(--text); }
  .details { margin: 4px 0 0; padding-left: 18px; color: var(--muted); }
  ul { margin: 0; padding-left: 18px; color: var(--muted); font-size: var(--fs-sm); }
  ul.entries { list-style: none; padding: 0; border: 1px solid var(--line); border-radius: var(--r-md); }
  .entries li { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(0, 1.4fr) auto; gap: 12px; align-items: baseline; padding: 4px 10px; border-top: 1px solid var(--line); color: var(--text); }
  .entries li:first-child { border-top: 0; }
  .ename { font-weight: 600; overflow-wrap: anywhere; }
  .epath { font: var(--fs-xs) var(--mono); color: var(--muted); overflow-wrap: anywhere; }
  .enote { font-size: var(--fs-xs); white-space: nowrap; }
  .enote.ok { color: var(--ok); }
  .enote.warn { color: var(--warn); }
  .enote.muted { color: var(--muted); }
  .item { border-left: 2px solid var(--line); padding: 2px 10px; margin: 4px 0; font-size: var(--fs-sm); }
  .item.skipped { border-color: var(--fail); }
  .tag { margin-left: 8px; color: var(--fail); font-size: var(--fs-xs); }
  .prob.error { color: var(--fail); }
  .prob.warning { color: var(--warn); }
  pre { margin: 2px 0; font: var(--fs-xs) var(--mono); white-space: pre-wrap; }
  .all summary { cursor: pointer; color: var(--muted); font-size: var(--fs-sm); }
  .files { max-height: 320px; overflow: auto; border: 1px solid var(--line); border-radius: var(--r-md); padding: 6px 10px; margin-top: 4px; }
</style>
