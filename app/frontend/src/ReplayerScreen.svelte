<script lang="ts">
  import RadioGroup from './RadioGroup.svelte'
  import Select from './Select.svelte'
  import Status from './Status.svelte'
  import Field from './Field.svelte'
  import { replayerDraft } from './lib/replayerdraft.svelte'
  import CliLine from './CliLine.svelte'
  import { serveCommand } from './lib/cliline'
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
  import { recordedHeadline, recordedList } from './lib/recorded'
  import type { ForwardPreview } from './lib/types'
  import Tabs from './Tabs.svelte'
  import { copy } from './lib/clip'
  import { faultCount, faultsGate, headline, serveUrl } from './lib/replayerhead'

  let view = $state<ReplayerView | null>(null)
  let form = $state<ReplayerView['form']>({ addr: '', latency: '', sequence: false, cors: false, forward: '', forwardTo: '', forwardOnly: '', env: '', recordTo: '', recordFolder: '', recordHar: '', faultErrorPercent: '', faultStatus: '', faultDropPercent: '', faultJitter: '' })
  let seeded = false
  // Every change is kept for the replay bar and for the next time this tab is shown.
  $effect(() => {
    if (seeded) replayerDraft.form = { ...form }
  })

  // The form starts from the defaults Go read; after that it is the user's.
  function adopt(v: ReplayerView) {
    view = v
    if (!seeded || v.state !== 'stopped') {
      // A stopped replayer shows what was typed last (here or in the bar); a running one shows what it started with.
      form = v.state === 'stopped' && replayerDraft.form ? { ...replayerDraft.form } : { ...v.form }
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
  const head = $derived(headline(view?.state ?? 'stopped', view?.addr ?? ''))
  const gate = $derived(faultsGate(form))
  let tab = $state('mode')
  const tabs = $derived([
    { id: 'mode', label: 'Mode' },
    { id: 'faults', label: 'Faults', count: faultCount(form) || undefined },
    { id: 'record', label: 'Record' },
  ])
</script>

{#if !view}
  <div class="page">
    {#if loadError}
      <p class="note">The replayer could not be loaded: {loadError}</p>
      <CliLine command={serveCommand(form)} />
    <div class="actions"><Button onclick={load}>Try again</Button></div>
    {:else}
      <p class="note">Loading the replayer…</p>
    {/if}
  </div>
{:else}
  <div class="page" class:running>
    <!-- The state and the two things done to it stay in view while the form below scrolls. -->
    <header class="bar">
      <span class="state" role="status"><Status plain tone={head.tone === 'ok' ? 'ok' : head.tone === 'busy' ? 'warn' : ''}>{head.text}</Status></span>
      {#if running}<Button small title="Copy the address a client calls" onclick={() => copy(serveUrl(view!.addr), 'the replayer address')}>Copy address</Button>{/if}
      <span class="spacer"></span>
      <Button onclick={() => ReplayerService.Clear()}>Clear</Button>
      {#if running}
        <Button variant="danger" disabled={view.state === 'stopping'} onclick={() => ReplayerService.Stop()}>{view.state === 'stopping' ? 'Stopping…' : 'Stop'}</Button>
      {:else}
        <Button variant="go" disabled={view.state === 'starting'} onclick={() => ReplayerService.Start(form)}>{view.state === 'starting' ? 'Starting…' : 'Start'}</Button>
      {/if}
    </header>
    {#if view.error}<p class="err">{view.error}</p>{/if}
    {#each view.notes as n}<p class="note">{n}</p>{/each}
    <div class="cols">
      <div class="formcol">
        <p class="note">Answers calls with the responses saved in your requests' examples, like <code>flowi serve</code>'s replayer. Triggers and schedules are not run here. From a request's screen you can start it and copy the address of that request.</p>
        <Tabs label="Replayer settings" items={tabs} active={tab} onpick={(id) => (tab = id)} />
        {#if tab === 'mode'}
          <div class="grid">
            <Field label="Listen on (host:port)"><TextInput mono bind:value={form.addr} disabled={running} /></Field>
            <Field label="Latency (e.g. 250ms)"><TextInput mono bind:value={form.latency} disabled={running} /></Field>
            <div class="wide"><RadioGroup legend="Calls no saved response can answer" options={modes} bind:value={form.forward} row boxed disabled={running} /></div>
          </div>
          <div class="checks">
            <Toggle checked={form.sequence} disabled={running} onchange={(on) => (form.sequence = on)} title="When one request has several saved responses, answer the first call with the first, the second call with the second, and so on, instead of always the same one.">Answer repeated calls with the saved responses in turn</Toggle>
            <Toggle checked={form.cors} disabled={running} onchange={(on) => (form.cors = on)}>Allow browsers (CORS)</Toggle>
          </div>
        {:else if tab === 'faults'}
          {#if !gate.enabled}<p class="note reason" role="note">{gate.reason}</p>{/if}
          <fieldset class="faults" disabled={running || !gate.enabled}>
            <legend>Make it misbehave on purpose (to try retries, timeouts and error handling)</legend>
            <Field label="Answer with an error (% of calls)"><TextInput mono bind:value={form.faultErrorPercent} placeholder="0" inputmode="decimal" /></Field>
            <Field label="Error status (default 503)"><TextInput mono bind:value={form.faultStatus} placeholder="503" inputmode="numeric" /></Field>
            <Field label="Drop the connection (% of calls)"><TextInput mono bind:value={form.faultDropPercent} placeholder="0" inputmode="decimal" /></Field>
            <Field label="Extra random delay up to"><TextInput mono bind:value={form.faultJitter} placeholder="e.g. 300ms" /></Field>
            <p class="note wide">A request can override this for one call with the header <code>X-Flowi-Fault</code>: <code>none</code>, <code>drop</code>, <code>error</code> or <code>error:418</code>. Answers made this way show <code>fault</code> in the Matched column. Faults apply only to calls that match a saved request, and not while forwarding.</p>
          </fieldset>
        {:else}
          <div class="grid">
            <details class="wide where" class:off={!form.forward} open>
              <summary>Limit or redirect where calls go (optional){#if !form.forward} — pass through to use these{/if}</summary>
              <fieldset class="inner" disabled={running || !form.forward}>
                <div class="grid">
                  <Field label="Send everything to this one API (leave empty to use each request's own API)" wide><TextInput mono bind:value={form.forwardTo} disabled={running || !form.forward} placeholder="https://api.example.test" /></Field>
                  <Field label="Only forward to these APIs (comma separated base URLs; leave empty to allow any)" wide><TextInput mono bind:value={form.forwardOnly} disabled={running || !form.forward || !!form.forwardTo.trim()} placeholder="https://api.example.test, https://other.example.test" /></Field>
                  <Field label="Save requests of new calls in (pass through and save, with one API set above)" wide>
                    <Select bind:value={form.recordTo} disabled={running || form.forward !== 'save'}>{#each view.targets as t}<option value={t.id}>{t.label}</option>{/each}</Select>
                  </Field>
                  <Field label="Folder in that collection (default: recorded; a path like api/recorded)" wide>
                    <TextInput mono bind:value={form.recordFolder} disabled={running || form.forward !== 'save'} placeholder="recorded" />
                  </Field>
                </div>
              </fieldset>
            </details>
            <Field label="Also write every call the replayer handles to a HAR file (in order, credentials masked; Import → HAR turns it into a flow)" wide>
              <TextInput mono bind:value={form.recordHar} disabled={running} placeholder="/path/to/session.har (optional)" />
            </Field>
          </div>
        {/if}
        <div class="preview" aria-label="What the replayer will do">
          <strong>What it will do</strong>
          <ul>{#each preview as l}<li class={l.tone}>{l.text}</li>{/each}</ul>
        </div>
      </div>
      <div class="logcol">
        {#if view.recorded.count > 0}
          {@const list = recordedList(view.recorded)}
          <div class="recorded" role="status">
            <strong>{recordedHeadline(view.recorded)}</strong>
            <ul>{#each list.shown as r (r)}<li><code>{r}</code></li>{/each}{#if list.more}<li class="note">and {list.more} more</li>{/if}</ul>
            <p class="note">They are saved in the collection's requests folder. Find them in the sidebar under that folder.</p>
          </div>
        {/if}
        <TrafficLog rows={view.traffic} addr={view.addr} />
      </div>
    </div>
  </div>
{/if}

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: 0 32px 56px; display: flex; flex-direction: column; gap: 10px; }
  .page:not(.running) { max-width: 980px; }
  .bar { position: sticky; top: 0; display: flex; align-items: center; gap: 10px; padding: 14px 0 10px; background: var(--bg); border-bottom: 1px solid var(--line); }
  .state { display: inline-flex; align-items: center; gap: 8px; font-weight: 600; }
  .cols { display: flex; flex-direction: column; gap: 16px; }
  /* While it runs, the calls it answers are beside the settings, not below them. */
  .running .cols { flex-direction: row; align-items: flex-start; }
  .running .formcol { flex: none; width: 440px; }
  .running .logcol { flex: 1; min-width: 0; }
  .formcol, .logcol { display: flex; flex-direction: column; gap: 10px; }
  .reason { color: var(--warn); }
  .recorded { border: 1px solid var(--line); border-radius: var(--r-md); background: var(--sunk); padding: var(--sp-3); display: flex; flex-direction: column; gap: var(--sp-2); }
  .recorded ul { margin: 0; padding-left: 18px; font: var(--fs-sm) var(--mono); }
  .note { color: var(--muted); margin: 0; font-size: var(--fs-sm); }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
  .wide { grid-column: 1 / -1; }
  .faults { border: 1px solid var(--line); border-radius: var(--r-lg); padding: var(--sp-2) var(--sp-3); margin: var(--sp-2) 0; display: grid; grid-template-columns: 1fr 1fr; gap: var(--sp-2) var(--sp-3); }
  .faults legend { color: var(--muted); font-size: var(--fs-sm); padding: 0 6px; }
  .faults .wide { grid-column: 1 / -1; }
  .where.off { opacity: 0.6; }
  .inner { border: 0; padding: 0; margin: 0; min-width: 0; }
  .where summary { cursor: pointer; color: var(--muted); font-size: var(--fs-sm); margin-bottom: 8px; }
  .preview { border: 1px solid var(--line); border-radius: var(--r-lg); padding: var(--sp-2) var(--sp-3); font-size: var(--fs-sm); }
  .preview ul { margin: 4px 0 0; padding-left: 18px; display: flex; flex-direction: column; gap: 3px; }
  .preview .ok { color: var(--ok); }
  .preview .warn { color: var(--muted); }
  .preview .bad { color: var(--fail); }
  .checks { display: flex; gap: 20px; flex-wrap: wrap; }
  .spacer { flex: 1; }
  .err { color: var(--fail); margin: 0; }
</style>
