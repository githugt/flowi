<script lang="ts">
  import type { Snippet } from 'svelte'
  import { modal } from './lib/modal'

  // A small panel that opens at a point (where something was clicked), over a transparent layer: a press anywhere
  // outside closes it, Escape closes it from inside. It stays on screen: x and y are where it opens, `height` is
  // about how tall it gets, and it is moved up and left only as far as needed to keep that much inside the window.
  // `above` opens it over the point (a bar at the foot of a screen) and `end` puts its right edge on x (a button at the
  // end of a field). `tone="data"` gives it the data colour as its edge. Named for a screen reader, Tab kept inside, focus returned on close (lib/modal), like Modal.
  // Before: three screens (the step picker, a Route's test, an input or output of the flow) each wrote the layer,
  // the box, the clamp to the window edge, and two z-index numbers.
  let {
    x,
    y,
    label,
    onclose,
    width = 360,
    height = 300,
    maxHeight,
    tone = '',
    above = false,
    end = false,
    children,
  }: { x: number; y: number; label: string; onclose: () => void; width?: number; height?: number; maxHeight?: string; tone?: '' | 'data'; above?: boolean; end?: boolean; children: Snippet } = $props()

  // Room kept between the box and the window edge.
  const EDGE = 20
  const left = $derived(Math.max(EDGE / 2, Math.min(end ? x - width : x, window.innerWidth - width - EDGE)))
  const top = $derived(Math.max(EDGE / 2, Math.min(y + 8, window.innerHeight - height)))
  // Opening upward is fixed to the bottom edge, so it grows away from the button whatever its height turns out to be.
  const bottom = $derived(Math.max(EDGE / 2, window.innerHeight - y + 8))
</script>

<div class="layer" role="presentation" onpointerdown={onclose}></div>
<div class="popover pop-in" class:data={tone === 'data'} style:left="{left}px" style:top={above ? undefined : `${top}px`} style:bottom={above ? `${bottom}px` : undefined} style:width="{width}px" style:max-height={maxHeight} use:modal role="dialog" aria-label={label} tabindex="-1"
  onkeydown={(e) => { if (e.key === 'Escape' && !e.defaultPrevented) { e.preventDefault(); onclose() } }}>
  {@render children()}
</div>

<style>
  .layer { position: fixed; inset: 0; z-index: var(--z-menu); }
  .popover { position: fixed; z-index: calc(var(--z-menu) + 1); box-sizing: border-box; display: flex; flex-direction: column; gap: 8px; padding: 12px; background: var(--raised); border: 1px solid var(--line); border-radius: var(--r-xl); box-shadow: var(--shadow); overflow: auto; }
  /* A panel about the flow's data wires has the data colour for its edge. */
  .popover.data { border-color: var(--data); }
</style>
