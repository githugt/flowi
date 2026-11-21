<script lang="ts">
  import Button from './Button.svelte'
  import IconButton from './IconButton.svelte'
  import Select from './Select.svelte'
  // The values a set step writes, one row each: a name, where the value comes from, and what that source needs.
  import type { ValueRow } from './lib/types'

  let { rows, onchange }: { rows: ValueRow[]; onchange: (rows: ValueRow[]) => void } = $props()

  const SOURCES: [string, string][] = [
    ['', 'a value or expression'],
    ['literal', 'text as written'],
    ['response', 'from the response'],
    ['generate', 'generated'],
  ]
  const blank = (): ValueRow => ({ name: '', from: '', value: '', in: '', path: '', header: '', cookie: '', regex: '', seed: '' })

  let local = $state<ValueRow[]>([])
  let within = $state(false)
  $effect(() => {
    if (!within) local = rows.map((r) => ({ ...r }))
  })
  let box = $state<HTMLDivElement>()
  function leave(e: FocusEvent) {
    if (!box?.contains(e.relatedTarget as Node | null)) within = false
  }
  // A row with no name is not sent: it stays in the form until it gets one.
  const commit = () => onchange(local.filter((r) => r.name.trim() !== ''))
</script>

<div class="values" bind:this={box} onfocusin={() => (within = true)} onfocusout={leave}>
  {#each local as row, i}
    <div class="row">
      <div class="line">
        <input class="k" placeholder="name" aria-label="Variable name" bind:value={row.name} oninput={commit} spellcheck="false" />
        <Select label="Where the value comes from" bind:value={row.from} onchange={commit}>
          {#each SOURCES as [id, label]}<option value={id}>{label}</option>{/each}
        </Select>
        <IconButton icon="close" size={12} danger label="Remove this value" onclick={() => { local.splice(i, 1); commit() }} />
      </div>
      {#if row.from === 'response'}
        <div class="line">
          <Select label="Which part of the response" bind:value={row.in} onchange={commit}>
            <option value="">choose…</option>
            <option value="body">body</option><option value="header">header</option><option value="cookie">cookie</option><option value="status">status</option>
          </Select>
          {#if row.in === 'body'}<input class="v" placeholder="$.path.to.value" aria-label="JSONPath" bind:value={row.path} oninput={commit} spellcheck="false" />{/if}
          {#if row.in === 'header'}<input class="v" placeholder="header name" aria-label="Header" bind:value={row.header} oninput={commit} spellcheck="false" />{/if}
          {#if row.in === 'cookie'}<input class="v" placeholder="cookie name" aria-label="Cookie" bind:value={row.cookie} oninput={commit} spellcheck="false" />{/if}
          {#if row.in === 'body' || row.in === 'header' || row.in === 'cookie'}<input class="v narrow" placeholder="regex (optional)" aria-label="Regex" bind:value={row.regex} oninput={commit} spellcheck="false" />{/if}
        </div>
      {:else}
        <div class="line">
          <input class="v" placeholder={row.from === 'generate' ? '{{$first-name}}' : row.from === 'literal' ? 'text' : '{{nodes.login.token}}'} aria-label="Value" bind:value={row.value} oninput={commit} spellcheck="false" />
          {#if row.from === 'generate'}<input class="v narrow" placeholder="seed (optional)" aria-label="Seed" bind:value={row.seed} oninput={commit} spellcheck="false" />{/if}
        </div>
      {/if}
    </div>
  {/each}
  <Button variant="link" onclick={() => local.push(blank())}>+ Add value</Button>
</div>

<style>
  .values { display: flex; flex-direction: column; gap: 10px; }
  .row { display: flex; flex-direction: column; gap: 4px; padding-bottom: 8px; border-bottom: 1px solid var(--line); }
  .row:last-of-type { border-bottom: 0; }
  .line { display: flex; gap: 4px; align-items: center; }
  input { flex: 1; min-width: 0; font: var(--fs-md) var(--mono); }
  input:focus { border-color: var(--accent); }
  .k { flex: 0 0 34%; }
  .narrow { flex: 0 0 38%; }
</style>
