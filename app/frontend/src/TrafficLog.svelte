<script lang="ts">
  import EmptyState from './EmptyState.svelte'
  import Status from './Status.svelte'
  import Segmented from './Segmented.svelte'
  import Verb from './Verb.svelte'
  import SearchField from './SearchField.svelte'
  import { TRAFFIC_MODES, filterTraffic, type TrafficMode, type TrafficRow } from './lib/traffic'
  import { callCurl } from './lib/callcurl'
  import { copy } from './lib/clip'
  import Button from './Button.svelte'
  import { statusTone as tone } from './lib/statustone'
  import { tookTone } from './lib/took'
  import Code from './Code.svelte'
  import { ReplayerService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { CallView, CallPart } from './lib/types'

  let { rows, addr = '' }: { rows: TrafficRow[]; addr?: string } = $props()

  let query = $state('')
  let mode = $state<TrafficMode>('all')
  const shown = $derived(filterTraffic(rows, query, mode))
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
    <Segmented label="Which calls" items={TRAFFIC_MODES} value={mode} onpick={(id) => (mode = id as typeof mode)} />
    <span class="count-note">{shown.length === rows.length ? `${rows.length} calls` : `${shown.length} of ${rows.length}`}</span>
  </div>
  {#each shown as t (t.seq)}
    <button class="r" class:open={open === t.seq} aria-expanded={open === t.seq} onclick={() => toggle(t.seq)}>
      <span class="at">{t.at}</span>
      <span class="v"><Verb verb={t.method} short /></span>
      <span class="path" title={t.path}>{t.path}</span>
      <span class="code"><Status block tone={tone(t.status)}>{t.status}</Status></span>
      <span class="match" title="How the call was answered">{t.match}{t.request ? ' · ' + t.request : ''}</span>
      <span class="took {tookTone(t.duration)}">{t.duration}</span>
    </button>
    {#if open === t.seq}
      <dl class="det">
        {#each detailOf(t) as [k, v]}<dt>{k}</dt><dd>{v}</dd>{/each}
      </dl>
      <div class="copyrow"><Button small variant="ghost" onclick={() => copy(callCurl(addr, t, calls[t.seq]?.captured ? calls[t.seq].sent : undefined), 'the call as curl')}>Copy as curl</Button></div>
      {#if calls[t.seq]?.captured}
        <div class="msgs">
          {@render part('What the caller sent', calls[t.seq].sent)}
          {@render part('What it was answered with', calls[t.seq].got)}
        </div>
      {/if}
    {/if}
  {/each}
  {#if !rows.length}<EmptyState icon="log" title="No calls yet" compact />{:else if !shown.length}<EmptyState kind="filtered" compact {query} onclear={() => { query = ''; mode = 'all' }} />{/if}
</div>

<style>
  .log { display: flex; flex-direction: column; gap: 1px; }
  .bar { display: flex; align-items: center; gap: 8px; padding-bottom: 6px; }
  .find { flex: 1; max-width: 280px; }
  .code { flex: none; width: 54px; }
  .count-note { margin-left: auto; color: var(--muted); font-size: var(--fs-sm); }
  .r { all: unset; box-sizing: border-box; width: 100%; cursor: pointer; display: flex; align-items: center; gap: 12px; height: var(--row-h-lg); padding: 0 10px; border-radius: var(--r-sm); font-size: var(--fs-sm); }
  .r:hover, .r.open { background: var(--tint-hover); }
  .r:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }
  .copyrow { padding: 2px 12px 4px 88px; background: var(--sunk); }
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
  /* Fixed widths, so the rows of 200 and 404 line up whatever the status and how it was answered say. */
  .match { flex: none; width: 28%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--muted); }
  .took { flex: none; width: 58px; text-align: right; color: var(--muted); font-family: var(--mono); }
  .took.slow { color: var(--warn); }
  .took.verySlow { color: var(--fail); }
  .none { color: var(--muted); font-size: var(--fs-sm); margin: 8px 0; }
</style>
