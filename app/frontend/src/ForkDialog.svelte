<script lang="ts">
  import { tick } from 'svelte'
  import Button from './Button.svelte'
  import Toggle from './Toggle.svelte'
  // Run a recorded flow again from one step with changed variables. The steps before it keep what the recording made,
  // and every call is answered from the recording: no server is asked, so a change that alters which call is made is
  // reported as "not recorded" instead of being sent.
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { errorText } from './lib/errors'
  import { ui } from './lib/ui.svelte'
  import type { ForkView } from './lib/types'

  type Row = { name: string; was: string; value: string; stale: string }
  let rows = $state<Row[]>([])
  let result = $state<ForkView | null>(null)
  let busy = $state(false)
  let only = $state(false)
  let answerStatus = $state('')
  let answerBody = $state('')
  // The body is JSON unless it is marked as text (an XML or HTML page, a plain message), which has a content type.
  let answerText = $state(false)
  // Real state, not derived from the fields: a derived `open` snaps shut on any re-render while the fields are empty.
  let answerOpen = $state(false)
  let answerContentType = $state('')
  let error = $state('')
  const ask = $derived(ui.fork)
  let box = $state<HTMLDivElement>()

  $effect(() => {
    const a = ui.fork
    result = null
    error = ''
    rows = []
    only = false
    answerStatus = ''
    answerBody = ''
    answerText = false
    answerOpen = false
    answerContentType = ''
    if (!a) return
    tick().then(() => box?.focus())
    FlowService.ForkVars(a.node, a.step).then(
      (v) => (rows = (v as unknown as { name: string; value: string; stale?: string }[]).map((x) => ({ name: x.name, was: x.value, value: x.value, stale: x.stale ?? '' }))),
      (e) => (error = errorText(e)),
    )
  })

  // Only what was changed is sent: an untouched variable keeps the recording's value, shortened or not.
  const edits = $derived(Object.fromEntries(rows.filter((r) => r.value !== r.was).map((r) => [r.name, r.value])))

  const changes = $derived(Object.keys(edits).length + (answerStatus || answerBody ? 1 : 0) + (only ? 1 : 0))

  async function fork() {
    if (!ask) return
    busy = true
    error = ''
    try {
      result = (await FlowService.Fork(ask.node, { from: ask.step, set: edits, only, answerStatus, answerBody, answerText, answerContentType })) as unknown as ForkView
    } catch (e) {
      error = errorText(e)
    } finally {
      busy = false
    }
  }
  function close() {
    ui.fork = null
  }
  function keys(e: KeyboardEvent) {
    if (e.key !== 'Escape') return
    e.preventDefault()
    close()
  }
</script>

{#if ask}
  <div class="scrim" role="presentation" onpointerdown={close}>
    <div bind:this={box} class="dialog" role="dialog" aria-label="Fork the recording" tabindex="-1" onpointerdown={(e) => e.stopPropagation()} onkeydown={keys}>
      <h3>Fork from <code>{ask.step}</code></h3>
      <p class="muted">The steps before it keep what the recording made. Change a variable, then run again from here: every call is answered from the recording, none is sent.</p>
      {#if rows.length}
        <table>
          <thead><tr><th>Variable</th><th>Replayed from the recording, not live</th></tr></thead>
          <tbody>{#each rows as r (r.name)}<tr><td class="name">{r.name}</td><td><input bind:value={r.value} spellcheck="false" class:changed={r.value !== r.was} aria-label={r.name} />{#if r.stale && r.value === r.was}<span class="stale" role="note">Stale: {r.stale}. Change it, or the fork reuses it as recorded.</span>{/if}</td></tr>{/each}</tbody>
        </table>
      {:else if !error}<p class="muted">No variable was set before this step.</p>{/if}
      <Toggle checked={only} onchange={(on) => (only = on)}>Only this step: leave the steps after it as recorded</Toggle>
      <details class="answer" bind:open={answerOpen}>
        <summary>Answer this step's request differently</summary>
        <p class="muted">Instead of the recorded response, as if the server had answered this. Leave empty to keep what was recorded.</p>
        <label><span>Status</span><input bind:value={answerStatus} spellcheck="false" inputmode="numeric" placeholder="503" aria-label="Status to answer with" class:changed={!!answerStatus} /></label>
        <label><span>Body is</span>
          <select bind:value={answerText} aria-label="Kind of body to answer with"><option value={false}>JSON</option><option value={true}>Text (not JSON)</option></select></label>
        <label><span>{answerText ? 'Text body' : 'JSON body'}</span><textarea bind:value={answerBody} spellcheck="false" placeholder={answerText ? '<h1>Bad gateway</h1>' : '{"error": "down"}'} aria-label="Body to answer with" class:changed={!!answerBody}></textarea></label>
        {#if answerText}<label><span>Content type (optional)</span><input bind:value={answerContentType} spellcheck="false" placeholder="text/html" aria-label="Content type of the body" /></label>{/if}
      </details>
      <div class="actions">
        <Button type="button" onclick={close}>Close</Button>
        <Button type="button" disabled={busy} onclick={fork}>{busy ? 'Running…' : changes ? `Fork with ${changes} change${changes === 1 ? '' : 's'}` : 'Run again unchanged'}</Button>
      </div>
      {#if result}
        <section aria-label="Result">
          <h4>The fork {result.status}</h4>
          <p class="muted">{result.steps.map((s) => `${s.id} ${s.status}`).join(' · ')}</p>
          {#if result.why}<p class="err">{result.why}</p>{/if}
          {#each result.changes as c}<div class="change"><code>{c.node}</code> {c.what}: <span class="muted">{c.from}</span> → <strong>{c.to}</strong></div>{:else}<p class="muted">Nothing differs from the recording.</p>{/each}
        </section>
      {/if}
      {#if error}<p class="err">{error}</p>{/if}
    </div>
  </div>
{/if}

<style>
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 60; }
  .dialog { width: 600px; max-width: calc(100vw - 32px); max-height: 86vh; overflow: auto; display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); outline: none; }
  h3, h4 { margin: 0; }
  h4 { font-size: var(--fs-sm); }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .err { color: var(--fail, var(--text)); margin: 0; font-size: var(--fs-sm); }
  table { width: 100%; border-collapse: collapse; font-size: var(--fs-sm); }
  th { text-align: left; color: var(--muted); font-weight: 500; padding: 2px 8px 2px 0; }
  td { padding: 3px 8px 3px 0; vertical-align: middle; }
  td.name { font-family: var(--mono); font-weight: 600; font-size: var(--fs-xs); white-space: nowrap; }
  input { width: 100%; padding: 4px 6px; background: var(--input, var(--bg)); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-sm, 4px); font: var(--fs-xs) var(--mono); }
  input.changed { border-color: var(--accent); }
  .answer { font-size: var(--fs-sm); }
  .answer summary { cursor: default; color: var(--accent); }
  .answer label { display: flex; flex-direction: column; gap: 3px; margin: 6px 0; color: var(--muted); font-size: var(--fs-xs); }
  textarea { min-height: 60px; padding: 4px 6px; background: var(--input, var(--bg)); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-sm, 4px); font: var(--fs-xs) var(--mono); }
  textarea.changed { border-color: var(--accent); }
  .stale { display: block; margin-top: 2px; color: var(--warn); font-size: var(--fs-xs); }
  .change { font-size: var(--fs-sm); padding: 2px 0; }
  code { font-family: var(--mono); font-size: var(--fs-xs); }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
</style>
