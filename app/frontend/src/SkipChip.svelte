<script lang="ts">
  import Button from './Button.svelte'
  import Popover from './Popover.svelte'
  import Toggle from './Toggle.svelte'
  // "2 skipped" in the flow's header: the steps this open flow leaves out of its runs, what that would break, and the
  // choice to keep their last values instead. Nothing here is saved in the flow file.
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { FlowView } from './lib/types'
  import { plural } from './lib/plural'

  let { view }: { view: FlowView } = $props()
  const node = $derived(view.node)
  const skips = $derived(view.skips)
  const errors = $derived(skips.problems.filter((p) => p.required).length)
  let open = $state(false)
  let at = $state({ x: 0, y: 0 })
</script>

{#if skips.steps.length}
  <span>
    <button class="chip" class:bad={errors > 0} aria-haspopup="dialog" aria-expanded={open} title="Steps left out of the next runs" onclick={(e) => { const r = e.currentTarget.getBoundingClientRect(); at = { x: r.left, y: r.bottom }; open = !open }}>
      {skips.steps.length} skipped{errors ? ` · ${plural(errors, 'problem')}` : ''}
    </button>
    {#if open}
      <Popover x={at.x} y={at.y} width={340} height={220} label="Skipped steps" onclose={() => (open = false)}>
        {#each skips.steps as id (id)}
          <div class="row"><code>{id}</code><Button small onclick={() => FlowService.ToggleSkip(node, id)}>Run it</Button></div>
        {/each}
        <Toggle checked={skips.hold} onchange={(on) => FlowService.SetHold(node, on)} title="The steps that read a skipped step get the value it had in the last run">Keep their last values</Toggle>
        {#each skips.problems as p (p.reader + p.skipped + p.text)}
          <p class="problem" class:warn={!p.required}>{p.text}</p>
        {/each}
      </Popover>
    {/if}
  </span>
{/if}

<style>
  .chip { border-radius: var(--r-pill); padding: 1px 10px; font: inherit; font-size: var(--fs-xs); background: transparent; border: 1px solid var(--muted); color: var(--muted); white-space: nowrap; }
  .chip.bad { border-color: var(--fail); color: var(--fail); }
  .row { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .problem { margin: 0; color: var(--fail); font-size: var(--fs-sm); }
  .problem.warn { color: var(--warn); }
</style>
