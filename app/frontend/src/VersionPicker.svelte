<script lang="ts">
  // One picker for the version and the overlays laid over it. Versions are chosen one at a time; overlays toggle,
  // so stacking ("vip+slow") works from the same list: they apply in the order they were ticked, the later one wins.
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { RequestView } from './lib/types'
  import Menu, { type MenuItem } from './Menu.svelte'
  import Icon from './Icon.svelte'
  import { anchorAt, type MenuAt } from './lib/menuanchor'
  import { pickerButton, pickerItems, toggleOverlay } from './lib/versionpicker'

  let { view, node, onmanage, onnew }: { view: RequestView; node: string; onmanage: () => void; onnew: () => void } = $props()

  let open = $state(false)
  let at = $state<MenuAt>({ x: 0, y: 0 })
  const stack = $derived(view.flowiOverlay ? view.flowiOverlay.split('+') : [])
  const items = $derived(
    pickerItems({ versions: view.versions, names: view.versionNames, def: view.default, selected: view.selected, overlays: view.flowiOverlays.map((o) => o.name), stack }),
  )
  const button = $derived(pickerButton(view.versions.length, view.selected, stack, view.flowiOverlays.length))
  const menuItems = $derived<MenuItem[]>([
    ...['Versions', 'Overlays'].flatMap((g) =>
      items.filter((i) => i.group === g).map((i, n): MenuItem => ({ label: i.label, checked: i.on, multi: g === 'Overlays', heading: n === 0 ? g : undefined, onselect: () => pick(g, i.id) })),
    ),
    { label: 'New version…', onselect: onnew, separated: true },
    { label: view.flowiOverlays.length ? 'Manage overlays…' : 'Add an overlay…', onselect: onmanage },
  ])

  function pick(group: string, id: string) {
    if (group === 'Versions') {
      RequestService.SelectVersion(node, id)
    } else {
      RequestService.SelectFlowiOverlay(node, toggleOverlay(stack, id))
    }
  }
</script>

<div class="picker">
  <button class="btn" aria-haspopup="menu" aria-expanded={open} aria-label="Version and overlay" title="The version of this request, and the flowi overlays laid over it: the same request as a VIP customer, a slow one…" onclick={(e) => { if (button.direct) return onnew(); at = anchorAt(e.currentTarget.getBoundingClientRect(), { end: true }); open = !open }}>
    {#if button.prefix}<span class="muted">{button.prefix}</span> {/if}{button.text}{#if !button.direct} <span class="caret"><Icon name="chevd" size={12} /></span>{/if}
  </button>
  {#if open}<Menu {at} minWidth={200} label="Version and overlay" items={menuItems} onclose={() => (open = false)} />{/if}
</div>

<style>
  .btn { height: 32px; box-sizing: border-box; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 8px; font: inherit; white-space: nowrap; }
  .muted { color: var(--muted); }
  .caret { display: inline-flex; vertical-align: middle; color: var(--muted); }
</style>
