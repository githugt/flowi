<script lang="ts">
  import EmptyState from './EmptyState.svelte'
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { FlowView, NodeData } from './lib/types'
  import Parts from './Parts.svelte'
  import Select from './Select.svelte'
  import Hint from './Hint.svelte'
  import Button from './Button.svelte'

  let { view }: { view: FlowView } = $props()
  let data = $state<NodeData | null>(null)
  let format = $state('json')

  const step = $derived(view.selected)
  // Where the step takes values from and who takes values from it: wiring, so it shows before the step has run.
  const wiring = $derived(view.form.step)
  const hasValues = $derived(!!wiring && wiring.id === step && (wiring.uses.length > 0 || wiring.usedBy.length > 0))
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
  {#if hasValues && wiring}
    <h4><span>Values<Hint>Where this step takes values from, and who takes values from it. The canvas draws these as dotted lines under the cards.</Hint></span></h4>
    <div class="wires">
    {#each wiring.uses as u}
      <Button variant="ghost" small onclick={() => FlowService.Select(view.node, u.from)}>← <code>{u.name}</code>{#if u.type} <span class="type" title="What the value is">{u.type}</span>{/if} · from {u.from}</Button>
    {/each}
    {#each wiring.usedBy as u}
      <Button variant="ghost" small onclick={() => FlowService.Select(view.node, u.to)}>→ <code>{u.name}</code>{#if u.type} <span class="type" title="What the value is">{u.type}</span>{/if} · used by {u.to}</Button>
    {/each}
    </div>
  {/if}
  {#if !step}
    <EmptyState icon="flow" title="No step selected" hint="Select a step to see what the last run did there." compact />
  {:else if !data || !data.status}
    <EmptyState icon="clock" title="{step} has not run yet" compact />
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
        <Select bind:value={format} label="Body format" size="sm">
          <option value="json">JSON</option>
          <option value="text">Text</option>
          <option value="raw">Raw</option>
        </Select>
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
  .type { color: var(--muted); font-size: var(--fs-xs); border: 1px solid var(--line); border-radius: var(--r-pill); padding: 0 6px; }
  .wires { display: flex; flex-direction: column; align-items: flex-start; gap: 2px; }
  .test { font-size: var(--fs-sm); color: var(--ok); }
  .test.fail { color: var(--fail); }
  pre { margin: 2px 0 0; font: var(--fs-xs) var(--mono); white-space: pre-wrap; -webkit-user-select: text; user-select: text; }
  .diag { color: var(--fail); }
</style>
