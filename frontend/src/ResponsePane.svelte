<script lang="ts">
  import { errorText } from './lib/errors'
  import { RequestService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import type { RequestView } from './lib/types'
  import Parts from './Parts.svelte'
  import Problem from './Problem.svelte'
  import EmptyState from './EmptyState.svelte'
  import JsonNode from './JsonNode.svelte'
  import Code from './Code.svelte'
  import { Clipboard } from '@wailsio/runtime'
  import { ui } from './lib/ui.svelte'
  import { app } from './lib/app.svelte'

  let { view }: { view: RequestView } = $props()
  let tab = $state<'received' | 'sent' | 'timing' | 'messages' | 'pipeline'>('received')
  let format = $state('tree')
  const r = $derived(view.response)
  // Go ends the pipeline with the engine's own steps under a "Built in" heading (schema learning):
  // they run with every send, hooks or not, so they are drawn apart from the hooks.
  const builtInAt = $derived(view.pipeline.findIndex((p, i) => p.kind === 'section' && view.pipeline[i + 1]?.kind === 'builtin'))
  const ownRows = $derived(builtInAt < 0 ? view.pipeline : view.pipeline.slice(0, builtInAt))
  const builtInRows = $derived(builtInAt < 0 ? [] : view.pipeline.slice(builtInAt))
  // The Hooks tab's dot: how the last send's hooks went.
  const pipeTone = $derived.by(() => {
    const steps = ownRows.filter((p) => p.kind !== 'section')
    if (steps.some((p) => p.state === 'failed')) return 'bad'
    if (steps.some((p) => p.state === 'running')) return 'run'
    return steps.some((p) => p.state === 'passed') ? 'ok' : ''
  })
  // The hook steps (the request itself is not one) and how they went.
  const hookSteps = $derived(view.pipeline.filter((p) => p.kind === 'node'))
  const hookPassed = $derived(hookSteps.filter((p) => p.state === 'passed').length)
  const hookFailed = $derived(hookSteps.filter((p) => p.state === 'failed').length)
  const hookWaiting = $derived(hookSteps.filter((p) => !['passed', 'failed', 'running'].includes(p.state)).length)
  let query = $state('')
  let picked = $state('')
  const tree = $derived(format === 'tree' && r?.bodyJson !== undefined && r?.bodyJson !== null)
  // What the Response tab lists. The tree shows the body itself, and the Timing tab the timings, so neither
  // is listed again here.
  const received = $derived(
    (r?.received ?? []).filter((p) => !(tree && p.kind === 'code') && !(r?.phases.length && p.heading === 'Timings')),
  )
  const emptyHint: Record<string, string> = {
    received: 'Send the request to see its response.',
    sent: 'Send the request to see exactly what went out.',
    timing: 'Send the request to see where the time went.',
    messages: 'Send to see the messages sent and received.',
  }

  function copyPath() {
    Clipboard.SetText(picked)
    ui.toast('Copied ' + picked)
  }

  function extract() {
    ui.prompt({
      title: 'Extract as variable',
      label: `Variable name for ${picked}`,
      initial: '',
      done: async (name) => {
        try {
          await RequestService.ExtractAsVariable(view.node, name, picked)
          ui.toast(`${name} is extracted from ${picked}; the request is saved.`)
        } catch (e) {
          ui.toast(errorText(e), 'error')
        }
      },
    })
  }

  async function saveBody() {
    try {
      const file = (await RequestService.SaveBody(view.node)) as string
      if (file) ui.toast('Saved: ' + file)
    } catch (e) {
      ui.toast(errorText(e), 'error')
    }
  }

  async function pin() {
    ui.prompt({
      title: 'Pin as example',
      label: 'Name (empty: the status code)',
      initial: '',
      warning: (await RequestService.PinWarning(view.node)) as string,
      done: async (name) => {
        try {
          ui.toast('Pinned: ' + (await RequestService.PinExample(view.node, name)))
        } catch (e) {
          ui.toast(errorText(e), 'error')
        }
      },
    })
  }
</script>

{#snippet timeline(rows: typeof view.pipeline)}
  <ol class="timeline">
    {#each rows as row}
      {#if row.kind === 'section'}
        <li class="sec"><h5>{row.title}</h5></li>
      {:else}
        <li class="tl {row.state}" class:api={row.kind === 'api'} class:builtin={row.kind === 'builtin'}>
          <span class="tick" aria-label={row.state}>{#if row.state === 'passed'}✓{:else if row.state === 'failed'}✗{:else if row.state === 'running'}…{:else}○{/if}</span>
          <div class="what">
            <span class="st">{row.title}</span>{#if row.kind === 'builtin'}<span class="tag">built in</span>{/if}
            <span class="muted">{row.kind === 'api' ? (row.method ?? '') + ' ' + (row.tip ?? '') : row.detail}</span>
            {#if row.reason}<div class="err" class:off={row.state === 'not-run'}>{row.reason}</div>{/if}
            {#if row.kind === 'builtin'}
              <!-- The version's own schemas.infer: saved with the request, like its other settings. -->
              <button class="mini switch" onclick={() => RequestService.SetSchemasInfer(view.node, row.state === 'not-run')}>
                {row.state === 'not-run' ? 'Turn on for this request' : 'Turn off for this request'}
              </button>
            {/if}
          </div>
          {#if row.ms}<span class="pms">{row.ms.toFixed(0)} ms</span>{/if}
        </li>
      {/if}
    {/each}
  </ol>
{/snippet}

<section>
  <div class="bar">
    <div class="tabs">
      <button class:active={tab === 'received'} onclick={() => (tab = 'received')}>Response{#if r}<span class="status-pill" class:ok={r.passed} class:bad={!r.passed}>{r.code}</span>{:else if view.sending}<b class="tone">…</b>{/if}</button>
      <!-- Every tab is there before a send, as httpie has it: each says "Not sent" until there is something to show. -->
      <button class:active={tab === 'sent'} onclick={() => (tab = 'sent')}>Request sent</button>
      {#if view.kind === 'async' || r?.messages.length}<button class:active={tab === 'messages'} onclick={() => (tab = 'messages')}>Messages{#if r?.messages.length}<span class="count">{r.messages.length}</span>{/if}</button>{/if}
      {#if view.kind === 'http'}<button class:active={tab === 'timing'} onclick={() => (tab = 'timing')}>Timing</button>{/if}
      <button class:active={tab === 'pipeline'} onclick={() => (tab = 'pipeline')}>Hooks{#if pipeTone}<b class="tone {pipeTone}">●</b>{/if}</button>
    </div>
  </div>
  {#if tab === 'pipeline'}
    <div class="body col">
      {#if hookSteps.length}
        <p class="sum">
          {hookSteps.length} {hookSteps.length === 1 ? 'hook' : 'hooks'}
          {#if hookPassed}<span class="okc">✓ {hookPassed}</span>{/if}
          {#if hookFailed}<span class="badc">✗ {hookFailed}</span>{/if}
          {#if hookWaiting}<span class="muted">○ {hookWaiting} not run</span>{/if}
        </p>
      {/if}
      {#if hookSteps.length}
        <!-- One run, in the order the engine takes it: the built-in steps come after the post-hooks. -->
        {@render timeline([...ownRows, ...builtInRows])}
        <button class="mini edit" onclick={() => app.openHooks(view.node, view.name)}>Edit this request's hooks…</button>
      {:else}
        <EmptyState icon="hooks" title="No hooks" hint="Hooks run steps before this request and after its response." compact>
          <button onclick={() => app.openHooks(view.node, view.name)}>Add hooks…</button>
        </EmptyState>
        {#if builtInRows.length}<div class="builtins">{@render timeline(builtInRows)}</div>{/if}
      {/if}
    </div>
  {:else if view.sending}
    <p class="muted">Sending…</p>
    {#each view.live as m}
      <div class="msg {m.direction}"><div class="mh">{m.direction === 'sent' ? '→ sent' : '← received'} · {m.at}</div><Code text={m.body} /></div>
    {/each}
  {:else if !r && view.sendError}
    <p class="err sel"><Problem text={view.sendError} /></p>
  {:else if !r}
    <EmptyState icon="notsent" title="Not sent" hint={emptyHint[tab] ?? emptyHint.received} />
  {:else}
    {#if r.diags.length}
      <details class="note" open={r.diags.length < 4}>
        <summary>{r.diags.length} {r.diags.length === 1 ? 'note' : 'notes'} on this response</summary>
        {#each r.diags as d}<p class="diag"><Problem text={d} /></p>{/each}
      </details>
    {:else if view.sendError}
      <details class="note" open>
        <summary>Notes from the send</summary>
        <p class="err sel"><Problem text={view.sendError} /></p>
      </details>
    {/if}
    {#if view.showingHistory}
      <p class="hist">An earlier response. <button class="mini" onclick={() => RequestService.ShowLatest(view.node)}>Back to the latest</button></p>
    {/if}
    <header>
      <span class="title" class:ok={r.passed} class:bad={!r.passed}>{r.title}</span>
      {#if r.took}<span class="meta" title="Time the send took">{r.took}</span>{/if}
      {#if r.size}<span class="meta" title="Size of the response body">{r.size}</span>{/if}
      <span class="muted">{r.at}</span>
      {#if tab === 'received'}
        <span class="grow"></span>
        <button class="mini" onclick={pin}>Pin as example…</button>
        <button class="mini" onclick={saveBody}>Save body…</button>
        <select bind:value={format} onchange={() => format !== 'tree' && RequestService.SetFormat(view.node, format)} aria-label="Body format">
          <option value="tree">Tree</option>
          <option value="json">JSON</option>
          <option value="text">Text</option>
          <option value="raw">Raw</option>
        </select>
      {/if}
    </header>
    {#if tab === 'timing' && !r.phases.length}
      <EmptyState icon="clock" title="No timings" hint="This send did not record where its time went." />
    {:else if tab === 'messages' && !r.messages.length}
      <EmptyState icon="stream" title="No messages" hint="Nothing was sent or received." />
    {:else if tab === 'timing'}
      {@const end = Math.max(...r.phases.map((p) => p.startMs + p.ms))}
      {@const total = Math.max(end, 1)}
      <div class="body">
        {#each r.phases as p}
          <div class="phase"><span class="pname">{p.name}</span>
            <div class="track"><div class="bar2" style:margin-left={`${(p.startMs / total) * 100}%`} style:width={`${Math.max((p.ms / total) * 100, 0.8)}%`}></div></div>
            <span class="pms">{p.ms.toFixed(1)} ms</span></div>
        {/each}
        <!-- The whole send: where the last phase ends, which also counts any gaps between phases. -->
        <div class="phase total"><span class="pname">Total</span><span></span><span class="pms">{end.toFixed(1)} ms</span></div>
      </div>
    {:else if tab === 'messages'}
      <div class="body">
        {#each r.messages as m}
          <div class="msg {m.direction}"><div class="mh">{m.direction === 'sent' ? '→ sent' : '← received'} · {m.at}{m.key ? ' · key ' + m.key : ''}</div><Code text={m.body} /></div>
        {/each}
      </div>
    {:else if tab === 'received' && tree}
      <div class="treebar">
        <input type="search" placeholder="Search the body" bind:value={query} spellcheck="false" />
        {#if picked}
          <code class="path">{picked}</code>
          <button class="mini" onclick={copyPath}>Copy path</button>
          <button class="mini" onclick={extract}>Extract as variable…</button>
        {/if}
      </div>
      <div class="body mono">
        <JsonNode value={r.bodyJson} name="$" path="$" selected={picked} {query} onpick={(p) => (picked = p)} />
      </div>
    {:else}
      <div class="body">
        <Parts parts={tab === 'received' ? received : r.sent} />
      </div>
    {/if}
  {/if}
</section>

<style>
  section { display: flex; flex-direction: column; min-height: 0; height: 100%; padding: 12px; gap: 8px; }
  header { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
  .grow { flex: 1; }
  /* Time and size beside the title, as quiet chips: the figures people look for first. */
  .meta { padding: 1px 8px; border-radius: var(--r-pill); background: var(--tint-hover); color: var(--text); font: 600 var(--fs-xs)/16px var(--mono); font-variant-numeric: tabular-nums; }
  .title { font-weight: 600; }
  .ok { color: var(--ok); }
  .bad { color: var(--fail); }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .err { color: var(--fail); margin: 0; }
  .sel, .diag { -webkit-user-select: text; user-select: text; }
  .note { font-size: var(--fs-sm); max-height: 30%; overflow: auto; }
  .note summary { color: var(--muted); cursor: default; }
  .note .err { white-space: pre-wrap; }
  /* Flush with the pane's top, as tall as the request's tab bar beside it, so the two line up. */
  .bar { display: flex; justify-content: space-between; align-items: flex-end; height: var(--tabbar-h); box-sizing: border-box; margin: -12px -12px 0; padding: 0 8px; border-bottom: 1px solid var(--line); flex: none; }
  .tabs { display: flex; flex-wrap: nowrap; overflow-x: auto; scrollbar-width: none; }
  .tabs button {
    background: none;
    border: 0;
    border-bottom: 2px solid transparent;
    color: var(--muted);
    font: inherit;
    padding: 8px 10px 6px;
    white-space: nowrap;
  }
  .tone { margin-left: 6px; font-size: var(--fs-xs); }
  .tabs .status-pill { margin-left: 8px; }
  .tone.run { color: var(--accent); }
  .tabs .active { color: var(--text); border-bottom-color: var(--accent); }
  select { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); font: inherit; font-size: var(--fs-xs); }
  .body { flex: 1; min-height: 0; overflow: auto; }
  .col { display: flex; flex-direction: column; }
  .col .edit { align-self: flex-start; }
  .mono { font: 12.5px/1.6 ui-monospace, Menlo, monospace; -webkit-user-select: text; user-select: text; }
  .treebar { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
  .treebar input { flex: 1; min-width: 120px; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 8px; font: inherit; }
  .path { color: var(--muted); font: var(--fs-xs) var(--mono); }
  .phase { display: grid; grid-template-columns: 80px 1fr 70px; gap: 8px; align-items: center; margin: 6px 0; font-size: var(--fs-sm); }
  .track { background: var(--bg); border-radius: var(--r-sm); height: 10px; }
  .bar2 { background: var(--accent); height: 10px; border-radius: var(--r-sm); }
  .pname { color: var(--muted); }
  .total { border-top: 1px solid var(--line); padding-top: 6px; }
  .total .pname, .total .pms { color: var(--text); font-weight: 600; }
  .pms { color: var(--muted); text-align: right; font: var(--fs-xs) var(--mono); }
  .msg { border-left: 2px solid var(--line); padding: 2px 10px; margin: 8px 0; }
  .msg.received { border-color: var(--ok); }
  .msg.sent { border-color: var(--accent); }
  .mh { color: var(--muted); font-size: var(--fs-xs); }
  .sum { margin: 0 0 8px; font-size: var(--fs-sm); color: var(--muted); display: flex; gap: 10px; }
  .okc { color: var(--ok); font-weight: 600; }
  .badc { color: var(--fail); font-weight: 600; }
  /* The hooks as a timeline: a tick for each step on a line that runs down to the request and past it. */
  .timeline { list-style: none; margin: 0; padding: 0; }
  .tl { position: relative; display: flex; align-items: flex-start; gap: 10px; padding: 4px 0 8px; font-size: var(--fs-sm); }
  .tl::before { content: ''; position: absolute; left: 9px; top: 24px; bottom: -2px; width: 2px; background: var(--line); }
  .timeline > li:last-child::before { display: none; }
  .tick { flex: none; width: 20px; height: 20px; border-radius: 50%; display: grid; place-items: center; font-size: var(--fs-xs); font-weight: 700; color: var(--muted); border: 2px solid var(--line); background: var(--bg); z-index: 1; }
  .tl.passed .tick { color: var(--on-accent); background: var(--ok); border-color: var(--ok); }
  .tl.failed .tick { color: var(--on-accent); background: var(--fail); border-color: var(--fail); }
  .tl.running .tick { color: var(--accent); border-color: var(--accent); }
  .tl.passed::before { background: var(--ok); }
  .tl.api .tick { border-radius: var(--r-md); }
  .tl.api .st { color: var(--accent); }
  .what { flex: 1; min-width: 0; display: flex; flex-wrap: wrap; gap: 2px 8px; align-items: baseline; padding-top: 2px; }
  .st { font-weight: 600; }
  .what .err { flex-basis: 100%; }
  .sec h5 { margin: 6px 0 4px 30px; }
  /* The line carries on past a section's heading, so before, the request and after read as one run. */
  .sec { position: relative; }
  .sec:not(:first-child):not(:last-child)::before { content: ''; position: absolute; left: 9px; top: -2px; bottom: -2px; width: 2px; background: var(--line); }
  .edit { margin-top: 12px; }
  .builtins { margin-top: 16px; }
  .builtins .sec h5 { margin-left: 0; }
  .tl.builtin .tick { border-style: dashed; }
  .tag { font-size: var(--fs-xs); color: var(--muted); border: 1px solid var(--line); border-radius: var(--r-pill); padding: 0 6px; }
  .err.off { color: var(--muted); }
  .switch { flex-basis: auto; margin-top: 4px; }
  .hist { margin: 0; padding: 4px 8px; font-size: var(--fs-sm); background: color-mix(in srgb, var(--accent) 14%, transparent); border-radius: var(--r-md); }
  .mini { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 1px 8px; font: inherit; font-size: var(--fs-xs); }
  h5 { margin: 8px 0 2px; color: var(--muted); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: 0.05em; }
  .diag { color: var(--fail); font-size: var(--fs-sm); margin: 2px 0; }
</style>
