<script lang="ts">
  import { FlowService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { app } from './lib/app.svelte'
  import { errorText } from './lib/errors'
  import type { DiffView, FlowView, RunComparison } from './lib/types'

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
</script>

<div>
  {#if view.run.summary || view.run.error}
    <p class:err={!!view.run.error} class="last">{view.run.error || view.run.summary}</p>
  {/if}
  {#if !runs.length}
    <p class="muted">No saved runs yet.</p>
  {/if}
  {#each runs as r, i}
    <button class="run" class:active={i === from || i === to} onclick={() => compare(i, 0)}>
      <span class="dot {r.status}"></span>{r.label}{i === 0 ? ' (latest)' : ''}
    </button>
  {/each}
  {#if from >= 0 && from !== to}
    <h5>Compare</h5>
    <div class="pick">
      <select aria-label="Earlier run" value={from} onchange={(e) => compare(Number(e.currentTarget.value), to)}>
        {#each runs as r, i}<option value={i}>{r.label}{i === 0 ? ' (latest)' : ''}</option>{/each}
      </select>
      <span>→</span>
      <select aria-label="Later run" value={to} onchange={(e) => compare(from, Number(e.currentTarget.value))}>
        {#each runs as r, i}<option value={i}>{r.label}{i === 0 ? ' (latest)' : ''}</option>{/each}
      </select>
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
            <td class="gap">{d.gap}{#if changedBody(d.step)}<span class="chev" title="Its outputs differ: click to see how">{open === d.step ? '▾' : '▸'} outputs</span>{/if}</td>
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
  .pick { display: flex; align-items: center; gap: 6px; margin-bottom: 6px; }
  .pick select { flex: 1; min-width: 0; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 4px; font: inherit; font-size: var(--fs-sm); }
  tr.body { cursor: default; }
  tr.body:hover td { background: color-mix(in srgb, var(--accent) 8%, transparent); }
  .chev { margin-left: 8px; color: var(--accent); }
  .val { font-family: var(--mono); word-break: break-all; }
  .lines pre { margin: 0 0 6px; padding: 6px 8px; background: var(--bg); border: 1px solid var(--line); border-radius: var(--r-md); font: var(--fs-xs) var(--mono); overflow: auto; max-height: 260px; }
  .lines span { display: block; white-space: pre; }
  .lines .add { background: color-mix(in srgb, var(--ok) 18%, transparent); }
  .lines .del { background: color-mix(in srgb, var(--fail) 18%, transparent); }
</style>
