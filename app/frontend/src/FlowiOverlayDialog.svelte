<script lang="ts">
  // Save as flowi overlay: the changes of the working copy against its saved version, each with a
  // choice. Ticked changes go to the overlay's shared part (every request that uses it gets them);
  // the others stay with this request. An existing overlay keeps what it holds for other requests.
  import { RequestService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { errorText } from './lib/errors'
  import { ui } from './lib/ui.svelte'
  import type { FlowiOverlayDiff } from './lib/types'

  let { node, onclose }: { node: string; onclose: () => void } = $props()

  let diff = $state<FlowiOverlayDiff | null>(null)
  let error = $state('')
  let name = $state('')
  let description = $state('')
  // The changes ticked to share, by their index in the diff.
  let shared = $state<Record<number, boolean>>({})

  $effect(() => {
    RequestService.FlowiOverlayDiff(node)
      .then((d) => {
        diff = d as unknown as FlowiOverlayDiff
        name = diff.open
        shared = {}
      })
      .catch((e) => (error = errorText(e)))
  })

  const sharedIndexes = $derived(Object.entries(shared).flatMap(([i, on]) => (on ? [Number(i)] : [])))
  const exists = $derived(!!diff && diff.names.includes(name))
  const nameProblem = $derived(name && !/^[A-Za-z0-9_-][A-Za-z0-9._-]*$/.test(name) ? 'Use letters, digits, “.”, “_” and “-”.' : '')
  const canSave = $derived(!!diff && !!name && !nameProblem && diff.ops.length > 0)

  async function save(drop = false) {
    try {
      const needsConfirm = await RequestService.SaveAsFlowiOverlay(node, name, description, sharedIndexes, drop)
      if (needsConfirm) {
        ui.confirm('Comments will be lost', 'The overlay file has comments; saving rewrites it without them. Save anyway?', () => save(true), 'Save')
        return
      }
      onclose()
    } catch (e) {
      error = errorText(e)
    }
  }
</script>

<div class="scrim" role="presentation" onpointerdown={onclose}>
  <form class="dialog" onpointerdown={(e) => e.stopPropagation()} onsubmit={(e) => { e.preventDefault(); save() }}>
    <h3>Save as flowi overlay</h3>
    {#if error}
      <p class="err">{error}</p>
    {:else if !diff}
      <p class="muted">Comparing with the saved version…</p>
    {:else}
      <p class="muted">An overlay is the same request with some changes laid over it when it is sent: as a VIP customer, slow, with a bad token. The saved version stays as it is.</p>
      <label class="field">
        <span>Overlay</span>
        <input list="flowi-overlay-names" bind:value={name} placeholder="vip" aria-label="Overlay name" />
        <datalist id="flowi-overlay-names">{#each diff.names as n}<option value={n}></option>{/each}</datalist>
      </label>
      {#if nameProblem}<p class="err">{nameProblem}</p>{/if}
      <label class="field">
        <span>About</span>
        <input bind:value={description} placeholder={exists ? 'unchanged' : 'The VIP customer'} aria-label="Description" />
      </label>
      <h4>Changes from the saved version</h4>
      {#if diff.ops.length}
        <p class="muted small">Tick a change to share it: every request that uses this overlay gets it. The rest stay with this request.</p>
        <ul class="changes">
          {#each diff.ops as op}
            <li>
              <label>
                <input type="checkbox" bind:checked={shared[op.index]} />
                <span>{op.summary}</span>
                <span class="tag">{shared[op.index] ? 'shared' : 'this request'}</span>
              </label>
            </li>
          {/each}
        </ul>
      {:else}
        <p class="muted">Nothing differs from the saved version, so there is nothing to save.</p>
      {/if}
      {#if diff.unsupported.length}
        <h4>Not kept in an overlay</h4>
        <ul class="notes">{#each diff.unsupported as u}<li>{u}</li>{/each}</ul>
      {/if}
    {/if}
    <div class="actions">
      <button type="button" onclick={onclose}>Cancel</button>
      <button class="primary" type="submit" disabled={!canSave}>{exists ? 'Update overlay' : 'Save overlay'}</button>
    </div>
  </form>
</div>

<style>
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 60; }
  .dialog { width: 540px; max-width: calc(100vw - 32px); max-height: 82vh; overflow: auto; display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); }
  .dialog h3, .dialog h4, .dialog p { margin: 0; }
  h4 { font-size: var(--fs-sm); color: var(--muted); font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; }
  ul { margin: 0; padding: 0; list-style: none; display: flex; flex-direction: column; gap: 4px; }
  .notes li { color: var(--muted); font-size: var(--fs-sm); }
  label { display: flex; align-items: center; gap: 8px; }
  .field span { width: 64px; color: var(--muted); font-size: var(--fs-sm); }
  .field input { flex: 1; min-width: 0; }
  input:not([type='checkbox']) { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 8px; font: inherit; }
  .tag { margin-left: auto; padding: 1px 6px; border-radius: var(--r-sm); font-size: var(--fs-xs); color: var(--muted); background: color-mix(in srgb, var(--muted) 14%, transparent); white-space: nowrap; }
  .muted { color: var(--muted); font-size: var(--fs-sm); }
  .small { font-size: var(--fs-xs); }
  .err { color: var(--fail); font-size: var(--fs-sm); }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 12px; font: inherit; }
  button:disabled { opacity: 0.4; }
  .primary { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
</style>
