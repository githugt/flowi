<script lang="ts">
  // A request's checks as cards, one per check, each ticked or crossed by the latest send, with a bulk
  // edit for pasting a list. Like KVRows, it keeps what is being typed while Go round-trips the edit.
  import { tick } from 'svelte'
  import EmptyState from './EmptyState.svelte'
  import TemplateField from './TemplateField.svelte'
  import type { Item } from './lib/exprcomplete'
  import { checkStates, type CheckOutcome } from './lib/checks'

  let {
    checks,
    outcome,
    onchange,
    example,
    vocabulary,
  }: {
    checks: string[]
    outcome?: CheckOutcome
    onchange: (checks: string[]) => void
    /** A sample check shown as the placeholder. */
    example: string
    /** What a check can name, for completing the word being typed. */
    vocabulary?: () => Promise<Item[]>
  } = $props()

  let local = $state<string[]>([])
  let within = $state(false)
  $effect(() => {
    if (!within) local = [...checks]
  })

  let box = $state<HTMLDivElement>()
  // Focus moving between this form's own controls is not leaving it (see KVRows).
  function leave(e: FocusEvent) {
    if (!box?.contains(e.relatedTarget as Node | null)) within = false
  }

  const states = $derived(checkStates(local, outcome))
  const mark = { idle: '○', passed: '✓', failed: '✗', error: '!' }
  const hint = { idle: 'Not run yet', passed: 'Passed', failed: 'Failed', error: 'Could not be evaluated: see the notes on the response' }

  function commit() {
    onchange(local.filter((c) => c.trim() !== ''))
  }

  async function add() {
    local.push('')
    await tick()
    const inputs = box?.querySelectorAll<HTMLInputElement>('.c input')
    inputs?.[inputs.length - 1]?.focus()
  }

  let bulk = $state(false)
  let bulkText = $state('')
  async function startBulk() {
    bulkText = local.filter((c) => c.trim() !== '').join('\n')
    bulk = true
    await tick()
    box?.querySelector<HTMLTextAreaElement>('textarea')?.focus()
  }
  function bulkInput(text: string) {
    bulkText = text
    onchange(text.split('\n').filter((l) => l.trim() !== ''))
  }
</script>

{#if !local.length && !bulk}
  <EmptyState icon="assert" title="No checks" hint="A failing check fails the send." compact>
    <button onclick={add}>Add check</button><button onclick={startBulk}>Bulk edit</button>
  </EmptyState>
{/if}
<div class="cards" bind:this={box} hidden={!local.length && !bulk} onfocusin={() => (within = true)} onfocusout={leave}>
  {#if bulk}
    <textarea
      rows={Math.min(Math.max(bulkText.split('\n').length + 1, 6), 24)}
      value={bulkText}
      placeholder={example}
      spellcheck="false"
      aria-label="Checks, one per line"
      oninput={(e) => bulkInput(e.currentTarget.value)}
    ></textarea>
    <div class="tools">
      <span class="hint">One check per line</span>
      <button class="add" onclick={() => (bulk = false)}>Card edit</button>
    </div>
  {:else}
    {#each local as check, i}
      <div class="card {states[i]}">
        <span class="tick" title={hint[states[i]]} aria-label={hint[states[i]]}>{mark[states[i]]}</span>
        <div class="c"><TemplateField plain {vocabulary} placeholder={example} aria="Check {i + 1}" value={local[i]} oninput={(t) => { local[i] = t; commit() }} /></div>
        <button class="x" aria-label="Remove check" onclick={() => { local.splice(i, 1); commit() }}>×</button>
      </div>
    {/each}
    <div class="tools">
      <button class="add" onclick={add}>+ Add</button>
      <button class="add" onclick={startBulk}>Bulk edit</button>
    </div>
  {/if}
</div>

<style>
  .cards[hidden] { display: none; }
  .cards { display: flex; flex-direction: column; gap: 6px; }
  .card {
    display: flex; align-items: center; gap: 8px; padding: 6px 8px;
    background: var(--bg); border: 1px solid var(--line); border-left-width: 3px; border-radius: var(--r-md);
  }
  .card.passed { border-left-color: var(--ok); }
  .card.failed, .card.error { border-left-color: var(--fail); }
  .tick { flex: none; width: 16px; text-align: center; color: var(--muted); }
  .passed .tick { color: var(--ok); }
  .failed .tick, .error .tick { color: var(--fail); }
  .c { flex: 1; min-width: 0; }
  .c :global(input) { outline: 0; font: 12px ui-monospace, SFMono-Regular, Menlo, monospace; }
  .card:focus-within { border-color: var(--accent); }
  button { background: none; border: 0; color: var(--muted); font: inherit; }
  .add { color: var(--accent); padding: 2px 0; }
  .x:hover { color: var(--fail); }
  .tools { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .hint { color: var(--muted); font-size: var(--fs-xs); }
  textarea {
    resize: vertical; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md);
    padding: 6px 8px; font: 12px/1.5 ui-monospace, SFMono-Regular, Menlo, monospace; -webkit-user-select: text; user-select: text;
  }
  textarea:focus { border-color: var(--accent); }
</style>
