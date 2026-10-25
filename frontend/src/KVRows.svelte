<script lang="ts">
  // Rows of model.KeyValue (Key, Value, Description). Rows keep their
  // description when the key or value is edited; blank rows stay in the form
  // until they get a key.
  import type { KV } from './lib/types'
  import HiddenChars from './HiddenChars.svelte'
  import EmptyState from './EmptyState.svelte'
  import { tick } from 'svelte'
  import { rowsToText, textToRows } from './lib/bulk'

  let {
    rows,
    onchange,
    keyLabel = 'name',
    valueLabel = 'value',
    describe = false,
    empty,
    marks = {},
  }: {
    rows: KV[] | null
    onchange: (rows: KV[]) => void
    keyLabel?: string
    valueLabel?: string
    describe?: boolean
    /** With no rows, an EmptyState in place of the bare Add, its button labelled `add`. */
    empty?: { icon: string; title: string; hint: string; add: string }
    /** Rows a spec overlay sets, by key: the tooltip naming the overlay. */
    marks?: Record<string, string>
  } = $props()

  let local = $state<KV[]>([])
  let within = $state(false)
  $effect(() => {
    if (!within) local = (rows ?? []).map((r) => ({ ...r }))
  })

  let box = $state<HTMLDivElement>()
  // Focus moving between this form's own controls is not leaving it. On Linux (WebKitGTK) the Add button
  // takes the focus when clicked, so the move from it to the new row would otherwise reset the form
  // and drop the blank row before the click on it lands.
  function leave(e: FocusEvent) {
    if (!box?.contains(e.relatedTarget as Node | null)) within = false
  }
  // A row added from the empty state takes the focus, so typing can start at once.
  async function addFirst() {
    local.push({ Key: '', Value: '', Description: '' })
    await tick()
    box?.querySelector<HTMLInputElement>('.k')?.focus()
  }

  function commit() {
    onchange(local.filter((r) => r.Key.trim() !== ''))
  }

  // Bulk edit: the rows as "name: value" lines, for pasting or editing a list in one go. The text is
  // taken once on entry and then owned by the box; each edit is read back into rows.
  let bulk = $state(false)
  let bulkText = $state('')
  let bulkBase: KV[] = []
  async function startBulk() {
    bulkBase = local.filter((r) => r.Key.trim() !== '')
    bulkText = rowsToText(bulkBase)
    bulk = true
    await tick()
    box?.querySelector<HTMLTextAreaElement>('textarea')?.focus()
  }
  function bulkInput(text: string) {
    bulkText = text
    onchange(textToRows(text, bulkBase))
  }
</script>

{#if empty && !local.length && !bulk}
  <EmptyState icon={empty.icon} title={empty.title} hint={empty.hint} compact>
    <button onclick={addFirst}>{empty.add}</button><button onclick={startBulk}>Bulk edit</button>
  </EmptyState>
{/if}
<div class="rows" bind:this={box} hidden={!!empty && !local.length && !bulk} onfocusin={() => (within = true)} onfocusout={leave}>
  {#if bulk}
    <textarea
      class="bulk"
      rows={Math.min(Math.max(bulkText.split('\n').length + 1, 6), 24)}
      value={bulkText}
      placeholder={`${keyLabel}: ${valueLabel}`}
      spellcheck="false"
      aria-label="Rows, one per line"
      oninput={(e) => bulkInput(e.currentTarget.value)}
    ></textarea>
    <div class="tools">
      <span class="hint">One per line: {keyLabel}: {valueLabel}{describe ? ' · descriptions stay with their name' : ''}</span>
      <button class="add" onclick={() => (bulk = false)}>Key-value edit</button>
    </div>
  {:else}
  {#each local as row, i}
    <div class="row">
      <input class="k" placeholder={keyLabel} bind:value={row.Key} oninput={commit} spellcheck="false" />
      <span class="cell"><input class="v" placeholder={valueLabel} bind:value={row.Value} oninput={commit} spellcheck="false" /><HiddenChars value={row.Value} onclean={(t) => { row.Value = t; commit() }} /></span>
      {#if describe}
        <input class="d" placeholder="description" bind:value={row.Description} oninput={commit} />
      {/if}
      {#if marks[row.Key]}<span class="mark" title={marks[row.Key]}>overlay</span>{/if}
      <button class="x" aria-label="Remove row" onclick={() => { local.splice(i, 1); commit() }}>×</button>
    </div>
  {/each}
  <div class="tools">
    <button class="add" onclick={() => local.push({ Key: '', Value: '', Description: '' })}>+ Add</button>
    <button class="add" onclick={startBulk}>Bulk edit</button>
  </div>
  {/if}
</div>

<style>
  .rows[hidden] { display: none; }
  .rows { display: flex; flex-direction: column; gap: 6px; }
  .row { display: flex; gap: 6px; }
  input {
    flex: 1;
    min-width: 0;
    background: var(--bg);
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    padding: 5px 8px;
    min-height: 30px;
    box-sizing: border-box;
    font: 12px ui-monospace, SFMono-Regular, Menlo, monospace;
    -webkit-user-select: text;
    user-select: text;
  }
  .d { font-family: inherit; flex: 0.8; }
  .cell { position: relative; flex: 1; display: flex; min-width: 0; }
  input:focus { border-color: var(--accent); }
  .k { flex: 0 0 30%; }
  button { background: none; border: 0; color: var(--muted); font: inherit; }
  .add { color: var(--accent); padding: 2px 0; }
  .tools { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .hint { color: var(--muted); font-size: var(--fs-xs); }
  .bulk {
    resize: vertical; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md);
    padding: 6px 8px; font: 12px/1.5 ui-monospace, SFMono-Regular, Menlo, monospace; -webkit-user-select: text; user-select: text;
  }
  .bulk:focus { border-color: var(--accent); }
  .x:hover { color: var(--fail); }
  .mark {
    align-self: center; flex: none; padding: 1px 6px; border-radius: var(--r-sm); font-size: var(--fs-xs);
    color: var(--accent); background: color-mix(in srgb, var(--accent) 14%, transparent); cursor: help;
  }
</style>
