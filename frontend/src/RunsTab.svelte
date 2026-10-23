<script lang="ts">
  import { FlowService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { app } from './lib/app.svelte'
  import type { DiffView, FlowView } from './lib/types'

  let { view }: { view: FlowView } = $props()
  const runs = $derived(app.history[view.node] ?? [])
  let diff = $state<DiffView[]>([])
  let chosen = $state(-1)

  // The list is read when the tab opens and after each run ends.
  $effect(() => {
    void view.run.running
    FlowService.LoadHistory(view.node)
  })

  async function compare(i: number) {
    chosen = i
    diff = i > 0 ? ((await FlowService.CompareRun(view.node, i)) as unknown as DiffView[]) : []
  }
</script>

<div>
  {#if view.run.summary || view.run.error}
    <p class:err={!!view.run.error} class="last">{view.run.error || view.run.summary}</p>
  {/if}
  {#if !runs.length}
    <p class="muted">No saved runs yet.</p>
  {/if}
  {#each runs as r, i}
    <button class="run" class:active={i === chosen} onclick={() => compare(i)}>
      <span class="dot {r.status}"></span>{r.label}{i === 0 ? ' (latest)' : ''}
    </button>
  {/each}
  {#if chosen > 0}
    <h5>Compared with the latest run</h5>
    <table>
      <tbody>
        {#each diff as d}
          <tr class:changed={d.changed}>
            <td>{d.name || d.step}</td>
            <td>{d.before || '–'}</td>
            <td>→</td>
            <td>{d.after || '–'}</td>
            <td class="gap">{d.gap}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</div>

<style>
  .muted, .last { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 6px; }
  .err { color: var(--fail); }
  .run {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    text-align: left;
    background: none;
    border: 0;
    color: var(--text);
    font: inherit;
    font-size: var(--fs-sm);
    padding: 4px 6px;
    border-radius: var(--r-md);
  }
  .run:hover { background: color-mix(in srgb, var(--accent) 10%, transparent); }
  .run.active { background: color-mix(in srgb, var(--accent) 22%, transparent); }
  .dot { width: 8px; height: 8px; border-radius: 50%; background: var(--muted); flex: none; }
  .dot.passed { background: var(--ok); }
  .dot.failed, .dot.error, .dot.aborted { background: var(--fail); }
  h5 { margin: 10px 0 4px; color: var(--muted); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: 0.05em; }
  table { width: 100%; font-size: var(--fs-sm); border-collapse: collapse; }
  td { padding: 2px 4px; }
  .changed td { color: var(--accent); }
  .gap { color: var(--muted); text-align: right; }
</style>
