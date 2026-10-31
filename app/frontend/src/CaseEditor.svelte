<script lang="ts">
  // The test of one case of a Route, edited where its port is: a line that is true or false, written without braces.
  // The edit applies to the selected Route, which the canvas selected when the label was clicked.
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { FlowView } from './lib/types'

  let { view, index, x, y, onclose }: { view: FlowView; index: number; x: number; y: number; onclose: () => void } = $props()
  const node = $derived(view.node)
  const c = $derived(view.form.step?.cases?.[index])
  let text = $state('')
  let seeded = false
  $effect(() => {
    if (!seeded && c) {
      text = c.when ?? ''
      seeded = true
    }
  })
</script>

<svelte:window onkeydown={(e) => (e.key === 'Escape' || e.key === 'Enter') && onclose()} />
<div class="scrim" role="presentation" onpointerdown={onclose}></div>
<div class="case" style:left={`${Math.min(x, window.innerWidth - 380)}px`} style:top={`${Math.min(y + 8, window.innerHeight - 140)}px`} role="dialog" aria-label="Edit the test">
  {#if c}
    <label>
      <span class="label">{c.label}: take this path when</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input autofocus bind:value={text} oninput={() => FlowService.SetCaseWhen(node, index, text)} placeholder="nodes.login.status == 404" spellcheck="false" />
    </label>
    <span class="hint">A test that is true or false, without braces. Enter closes.</span>
  {:else}
    <span class="hint">This Route has one test; edit it in the side panel, or add an "else if" first.</span>
  {/if}
</div>

<style>
  .scrim { position: fixed; inset: 0; z-index: 29; }
  .case { position: fixed; z-index: 30; width: 360px; box-sizing: border-box; display: flex; flex-direction: column; gap: 6px; padding: 12px; background: var(--panel); border: 1px solid var(--line); border-radius: 14px; box-shadow: var(--shadow); }
  label { display: flex; flex-direction: column; gap: 3px; }
  .label { color: var(--muted); font-size: var(--fs-xs); }
  input { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 6px; font: inherit; font-family: var(--mono, monospace); }
  input:focus { border-color: var(--accent); outline: none; }
  .hint { color: var(--muted); font-size: var(--fs-sm); }
</style>
