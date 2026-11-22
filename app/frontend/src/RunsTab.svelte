<script lang="ts">
  import EmptyState from './EmptyState.svelte'
  import Button from './Button.svelte'
  import Select from './Select.svelte'
  import Icon from './Icon.svelte'
  import { openExternal } from './lib/openurl'
  import { FlowService, TracingService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { app } from './lib/app.svelte'
  import { errorText } from './lib/errors'
  import type { DiffView, FlowView, RunComparison } from './lib/types'
  import { statusMark } from './lib/statusmark'
  import { lastGreenIndex } from './lib/lastgreen'
  import { plural } from './lib/plural'
  import { ui } from './lib/ui.svelte'
  import { runPanelsOf, toggleRunPanel } from './lib/runpanels.svelte'

  let { view }: { view: FlowView } = $props()
  const runs = $derived(app.history[view.node] ?? [])
  // Two runs are compared: from (the earlier) and to. Clicking a run in the list makes it "from" and
  // compares it with the latest; the two lists change either end.
  let from = $state(-1)
  let to = $state(0)
  let diff = $state<DiffView[]>([])
  let depth = $state<RunComparison | null>(null)
  let open = $state<string | null>(null)
  let problem = $state('')
  // The run the last click chose: its buttons (steps, timeline, trace) show under it.
  let picked = $state('')
  const panels = $derived(runPanelsOf(view.node))

  // Where each run's trace is in the person's tracing tool (the otel.trace_url setting), when one is set.
  let traces = $state<Record<string, string>>({})
  $effect(() => {
    const ids = runs.map((r) => r.id)
    Promise.all(ids.map((id) => TracingService.TraceURL(id))).then((urls) => (traces = Object.fromEntries(ids.map((id, i) => [id, urls[i]]))))
  })

  // The list is read when the tab opens and after each run ends.
  $effect(() => {
    void view.run.running
    FlowService.LoadHistory(view.node)
  })

  async function compare(a: number, b: number) {
    from = a
    to = b
    open = null
    depth = null
    problem = ''
    if (a < 0 || a === b) {
      diff = []
      return
    }
    diff = (await FlowService.CompareRuns(view.node, a, b)) as unknown as DiffView[]
    try {
      depth = (await FlowService.CompareRunDetail(view.node, a, b)) as unknown as RunComparison
    } catch (e) {
      problem = errorText(e)
    }
  }
  const changedBody = (step: string) => !!depth?.steps?.[step]

  // The list changes under a comparison when runs go, so the picked runs and what was shown for them are let go.
  function forgetPicks() {
    from = -1
    to = 0
    diff = []
    depth = null
    open = null
    problem = ''
  }
  // Runs are a record the engine wrote, not something the person made, so one goes at once; all of them ask first.
  function clearOne(id: string) {
    forgetPicks()
    FlowService.DeleteRun(view.node, id).catch(ui.fail)
  }
  function clearAll() {
    ui.confirm('Clear all runs?', `The ${plural(runs.length, 'saved run')} of this flow will be deleted. This cannot be undone.`, () => {
      forgetPicks()
      return FlowService.ClearRuns(view.node).then(() => undefined)
    }, 'Clear all')
  }
  function runMenu(e: MouseEvent, id: string) {
    e.preventDefault()
    picked = id
    ui.openMenu(e.clientX, e.clientY, [
      { label: 'Steps', checked: panels.steps === id, run: () => toggleRunPanel(view.node, 'steps', id) },
      { label: 'Timeline', checked: panels.timeline === id, run: () => toggleRunPanel(view.node, 'timeline', id) },
      { label: '', separator: true, run: () => {} },
      { label: 'Clear this run', run: () => clearOne(id) },
      { label: 'Clear all runs…', run: clearAll },
    ])
  }

  // When the latest run went wrong, the run to blame is the last one that worked: it is compared at once, and the card
  // says so. It is done once for each latest run, so a comparison the person picks is not replaced.
  const green = $derived(lastGreenIndex(runs.map((r) => r.status)))
  let autoFor = ''
  $effect(() => {
    const latest = runs[0]?.id ?? ''
    if (latest && green > 0 && autoFor !== latest) {
      autoFor = latest
      void compare(green, 0)
    }
  })
</script>

<div>
  {#if view.run.summary || view.run.error}
    <p class:err={!!view.run.error} class="last">{view.run.error || view.run.summary}</p>
  {/if}
  {#if app.trends[view.node]?.trend}
    <p class="trend" title="How the latest runs ended, oldest first, and how long they usually take">{app.trends[view.node].trend}</p>
    {#if app.trends[view.node].slowNote}<p class="muted">{app.trends[view.node].slowNote}</p>{/if}
  {/if}
  {#if green > 0}
    <p class="since" role="status">
      <strong>The latest run went wrong.</strong> Since the last run that passed ({runs[green].label}):
      <Button small variant="ghost" onclick={() => compare(green, 0)}>Compare with it</Button>
    </p>
  {/if}
  {#if !runs.length}
    <EmptyState icon="clock" title="No saved runs yet" hint="Each run of this flow is kept here, so two runs can be compared." compact />
  {/if}
  {#if runs.length}
    <div class="listhead">
      <span>{plural(runs.length, 'saved run')}</span>
      <Button small variant="ghost" danger title="Delete every saved run of this flow" onclick={clearAll}>Clear all</Button>
    </div>
  {/if}
  {#each runs as r, i}
    <div class="runrow" role="presentation" oncontextmenu={(e) => runMenu(e, r.id)}>
      <button class="run" class:active={i === from || i === to} onclick={() => { picked = r.id; void compare(i, 0) }}>
        <span class="dot {r.status}" role="img" aria-label={r.status} title={r.status}>{statusMark(r.status)}</span>{r.label}{i === 0 ? ' (latest)' : ''}
      </button>
    </div>
    {#if picked === r.id || panels.steps === r.id || panels.timeline === r.id}
      <div class="acts">
        <Button small variant="ghost" pressed={panels.steps === r.id} title="List this run's steps: status, result and duration" onclick={() => toggleRunPanel(view.node, 'steps', r.id)}>Steps</Button>
        <Button small variant="ghost" pressed={panels.timeline === r.id} title="Show this run as a timeline: when each step started and how long it took" onclick={() => toggleRunPanel(view.node, 'timeline', r.id)}>Timeline</Button>
        {#if traces[r.id]}<Button small variant="ghost" title="Open this run's trace in your tracing tool" onclick={() => openExternal(traces[r.id])}>Open trace</Button>{/if}
      </div>
    {/if}
  {/each}
  {#if from >= 0 && from !== to}
    <h5>Compare</h5>
    <div class="pick">
      <Select fill label="Earlier run" value={from} onchange={(e) => compare(Number(e.currentTarget.value), to)}>
        {#each runs as r, i}<option value={i}>{r.label}{i === 0 ? ' (latest)' : ''}</option>{/each}
      </Select>
      <span>→</span>
      <Select fill label="Later run" value={to} onchange={(e) => compare(from, Number(e.currentTarget.value))}>
        {#each runs as r, i}<option value={i}>{r.label}{i === 0 ? ' (latest)' : ''}</option>{/each}
      </Select>
    </div>
    {#if problem}<p class="err">{problem}</p>{/if}
    <table>
      <tbody>
        {#each diff as d}
          <tr class:changed={d.changed} class:body={changedBody(d.step)} onclick={() => changedBody(d.step) && (open = open === d.step ? null : d.step)}>
            <td>{d.name || d.step}</td>
            <td>{d.before || '–'}</td>
            <td>→</td>
            <td>{d.after || '–'}</td>
            <td class="gap">{d.gap}{#if changedBody(d.step)}<span class="chev" title="Its outputs differ: click to see how"><Icon name={open === d.step ? 'chevd' : 'chevr'} size={12} /> outputs</span>{/if}</td>
          </tr>
          {#if open === d.step && depth?.steps?.[d.step]}
            <tr class="lines"><td colspan="5">
              <pre aria-label={`Outputs of ${d.name || d.step}, line by line`}>{#each depth.steps[d.step] as l}<span class={l.kind}>{l.kind === 'add' ? '+ ' : l.kind === 'del' ? '- ' : '  '}{l.text}
</span>{/each}</pre>
            </td></tr>
          {/if}
        {/each}
      </tbody>
    </table>
    {#if depth?.vars?.length}
      <h5>Variables that ended differently</h5>
      <table>
        <tbody>
          {#each depth.vars as v}
            <tr class="changed">
              <td>{v.name}</td>
              <td class="val">{v.hadBefore ? v.before : '–'}</td>
              <td>→</td>
              <td class="val">{v.hadAfter ? v.after : '–'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  {/if}
</div>

<style>
  .muted, .last { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 6px; }
  .err { color: var(--fail); }
  .since { margin: 0 0 var(--sp-2); font-size: var(--fs-sm); padding: 4px 8px; border-radius: var(--r-md); background: var(--fail-t); }
  .trend { margin: 0 0 var(--sp-2); font-family: var(--mono); font-size: var(--fs-sm); }
  .listhead { display: flex; align-items: center; justify-content: space-between; margin: 0 0 var(--sp-1); color: var(--muted); font-size: var(--fs-xs); }
  .runrow { display: flex; align-items: center; gap: var(--sp-2); }
  .runrow .run { flex: 1; min-width: 0; }
  .acts { display: flex; flex-wrap: wrap; gap: var(--sp-1); margin: 0 0 var(--sp-1) 28px; }
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
  .run:hover { background: var(--tint-hover); }
  .run.active { background: var(--tint-sel); }
  .dot { width: 14px; height: 14px; border-radius: 50%; background: var(--muted); color: var(--bg); font-size: 9px; font-weight: 700; line-height: 14px; text-align: center; flex: none; }
  .dot.passed { background: var(--ok); }
  .dot.failed, .dot.error, .dot.aborted { background: var(--fail); }
  h5 { margin: 10px 0 4px; color: var(--muted); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: 0.05em; }
  table { width: 100%; font-size: var(--fs-sm); border-collapse: collapse; }
  td { padding: 2px 4px; }
  .changed td { color: var(--accent); }
  .gap { color: var(--muted); text-align: right; }
  .pick { display: flex; align-items: center; gap: 6px; margin-bottom: 6px; }
  tr.body { cursor: default; }
  tr.body:hover td { background: var(--tint-hover); }
  .chev { display: inline-flex; align-items: center; gap: var(--sp-1); margin-left: 8px; color: var(--accent); }
  .val { font-family: var(--mono); word-break: break-all; }
  .lines pre { margin: 0 0 6px; padding: 6px 8px; background: var(--bg); border: 1px solid var(--line); border-radius: var(--r-md); font: var(--fs-xs) var(--mono); overflow: auto; max-height: 260px; }
  .lines span { display: block; white-space: pre; }
  .lines .add { background: color-mix(in srgb, var(--ok) 18%, transparent); }
  .lines .del { background: color-mix(in srgb, var(--fail) 18%, transparent); }
</style>
