<script lang="ts">
  // One picker for the version and the overlays laid over it. Versions are chosen one at a time; overlays toggle,
  // so stacking ("vip+slow") works from the same list: they apply in the order they were ticked, the later one wins.
  import { RequestService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import type { RequestView } from './lib/types'
  import { pickerItems, pickerLabel, toggleOverlay } from './lib/versionpicker'

  let { view, node, onmanage }: { view: RequestView; node: string; onmanage: () => void } = $props()

  let open = $state(false)
  const stack = $derived(view.flowiOverlay ? view.flowiOverlay.split('+') : [])
  const items = $derived(
    pickerItems({ versions: view.versions, names: view.versionNames, def: view.default, selected: view.selected, overlays: view.flowiOverlays.map((o) => o.name), stack }),
  )
  const groups = $derived(['Versions', 'Overlays'].map((g) => ({ name: g, items: items.filter((i) => i.group === g) })))

  function pick(group: string, id: string) {
    if (group === 'Versions') {
      open = false
      RequestService.SelectVersion(node, id)
    } else {
      RequestService.SelectFlowiOverlay(node, toggleOverlay(stack, id))
    }
  }
</script>

<svelte:window onpointerdown={() => (open = false)} />

<div class="picker" onpointerdown={(e) => e.stopPropagation()} role="presentation">
  <button class="btn" aria-haspopup="menu" aria-expanded={open} aria-label="Version and overlay" title="The version of this request, and the flowi overlays laid over it: the same request as a VIP customer, a slow one…" onclick={() => (open = !open)}>
    <span class="muted">Ver.</span> {pickerLabel(view.selected, stack)} <span class="caret">▾</span>
  </button>
  {#if open}
    <div class="menu" role="menu">
      {#each groups as g (g.name)}
        <div class="hd">{g.name}</div>
        {#each g.items as i (g.name + i.id)}
          <button role="menuitemcheckbox" aria-checked={i.on} onclick={() => pick(g.name, i.id)}><span class="tick">{i.on ? '✓' : ''}</span>{i.label}</button>
        {/each}
      {/each}
      <hr />
      <button role="menuitem" onclick={() => { open = false; onmanage() }}><span class="tick"></span>{view.flowiOverlays.length ? 'Manage overlays…' : 'Add an overlay…'}</button>
    </div>
  {/if}
</div>

<style>
  .picker { position: relative; }
  .btn { height: 32px; box-sizing: border-box; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 8px; font: inherit; white-space: nowrap; }
  .muted { color: var(--muted); }
  .caret { color: var(--muted); font-size: var(--fs-xs); }
  .menu { position: absolute; z-index: 20; top: calc(100% + 4px); right: 0; min-width: 200px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-md); box-shadow: var(--shadow); padding: 4px; display: flex; flex-direction: column; }
  .menu button { display: flex; gap: 6px; text-align: left; background: none; border: 0; color: var(--text); font: inherit; padding: 4px 8px; border-radius: var(--r-sm, 6px); }
  .menu button:hover { background: var(--tint-hover); }
  .tick { width: 14px; color: var(--accent); }
  .hd { color: var(--muted); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: 0.06em; padding: 6px 8px 2px; }
  hr { border: 0; border-top: 1px solid var(--line); margin: 4px 0; }
</style>
