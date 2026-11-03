<script lang="ts">
  import Verb from './Verb.svelte'
  import SearchField from './SearchField.svelte'
  import { filterTraffic, statusTone, type TrafficRow } from './lib/traffic'
  import { tookTone } from './lib/took'
  import Code from './Code.svelte'
  import { ReplayerService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { CallView, CallPart } from './lib/types'

  let { rows }: { rows: TrafficRow[] } = $props()

  let query = $state('')
  let failedOnly = $state(false)
  const shown = $derived(filterTraffic(rows, query, failedOnly))
  // The call opened in place, by its number, so it stays open as newer calls arrive above it.
  let open = $state<number | null>(null)
  // The headers and bodies of calls that were opened, by number: fetched once, as the rows do not carry them.
  let calls = $state<Record<number, CallView>>({})
  async function toggle(seq: number) {
    open = open === seq ? null : seq
    if (open === null || calls[seq]) return
    calls[seq] = (await ReplayerService.Call(seq)) as CallView
  }
  // What the log knows of a call besides the line: the saved answer it got and what was added on purpose.
  const detailOf = (t: TrafficRow): [string, string][] =>
    ([
      ['Matched request', t.request],
      ['Version', t.version],
      ['Answered with', t.example ? `${t.example}${t.source ? ` (${t.source})` : ''}` : ''],
      ['Delay added', t.delay],
      ['Time to answer', t.duration],
      ['From', t.remote],
    ] as [string, string][]).filter(([, v]) => v)
</script>

{#snippet part(title: string, p: CallPart)}
  <section>
    <h5>{title}</h5>
    {#if p.headers.length}
      <dl class="hdr">{#each p.headers as h}<dt>{h.name}</dt><dd>{h.value}</dd>{/each}</dl>
    {/if}
    {#if p.body}<Code text={p.body} />{:else}<p class="none">No body.</p>{/if}
    {#if p.truncated}<p class="none">Cut here: only the first part of the body is kept.</p>{/if}
  </section>
{/snippet}

<div class="log">
  <div class="bar">
    <div class="find"><SearchField small bind:value={query} placeholder="Filter calls" label="Filter calls" /></div>
    <div class="seg" role="radiogroup" aria-label="Which calls">
      <button role="radio" aria-checked={!failedOnly} class:on={!failedOnly} onclick={() => (failedOnly = false)}>All</button>
      <button role="radio" aria-checked={failedOnly} class:on={failedOnly} onclick={() => (failedOnly = true)}>Failed</button>
    </div>
    <span class="count-note">{shown.length === rows.length ? `${rows.length} calls` : `${shown.length} of ${rows.length}`}</span>
  </div>
  {#each shown as t (t.seq)}
    <button class="r" class:open={open === t.seq} aria-expanded={open === t.seq} onclick={() => toggle(t.seq)}>
      <span class="at">{t.at}</span>
      <span class="v"><Verb verb={t.method} short /></span>
      <span class="path" title={t.path}>{t.path}</span>
      <span class="status-pill" class:ok={statusTone(t.status) === 'success'} class:bad={statusTone(t.status) === 'server' || statusTone(t.status) === 'client'}>{t.status}</span>
      <span class="match" title="How the call was answered">{t.match}{t.request ? ' · ' + t.request : ''}</span>
      <span class="took {tookTone(t.duration)}">{t.duration}</span>
    </button>
    {#if open === t.seq}
      <dl class="det">
        {#each detailOf(t) as [k, v]}<dt>{k}</dt><dd>{v}</dd>{/each}
      </dl>
      {#if calls[t.seq]?.captured}
        <div class="msgs">
          {@render part('What the caller sent', calls[t.seq].sent)}
          {@render part('What it was answered with', calls[t.seq].got)}
        </div>
      {/if}
    {/if}
  {/each}
  {#if !rows.length}<p class="none">No calls yet.</p>{:else if !shown.length}<p class="none">No call matches.</p>{/if}
</div>

<style>
  .log { display: flex; flex-direction: column; gap: 1px; }
  .bar { display: flex; align-items: center; gap: 8px; padding-bottom: 6px; }
  .find { flex: 1; max-width: 280px; }
  .seg { display: inline-flex; border: 1px solid var(--line); border-radius: var(--r-md); overflow: hidden; }
  .seg button { background: none; border: 0; border-radius: 0; color: var(--muted); font: inherit; font-size: var(--fs-sm); height: var(--ctl-h-sm); padding: 0 10px; box-shadow: none; min-height: 0; }
  .seg button.on { background: var(--tint-sel); color: var(--text); }
  .count-note { margin-left: auto; color: var(--muted); font-size: var(--fs-sm); }
  .r { all: unset; box-sizing: border-box; width: 100%; cursor: pointer; display: flex; align-items: center; gap: 12px; height: var(--row-h-lg); padding: 0 10px; border-radius: var(--r-sm); font-size: var(--fs-sm); }
  .r:hover, .r.open { background: var(--tint-hover); }
  .r:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }
  .det { margin: 0; padding: 8px 12px 10px 88px; display: grid; grid-template-columns: max-content 1fr; gap: 3px 16px; font-size: var(--fs-sm); background: var(--sunk); border-radius: 0 0 var(--r-md) var(--r-md); }
  .det dt { color: var(--muted); }
  .msgs { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; padding: 0 12px 10px 88px; background: var(--sunk); border-radius: 0 0 var(--r-md) var(--r-md); margin: -6px 0 6px; }
  .msgs h5 { margin: 0 0 4px; color: var(--muted); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: 0.05em; }
  .msgs section { min-width: 0; display: flex; flex-direction: column; gap: 6px; }
  .hdr { margin: 0; display: grid; grid-template-columns: max-content 1fr; gap: 2px 12px; font-size: var(--fs-xs); }
  .hdr dt { color: var(--muted); }
  .hdr dd { margin: 0; font-family: var(--mono); overflow-wrap: anywhere; }
  .det dd { margin: 0; font-family: var(--mono); overflow-wrap: anywhere; }
  .at { flex: none; width: 66px; color: var(--muted); font-family: var(--mono); }
  .v { flex: none; width: 56px; display: inline-flex; }
  .path { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: var(--mono); }
  .status-pill { flex: none; }
  .match { flex: none; max-width: 30%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--muted); }
  .took { flex: none; width: 58px; text-align: right; color: var(--muted); font-family: var(--mono); }
  .took.slow { color: var(--warn); }
  .took.verySlow { color: var(--fail); }
  .none { color: var(--muted); font-size: var(--fs-sm); margin: 8px 0; }
</style>
