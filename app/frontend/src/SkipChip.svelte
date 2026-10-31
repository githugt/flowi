<script lang="ts">
  // "2 skipped" in the flow's header: the steps this open flow leaves out of its runs, what that would break, and the
  // choice to keep their last values instead. Nothing here is saved in the flow file.
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { FlowView } from './lib/types'

  let { view }: { view: FlowView } = $props()
  const node = $derived(view.node)
  const skips = $derived(view.skips)
  const errors = $derived(skips.problems.filter((p) => p.required).length)
  let open = $state(false)
</script>

<svelte:window onpointerdown={() => (open = false)} />

{#if skips.steps.length}
  <span class="wrap">
    <button class="chip" class:bad={errors > 0} aria-expanded={open} title="Steps left out of the next runs" onpointerdown={(e) => e.stopPropagation()} onclick={() => (open = !open)}>
      {skips.steps.length} skipped{errors ? ` · ${errors} problem${errors > 1 ? 's' : ''}` : ''}
    </button>
    {#if open}
      <div class="pop" role="dialog" aria-label="Skipped steps" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
        {#each skips.steps as id (id)}
          <div class="row"><code>{id}</code><button onclick={() => FlowService.ToggleSkip(node, id)}>Run it</button></div>
        {/each}
        <label class="hold" title="The steps that read a skipped step get the value it had in the last run">
          <input type="checkbox" checked={skips.hold} onchange={(e) => FlowService.SetHold(node, e.currentTarget.checked)} /> Keep their last values
        </label>
        {#each skips.problems as p (p.reader + p.skipped + p.text)}
          <p class="problem" class:warn={!p.required}>{p.text}</p>
        {/each}
      </div>
    {/if}
  </span>
{/if}

<style>
  .wrap { position: relative; }
  .chip { border-radius: var(--r-pill); padding: 1px 10px; font: inherit; font-size: var(--fs-xs); background: transparent; border: 1px solid var(--muted); color: var(--muted); white-space: nowrap; }
  .chip.bad { border-color: var(--fail); color: var(--fail); }
  .pop { position: absolute; z-index: 20; top: 28px; left: 0; width: 340px; box-sizing: border-box; display: flex; flex-direction: column; gap: 8px; padding: 10px; background: var(--panel); border: 1px solid var(--line); border-radius: 12px; box-shadow: var(--shadow); }
  .row { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .row button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 2px 10px; font: inherit; font-size: var(--fs-sm); }
  .hold { display: flex; align-items: center; gap: 6px; color: var(--text); font-size: var(--fs-sm); }
  .problem { margin: 0; color: var(--fail); font-size: var(--fs-sm); }
  .problem.warn { color: var(--warn); }
</style>
