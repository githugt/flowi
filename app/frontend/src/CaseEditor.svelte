<script lang="ts">
  import Popover from './Popover.svelte'
  import { isEnter } from './lib/enterkey'
  // The test of one case of a Route, edited where its port is: a line that is true or false, written without braces.
  // The edit applies to the selected Route, which the canvas selected when the label was clicked.
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import Button from './Button.svelte'
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

<svelte:window onkeydown={(e) => (e.key === 'Escape' || isEnter(e)) && onclose()} />
<Popover {x} {y} label="Edit the test" height={140} {onclose}>
  {#if c}
    <label>
      <span class="label">{c.label}: take this path when</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input autofocus bind:value={text} oninput={() => FlowService.SetCaseWhen(node, index, text)} placeholder="nodes.login.status == 404" spellcheck="false" />
    </label>
    <span class="hint">A test that is true or false, without braces. Enter closes.</span>
    {#if (view.form.step?.cases?.length ?? 0) > 1}
      <!-- The first true test wins, so the order matters; the steps in the path move with it. -->
      <div class="order">
        <Button small disabled={index === 0} onclick={() => { FlowService.MoveCase(node, index, -1); onclose() }}>Test earlier ↑</Button>
        <Button small disabled={index >= (view.form.step?.cases?.length ?? 0) - 1} onclick={() => { FlowService.MoveCase(node, index, 1); onclose() }}>Test later ↓</Button>
      </div>
    {/if}
  {:else}
    <span class="hint">This Route has one test; edit it in the side panel, or add an "else if" first.</span>
  {/if}
</Popover>

<style>
  label { display: flex; flex-direction: column; gap: 3px; }
  .label { color: var(--muted); font-size: var(--fs-xs); }
  input { font-family: var(--mono, monospace); }
  input:focus { border-color: var(--accent); outline: none; }
  .order { display: flex; gap: 6px; }
  .hint { color: var(--muted); font-size: var(--fs-sm); }
</style>
