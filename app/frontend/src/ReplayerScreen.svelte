<script lang="ts">
  import TextInput from './TextInput.svelte'
  import Toggle from './Toggle.svelte'
  import Button from './Button.svelte'
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { ReplayerService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { ReplayerView } from './lib/types'
  import TrafficLog from './TrafficLog.svelte'

  let view = $state<ReplayerView | null>(null)
  let form = $state({ addr: '', latency: '', sequence: false, cors: false, record: '', recordTo: '', recordHar: '', faultErrorPercent: '', faultStatus: '', faultDropPercent: '', faultJitter: '' })
  let seeded = false

  // The form starts from the defaults Go read; after that it is the user's.
  function adopt(v: ReplayerView) {
    view = v
    if (!seeded || v.state !== 'stopped') {
      form = { ...v.form }
      seeded = v.state === 'stopped' ? seeded : true
    }
    if (!seeded && v.form.addr) seeded = true
  }

  onMount(() => live<ReplayerView>('replayer', () => ReplayerService.State(), adopt))

  const running = $derived(view?.state === 'running' || view?.state === 'stopping')
</script>

{#if view}
  <div class="page">
    <p class="note">Answers calls with the responses saved in your requests' examples, like <code>flowi serve</code>'s replayer. Triggers and schedules are not run here. From a request's screen you can start it and copy the address of that request.</p>
    <div class="grid">
      <label class="f"><span>Listen on (host:port)</span><TextInput mono bind:value={form.addr} disabled={running} /></label>
      <label class="f"><span>Latency (e.g. 250ms)</span><TextInput mono bind:value={form.latency} disabled={running} /></label>
      <label class="f wide"><span>Record from an upstream API (empty: replay)</span><TextInput mono bind:value={form.record} disabled={running} placeholder="https://api.example.test" /></label>
      {#if form.record.trim()}
        <label class="f wide"><span>Save requests of new calls in</span>
          <select bind:value={form.recordTo} disabled={running}>{#each view.targets as t}<option value={t.id}>{t.label}</option>{/each}</select>
        </label>
        <label class="f wide"><span>Also write the calls to a HAR file (in order, credentials masked; Import → HAR turns it into a flow)</span>
          <TextInput mono bind:value={form.recordHar} disabled={running} placeholder="/path/to/session.har" />
        </label>
        <p class="note wide">Point your app or client at the listen address instead of the real API. Each call is forwarded to the upstream API and its answer returned. A call that matches no request yet becomes one (ids in the path become <code>{'{{id}}'}</code>, credentials become <code>{'{{secret.…}}'}</code> placeholders), and every response is saved so the replayer can answer it later.</p>
      {/if}
    </div>
    {#if !form.record.trim()}
      <fieldset class="faults" disabled={running}>
        <legend>Make it misbehave on purpose (to try retries, timeouts and error handling)</legend>
        <label class="f"><span>Answer with an error (% of calls)</span><TextInput mono bind:value={form.faultErrorPercent} placeholder="0" inputmode="decimal" /></label>
        <label class="f"><span>Error status (default 503)</span><TextInput mono bind:value={form.faultStatus} placeholder="503" inputmode="numeric" /></label>
        <label class="f"><span>Drop the connection (% of calls)</span><TextInput mono bind:value={form.faultDropPercent} placeholder="0" inputmode="decimal" /></label>
        <label class="f"><span>Extra random delay up to</span><TextInput mono bind:value={form.faultJitter} placeholder="e.g. 300ms" /></label>
        <p class="note wide">A request can override this for one call with the header <code>X-Flowi-Fault</code>: <code>none</code>, <code>drop</code>, <code>error</code> or <code>error:418</code>. Answers made this way show <code>fault</code> in the Matched column. Faults apply only to calls that match a saved request, and not while recording.</p>
      </fieldset>
    {/if}
    <div class="checks">
      <Toggle checked={form.sequence} disabled={running} onchange={(on) => (form.sequence = on)}>Answer repeated calls in sequence</Toggle>
      <Toggle checked={form.cors} disabled={running} onchange={(on) => (form.cors = on)}>Allow browsers (CORS)</Toggle>
    </div>
    <div class="actions">
      {#if running}
        <Button variant="danger" disabled={view.state === 'stopping'} onclick={() => ReplayerService.Stop()}>{view.state === 'stopping' ? 'Stopping…' : 'Stop'}</Button>
        <span class="ok">● listening on {view.addr}</span>
      {:else}
        <Button variant="go" disabled={view.state === 'starting'} onclick={() => ReplayerService.Start(form)}>{view.state === 'starting' ? 'Starting…' : 'Start'}</Button>
      {/if}
      <span class="spacer"></span>
      <Button onclick={() => ReplayerService.Clear()}>Clear</Button>
    </div>
    {#if view.error}<p class="err">{view.error}</p>{/if}
    {#each view.notes as n}<p class="note">{n}</p>{/each}

    <TrafficLog rows={view.traffic} />
  </div>
{/if}

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: 28px 32px 56px; max-width: 980px; display: flex; flex-direction: column; gap: 10px; }
  .note { color: var(--muted); margin: 0; font-size: var(--fs-sm); }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
  .wide { grid-column: 1 / -1; }
  .f { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  select { background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: inherit; }
  .faults { border: 1px solid var(--line); border-radius: var(--r-lg); padding: var(--sp-2) var(--sp-3); margin: var(--sp-2) 0; display: grid; grid-template-columns: 1fr 1fr; gap: var(--sp-2) var(--sp-3); }
  .faults legend { color: var(--muted); font-size: var(--fs-sm); padding: 0 6px; }
  .faults .wide { grid-column: 1 / -1; }
  .checks { display: flex; gap: 20px; }
  .actions { display: flex; gap: 10px; align-items: center; }
  .spacer { flex: 1; }
  .ok { color: var(--ok); }
  .err { color: var(--fail); margin: 0; }
</style>
