<script lang="ts">
  // Update from spec: import a kept spec again with its overlays, from the kept copy or a newer
  // document. Requests it replaces keep your edits, checks, extractions and hooks; the preview says
  // what changes before anything is written.
  import { ManagerService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { errorText } from './lib/errors'
  import { ui } from './lib/ui.svelte'
  import type { KeptSpec, SpecUpdate } from './lib/types'
  import PathField from './PathField.svelte'

  let { collection, spec, onclose }: { collection: string; spec: KeptSpec; onclose: () => void } = $props()

  let from = $state<'kept' | 'newer'>('kept')
  let source = $state('')
  let preview = $state<SpecUpdate | null>(null)
  let error = $state('')
  let busy = $state(false)

  async function look() {
    error = ''
    preview = null
    busy = true
    try {
      preview = (await ManagerService.PreviewSpecUpdate(collection, spec.name, from === 'kept' ? '' : source)) as unknown as SpecUpdate
    } catch (e) {
      error = errorText(e)
    } finally {
      busy = false
    }
  }

  async function apply() {
    busy = true
    try {
      await ManagerService.ApplySpecUpdate(collection, spec.name)
      ui.toast(`Updated from ${spec.spec}`)
      onclose()
    } catch (e) {
      error = errorText(e)
    } finally {
      busy = false
    }
  }

  // A preview is of one source: choosing another asks again.
  $effect(() => {
    void source
    preview = null
  })

  const nothing = $derived(preview && !preview.added.length && !preview.changed.length && !preview.removed.length)
</script>

<div class="scrim" role="presentation" onpointerdown={onclose}>
  <div class="dialog" role="dialog" aria-label="Update from spec" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
    <h3>Update from spec</h3>
    <p class="muted">
      Imports <code>{spec.spec}</code> again{spec.overlays.length ? `, with ${spec.overlays.join(', ')}` : ''}. Requests keep what you changed and what only Flow-I has (checks, extractions, hooks, settings).
    </p>
    <fieldset>
      <label><input type="radio" bind:group={from} value="kept" onchange={() => (preview = null)} /> The kept copy</label>
      <label><input type="radio" bind:group={from} value="newer" onchange={() => (preview = null)} /> A newer version of the spec (it becomes the kept copy)</label>
    </fieldset>
    {#if from === 'newer'}
      <PathField bind:value={source} kind="file" label="File, or an http(s) URL" title="Choose the newer spec" />
    {/if}
    {#if error}<p class="err">{error}</p>{/if}
    {#if preview}
      {#if nothing}
        <p class="muted">Nothing changes: the collection already says what the spec says.</p>
      {:else}
        {#each [['Added', preview.added], ['Changed', preview.changed], ['Removed', preview.removed]] as [title, files]}
          {#if files.length}
            <h4>{title} ({files.length})</h4>
            <ul>{#each files as f}<li><code>{f}</code></li>{/each}</ul>
          {/if}
        {/each}
      {/if}
      {#if preview.notes.length}
        <h4>Notes</h4>
        <ul class="notes">{#each preview.notes as n}<li>{n}</li>{/each}</ul>
      {/if}
    {/if}
    <div class="actions">
      <button type="button" onclick={onclose}>Cancel</button>
      {#if preview && !nothing}
        <button class="primary" disabled={busy} onclick={apply}>Update</button>
      {:else}
        <button class="primary" disabled={busy || (from === 'newer' && !source.trim())} onclick={look}>Preview</button>
      {/if}
    </div>
  </div>
</div>

<style>
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 60; }
  .dialog { width: 520px; max-height: 82vh; overflow: auto; display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); }
  .dialog h3, .dialog h4, .dialog p { margin: 0; }
  h4 { font-size: var(--fs-sm); color: var(--muted); font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; }
  ul { margin: 0; padding-left: 18px; display: flex; flex-direction: column; gap: 2px; }
  .notes li { color: var(--muted); font-size: var(--fs-sm); }
  fieldset { border: 0; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; }
  label { display: flex; align-items: center; gap: 8px; }
  code { font-size: var(--fs-xs); }
  .muted { color: var(--muted); font-size: var(--fs-sm); }
  .err { color: var(--fail); }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 12px; font: inherit; }
  button:disabled { opacity: 0.4; }
  .primary { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
</style>
