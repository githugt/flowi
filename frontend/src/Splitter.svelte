<script lang="ts">
  import { onMount } from 'svelte'

  // A drag handle between two panes. It resizes the pane it sits next to
  // (`side`: which side of the handle that pane is on), keeps the width in
  // localStorage under `name`, and reports it through the bindable `width`.
  let {
    width = $bindable(300),
    name,
    side = 'left',
    min = 180,
    max = 720,
  }: { width?: number; name: string; side?: 'left' | 'right'; min?: number; max?: number } = $props()

  onMount(() => {
    try {
      const saved = Number(localStorage.getItem('split:' + name))
      if (saved >= min && saved <= max) width = saved
    } catch {
      // no storage: the default width stays
    }
  })

  function down(e: PointerEvent) {
    const startX = e.clientX
    const start = width
    const el = e.currentTarget as HTMLElement
    el.setPointerCapture(e.pointerId)
    const move = (ev: PointerEvent) => {
      const dx = ev.clientX - startX
      width = Math.min(max, Math.max(min, side === 'left' ? start + dx : start - dx))
    }
    const up = () => {
      el.removeEventListener('pointermove', move)
      el.removeEventListener('pointerup', up)
      try {
        localStorage.setItem('split:' + name, String(width))
      } catch {
        // the width just is not remembered
      }
    }
    el.addEventListener('pointermove', move)
    el.addEventListener('pointerup', up)
  }
</script>

<div class="split" role="separator" aria-orientation="vertical" aria-label="Resize" onpointerdown={down}></div>

<style>
  .split { width: 5px; margin: 0 -2px; flex: none; cursor: col-resize; position: relative; z-index: 5; touch-action: none; }
  .split:hover, .split:active { background: color-mix(in srgb, var(--accent) 45%, transparent); }
</style>
