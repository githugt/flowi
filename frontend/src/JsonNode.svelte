<script lang="ts">
  import JsonNode from './JsonNode.svelte'
  import { childPath, matches } from './lib/jsonpath'

  let {
    value,
    name,
    path,
    selected,
    query,
    onpick,
    depth = 0,
  }: { value: unknown; name: string; path: string; selected: string; query: string; onpick: (path: string) => void; depth?: number } = $props()

  const container = $derived(value !== null && typeof value === 'object')
  const entries = $derived(container ? Object.entries(value as object) : [])
  const isArray = $derived(Array.isArray(value))
  // svelte-ignore state_referenced_locally
  let open = $state(depth < 2) // only the first two levels start open
  // A search opens what matches.
  const shown = $derived(query ? true : open)
  const preview = $derived(container ? (isArray ? `[${entries.length} items]` : `{${entries.length} keys}`) : JSON.stringify(value))
  const kind = $derived(value === null ? 'null' : typeof value)
</script>

{#if matches(value, name, query)}
  <div class="node">
    <div class="row" class:sel={selected === path} style:padding-left={`${depth * 14}px`}>
      <button class="twist" onclick={() => (open = !open)} disabled={!container} aria-label={open ? 'Collapse' : 'Expand'}>{container ? (shown ? '▾' : '▸') : ''}</button>
      <button class="label" onclick={() => onpick(path)}>
        <span class="key">{name}</span><span class="colon">:</span>
        <span class="val {kind}">{preview}</span>
      </button>
    </div>
    {#if container && shown}
      {#each entries as [k, v] (k)}
        <JsonNode value={v} name={k} path={childPath(path, isArray ? Number(k) : k)} {selected} {query} {onpick} depth={depth + 1} />
      {/each}
    {/if}
  </div>
{/if}

<style>
  .row { display: flex; align-items: baseline; gap: 2px; border-radius: var(--r-sm); }
  .row.sel { background: color-mix(in srgb, var(--accent) 25%, transparent); }
  button { background: none; border: 0; color: inherit; font: inherit; padding: 0; text-align: left; }
  .twist { width: 14px; color: var(--muted); font-size: var(--fs-xs); }
  .label { display: flex; gap: 6px; flex: 1; min-width: 0; }
  .key { color: var(--accent); }
  .colon { color: var(--muted); }
  .val { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .val.string { color: var(--ok); }
  .val.number { color: var(--warn); }
  .val.boolean, .val.null { color: var(--literal); }
  .val.object { color: var(--muted); }
</style>
