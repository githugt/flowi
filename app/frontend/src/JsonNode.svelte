<script lang="ts">
  import JsonNode from './JsonNode.svelte'
  import Icon from './Icon.svelte'
  import { childPath, matches } from './lib/jsonpath'
  import { plural } from './lib/plural'
  import { copy } from './lib/clip'
  import { valueName, valueText } from './lib/jsonmenu'
  import { ui } from './lib/ui.svelte'

  let {
    value,
    name,
    path,
    selected,
    query,
    onpick,
    depth = 0,
    all = null,
  }: { value: unknown; name: string; path: string; selected: string; query: string; onpick: (path: string) => void; depth?: number; all?: boolean | null } = $props()

  const container = $derived(value !== null && typeof value === 'object')
  const entries = $derived(container ? Object.entries(value as object) : [])
  const isArray = $derived(Array.isArray(value))
  // svelte-ignore state_referenced_locally
  let open = $state(all ?? depth < 2) // only the first two levels start open, unless Expand all or Collapse all was used
  // A search opens what matches.
  const shown = $derived(query ? true : open)
  const preview = $derived(container ? (isArray ? `[${plural(entries.length, 'item')}]` : `{${plural(entries.length, 'key')}}`) : JSON.stringify(value))
  const kind = $derived(value === null ? 'null' : typeof value)
  // Right-click copies what the row holds or where it is.
  function menu(e: MouseEvent) {
    e.preventDefault()
    ui.openMenu(e.clientX, e.clientY, [
      { label: 'Copy value', run: () => void copy(valueText(value), valueName(path)) },
      { label: 'Copy path', run: () => void copy(path, 'the path') },
    ])
  }
</script>

{#if matches(value, name, query)}
  <div class="node">
    <div class="row" class:sel={selected === path} data-path={path} style:padding-left={`${depth * 14}px`}>
      <button class="twist" onclick={() => (open = !open)} disabled={!container} aria-label={open ? 'Collapse' : 'Expand'}>{#if container}<Icon name={shown ? 'chevd' : 'chevr'} size={12} />{/if}</button>
      <button class="label" onclick={() => onpick(path)} oncontextmenu={menu}>
        <span class="key">{name}</span><span class="colon">:</span>
        <span class="val {kind}">{preview}</span>
      </button>
    </div>
    {#if container && shown}
      {#each entries as [k, v] (k)}
        <JsonNode value={v} name={k} path={childPath(path, isArray ? Number(k) : k)} {selected} {query} {onpick} {all} depth={depth + 1} />
      {/each}
    {/if}
  </div>
{/if}

<style>
  .row { display: flex; align-items: baseline; gap: 2px; border-radius: var(--r-sm); }
  .row.sel { background: var(--tint-sel); }
  button { background: none; border: 0; color: inherit; font: inherit; padding: 0; text-align: left; }
  .twist { display: inline-flex; align-items: center; justify-content: center; width: 14px; color: var(--muted); }
  .label { display: flex; gap: 6px; flex: 1; min-width: 0; }
  .key { color: var(--accent); }
  .colon { color: var(--muted); }
  .val { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .val.string { color: var(--ok); }
  .val.number { color: var(--warn); }
  .val.boolean, .val.null { color: var(--literal); }
  .val.object { color: var(--muted); }
</style>
