<script lang="ts">
  // A "?" beside a heading or label: the explanation shows on hover or keyboard focus,
  // so forms stay short and the words are there for whoever wants them.
  import type { Snippet } from 'svelte'

  let { children }: { children: Snippet } = $props()
</script>

<span class="hint">
  <button type="button" class="q" aria-label="Explain">?</button>
  <span class="pop" role="tooltip">{@render children()}</span>
</span>

<style>
  .hint { position: relative; display: inline-flex; vertical-align: middle; margin-left: var(--sp-1); font-weight: 400; text-transform: none; letter-spacing: 0; }
  .q { width: 15px; height: 15px; padding: 0; border-radius: 50%; border: 1px solid var(--line); background: transparent; color: var(--muted); font: 600 10px/13px var(--font, system-ui); cursor: help; }
  .q:hover, .q:focus-visible { color: var(--text); border-color: var(--accent); }
  .pop {
    display: none; position: absolute; z-index: 40; top: calc(100% + 6px); right: -8px; width: 260px; padding: var(--sp-2) 10px;
    background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-lg); box-shadow: var(--shadow);
    font-size: var(--fs-sm); line-height: 1.45;
  }
  .hint:hover .pop, .q:focus-visible + .pop { display: block; }
</style>
