<script lang="ts">
  import Popover from './Popover.svelte'
  import Button from './Button.svelte'
  // What the flow has to tell: errors, steps out of order, Flowi-only steps,
  // a change on disk, unsaved edits. A chip opens its message under it.
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { Chip, FlowView } from './lib/types'

  let { view }: { view: FlowView } = $props()
  let open = $state<string | null>(null)
  let at = $state({ x: 0, y: 0 })
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

<div class="chips">
  {#each view.chips as c (c.kind)}
    <span class="chip">
      <button class={c.kind} aria-haspopup="dialog" aria-expanded={open === c.kind} onclick={(e) => { const r = e.currentTarget.getBoundingClientRect(); at = { x: r.left, y: r.bottom }; open = open === c.kind ? null : c.kind }}>{label(c)}</button>
      {#if open === c.kind && c.kind !== 'unsaved'}
        <Popover x={at.x} y={at.y} width={320} height={160} label={label(c)} onclose={() => (open = null)}>
          {#if c.kind === 'error'}
            <p>{c.text}</p>
          {:else if c.kind === 'needs'}
            <p>These steps wait for a step that comes after them, so they can never start and a run fails: {c.steps?.join(', ')}</p>
            <Button variant="primary" small onclick={() => { open = null; FlowService.RepairNeeds(node) }}>Repair</Button>
          {:else if c.kind === 'warning'}
            <p>{c.steps?.length} steps are Flowi steps that other Arazzo tools cannot run: {c.steps?.join(', ')}</p>
          {:else if c.kind === 'disk'}
            <p>This file changed on disk.</p>
            <Button variant="primary" small onclick={() => { open = null; FlowService.Revert(node) }}>Load from disk</Button>
          {/if}
        </Popover>
      {/if}
    </span>
  {/each}
</div>

<style>
  .chips { display: flex; gap: 6px; flex: none; }
  .chip { position: relative; }
  .chip > button { white-space: nowrap; border-radius: var(--r-pill); padding: 1px 10px; font: inherit; font-size: var(--fs-xs); background: transparent; border: 1px solid var(--line); color: var(--muted); }
  .error, .needs { color: var(--fail) !important; border-color: var(--fail) !important; }
  .warning, .disk { color: var(--warn) !important; border-color: var(--warn) !important; }
  .unsaved { color: var(--accent) !important; }
  p { margin: 0; font-size: var(--fs-sm); line-height: 1.45; }
</style>
