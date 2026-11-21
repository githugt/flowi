<script lang="ts">
  // A "?" beside a heading or label: the explanation shows on hover or keyboard focus,
  // so forms stay short and the words are there for whoever wants them.
  import type { Snippet } from 'svelte'
  import Tip from './Tip.svelte'

  let { children }: { children: Snippet } = $props()
</script>

<span class="hint">
  <Tip>
    {#snippet content()}{@render children()}{/snippet}
    <button type="button" class="q" aria-label="Explain">?</button>
  </Tip>
</span>

<style>
  .hint { display: inline-flex; vertical-align: middle; margin-left: var(--sp-1); font-weight: 400; text-transform: none; letter-spacing: 0; }
  .q { width: 15px; height: 15px; padding: 0; border-radius: 50%; border: 1px solid var(--line); background: transparent; color: var(--muted); font: 600 10px/1 var(--font, system-ui); cursor: help;
    /* centred by the box, not by line height: the font's metrics put the mark low on Linux */
    display: inline-grid; place-items: center; }
  .q:hover, .q:focus-visible { color: var(--text); border-color: var(--accent); }
</style>
