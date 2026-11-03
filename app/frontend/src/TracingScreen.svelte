<script lang="ts">
  // Where flowi's traces go: the address of the person's own OpenTelemetry collector. This is their data sent to
  // their own server, so it is kept apart from Usage reporting, which says what is sent to the project.
  import { onMount } from 'svelte'
  import { TracingService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { live } from './lib/live'
  import Button from './Button.svelte'
  import Toggle from './Toggle.svelte'
  import TextInput from './TextInput.svelte'
  import { agoText } from './lib/tracing'

  interface TracingView {
    endpoint: string
    headerNames: string[]
    propagate: boolean
    problem?: string
    running: boolean
    sent: number
    failed: number
    dropped: number
    lastError?: string
    lastExport?: string
    test: { state: string; message: string }
  }
  let view = $state<TracingView | null>(null)
  // The form is the person's draft: it is filled once from what is stored, and Go's later changes (the export
  // counts) must not overwrite what they are typing.
  let endpoint = $state('')
  let headers = $state('')
  let propagate = $state(false)
  let filled = false
  let now = $state(Date.now())

  function follow(v: TracingView) {
    view = v
    if (filled) return
    filled = true
    endpoint = v.endpoint
    propagate = v.propagate
  }
  onMount(() => {
    const off = live<TracingView>('tracing', () => TracingService.State(), follow)
    // The counts change while the page is open, without a settings change to announce them.
    const tick = setInterval(() => {
      now = Date.now()
      TracingService.State().then((v) => view && (view = { ...(v as TracingView) }))
    }, 3000)
    return () => (off(), clearInterval(tick))
  })

  const lines = $derived(headers.split('\n').map((l) => l.trim()).filter(Boolean))
  const changed = $derived(!!view && (endpoint.trim() !== view.endpoint || propagate !== view.propagate || lines.length > 0))
  const save = () => TracingService.Save(endpoint, lines, propagate).then(() => (headers = ''))
</script>

{#if view}
  <div class="page">
    <h2>Tracing</h2>
    <p class="expl">
      Send a trace of every send and flow run to your own OpenTelemetry collector, such as Jaeger, Grafana Tempo or Honeycomb, to see how long each step took and where runs fail. Nothing is sent until you enter an address.
      This goes to your server only; it is separate from Usage reporting.
    </p>

    <div class="state" class:on={view.running}>
      <span class="dot" aria-hidden="true"></span>
      <strong>{view.running ? 'Sending traces' : 'Off'}</strong>
      {#if view.running}
        <span class="muted">{view.sent} sent{view.failed ? ` · ${view.failed} failed` : ''}{view.dropped ? ` · ${view.dropped} dropped` : ''}{view.lastExport ? ` · last ${agoText(view.lastExport, now)}` : ' · none yet; run something'}</span>
      {:else}
        <span class="muted">Enter a collector address below to switch it on.</span>
      {/if}
    </div>
    {#if view.running && view.lastError}<p class="err">The last export failed: {view.lastError}</p>{/if}
    {#if view.problem}<p class="err">{view.problem}</p>{/if}

    <div class="form">
      <label class="row">
        <span class="name">Collector address</span>
        <TextInput bind:value={endpoint} mono placeholder="http://localhost:4318" label="Collector address" />
        <span class="hint">An OTLP/HTTP address. Leave it empty to keep tracing off.</span>
      </label>

      <div class="row">
        <span class="name">Headers</span>
        {#if view.headerNames.length}
          <div class="have">
            {#each view.headerNames as n}<span class="chip">{n} <i>••••</i></span>{/each}
            <Button small variant="ghost" onclick={() => TracingService.ClearHeaders()}>Remove</Button>
          </div>
        {/if}
        <textarea bind:value={headers} rows="2" spellcheck="false" aria-label="New headers" placeholder={view.headerNames.length ? 'Leave empty to keep the headers above, or enter new ones to replace them' : 'Authorization=Bearer …   (one per line)'}></textarea>
        <span class="hint">For a collector that needs a token. Values are stored, never shown again.</span>
      </div>

      <div class="row">
        <Toggle checked={propagate} onchange={(v) => (propagate = v)}>Let the APIs I call join the trace</Toggle>
        <span class="hint">Adds a <code>traceparent</code> header to outgoing requests, so a service that supports it continues the same trace.</span>
      </div>

      <div class="actions">
        <Button variant="primary" disabled={!changed} onclick={save}>Save</Button>
        <Button disabled={!view.endpoint || view.test.state === 'testing'} title={view.endpoint ? '' : 'Save an address first'} onclick={() => TracingService.Test()}>Send test trace</Button>
        {#if view.test.state === 'testing'}
          <span class="muted">Sending…</span>
        {:else if view.test.state === 'ok'}
          <span class="ok">✓ {view.test.message}</span>
        {:else if view.test.state === 'failed'}
          <span class="err">{view.test.message}</span>
        {/if}
      </div>
    </div>

    <h4>What a trace holds</h4>
    <ul class="holds">
      <li>One trace per run, with a span for each step and hook, and its timing and result.</li>
      <li>For each request: the method, the address, the status and how long it took.</li>
      <li>Never request or response bodies, headers or secret values.</li>
    </ul>
  </div>
{/if}

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: 28px 32px 56px; max-width: 760px; }
  h2 { margin: 0 0 8px; }
  h4 { margin: 24px 0 6px; }
  .expl { line-height: 1.5; }
  .muted { color: var(--muted); }
  .state { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin: 14px 0 6px; padding: 10px 14px; border: 1px solid var(--line); border-radius: var(--r-lg); }
  .state .dot { width: 8px; height: 8px; border-radius: 50%; background: var(--muted); }
  .state.on { border-color: color-mix(in srgb, var(--ok) 45%, var(--line)); }
  .state.on .dot { background: var(--ok); box-shadow: 0 0 0 4px var(--ok-t); }
  .err { color: var(--fail); margin: 6px 0; }
  .ok { color: var(--ok); }
  .form { display: flex; flex-direction: column; gap: 18px; margin-top: 16px; }
  .row { display: flex; flex-direction: column; gap: 6px; }
  .name { font-weight: 600; }
  .hint { color: var(--muted); font-size: var(--fs-sm); }
  .have { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; }
  .chip { display: inline-flex; gap: 6px; padding: 2px 10px; border: 1px solid var(--line); border-radius: var(--r-pill); font: var(--fs-sm) var(--mono, ui-monospace, monospace); }
  .chip i { font-style: normal; color: var(--muted); }
  textarea { box-sizing: border-box; width: 100%; padding: 6px 8px; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); font: var(--fs-sm) var(--mono, ui-monospace, monospace); resize: vertical; -webkit-user-select: text; user-select: text; }
  textarea:focus { border-color: var(--accent); }
  textarea::placeholder { color: var(--muted); }
  .actions { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; }
  .holds { margin: 0; padding-left: 20px; line-height: 1.6; color: var(--muted); }
  code { font: var(--fs-sm) var(--mono, ui-monospace, monospace); }
</style>
