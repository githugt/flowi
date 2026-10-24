<script lang="ts">
  import HiddenChars from './HiddenChars.svelte'
  import type { Pair } from './lib/types'

  let {
    rows,
    onchange,
    keyLabel = 'name',
    valueLabel = 'value',
  }: { rows: Pair[]; onchange: (rows: Pair[]) => void; keyLabel?: string; valueLabel?: string } = $props()

  let local = $state<Pair[]>([])
  let within = $state(false)
  $effect(() => {
    if (!within) local = rows.map((r) => ({ ...r }))
  })

  let box = $state<HTMLDivElement>()
  // Focus moving between this form's own controls is not leaving it (see KVRows).
  function leave(e: FocusEvent) {
    if (!box?.contains(e.relatedTarget as Node | null)) within = false
  }

  // Blank rows stay in the form until they get a name; Go only hears of named ones.
  function commit() {
    onchange(local.filter((r) => r.key.trim() !== ''))
  }
</script>

<div class="pairs" bind:this={box} onfocusin={() => (within = true)} onfocusout={leave}>
  {#each local as row, i}
    <div class="row">
      <input class="k" placeholder={keyLabel} bind:value={row.key} oninput={commit} spellcheck="false" />
      <span class="cell"><input class="v" placeholder={valueLabel} bind:value={row.value} oninput={commit} spellcheck="false" /><HiddenChars value={row.value} onclean={(t) => { row.value = t; commit() }} /></span>
      <button
        class="x"
        aria-label="Remove row"
        onclick={() => {
          local.splice(i, 1)
          commit()
        }}>×</button
      >
    </div>
  {/each}
  <button class="add" onclick={() => local.push({ key: '', value: '' })}>+ Add</button>
</div>

<style>
  .pairs { display: flex; flex-direction: column; gap: 4px; }
  .row { display: flex; gap: 4px; }
  input {
    flex: 1;
    min-width: 0;
    background: var(--bg);
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    padding: 4px 6px;
    font: 12px ui-monospace, SFMono-Regular, Menlo, monospace;
    -webkit-user-select: text;
    user-select: text;
  }
  input:focus { border-color: var(--accent); }
  .k { flex: 0 0 38%; }
  .cell { position: relative; flex: 1; display: flex; min-width: 0; }
  button { background: none; border: 0; color: var(--muted); font: inherit; }
  .add { align-self: flex-start; color: var(--accent); padding: 2px 0; }
  .x:hover { color: var(--fail); }
</style>
