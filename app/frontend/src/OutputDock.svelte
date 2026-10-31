<script lang="ts">
  // What a step did in the last run, under its card: tabs by kind of step (Input, Output and Checks for a request,
  // Messages for a message request, Output for the rest), and a footer with how it went. Pinning keeps it open
  // while another step is selected.
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { FlowView, NodeData } from './lib/types'
  import { dockTabs, type DockTab } from './lib/card'
  import Parts from './Parts.svelte'
  import Code from './Code.svelte'

  let { view, step, pinned = false, onpin }: { view: FlowView; step: string; pinned?: boolean; onpin: () => void } = $props()

  let data = $state<NodeData | null>(null)
  let format = $state('json')
  let chosen = $state<DockTab | ''>('')
  // Refetch when the step's status or the run state changes: the status is the cheap signal that the data changed.
  const stamp = $derived(`${step}|${view.run.statuses[step] ?? ''}|${view.run.running}|${format}`)
  $effect(() => {
    void stamp
    FlowService.NodeData(view.node, step, format).then((d) => (data = d as unknown as NodeData))
  })

  const tabs = $derived(data ? dockTabs(data) : [])
  const tab = $derived<DockTab | ''>(chosen && tabs.includes(chosen) ? chosen : (tabs[0] ?? ''))
  const LABELS: Record<DockTab, string> = { input: 'Input', output: 'Output', checks: 'Checks', messages: 'Messages', problems: 'Problems' }
</script>

{#if data && data.status}
  <div class="dock" role="region" aria-label={`What ${step} did`}>
    <div class="tabs" role="tablist">
      {#each tabs as t}
        <button role="tab" aria-selected={tab === t} class:on={tab === t} onclick={() => (chosen = t)}>{LABELS[t]}{#if t === 'checks'}<span class="count">{data.tests.length}</span>{/if}{#if t === 'messages'}<span class="count">{data.messages.length}</span>{/if}</button>
      {/each}
      <span class="grow"></span>
      <button class="pin" class:on={pinned} aria-pressed={pinned} title={pinned ? 'Unpin: close this when another step is selected' : 'Pin: keep this open'} onclick={onpin}>{pinned ? 'Pinned' : 'Pin'}</button>
    </div>
    <div class="body">
      {#if tab === 'input'}
        <Parts parts={data.input} />
      {:else if tab === 'output'}
        {#if data.kind === 'request'}
          <select bind:value={format} aria-label="Body format"><option value="json">JSON</option><option value="text">Text</option><option value="raw">Raw</option></select>
        {/if}
        <Parts parts={data.output} />
      {:else if tab === 'messages'}
        {#each data.messages as m}
          <div class="msg"><span class="dir {m.direction}">{m.direction === 'sent' ? '→ sent' : '← received'}</span><Code text={m.text} /></div>
        {/each}
      {:else if tab === 'checks'}
        {#each data.tests as t}
          <div class="test" class:fail={!t.passed}>{t.passed ? '✓' : '✗'} {t.name}{#if t.detail}<pre>{t.detail}</pre>{/if}</div>
        {/each}
      {:else if tab === 'problems'}
        {#each data.diags as d}<pre class="diag">{d}</pre>{/each}
      {:else}
        <p class="muted">{step} ran and has nothing to show.</p>
      {/if}
    </div>
    <div class="foot"><span class="status {data.status}">{data.status}</span><span>{data.duration}</span>{#if data.size}<span>{data.size}</span>{/if}{#if data.instances > 1}<span>{data.instances} runs</span>{/if}</div>
  </div>
{/if}

<style>
  .dock { display: flex; flex-direction: column; background: var(--panel); border: 1px solid var(--line); border-radius: 14px; box-shadow: var(--shadow); overflow: hidden; max-height: 320px; }
  .tabs { display: flex; align-items: center; gap: 2px; padding: 6px 8px 0; border-bottom: 1px solid var(--line); }
  .tabs button { background: none; border: 0; border-bottom: 2px solid transparent; color: var(--muted); font: inherit; font-size: var(--fs-sm); padding: 4px 8px; }
  .tabs button.on { color: var(--text); border-bottom-color: var(--accent); }
  .tabs .pin { border-radius: var(--r-md); border: 1px solid var(--line); padding: 1px 8px; margin-bottom: 4px; }
  .tabs .pin.on { color: var(--accent); border-color: var(--accent); }
  .grow { flex: 1; }
  .body { padding: 8px 12px; overflow: auto; min-height: 0; display: flex; flex-direction: column; gap: 6px; }
  .foot { display: flex; gap: 12px; padding: 6px 12px; border-top: 1px solid var(--line); color: var(--muted); font-size: var(--fs-xs); }
  .status { font-weight: 700; }
  .status.passed { color: var(--ok); }
  .status.failed, .status.error, .status.aborted { color: var(--fail); }
  .msg { display: flex; flex-direction: column; gap: 2px; }
  .dir { font-size: var(--fs-xs); font-weight: 700; color: var(--muted); }
  .dir.sent { color: var(--accent); }
  .dir.received { color: var(--ok); }
  .test { font-size: var(--fs-sm); color: var(--ok); }
  .test.fail { color: var(--fail); }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  pre { margin: 2px 0 0; font: var(--fs-xs) var(--mono); white-space: pre-wrap; -webkit-user-select: text; user-select: text; }
  .diag { color: var(--fail); }
  select { align-self: flex-end; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); font: inherit; font-size: var(--fs-xs); }
</style>
