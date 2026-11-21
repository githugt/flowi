<script lang="ts">
  import { onMount } from 'svelte'
  import { widthAfterKey, wantsCollapse } from './lib/splitter'

  // A drag handle between two panes (axis y: a horizontal handle above a pane, which then has `width` as its height). It resizes the pane it sits next to
  // (`side`: which side of the handle that pane is on), keeps the width in
  // localStorage under `name`, and reports it through the bindable `width`. With `oncollapse`, dragging well below the
  // minimum folds the pane: the handle turns to the failure colour while the drag is past that point, and letting go
  // there calls `oncollapse` (the width stays at the minimum, so unfolding brings the pane back at a usable size).
  let {
    width = $bindable(300),
    name,
    side = 'left',
    min = 180,
    max = 720,
    axis = 'x',
    oncollapse,
  }: { width?: number; name: string; side?: 'left' | 'right'; min?: number; max?: number; axis?: 'x' | 'y'; oncollapse?: () => void } = $props()

  let folding = $state(false)

  // The width the pane started with: a double-click goes back to it.
  const initial = width

  function remember() {
    try {
      localStorage.setItem('split:' + name, String(width))
    } catch {
      // the width just is not remembered
    }
  }
  function reset() {
    width = Math.min(max, Math.max(min, initial))
    remember()
  }
  function key(e: KeyboardEvent) {
    const next = widthAfterKey(width, e.key, e.shiftKey, side, min, max, axis)
    if (next === null) return
    e.preventDefault()
    width = next
    remember()
  }

  onMount(() => {
    try {
      const saved = Number(localStorage.getItem('split:' + name))
      if (saved >= min && saved <= max) width = saved
    } catch {
      // no storage: the default width stays
    }
  })

  function down(e: PointerEvent) {
    const startX = axis === 'y' ? e.clientY : e.clientX
    const start = width
    const el = e.currentTarget as HTMLElement
    el.setPointerCapture(e.pointerId)
    const move = (ev: PointerEvent) => {
      const dx = (axis === 'y' ? ev.clientY : ev.clientX) - startX
      // The pane under a horizontal handle grows as the handle goes up.
      const raw = axis === 'y' ? start - dx : side === 'left' ? start + dx : start - dx
      width = Math.min(max, Math.max(min, raw))
      folding = !!oncollapse && wantsCollapse(raw, min)
    }
    // A drag ends on release or when the system takes the pointer away (a touch turned into a scroll, a dialog opened,
    // the window lost focus). Before: only release was handled, so a cancelled drag left both listeners on the handle and
    // the width was not kept.
    const up = () => {
      el.removeEventListener('pointermove', move)
      el.removeEventListener('pointerup', up)
      el.removeEventListener('pointercancel', up)
      el.removeEventListener('lostpointercapture', up)
      remember()
      if (folding) {
        folding = false
        oncollapse?.()
      }
    }
    el.addEventListener('pointermove', move)
    el.addEventListener('pointerup', up)
    el.addEventListener('pointercancel', up)
    el.addEventListener('lostpointercapture', up)
  }
</script>

<!-- A focusable separator is the WAI-ARIA window splitter: it takes the arrow keys, so the lint rules for a plain separator do not apply. -->
<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
<div class="split" class:y={axis === 'y'} class:folding role="separator" aria-orientation={axis === 'y' ? 'horizontal' : 'vertical'} aria-label="Resize" aria-valuenow={Math.round(width)} aria-valuemin={min} aria-valuemax={max} tabindex="0" title={oncollapse ? 'Drag, or use the arrow keys. Double-click to reset. Drag far past the smallest size to fold it.' : 'Drag, or use the arrow keys. Double-click to reset.'} onpointerdown={down} ondblclick={reset} onkeydown={key}></div>

<style>
  .split { width: 5px; margin: 0 -2px; flex: none; cursor: col-resize; position: relative; z-index: var(--z-handle); touch-action: none; }
  /* The handle is drawn 5 px wide but takes a pointer 13 px wide. */
  .split::before { content: ''; position: absolute; inset: 0 -4px; }
  .split.y { width: auto; height: 5px; margin: -2px 0; cursor: row-resize; }
  .split.y::before { inset: -4px 0; }
  .split:focus-visible { outline: 2px solid var(--accent); outline-offset: -1px; }
  .split.folding, .split.folding:hover { background: var(--fail); }
  .split:hover, .split:active { background: color-mix(in srgb, var(--accent) 45%, transparent); }
</style>
