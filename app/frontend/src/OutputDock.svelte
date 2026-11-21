<script lang="ts">
  import EmptyState from './EmptyState.svelte'
  // What a step did in the last run, under its card: tabs by kind of step (Input, Output and Checks for a request,
  // Messages for a message request, Output for the rest), and a footer with how it went. Pinning keeps it open
  // while another step is selected.
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { FlowView, NodeData } from './lib/types'
  import { dockTabs, firstDockTab, type DockTab } from './lib/card'
  import Parts from './Parts.svelte'
  import Select from './Select.svelte'
  import Tabs from './Tabs.svelte'
  import MessageFeed from './MessageFeed.svelte'
  import ResultGrid from './ResultGrid.svelte'

  let { view, step, pinned = false, onpin }: { view: FlowView; step: string; pinned?: boolean; onpin: () => void } = $props()

  let data = $state<NodeData | null>(null)
  let format = $state('json')
  let chosen = $state<DockTab | ''>('')
  // Refetch when the step's status or the run state changes: the status is the cheap signal that the data changed.
  const stamp = $derived(`${step}|${view.run.statuses[step] ?? ''}|${view.run.running}|${format}`)
  // A late answer for a step that is no longer the one shown must not replace the data of the new one.
  let latest = 0
  let problem = $state('')
  $effect(() => {
    void stamp
    const mine = ++latest
    FlowService.NodeData(view.node, step, format).then(
      (d) => {
        if (mine !== latest) return
        problem = ''
        data = d as unknown as NodeData
      },
      (e) => {
        if (mine === latest) problem = String(e)
      },
    )
  })

  const tabs = $derived(data ? dockTabs(data) : [])
  const tab = $derived<DockTab | ''>(chosen && tabs.includes(chosen) ? chosen : data ? firstDockTab(tabs, data.status, data.tests.filter((t) => !t.passed).length) : '')
  const LABELS: Record<DockTab, string> = { input: 'Input', output: 'Output', checks: 'Checks', messages: 'Messages', problems: 'Problems' }
</script>

{#if problem}
  <div class="dock" role="region" aria-label={`What ${step} did`}><div class="body"><p class="muted">Could not load what {step} did: {problem}</p></div></div>
{:else if data && data.status}
  {@const d = data}
  <div class="dock" role="region" aria-label={`What ${step} did`}>
    <div class="tabbar">
      <Tabs label="What the step did" size="sm" active={tab} onpick={(id) => (chosen = id as DockTab)} items={tabs.map((t) => ({ id: t, label: LABELS[t], count: t === 'checks' ? d.tests.length : t === 'messages' ? d.messages.length : undefined }))}>
        <button class="pin" class:on={pinned} aria-pressed={pinned} title={pinned ? 'Unpin: close this when another step is selected' : 'Pin: keep this open'} onclick={onpin}>{pinned ? 'Pinned' : 'Pin'}</button>
      </Tabs>
    </div>
    <div class="body">
      {#if tab === 'input'}
        <Parts parts={data.input} />
      {:else if tab === 'output'}
        {#if data.kind === 'request'}
          <span class="fmt"><Select bind:value={format} label="Body format" size="sm"><option value="json">JSON</option><option value="text">Text</option><option value="raw">Raw</option></Select></span>
        {/if}
        {#if data.table}<ResultGrid table={data.table} />{/if}
        <Parts parts={data.table ? data.output.filter((p) => p.heading !== 'Body') : data.output} />
      {:else if tab === 'messages'}
        <MessageFeed messages={data.messages} />
      {:else if tab === 'checks'}
        {#each data.tests as t}
          <div class="test" class:fail={!t.passed}>{t.passed ? '✓' : '✗'} {t.name}{#if t.detail}<pre>{t.detail}</pre>{/if}</div>
        {/each}
      {:else if tab === 'problems'}
        {#each data.diags as d}<pre class="diag">{d}</pre>{/each}
      {:else}
        <EmptyState icon="note" title="{step} has nothing to show" hint="It ran, but made no output." compact />
      {/if}
    </div>
    <div class="foot"><span class="status {data.status}">{data.status}</span><span>{data.duration}</span>{#if data.size}<span>{data.size}</span>{/if}{#if data.instances > 1}<span>{data.instances} runs</span>{/if}</div>
  </div>
{:else if data}
  <!-- A pinned box stays while a run starts, instead of vanishing until the step has finished. -->
  <div class="dock" role="region" aria-label={`What ${step} did`}>
    <div class="tabs"><span class="grow"></span><button class="pin" class:on={pinned} aria-pressed={pinned} title={pinned ? "Unpin: close this when another step is selected" : "Pin: keep this open"} onclick={onpin}>{pinned ? "Pinned" : "Pin"}</button></div>
    <div class="body"><EmptyState icon="clock" title={view.run.running ? `${step} is running…` : `${step} has not run yet`} compact /></div>
  </div>
{/if}

<style>
  .dock { display: flex; flex-direction: column; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-xl); box-shadow: var(--shadow); overflow: hidden; max-height: 320px; }
  .tabbar { padding: 6px 8px 0; }
  .tabs { display: flex; align-items: center; gap: 2px; padding: 6px 8px 0; border-bottom: 1px solid var(--line); }
  .pin { background: none; color: var(--muted); font: inherit; font-size: var(--fs-sm); border-radius: var(--r-md); border: 1px solid var(--line); padding: 1px 8px; margin-bottom: 4px; }
  .pin.on { color: var(--accent); border-color: var(--accent); }
  .grow { flex: 1; }
  .body { padding: 8px 12px; overflow: auto; min-height: 0; display: flex; flex-direction: column; gap: 6px; }
  .foot { display: flex; gap: 12px; padding: 6px 12px; border-top: 1px solid var(--line); color: var(--muted); font-size: var(--fs-xs); }
  .status { font-weight: 700; }
  .status.passed { color: var(--ok); }
  .status.failed, .status.error, .status.aborted { color: var(--fail); }
  .test { font-size: var(--fs-sm); color: var(--ok); }
  .test.fail { color: var(--fail); }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  pre { margin: 2px 0 0; font: var(--fs-xs) var(--mono); white-space: pre-wrap; -webkit-user-select: text; user-select: text; }
  .diag { color: var(--fail); }
  .fmt { align-self: flex-end; }
</style>
