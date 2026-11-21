<script lang="ts">
  import Modal from './Modal.svelte'
  import Button from './Button.svelte'
  import Icon from './Icon.svelte'
  import Checkbox from './Checkbox.svelte'
  // Save to spec overlay: what saving this version to its spec's overlay would change, each change
  // chosen or not, with what stays in Flowi and why. Saving replaces what the overlay said about
  // this operation, so a change already in it is chosen too: leaving it out takes it out.
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { errorText } from './lib/errors'
  import type { OverlayPreview } from './lib/types'

  let { node, onclose }: { node: string; onclose: () => void } = $props()

  let preview = $state<OverlayPreview | null>(null)
  let error = $state('')
  let chosen = $state<boolean[]>([])
  let showYaml = $state(false)
  let yaml = $state('')

  $effect(() => {
    RequestService.PreviewOverlay(node)
      .then((p) => {
        preview = p as unknown as OverlayPreview
        chosen = preview.changes.map(() => true)
      })
      .catch((e) => (error = errorText(e)))
  })

  const picked = $derived(chosen.flatMap((on, i) => (on ? [i] : [])))

  $effect(() => {
    if (!showYaml || !preview) return
    const want = picked
    RequestService.OverlayYAML(node, want)
      .then((t) => (yaml = t))
      .catch((e) => (error = errorText(e)))
  })

  async function save() {
    try {
      await RequestService.SaveToOverlay(node, picked)
      onclose()
    } catch (e) {
      error = errorText(e)
    }
  }
</script>

<Modal title="Save to the API spec's overlay file" width={520} maxHeight="82vh" scroll {onclose} onsubmit={() => save()}>
    {#if error}
      <p class="err">{error}</p>
    {:else if !preview}
      <p class="muted">Comparing with the spec…</p>
    {:else}
      <p class="muted">Saves this version, and writes what the spec can say to <code>{preview.edits}</code>, an overlay on <code>{preview.spec}</code>. Every import of the spec applies it.</p>
      <h4>Goes into the overlay</h4>
      {#if preview.changes.length}
        <ul class="changes">
          {#each preview.changes as c, i}
            <li>
              <Checkbox bind:checked={chosen[i]} title={c.target}>
                {c.what}
                {#if c.saved}<span class="tag">already in the overlay</span>{/if}
              </Checkbox>
            </li>
          {/each}
        </ul>
      {:else}
        <p class="muted">Nothing: the spec already says what this version says.</p>
      {/if}
      {#if preview.notCarried.length}
        <h4>Stays in Flowi only</h4>
        <ul class="notes">
          {#each preview.notCarried as n}<li><strong>{n.what}</strong>: {n.why}</li>{/each}
        </ul>
      {/if}
      {#if preview.changes.length}
        <Button type="button" variant="link" onclick={() => (showYaml = !showYaml)}><Icon name={showYaml ? 'chevd' : 'chevr'} size={12} /> The overlay as it will be written</Button>
        {#if showYaml}<pre class="yaml">{yaml}</pre>{/if}
      {/if}
    {/if}
  {#snippet actions()}
    <Button type="button" onclick={onclose}>Cancel</Button>
    <Button variant="primary" type="submit" disabled={!preview}>Save</Button>
  {/snippet}
</Modal>

<style>
  h4 { margin: 0; font-size: var(--fs-sm); color: var(--muted); font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; }
  ul { margin: 0; padding: 0; list-style: none; display: flex; flex-direction: column; gap: 4px; }
  .notes li { color: var(--muted); font-size: var(--fs-sm); }
  .notes strong { color: var(--text); font-weight: 500; }
  .tag { padding: 1px 6px; border-radius: var(--r-sm); font-size: var(--fs-xs); color: var(--muted); background: color-mix(in srgb, var(--muted) 14%, transparent); }
  code { font-size: var(--fs-xs); }
  .muted { color: var(--muted); font-size: var(--fs-sm); }
  .err { color: var(--fail); }
  .yaml { margin: 0; max-height: 240px; overflow: auto; padding: 8px; background: var(--bg); border: 1px solid var(--line); border-radius: var(--r-md); font: var(--fs-sm)/1.5 ui-monospace, SFMono-Regular, Menlo, monospace; -webkit-user-select: text; user-select: text; }
</style>
