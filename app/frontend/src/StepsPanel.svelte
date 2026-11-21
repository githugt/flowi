<script lang="ts">
  import EmptyState from './EmptyState.svelte'
  import Button from './Button.svelte'
  import { filterRows, stepRows } from './lib/steplist'
  import type { FlowView } from './lib/types'

  // The flow as a list: every step in canvas order with its status, result and a bar for how long it took. A long flow is
  // thousands of pixels wide on the canvas; here step 19 is one click away. A step's name selects it on the canvas.
  let { view, onselect }: { view: FlowView; onselect: (step: string) => void } = $props()

  let text = $state('')
  const all = $derived(stepRows(view.diagram.units, view.run.statuses, view.run.results, view.run.timeline ?? []))
  const rows = $derived(filterRows(all, text))
</script>

<section class="steps" aria-label="Steps">
  <p class="head">
    <strong>Steps</strong>
    <span class="muted">{all.length}</span>
    <input type="search" placeholder="Filter by name or type" aria-label="Filter the steps" bind:value={text} />
  </p>
  {#if rows.length === 0}
    {#if all.length}<EmptyState kind="filtered" compact query={text} onclear={() => (text = '')} />{:else}<EmptyState icon="flow" title="This flow has no steps yet" compact />{/if}
  {:else}
    <ul>
      {#each rows as r (r.id)}
        <li class:selected={view.selected === r.id} style:padding-left="{r.depth * 14}px">
          <span class="mark {r.status}" aria-label={r.status || 'not run'}>{r.mark}</span>
          <Button variant="ghost" small pressed={view.selected === r.id} title="Show {r.id} on the canvas" onclick={() => onselect(r.id)}>{r.id}</Button>
          <span class="type">{r.type}</span>
          <span class="track" aria-hidden="true">{#if r.bar}<span class="bar {r.status}" style:left="{r.bar.left}%" style:width="{r.bar.width}%"></span>{/if}</span>
          <span class="result">{r.result}</span>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .steps { padding: var(--sp-2) var(--sp-3); border-bottom: 1px solid var(--line); max-height: 40%; overflow: auto; flex: none; }
  .head { margin: 0 0 var(--sp-2); display: flex; gap: var(--sp-2); align-items: center; font-size: var(--fs-sm); }
  .head input { margin-left: auto; width: 200px; }
  .muted { color: var(--muted); font-size: var(--fs-sm); }
  ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 1px; }
  li { display: grid; grid-template-columns: 18px minmax(120px, 200px) 90px 1fr minmax(80px, 160px); align-items: center; gap: var(--sp-2); box-sizing: border-box; }
  li.selected { background: var(--tint-sel); border-radius: var(--r-sm); }
  .mark { text-align: center; font-weight: 700; color: var(--muted); }
  .mark.passed { color: var(--ok); }
  .mark.failed, .mark.error, .mark.aborted { color: var(--fail); }
  .mark.running { color: var(--info); }
  li :global(button) { justify-self: start; max-width: 100%; overflow: hidden; text-overflow: ellipsis; }
  .type { color: var(--muted); font-size: var(--fs-xs); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .track { position: relative; height: 8px; background: color-mix(in srgb, var(--muted) 10%, transparent); border-radius: var(--r-sm); }
  .bar { position: absolute; top: 0; bottom: 0; border-radius: var(--r-sm); background: var(--muted); }
  .bar.passed { background: var(--ok); }
  .bar.running { background: var(--info); }
  .bar.failed, .bar.error, .bar.aborted { background: var(--fail); }
  .result { color: var(--muted); font-size: var(--fs-xs); text-align: right; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
