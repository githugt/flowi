<script lang="ts">
  // The bar over the canvas while several steps are selected: wrap them in a Loop, a Group or a Route in one undo
  // step. Go says when the selection cannot be wrapped (the steps are not next to each other) and the message shows.
  import Button from './Button.svelte'
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import { wrapKinds, wrapLabel, type WrapKind } from './lib/wrapbar'

  let { node, count }: { node: string; count: number } = $props()

  const wrap = (kind: WrapKind) => FlowService.Wrap(node, kind).catch(ui.fail)
</script>

<div class="bar" role="toolbar" aria-label="Wrap the selected steps">
  <span class="label">{wrapLabel(count)}</span>
  {#each wrapKinds as k (k.kind)}
    <Button small title={k.hint} onclick={() => wrap(k.kind)}>{k.label}</Button>
  {/each}
</div>

<style>
  .bar { position: absolute; left: 50%; bottom: var(--sp-4); transform: translateX(-50%); display: flex; align-items: center; gap: var(--sp-2); padding: var(--sp-2) var(--sp-3); background: var(--raised); border: 1px solid var(--line); border-radius: var(--r-lg); box-shadow: var(--shadow); }
  .label { color: var(--muted); font-size: var(--fs-sm); }
</style>
