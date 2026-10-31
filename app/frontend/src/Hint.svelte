<script lang="ts">
  // A "?" beside a heading or label: the explanation shows on hover or keyboard focus,
  // so forms stay short and the words are there for whoever wants them.
  import type { Snippet } from 'svelte'

  let { children }: { children: Snippet } = $props()

  const popWidth = 260
  const edge = 8
  let button: HTMLButtonElement | undefined = $state()
  let left = $state(0)
  let top = $state(0)

  // The popup is fixed to the window, so a scrolling or clipping panel around the "?" cannot cut it off;
  // it opens below the mark and is kept inside the window's edges.
  function place() {
    if (!button) return
    const r = button.getBoundingClientRect()
    const fits = Math.max(edge, Math.min(r.right + edge - popWidth, window.innerWidth - popWidth - edge))
    left = fits
    top = r.bottom + 6
  }
</script>

<span class="hint" onmouseenter={place} onfocusin={place} role="presentation">
  <button bind:this={button} type="button" class="q" aria-label="Explain">?</button>
  <span class="pop" role="tooltip" style="left:{left}px;top:{top}px">{@render children()}</span>
</span>

<style>
  .hint { position: relative; display: inline-flex; vertical-align: middle; margin-left: var(--sp-1); font-weight: 400; text-transform: none; letter-spacing: 0; }
  .q { width: 15px; height: 15px; padding: 0; border-radius: 50%; border: 1px solid var(--line); background: transparent; color: var(--muted); font: 600 10px/1 var(--font, system-ui); cursor: help;
    /* centred by the box, not by line height: the font's metrics put the mark low on Linux */
    display: inline-grid; place-items: center; }
  .q:hover, .q:focus-visible { color: var(--text); border-color: var(--accent); }
  .pop {
    display: none; position: fixed; z-index: 40; width: 260px; padding: var(--sp-2) 10px;
    background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-lg); box-shadow: var(--shadow);
    font-size: var(--fs-sm); line-height: 1.45;
  }
  .hint:hover .pop, .q:focus-visible + .pop { display: block; }
</style>
