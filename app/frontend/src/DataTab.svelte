<script lang="ts">
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { FlowView, NodeData } from './lib/types'
  import Parts from './Parts.svelte'

  let { view }: { view: FlowView } = $props()
  let data = $state<NodeData | null>(null)
  let format = $state('json')

  const step = $derived(view.selected)
  // Refetch when the step, its status or the run state changes: the status is
  // the cheap signal that the data changed.
  const stamp = $derived(`${step}|${view.run.statuses[step] ?? ''}|${view.run.running}|${format}`)

  $effect(() => {
    void stamp
    if (!step) {
      data = null
      return
    }
    FlowService.NodeData(view.node, step, format).then((d) => (data = d as unknown as NodeData))
  })
</script>

<div class="data">
  {#if !step}
    <p class="muted">Select a step to see what the last run did there.</p>
  {:else if !data || !data.status}
    <p class="muted">{step} has not run yet.</p>
  {:else}
    <h3>{step} <span class="status {data.status}">{data.status}</span></h3>
    <p class="muted">{data.instances} {data.instances === 1 ? 'instance' : 'instances'} · {data.duration}</p>
    {#if data.tests.length}
      <h5>Checks</h5>
      {#each data.tests as t}
        <div class="test" class:fail={!t.passed}>{t.passed ? '✓' : '✗'} {t.name}{#if t.detail}<pre>{t.detail}</pre>{/if}</div>
      {/each}
    {/if}
    {#if data.input.length}
      <h4>Input</h4>
      <Parts parts={data.input} />
    {/if}
    {#if data.output.length}
      <h4>
        Output
        <select bind:value={format} aria-label="Body format">
          <option value="json">JSON</option>
          <option value="text">Text</option>
          <option value="raw">Raw</option>
        </select>
      </h4>
      <Parts parts={data.output} />
    {/if}
    {#if data.diags.length}
      <h4>Diagnostics</h4>
      {#each data.diags as d}<pre class="diag">{d}</pre>{/each}
    {/if}
  {/if}
</div>

<style>
  .data { display: flex; flex-direction: column; gap: 4px; }
  h3 { margin: 0; font-size: var(--fs-md); }
  h4 { margin: 10px 0 2px; font-size: var(--fs-sm); display: flex; justify-content: space-between; align-items: center; }
  h5 { margin: 8px 0 2px; color: var(--muted); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: 0.05em; }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .status { font-size: var(--fs-xs); font-weight: 600; margin-left: 6px; }
  .status.passed { color: var(--ok); }
  .status.failed, .status.error, .status.aborted { color: var(--fail); }
  .test { font-size: var(--fs-sm); color: var(--ok); }
  .test.fail { color: var(--fail); }
  pre { margin: 2px 0 0; font: var(--fs-xs) var(--mono); white-space: pre-wrap; -webkit-user-select: text; user-select: text; }
  .diag { color: var(--fail); }
  select { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); font: inherit; font-size: var(--fs-xs); }
</style>
