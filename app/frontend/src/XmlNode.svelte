<script lang="ts">
  import XmlNode from './XmlNode.svelte'
  import type { XmlElement } from './lib/xml'

  let {
    el,
    paths,
    selected,
    query,
    onpick,
    depth = 0,
  }: { el: XmlElement; paths: Map<XmlElement, string>; selected: string; query: string; onpick: (path: string) => void; depth?: number } = $props()

  const path = $derived(paths.get(el) ?? '')
  const q = $derived(query.trim().toLowerCase())
  const hasChildren = $derived(el.children.length > 0)
  // svelte-ignore state_referenced_locally
  let open = $state(depth < 3) // multi-status answers are three deep before the properties
  // A search opens what matches.
  const shown = $derived(q ? true : open)

  const own = (e: XmlElement) => [e.name, e.text, ...e.attrs.flatMap((a) => [a.name, a.value])].some((t) => t.toLowerCase().includes(q))
  const matches = (e: XmlElement): boolean => !q || own(e) || e.children.some(matches)
</script>

{#if matches(el)}
  <div class="node">
    <div class="row" class:sel={selected === path} style:padding-left={`${depth * 14}px`}>
      <button class="twist" onclick={() => (open = !open)} disabled={!hasChildren} aria-label={open ? 'Collapse' : 'Expand'}>{hasChildren ? (shown ? '▾' : '▸') : ''}</button>
      <button class="label" onclick={() => onpick(path)}>
        <span class="tag">&lt;{el.name}</span>{#each el.attrs as a}<span class="attr"> {a.name}=<span class="aval">"{a.value}"</span></span>{/each}<span class="tag">{hasChildren || el.text ? '>' : '/>'}</span>
        {#if el.text}<span class="text">{el.text}</span>{/if}
        {#if hasChildren && !shown}<span class="count">{el.children.length} {el.children.length === 1 ? 'element' : 'elements'}</span>{/if}
      </button>
    </div>
    {#if hasChildren && shown}
      {#each el.children as c}
        <XmlNode el={c} {paths} {selected} {query} {onpick} depth={depth + 1} />
      {/each}
    {/if}
  </div>
{/if}

<style>
  .row { display: flex; align-items: baseline; gap: 2px; border-radius: var(--r-sm); }
  .row.sel { background: color-mix(in srgb, var(--accent) 25%, transparent); }
  button { background: none; border: 0; color: inherit; font: inherit; padding: 0; text-align: left; }
  .twist { width: 14px; color: var(--muted); font-size: var(--fs-xs); }
  .label { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tag { color: var(--accent); }
  .attr { color: var(--muted); }
  .aval { color: var(--ok); }
  .text { color: var(--text); margin-left: 6px; }
  .count { color: var(--muted); margin-left: 6px; font-size: var(--fs-xs); }
</style>
