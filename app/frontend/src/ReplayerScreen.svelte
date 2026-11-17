<script lang="ts">
  import TextInput from './TextInput.svelte'
  import Toggle from './Toggle.svelte'
  import Button from './Button.svelte'
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { ReplayerService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { ReplayerView } from './lib/types'
  import { errorText } from './lib/errors'
  import TrafficLog from './TrafficLog.svelte'
  import { forwardPreview } from './lib/replayer'
  import type { ForwardPreview } from './lib/types'

  let view = $state<ReplayerView | null>(null)
  let form = $state<ReplayerView['form']>({ addr: '', latency: '', sequence: false, cors: false, forward: '', forwardTo: '', forwardOnly: '', env: '', recordTo: '', recordHar: '', faultErrorPercent: '', faultStatus: '', faultDropPercent: '', faultJitter: '' })
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

  // Before this, a state that never came (or failed) left the tab blank, with nothing to say why or to try again.
  let loadError = $state('')
  let stop = () => {}
  function load() {
    loadError = ''
    stop()
    stop = live<ReplayerView>('replayer', () => ReplayerService.State().catch((e) => { loadError = errorText(e); throw e }), adopt)
  }
  onMount(() => {
    load()
    return () => stop()
  })

  // Where each API of the saved requests is, for the preview; looked up again when the saved requests may have changed.
  let targets = $state<ForwardPreview | null>(null)
  $effect(() => {
    if (view && !running) ReplayerService.Preview().then((p) => (targets = p)).catch(() => (targets = null))
  })
  const preview = $derived(forwardPreview(form, targets, (id) => view?.targets.find((t) => t.id === id)?.label ?? ''))
  const modes = [
    { value: '', label: 'Replay only', hint: 'Saved responses only; calls with no saved answer are not found.' },
    { value: 'pass', label: 'Pass through', hint: 'Calls with no saved answer go to the real API; nothing is saved.' },
    { value: 'save', label: 'Pass through and save', hint: 'As pass through, and the answers are saved so the next call is replayed.' }
  ] as const

  const running = $derived(view?.state === 'running' || view?.state === 'stopping')
</script>

{#if !view}
  <div class="page">
    {#if loadError}
      <p class="note">The replayer could not be loaded: {loadError}</p>
      <div class="actions"><Button onclick={load}>Try again</Button></div>
    {:else}
      <p class="note">Loading the replayer…</p>
    {/if}
  </div>
{:else}
  <div class="page">
    <p class="note">Answers calls with the responses saved in your requests' examples, like <code>flowi serve</code>'s replayer. Triggers and schedules are not run here. From a request's screen you can start it and copy the address of that request.</p>
    <div class="grid">
      <label class="f"><span>Listen on (host:port)</span><TextInput mono bind:value={form.addr} disabled={running} /></label>
      <label class="f"><span>Latency (e.g. 250ms)</span><TextInput mono bind:value={form.latency} disabled={running} /></label>
      <fieldset class="modes wide" disabled={running}>
        <legend>Calls no saved response can answer</legend>
        {#each modes as m}
          <label class="mode" title={m.hint}><input type="radio" name="forward" value={m.value} bind:group={form.forward} />{m.label}</label>
        {/each}
      </fieldset>
      {#if form.forward}
        <details class="wide where">
          <summary>Limit or redirect where calls go (optional)</summary>
          <div class="grid">
            <label class="f wide"><span>Send everything to this one API (leave empty to use each request's own API)</span><TextInput mono bind:value={form.forwardTo} disabled={running} placeholder="https://api.example.test" /></label>
            <label class="f wide"><span>Only forward to these APIs (comma separated base URLs; leave empty to allow any)</span><TextInput mono bind:value={form.forwardOnly} disabled={running || !!form.forwardTo.trim()} placeholder="https://api.example.test, https://other.example.test" /></label>
            {#if form.forward === 'save'}
              <label class="f wide"><span>Save requests of new calls in (when one API is set above)</span>
                <select bind:value={form.recordTo} disabled={running}>{#each view.targets as t}<option value={t.id}>{t.label}</option>{/each}</select>
              </label>
            {/if}
            <label class="f wide"><span>Also write the forwarded calls to a HAR file (in order, credentials masked; Import → HAR turns it into a flow)</span>
              <TextInput mono bind:value={form.recordHar} disabled={running} placeholder="/path/to/session.har" />
            </label>
          </div>
        </details>
      {/if}
      <div class="preview wide" aria-label="What the replayer will do">
        <strong>What it will do</strong>
        <ul>{#each preview as l}<li class={l.tone}>{l.text}</li>{/each}</ul>
      </div>
    </div>
    {#if !form.forward}
      <fieldset class="faults" disabled={running}>
        <legend>Make it misbehave on purpose (to try retries, timeouts and error handling)</legend>
        <label class="f"><span>Answer with an error (% of calls)</span><TextInput mono bind:value={form.faultErrorPercent} placeholder="0" inputmode="decimal" /></label>
        <label class="f"><span>Error status (default 503)</span><TextInput mono bind:value={form.faultStatus} placeholder="503" inputmode="numeric" /></label>
        <label class="f"><span>Drop the connection (% of calls)</span><TextInput mono bind:value={form.faultDropPercent} placeholder="0" inputmode="decimal" /></label>
        <label class="f"><span>Extra random delay up to</span><TextInput mono bind:value={form.faultJitter} placeholder="e.g. 300ms" /></label>
        <p class="note wide">A request can override this for one call with the header <code>X-Flowi-Fault</code>: <code>none</code>, <code>drop</code>, <code>error</code> or <code>error:418</code>. Answers made this way show <code>fault</code> in the Matched column. Faults apply only to calls that match a saved request, and not while forwarding.</p>
      </fieldset>
    {/if}
    <div class="checks">
      <Toggle checked={form.sequence} disabled={running} onchange={(on) => (form.sequence = on)} title="When one request has several saved responses, answer the first call with the first, the second call with the second, and so on, instead of always the same one.">Answer repeated calls with the saved responses in turn</Toggle>
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
  .faults { border: 1px solid var(--line); border-radius: var(--r-lg); padding: var(--sp-2) var(--sp-3); margin: var(--sp-2) 0; display: grid; grid-template-columns: 1fr 1fr; gap: var(--sp-2) var(--sp-3); }
  .faults legend { color: var(--muted); font-size: var(--fs-sm); padding: 0 6px; }
  .faults .wide { grid-column: 1 / -1; }
  .modes { border: 1px solid var(--line); border-radius: var(--r-lg); padding: var(--sp-2) var(--sp-3); display: flex; gap: 20px; flex-wrap: wrap; }
  .modes legend { color: var(--muted); font-size: var(--fs-sm); padding: 0 6px; }
  .mode { display: flex; gap: 6px; align-items: center; }
  .where summary { cursor: pointer; color: var(--muted); font-size: var(--fs-sm); margin-bottom: 8px; }
  .preview { border: 1px solid var(--line); border-radius: var(--r-lg); padding: var(--sp-2) var(--sp-3); font-size: var(--fs-sm); }
  .preview ul { margin: 4px 0 0; padding-left: 18px; display: flex; flex-direction: column; gap: 3px; }
  .preview .ok { color: var(--ok); }
  .preview .warn { color: var(--muted); }
  .preview .bad { color: var(--fail); }
  .checks { display: flex; gap: 20px; }
  .actions { display: flex; gap: 10px; align-items: center; }
  .spacer { flex: 1; }
  .ok { color: var(--ok); }
  .err { color: var(--fail); margin: 0; }
</style>
