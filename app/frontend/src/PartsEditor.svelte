<script lang="ts">
  // The parts of a multipart body: a name and either text or a file, so one request can send several
  // files. A row without a name is not kept. Paths are relative to the collection.
  import EmptyState from './EmptyState.svelte'
  import { tick } from 'svelte'

  interface Part {
    name: string
    file: boolean
    value: string
    contentType: string
  }
  let { parts, onchange }: { parts: Part[]; onchange: (parts: Part[]) => void } = $props()

  let local = $state<Part[]>([])
  let within = $state(false)
  $effect(() => {
    if (!within) local = parts.map((p) => ({ ...p }))
  })
  let box = $state<HTMLDivElement>()

  function leave(e: FocusEvent) {
    if (!box?.contains(e.relatedTarget as Node | null)) within = false
  }
  const send = () => onchange(local.map((p) => ({ ...p })))
  async function add(file = false) {
    within = true
    local = [...local, { name: '', file, value: '', contentType: '' }]
    await tick()
    box?.querySelectorAll<HTMLInputElement>('input.name')[local.length - 1]?.focus()
  }
  function remove(i: number) {
    local = local.filter((_, j) => j !== i)
    send()
  }
</script>

<div class="parts" bind:this={box} onfocusin={() => (within = true)} onfocusout={leave}>
  {#if !local.length}
    <EmptyState icon="list" title="No parts" hint="Each part is sent under its name, as text or as a file. Use several parts with the same name for several files." compact />
  {/if}
  {#each local as p, i}
    <div class="row">
      <input class="name" aria-label="Part name" placeholder="name" bind:value={p.name} onchange={send} spellcheck="false" />
      <select aria-label="Part kind" bind:value={p.file} onchange={send}>
        <option value={false}>text</option>
        <option value={true}>file</option>
      </select>
      <input class="value" aria-label={p.file ? 'File path' : 'Text'} placeholder={p.file ? 'path/in/the/collection.csv' : 'value'} bind:value={p.value} onchange={send} spellcheck="false" />
      {#if p.file}<input class="type" aria-label="Content type" placeholder="content type" bind:value={p.contentType} onchange={send} spellcheck="false" />{/if}
      <button aria-label="Remove part" title="Remove the part" onclick={() => remove(i)}>×</button>
    </div>
  {/each}
  <div class="add">
    <button onclick={() => add(false)}>Add text part</button>
    <button onclick={() => add(true)}>Add file part</button>
  </div>
</div>

<style>
  .parts { display: flex; flex-direction: column; gap: 6px; }
  /* Name, kind and remove on one line; the text or path, and a file's content type, each on a line of their own. */
  .row { display: grid; grid-template-columns: minmax(80px, 1fr) auto auto; gap: 6px; align-items: center; padding-bottom: 8px; border-bottom: 1px solid var(--line); }
  .row button { grid-column: 3; grid-row: 1; }
  input, select { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 8px; font: inherit; }
  .name { min-width: 0; }
  .value, .type { grid-column: 1 / -1; min-width: 0; }
  .value { font-family: var(--mono); font-size: var(--fs-sm); }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 10px; font: inherit; }
  .row button { padding: 2px 8px; color: var(--muted); }
  .add { display: flex; gap: 6px; }
</style>
