<script lang="ts">
  // Two questions about the API descriptions you imported: how much of them do your flows use (coverage), and what would
  // a changed description break (before you import it). Neither writes anything.
  import { onMount } from 'svelte'
  import { errorText } from './lib/errors'
  import { ChecksService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import PathField from './PathField.svelte'
  import type { CoverageView, SpecChangeView } from './lib/types'

  let targets = $state<{ id: string; label: string }[]>([])
  let collection = $state('')
  let specs = $state<string[]>([])
  let spec = $state('')

  let coverage = $state<CoverageView | null>(null)
  let coverageBusy = $state(false)
  let coverageError = $state('')

  let newSpec = $state('')
  let change = $state<SpecChangeView | null>(null)
  let changeBusy = $state(false)
  let changeError = $state('')
  let failUnder = $state(0)

  onMount(async () => {
    targets = (await ChecksService.Targets()) as unknown as { id: string; label: string }[]
    if (targets.length) collection = targets[0].id
  })

  $effect(() => {
    const c = collection
    if (!c) return
    ChecksService.Specs(c).then((names) => {
      specs = names as string[]
      spec = specs.length === 1 ? specs[0] : ''
    })
  })

  async function measure() {
    coverageBusy = true
    coverageError = ''
    try {
      coverage = (await ChecksService.Coverage(collection)) as unknown as CoverageView
    } catch (e) {
      coverageError = errorText(e)
      coverage = null
    } finally {
      coverageBusy = false
    }
  }

  async function check() {
    changeBusy = true
    changeError = ''
    try {
      change = (await ChecksService.SpecChange(collection, spec, newSpec)) as unknown as SpecChangeView
    } catch (e) {
      changeError = errorText(e)
      change = null
    } finally {
      changeBusy = false
    }
  }

  const pct = (part: number, whole: number) => (whole ? Math.round((part * 100) / whole) : 0)
  const unused = $derived(coverage?.operations.filter((o) => !o.used.length) ?? [])
  const used = $derived(coverage?.operations.filter((o) => o.used.length) ?? [])
  const belowThreshold = $derived(!!coverage && failUnder > 0 && pct(coverage.operationsUsed, coverage.operationsTotal) < failUnder)
</script>

<div class="page">
  <label class="f"><span>Collection</span>
    <select bind:value={collection}>{#each targets as t}<option value={t.id}>{t.label}</option>{/each}</select>
  </label>

  <h3>Coverage: how much of the spec do your flows use?</h3>
  <p class="note">For every operation of an imported API description: which flows call it, which of its documented statuses a saved run ever got, and which response fields a flow reads. Statuses come from the run history, so a flow that never ran leaves them unseen.</p>
  <div class="line">
    <button class="primary" disabled={!collection || coverageBusy} onclick={measure}>{coverageBusy ? 'Measuring…' : 'Measure'}</button>
    <label class="inline">Flag it below <input class="num" type="number" min="0" max="100" bind:value={failUnder} /> % of operations used</label>
  </div>
  {#if coverageError}<p class="err">{coverageError}</p>{/if}
  {#if coverage}
    {#if !coverage.operations.length}
      <p class="muted">No operation to measure: this collection has no request imported from an OpenAPI description with response schemas.</p>
    {:else}
      <div class="totals" class:bad={belowThreshold}>
        <div><strong>{coverage.operationsUsed} of {coverage.operationsTotal}</strong> operations used by a flow ({pct(coverage.operationsUsed, coverage.operationsTotal)}%)</div>
        <div><strong>{coverage.statusesSeen} of {coverage.statusesTotal}</strong> documented statuses seen in a run ({pct(coverage.statusesSeen, coverage.statusesTotal)}%)</div>
        <div><strong>{coverage.fieldsRead} of {coverage.fieldsTotal}</strong> response fields read by a flow ({pct(coverage.fieldsRead, coverage.fieldsTotal)}%)</div>
        {#if belowThreshold}<div class="badnote">Below {failUnder}% of operations used.</div>{/if}
      </div>
      {#each used as op (op.request + op.version)}
        <div class="op">
          <div class="head"><strong>{op.request}</strong> <span class="muted">version {op.version}</span> <span class="muted">used by {op.used.join(', ')}</span></div>
          <div class="statuses">
            {#each op.statuses as s (s.status)}<span class="chip" class:seen={s.seen} title={s.seen ? 'a saved run got this status' : 'no saved run got this status'}>{s.status} {s.seen ? '✓' : '✗'}</span>{/each}
          </div>
          <div class="muted">fields: {op.read.length} of {op.read.length + op.unread.length} read{#if op.unread.length} · unread: {op.unread.slice(0, 6).join(', ')}{op.unread.length > 6 ? ` and ${op.unread.length - 6} more` : ''}{/if}</div>
        </div>
      {/each}
      {#if unused.length}
        <h4>Not used by any flow</h4>
        {#each unused as op (op.request + op.version)}<div class="op unused"><strong>{op.request}</strong> <span class="muted">version {op.version}</span></div>{/each}
      {/if}
    {/if}
  {/if}

  <h3>Spec change: what would a new description break?</h3>
  <p class="note">Choose the changed OpenAPI description (a file or an address). It is compared with the one this collection kept when you imported it, in memory: nothing is written. Each flow, hook and check that uses a request is judged by the response schemas the new description gives, and everything that uses a request the new description dropped is broken. Only what the change breaks is listed.</p>
  <div class="grid">
    <label class="f wide"><span>New description</span><PathField bind:value={newSpec} label="" kind="file" /></label>
    {#if specs.length > 1}
      <label class="f"><span>Compare with the kept description</span>
        <select bind:value={spec}><option value="">choose…</option>{#each specs as s}<option value={s}>{s}</option>{/each}</select>
      </label>
    {/if}
  </div>
  <div class="line">
    <button class="primary" disabled={!collection || !newSpec || changeBusy || (specs.length > 1 && !spec)} onclick={check}>{changeBusy ? 'Checking…' : 'Check'}</button>
  </div>
  {#if !specs.length}<p class="muted">This collection kept no OpenAPI description, so only what the new one has can be judged, never what it dropped.</p>{/if}
  {#if changeError}<p class="err">{changeError}</p>{/if}
  {#if change}
    {#if !change.broken}
      <p class="okline">Nothing breaks if {change.spec} changes as given.</p>
    {:else}
      <p class="badline">{change.breakages.length} consumer{change.breakages.length > 1 ? 's' : ''} would break.</p>
      {#each change.breakages as b, i (i)}
        <div class="op">
          <div class="head"><strong>{b.request}</strong> <span class="muted">version {b.version}</span></div>
          <div>✗ {b.where}</div>
          {#if b.reason}<div class="muted">{b.reason}</div>{/if}
          {#each b.reads as r}<div class="read"><code>{r.expr}</code> <span class="muted">{r.why}</span></div>{/each}
        </div>
      {/each}
    {/if}
  {/if}
</div>

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: 16px 24px 48px; max-width: 860px; }
  h3 { margin: 22px 0 4px; }
  h4 { margin: 14px 0 4px; color: var(--muted); }
  .note, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 6px; }
  .err { color: var(--fail); }
  .f { display: flex; flex-direction: column; gap: 2px; font-size: var(--fs-sm); max-width: 520px; }
  .f span { color: var(--muted); }
  .grid { display: flex; flex-direction: column; gap: 8px; }
  .line { display: flex; align-items: center; gap: 14px; margin: 8px 0; }
  .inline { font-size: var(--fs-sm); color: var(--muted); display: flex; align-items: center; gap: 6px; }
  .num { width: 56px; }
  select, input { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 8px; font: inherit; }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 14px; font: inherit; cursor: pointer; }
  button:disabled { opacity: 0.4; }
  .primary:not(:disabled) { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
  .totals { border: 1px solid var(--line); border-radius: var(--r-lg); padding: 8px 12px; display: flex; flex-direction: column; gap: 2px; margin: 8px 0; font-size: var(--fs-sm); }
  .totals.bad { border-color: var(--fail); }
  .badnote, .badline { color: var(--fail); }
  .okline { color: var(--ok); }
  .op { border-bottom: 1px solid var(--line); padding: 6px 2px; font-size: var(--fs-sm); display: flex; flex-direction: column; gap: 3px; }
  .op.unused { flex-direction: row; gap: 8px; }
  .statuses { display: flex; gap: 6px; flex-wrap: wrap; }
  .chip { border: 1px solid var(--line); border-radius: var(--r-pill); padding: 0 8px; font-size: var(--fs-xs); color: var(--muted); }
  .chip.seen { color: var(--text); border-color: var(--accent); }
  .read { padding-left: 14px; }
  code { font: var(--fs-xs) var(--mono); }
</style>
