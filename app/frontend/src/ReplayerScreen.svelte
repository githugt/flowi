<script lang="ts">
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { ReplayerService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { ReplayerView } from './lib/types'

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
  const tone = (s: number) => (s >= 500 ? 'server' : s >= 400 ? 'client' : s >= 300 ? 'redirect' : 'success')
</script>

{#if view}
  <div class="page">
    <p class="note">Answers calls with the responses saved in your requests' examples, like <code>flowi serve</code>'s replayer. Triggers and schedules are not run here. From a request's screen you can start it and copy the address of that request.</p>
    <div class="grid">
      <label class="f"><span>Listen on (host:port)</span><input bind:value={form.addr} disabled={running} spellcheck="false" /></label>
      <label class="f"><span>Latency (e.g. 250ms)</span><input bind:value={form.latency} disabled={running} spellcheck="false" /></label>
      <label class="f wide"><span>Record from an upstream API (empty: replay)</span><input bind:value={form.record} disabled={running} placeholder="https://api.example.test" spellcheck="false" /></label>
      {#if form.record.trim()}
        <label class="f wide"><span>Save requests of new calls in</span>
          <select bind:value={form.recordTo} disabled={running}>{#each view.targets as t}<option value={t.id}>{t.label}</option>{/each}</select>
        </label>
        <label class="f wide"><span>Also write the calls to a HAR file (in order, credentials masked; Import → HAR turns it into a flow)</span>
          <input bind:value={form.recordHar} disabled={running} placeholder="/path/to/session.har" spellcheck="false" />
        </label>
        <p class="note wide">Point your app or client at the listen address instead of the real API. Each call is forwarded to the upstream API and its answer returned. A call that matches no request yet becomes one (ids in the path become <code>{'{{id}}'}</code>, credentials become <code>{'{{secret.…}}'}</code> placeholders), and every response is saved so the replayer can answer it later.</p>
      {/if}
    </div>
    {#if !form.record.trim()}
      <fieldset class="faults" disabled={running}>
        <legend>Make it misbehave on purpose (to try retries, timeouts and error handling)</legend>
        <label class="f"><span>Answer with an error (% of calls)</span><input bind:value={form.faultErrorPercent} placeholder="0" inputmode="decimal" spellcheck="false" /></label>
        <label class="f"><span>Error status (default 503)</span><input bind:value={form.faultStatus} placeholder="503" inputmode="numeric" spellcheck="false" /></label>
        <label class="f"><span>Drop the connection (% of calls)</span><input bind:value={form.faultDropPercent} placeholder="0" inputmode="decimal" spellcheck="false" /></label>
        <label class="f"><span>Extra random delay up to</span><input bind:value={form.faultJitter} placeholder="e.g. 300ms" spellcheck="false" /></label>
        <p class="note wide">A request can override this for one call with the header <code>X-Flowi-Fault</code>: <code>none</code>, <code>drop</code>, <code>error</code> or <code>error:418</code>. Answers made this way show <code>fault</code> in the Matched column. Faults apply only to calls that match a saved request, and not while recording.</p>
      </fieldset>
    {/if}
    <div class="checks">
      <label><input type="checkbox" bind:checked={form.sequence} disabled={running} /> Answer repeated calls in sequence</label>
      <label><input type="checkbox" bind:checked={form.cors} disabled={running} /> Allow browsers (CORS)</label>
    </div>
    <div class="actions">
      {#if running}
        <button disabled={view.state === 'stopping'} onclick={() => ReplayerService.Stop()}>{view.state === 'stopping' ? 'Stopping…' : 'Stop'}</button>
        <span class="ok">● listening on {view.addr}</span>
      {:else}
        <button class="primary" disabled={view.state === 'starting'} onclick={() => ReplayerService.Start(form)}>{view.state === 'starting' ? 'Starting…' : 'Start'}</button>
      {/if}
      <span class="spacer"></span>
      <button onclick={() => ReplayerService.Clear()}>Clear</button>
    </div>
    {#if view.error}<p class="err">{view.error}</p>{/if}
    {#each view.notes as n}<p class="note">{n}</p>{/each}

    <table>
      <thead><tr><th>Time</th><th>Call</th><th>Status</th><th>Matched</th><th>Took</th></tr></thead>
      <tbody>
        {#each view.traffic as t (t.seq)}
          <tr>
            <td class="muted">{t.at}</td>
            <td><strong>{t.method}</strong> {t.path}</td>
            <td class={tone(t.status)}>{t.status}</td>
            <td class="muted">{t.match}{t.request ? ' · ' + t.request : ''}</td>
            <td class="muted">{t.duration}</td>
          </tr>
        {/each}
      </tbody>
    </table>
    {#if !view.traffic.length}<p class="note">No calls yet.</p>{/if}
  </div>
{/if}

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: 16px 24px 48px; max-width: 980px; display: flex; flex-direction: column; gap: 10px; }
  .note { color: var(--muted); margin: 0; font-size: var(--fs-sm); }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
  .wide { grid-column: 1 / -1; }
  .f { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  select { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: inherit; }
  input:not([type]) { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: var(--fs-sm) var(--mono); -webkit-user-select: text; user-select: text; }
  .faults { border: 1px solid var(--line); border-radius: var(--r-lg); padding: var(--sp-2) var(--sp-3); margin: var(--sp-2) 0; display: grid; grid-template-columns: 1fr 1fr; gap: var(--sp-2) var(--sp-3); }
  .faults legend { color: var(--muted); font-size: var(--fs-sm); padding: 0 6px; }
  .faults .wide { grid-column: 1 / -1; }
  .checks { display: flex; gap: 20px; }
  .actions { display: flex; gap: 10px; align-items: center; }
  .spacer { flex: 1; }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 16px; font: inherit; }
  button:disabled { opacity: 0.4; }
  .primary:not(:disabled) { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
  .ok { color: var(--ok); }
  .err { color: var(--fail); margin: 0; }
  table { border-collapse: collapse; width: 100%; font: var(--fs-sm) var(--mono); }
  th { text-align: left; color: var(--muted); font-weight: 600; padding: 4px 8px; border-bottom: 1px solid var(--line); }
  td { padding: 3px 8px; }
  .muted { color: var(--muted); }
  .success { color: var(--ok); }
  .redirect { color: var(--info); }
  .client { color: var(--warn); }
  .server { color: var(--fail); }
</style>
