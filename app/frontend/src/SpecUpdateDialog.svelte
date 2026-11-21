<script lang="ts">
  import Modal from './Modal.svelte'
  import Button from './Button.svelte'
  // Update from spec: import a kept spec again with its overlays, from the kept copy or a newer
  // document. Requests it replaces keep your edits, checks, extractions and hooks; the preview says
  // what changes before anything is written.
  import { ManagerService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { errorText } from './lib/errors'
  import { ui } from './lib/ui.svelte'
  import { breakHeading, updateLabel } from './lib/specupdate'
  import type { KeptSpec, SpecUpdate } from './lib/types'
  import PathField from './PathField.svelte'
  import RadioGroup from './RadioGroup.svelte'

  let { collection, spec, onclose }: { collection: string; spec: KeptSpec; onclose: () => void } = $props()

  let from = $state<'kept' | 'newer'>('kept')
  const SOURCES = [
    { value: 'kept', label: 'The kept copy' },
    { value: 'newer', label: 'A newer version of the spec (it becomes the kept copy)' },
  ] as const
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
      await ManagerService.ApplySpecUpdate(collection, spec.name, preview?.source ?? '')
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

<Modal title="Update from spec" width={520} maxHeight="82vh" scroll {onclose}>
    <p class="muted">
      Imports <code>{spec.spec}</code> again{spec.overlays.length ? `, with ${spec.overlays.join(', ')}` : ''}. Requests keep what you changed and what only Flowi has (checks, extractions, hooks, settings).
    </p>
    <RadioGroup
      label="Update from"
      value={from}
      options={SOURCES}
      onchange={(v) => {
        from = v as typeof from
        preview = null
      }} />
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
      {#if breakHeading(preview)}
        <h4 class:bad={preview.breakages.length > 0}>{breakHeading(preview)}</h4>
        {#if preview.breakages.length}
          <ul class="breaks">
            {#each preview.breakages as b}<li><code>{b.request}</code> <span class="muted">({b.version}, {b.where})</span>: {b.reason}</li>{/each}
          </ul>
        {/if}
      {/if}
      {#if preview.notes.length}
        <h4>Notes</h4>
        <ul class="notes">{#each preview.notes as n}<li>{n}</li>{/each}</ul>
      {/if}
    {/if}
  {#snippet actions()}
      <Button type="button" onclick={onclose}>Cancel</Button>
      {#if preview && !nothing}
        <Button variant={preview.breakages.length ? 'danger' : 'primary'} disabled={busy} onclick={apply}>{updateLabel(preview)}</Button>
      {:else}
        <Button variant="primary" disabled={busy || (from === 'newer' && !source.trim())} onclick={look}>Preview</Button>
      {/if}
  {/snippet}
</Modal>

<style>
  h4 { margin: 0; font-size: var(--fs-sm); color: var(--muted); font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; }
  ul { margin: 0; padding-left: 18px; display: flex; flex-direction: column; gap: 2px; }
  h4.bad { color: var(--fail); }
  .breaks li { font-size: var(--fs-sm); }
  .notes li { color: var(--muted); font-size: var(--fs-sm); }
  code { font-size: var(--fs-xs); }
  .muted { color: var(--muted); font-size: var(--fs-sm); }
  .err { color: var(--fail); }
</style>
