<script lang="ts">
  // What the flow has to tell: errors, steps out of order, Flowi-only steps,
  // a change on disk, unsaved edits. A chip opens its message under it.
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { Chip, FlowView } from './lib/types'

  let { view }: { view: FlowView } = $props()
  let open = $state<string | null>(null)
  const node = $derived(view.node)

  const label = (c: Chip) =>
    ({
      error: 'Error',
      needs: 'Steps out of order',
      warning: `${c.steps?.length ?? 0} Flowi steps`,
      disk: 'Changed on disk',
      unsaved: 'Unsaved',
    })[c.kind]
</script>

<svelte:window onpointerdown={() => (open = null)} />

<div class="chips">
  {#each view.chips as c (c.kind)}
    <span class="chip">
      <button class={c.kind} onpointerdown={(e) => e.stopPropagation()} onclick={() => (open = open === c.kind ? null : c.kind)}>{label(c)}</button>
      {#if open === c.kind && c.kind !== 'unsaved'}
        <div class="pop" role="dialog" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
          {#if c.kind === 'error'}
            <p>{c.text}</p>
          {:else if c.kind === 'needs'}
            <p>These steps wait for a step that comes after them, so they can never start and a run fails: {c.steps?.join(', ')}</p>
            <button class="act" onclick={() => { open = null; FlowService.RepairNeeds(node) }}>Repair</button>
          {:else if c.kind === 'warning'}
            <p>{c.steps?.length} steps are Flowi steps that other Arazzo tools cannot run: {c.steps?.join(', ')}</p>
          {:else if c.kind === 'disk'}
            <p>This file changed on disk.</p>
            <button class="act" onclick={() => { open = null; FlowService.Revert(node) }}>Load from disk</button>
          {/if}
        </div>
      {/if}
    </span>
  {/each}
</div>

<style>
  .chips { display: flex; gap: 6px; flex: none; }
  .chip > button { white-space: nowrap; }
  .chip { position: relative; }
  .chip > button { border-radius: var(--r-pill); padding: 1px 10px; font: inherit; font-size: var(--fs-xs); background: transparent; border: 1px solid var(--line); color: var(--muted); }
  .error, .needs { color: var(--fail) !important; border-color: var(--fail) !important; }
  .warning, .disk { color: var(--warn) !important; border-color: var(--warn) !important; }
  .unsaved { color: var(--accent) !important; }
  .pop { position: absolute; top: 100%; left: 0; margin-top: 6px; width: 320px; padding: 10px 12px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); box-shadow: 0 8px 24px rgb(0 0 0 / 0.35); z-index: 25; font-size: var(--fs-sm); }
  .pop p { margin: 0 0 6px; line-height: 1.45; }
  .act { background: var(--accent); color: var(--on-accent); border: 0; border-radius: var(--r-md); padding: 3px 12px; font: inherit; }
</style>
