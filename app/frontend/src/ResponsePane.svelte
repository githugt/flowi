<script lang="ts">
  import { tick } from 'svelte'
  import { errorText } from './lib/errors'
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { RequestView } from './lib/types'
  import Parts from './Parts.svelte'
  import Icon from './Icon.svelte'
  import { flowEnd, groupPipeline } from './lib/pipelinegroups'
  import Problem from './Problem.svelte'
  import EmptyState from './EmptyState.svelte'
  import JsonNode from './JsonNode.svelte'
  import XmlNode from './XmlNode.svelte'
  import { multiStatus, parseXml, pathsOf } from './lib/xml'
  import MessageFeed from './MessageFeed.svelte'
  import { tookTone } from './lib/took'
  import { cellOf } from './lib/cells'
  import { Clipboard } from '@wailsio/runtime'
  import { ui } from './lib/ui.svelte'
  import { app } from './lib/app.svelte'

  let { view }: { view: RequestView } = $props()
  let tab = $state<'received' | 'sent' | 'timing' | 'messages' | 'pipeline'>('received')
  const r = $derived(view.response)
  // How the body is shown. Until one is picked, a dataset's rows are read as a table and any other response as a tree.
  let chosenFormat = $state('')
  // Rows need a result: a query that failed has none, so it falls back to the tree.
  const shape = $derived(chosenFormat === 'rows' && !r?.table ? 'tree' : chosenFormat || (view.kind === 'dataset' && r?.table ? 'rows' : 'tree'))
  function pickFormat(f: string) {
    chosenFormat = f
    if (f !== 'tree' && f !== 'status' && f !== 'rows') RequestService.SetFormat(view.node, f)
  }
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
  // One line runs behind every tick, from the first to the last, so the steps read as one run across the cards.
  // It is measured, not computed from the CSS, because a step's height depends on how its text wraps.
  let cardsEl = $state<HTMLElement>()
  let flow = $state({ top: 0, height: 0, x: 0, ran: false })
  function measureFlow() {
    const ticks = cardsEl ? [...cardsEl.querySelectorAll<HTMLElement>('.tick')] : []
    if (!cardsEl || ticks.length < 2) return void (flow = { top: 0, height: 0, x: 0, ran: false })
    // After a send the line carries on only as far as the run did: to the step that failed, or the last that ran.
    // Steps after that never ran, so there is nothing to join them to. Before any send it shows the whole way.
    const states = ticks.map((t) => t.getAttribute('aria-label') ?? '')
    const { end, ran } = flowEnd(states)
    const box = cardsEl.getBoundingClientRect()
    const centre = (t: HTMLElement) => t.getBoundingClientRect().top + t.offsetHeight / 2 - box.top
    const first = ticks[0].getBoundingClientRect()
    const top = centre(ticks[0])
    flow = { top, height: centre(ticks[end]) - top, x: first.left + first.width / 2 - box.left, ran }
  }
  $effect(() => {
    void foldedCards
    void view.pipeline
    if (!cardsEl) return
    tick().then(measureFlow)
    const watch = new ResizeObserver(measureFlow)
    watch.observe(cardsEl)
    return () => watch.disconnect()
  })
  // Cards the owner has folded, by heading; every card starts open.
  let foldedCards = $state(new Set<string>())
  function toggleCard(key: string) {
    const next = new Set(foldedCards)
    if (!next.delete(key)) next.add(key)
    foldedCards = next
  }
  let query = $state('')
  let picked = $state('')
  const tree = $derived(shape === 'tree' && r?.bodyJson !== undefined && r?.bodyJson !== null)
  // An XML answer (WebDAV's 207 Multi-Status among them) is read in the window; a body that is not well-formed stays text.
  const xml = $derived(r?.bodyXml ? parseXml(r.bodyXml) : null)
  const xmlRoot = $derived(xml && 'root' in xml ? xml.root : null)
  const xmlPaths = $derived(xmlRoot ? pathsOf(xmlRoot) : new Map())
  const entries = $derived(xmlRoot ? multiStatus(xmlRoot) : null)
  const xmlTree = $derived(shape === 'tree' && !!xmlRoot)
  const statusList = $derived(shape === 'status' && !!entries)
  const rowsShown = $derived(shape === 'rows' && !!r?.table)
  const bodyShown = $derived(tree || xmlTree || statusList || rowsShown)
  // What the Response tab lists. The tree shows the body itself, and the Timing tab the timings, so neither
  // is listed again here.
  const received = $derived(
    (r?.received ?? []).filter((p) => !(bodyShown && p.kind === 'code') && !(r?.phases.length && p.heading === 'Timings')),
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
  <div class="cards" bind:this={cardsEl} style:--flow-top={`${flow.top}px`} style:--flow-h={`${flow.height}px`} style:--flow-x={`${flow.x}px`}>
    {#if flow.height > 0}<span class="flow" class:ok={flow.ran} aria-hidden="true"></span>{/if}
    {#each groupPipeline(rows) as g (g.title + g.steps[0].title)}
      {@const key = g.title || g.steps[0].title}
      {@const folded = g.title !== '' && foldedCards.has(key)}
      <div class="hcard {g.tone}">
        {#if g.title}
          <button class="chead" aria-expanded={!folded} onclick={() => toggleCard(key)}>
            <span class="chev" class:shut={folded}><Icon name="chevd" size={14} /></span>
            <h5>{g.title}</h5>
            <span class="count">{g.steps.length} {g.steps.length === 1 ? 'step' : 'steps'}</span>
          </button>
        {/if}
        {#if !folded}
          <ol class="timeline">
            {#each g.steps as row}
              <li class="tl {row.state}" class:api={row.kind === 'api'} class:builtin={row.kind === 'builtin'}>
                <span class="tick" aria-label={row.state}>{#if row.state === 'passed'}✓{:else if row.state === 'failed'}✗{:else if row.state === 'running'}…{:else if row.state === 'not-run'}<Icon name="pause" size={10} />{:else}○{/if}</span>
                <div class="what">
                  <span class="st">{row.title}</span>{#if row.kind === 'builtin'}<span class="tag">built in</span>{/if}{#if row.kind === 'builtin' && row.state === 'not-run'}<span class="tag paused">paused</span>{/if}
                  <span class="muted">{row.kind === 'api' ? (row.method ?? '') + ' ' + (row.tip ?? '') : row.detail}</span>
                  {#if row.reason}<div class="err" class:off={row.state === 'not-run'}>{row.reason}</div>{/if}
                </div>
                {#if row.kind === 'builtin'}
                  <!-- The version's own schemas.infer: saved with the request, like its other settings. The icon is what a
                       press does: pause while it runs, play while it is paused. -->
                  {@const paused = row.state === 'not-run'}
                  <button class="switch" class:paused title={paused ? 'Turn on for this request' : 'Turn off for this request'} aria-label={paused ? 'Turn on for this request' : 'Turn off for this request'} onclick={() => RequestService.SetSchemasInfer(view.node, paused)}>
                    <Icon name={paused ? 'play' : 'pause'} size={13} />
                  </button>
                {:else if row.ms}<span class="pms">{row.ms.toFixed(0)} ms</span>{/if}
              </li>
            {/each}
          </ol>
        {/if}
      </div>
    {/each}
  </div>
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
    {#if view.live.length}<MessageFeed messages={view.live} live />{/if}
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
      {#if r.took}<span class="meta {tookTone(r.took)}" title="Time the send took">{r.took}</span>{/if}
      {#if r.size}<span class="meta" title="Size of the response body">{r.size}</span>{/if}
      <span class="muted">{r.at}</span>
      {#if tab === 'received'}
        <span class="grow"></span>
        <button class="mini" onclick={pin}>Pin as example…</button>
        <button class="mini" onclick={saveBody}>Save body…</button>
        <select value={shape} onchange={(e) => pickFormat(e.currentTarget.value)} aria-label="Body format">
          {#if r.table}<option value="rows">Rows</option>{/if}
          <option value="tree">Tree</option>
          {#if entries}<option value="status">Multi-status</option>{/if}
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
      <div class="body"><MessageFeed messages={r.messages} /></div>
    {:else if tab === 'received' && rowsShown && r.table}
      <div class="body">
        <p class="muted rowsum">
          {r.table.rowCount} {r.table.rowCount === 1 ? 'row' : 'rows'}{#if r.table.truncated}. The store had more: raise “Keep at most … rows” in the Query tab to see them.{/if}
        </p>
        {#if r.table.columns.length}
          <div class="grid">
            <table>
              <thead><tr><th class="idx">#</th>{#each r.table.columns as c}<th>{c}</th>{/each}</tr></thead>
              <tbody>
                {#each r.table.rows as row, i}
                  <tr><td class="idx">{i + 1}</td>{#each row as v}{@const cell = cellOf(v)}<td class={cell.kind}>{cell.text}</td>{/each}</tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
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
    {:else if tab === 'received' && xmlTree && xmlRoot}
      <div class="treebar">
        <input type="search" placeholder="Search the body" bind:value={query} spellcheck="false" />
        {#if picked}
          <code class="path">{picked}</code>
          <button class="mini" onclick={copyPath}>Copy path</button>
          <button class="mini" onclick={extract}>Extract as variable…</button>
        {/if}
      </div>
      <div class="body mono">
        <XmlNode el={xmlRoot} paths={xmlPaths} selected={picked} {query} onpick={(p) => (picked = p)} />
      </div>
    {:else if tab === 'received' && statusList && entries}
      <div class="body">
        <p class="muted">{entries.length} {entries.length === 1 ? 'resource' : 'resources'} in the answer</p>
        {#each entries as e}
          <div class="ms">
            <div class="mshead"><code class="mshref">{e.href}</code>{#if e.status}<span class="mstat bad">{e.status}</span>{/if}</div>
            {#each e.props as p}
              <div class="msprop"><span class="mskey">{p.name}</span><span class="msval">{p.value || '—'}</span><span class="mstat" class:bad={!p.status.includes(' 2')}>{p.status}</span></div>
            {/each}
          </div>
        {/each}
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
  .meta.slow { background: var(--warn-t); color: var(--warn); }
  .meta.verySlow { background: var(--fail-t); color: var(--fail); }
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
  select { background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); font: inherit; font-size: var(--fs-xs); }
  .body { flex: 1; min-height: 0; overflow: auto; }
  .col { display: flex; flex-direction: column; }
  .col .edit { align-self: flex-start; }
  .mono { font: var(--fs-sm)/1.6 ui-monospace, Menlo, monospace; -webkit-user-select: text; user-select: text; }
  .ms { border-left: 2px solid var(--line); padding: 2px 10px; margin: 6px 0; }
  .mshead { display: flex; gap: 10px; align-items: baseline; flex-wrap: wrap; }
  .mshref { font: 600 var(--fs-sm) var(--mono); }
  .msprop { display: flex; flex-wrap: wrap; align-items: baseline; gap: 0 10px; font-size: var(--fs-sm); padding: 2px 0; }
  .mskey { color: var(--accent); font-family: var(--mono); flex: none; }
  .msval { flex: 1 0 100%; order: 3; font-family: var(--mono); overflow-wrap: anywhere; color: var(--text); }
  .mstat { color: var(--muted); font-size: var(--fs-xs); margin-left: auto; text-align: right; }
  .mstat.bad { color: var(--fail); }
  .rowsum { margin: 0 0 6px; }
  .grid { overflow: auto; border: 1px solid var(--line); border-radius: var(--r-md); }
  .grid table { border-collapse: collapse; width: 100%; font: var(--fs-sm)/1.5 ui-monospace, Menlo, monospace; -webkit-user-select: text; user-select: text; }
  .grid th, .grid td { padding: 3px 10px; border-bottom: 1px solid var(--line); text-align: left; white-space: nowrap; max-width: 28em; overflow: hidden; text-overflow: ellipsis; }
  .grid th { position: sticky; top: 0; background: var(--bg); color: var(--muted); font-weight: 600; font-size: var(--fs-xs); z-index: 1; }
  .grid td.number { text-align: right; color: var(--syn-number); }
  .grid td.bool { color: var(--syn-atom); }
  .grid td.null { color: var(--muted); font-style: italic; }
  .grid .idx { color: var(--muted); text-align: right; width: 1%; }
  .treebar { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
  .treebar input { flex: 1; min-width: 120px; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 8px; font: inherit; }
  .path { color: var(--muted); font: var(--fs-xs) var(--mono); }
  .phase { display: grid; grid-template-columns: 80px 1fr 70px; gap: 8px; align-items: center; margin: 6px 0; font-size: var(--fs-sm); }
  .track { background: var(--bg); border-radius: var(--r-sm); height: 10px; }
  .bar2 { background: var(--accent); height: 10px; border-radius: var(--r-sm); }
  .pname { color: var(--muted); }
  .total { border-top: 1px solid var(--line); padding-top: 6px; }
  .total .pname, .total .pms { color: var(--text); font-weight: 600; }
  .pms { color: var(--muted); text-align: right; font: var(--fs-xs) var(--mono); }
  .sum { margin: 0 0 8px; font-size: var(--fs-sm); color: var(--muted); display: flex; gap: 10px; }
  .okc { color: var(--ok); font-weight: 600; }
  .badc { color: var(--fail); font-weight: 600; }
  /* The hooks as cards, one per heading: a tick for each step on a line that joins the steps of that card only. */
  .cards { position: relative; display: flex; flex-direction: column; gap: 8px; }
  /* The run's line: centred on the ticks (measured), over the card backgrounds and the headings, and under
     the ticks. */
  .flow { position: absolute; left: calc(var(--flow-x) - 1px); top: var(--flow-top); height: var(--flow-h); width: 2px; background: var(--line); pointer-events: none; }
  .flow.ok { background: var(--ok); }
  .hcard { border: 1px solid var(--line); border-radius: var(--r-lg, 10px); background: var(--panel); overflow: hidden; }
  .hcard.passed { border-color: color-mix(in srgb, var(--ok) 35%, var(--line)); background: color-mix(in srgb, var(--ok) 6%, var(--panel)); }
  .hcard.failed { border-color: color-mix(in srgb, var(--fail) 45%, var(--line)); background: color-mix(in srgb, var(--fail) 7%, var(--panel)); }
  .hcard.off { border-style: dashed; border-color: color-mix(in srgb, var(--warn) 45%, var(--line)); background: color-mix(in srgb, var(--warn) 6%, var(--panel)); }
  .chead { all: unset; box-sizing: border-box; width: 100%; display: flex; align-items: center; gap: 8px; padding: 6px 10px; cursor: pointer; background: color-mix(in srgb, var(--text) 5%, var(--panel)); border-bottom: 1px solid var(--line); }
  .chead:hover { background: var(--tint-hover); }
  .chead:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }
  .chead h5 { margin: 0; }
  .chev { display: grid; color: var(--muted); transition: transform 0.12s; }
  .chev.shut { transform: rotate(-90deg); }
  .hcard .count { margin-left: auto; font-size: var(--fs-xs); color: var(--muted); padding: 0 8px; border-radius: var(--r-pill); background: color-mix(in srgb, var(--text) 7%, transparent); }
  .timeline { list-style: none; margin: 0; padding: 6px 10px; }
  .tl { position: relative; display: flex; align-items: flex-start; gap: 10px; padding: 4px 0 8px; font-size: var(--fs-sm); }
  .timeline > li:last-child { padding-bottom: 4px; }
  .tick { flex: none; width: 20px; height: 20px; border-radius: 50%; display: grid; place-items: center; font-size: var(--fs-xs); font-weight: 700; color: var(--muted); border: 2px solid var(--line); background: var(--bg); position: relative; z-index: 2; }
  .tl.passed .tick { color: var(--on-accent); background: var(--ok); border-color: var(--ok); }
  .tl.failed .tick { color: var(--on-accent); background: var(--fail); border-color: var(--fail); }
  .tl.running .tick { color: var(--accent); border-color: var(--accent); }
  .tl.not-run .tick { color: var(--warn); border-color: var(--warn); }
  .tl.api .tick { border-radius: var(--r-md); }
  .tl.api .st { color: var(--accent); }
  .what { flex: 1; min-width: 0; display: flex; flex-wrap: wrap; gap: 2px 8px; align-items: baseline; padding-top: 2px; }
  .st { font-weight: 600; }
  .what .err { flex-basis: 100%; }
  .edit { margin-top: 12px; }
  .builtins { margin-top: 16px; }
  .tl.builtin .tick { border-style: dashed; }
  .tag.paused { color: var(--warn); border-color: color-mix(in srgb, var(--warn) 50%, var(--line)); }
  .tag { font-size: var(--fs-xs); color: var(--muted); border: 1px solid var(--line); border-radius: var(--r-pill); padding: 0 6px; }
  .err.off { color: var(--muted); }
  /* Press to pause the step, or to start it again: in the colour the debugger uses for a pause. */
  .switch { flex: none; display: grid; place-items: center; width: var(--ctl-h-sm, 24px); height: var(--ctl-h-sm, 24px); border: 0; border-radius: var(--r-md); background: var(--raised); color: var(--muted); box-shadow: inset 0 0 0 1px var(--line); cursor: pointer; }
  .switch:hover { color: var(--text); }
  .switch.paused { color: var(--warn); background: var(--warn-t); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--warn) 50%, var(--line)); }
  .hist { margin: 0; padding: 4px 8px; font-size: var(--fs-sm); background: color-mix(in srgb, var(--accent) 14%, transparent); border-radius: var(--r-md); }
  .mini { background: var(--raised); color: var(--text); border: 0; box-shadow: inset 0 0 0 1px var(--line); border-radius: var(--r-md); padding: 0 10px; min-height: var(--ctl-h-sm); font: inherit; font-weight: 500; font-size: var(--fs-xs); }
  h5 { margin: 8px 0 2px; color: var(--muted); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: 0.05em; }
  .diag { color: var(--fail); font-size: var(--fs-sm); margin: 2px 0; }
</style>
