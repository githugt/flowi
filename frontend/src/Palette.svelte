<script lang="ts">
  import type { PaletteEntry } from './lib/types'

  let {
    entries,
    onadd,
    ondragstart,
  }: {
    entries: PaletteEntry[]
    onadd: (type: string) => void
    ondragstart: (type: string, e: PointerEvent) => void
  } = $props()

  const groups = $derived.by(() => {
    const out: { name: string; items: PaletteEntry[] }[] = []
    for (const e of entries) {
      const last = out[out.length - 1]
      if (last && last.name === e.group) last.items.push(e)
      else out.push({ name: e.group, items: [e] })
    }
    return out
  })
</script>

<aside>
  {#each groups as g}
    <h4>{g.name}</h4>
    {#each g.items as e}
      <button
        title={e.summary}
        onpointerdown={(ev) => ondragstart(e.type, ev)}
        onclick={() => onadd(e.type)}>{e.type}</button
      >
    {/each}
  {/each}
</aside>

<style>
  aside {
    width: 132px;
    flex: none;
    overflow: auto;
    padding: 8px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    border-right: 1px solid var(--line);
    background: var(--panel);
  }
  h4 { margin: 8px 0 2px; color: var(--muted); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: 0.06em; }
  button {
    text-align: left;
    background: var(--bg);
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    padding: 4px 8px;
    font: inherit;
    touch-action: none;
  }
  button:hover { border-color: var(--accent); }
</style>
