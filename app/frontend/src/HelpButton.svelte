<script lang="ts">
  // The round "?" every help in the window uses: Hint (an explanation on hover or focus) and ExprHelp (a panel on
  // click) both draw it, so there is one size, one centring and one hover. Draw a new help with this, never with a
  // "?" in a button of its own (lib/helpbutton.test.ts fails on one).
  // Before: Hint and ExprHelp each styled their own. ExprHelp's font line was invalid CSS, so its mark sat low and its
  // dot touched the ring, while Hint's was right.
  import type { HTMLButtonAttributes } from 'svelte/elements'

  let { el = $bindable(), label, ...rest }: HTMLButtonAttributes & { el?: HTMLButtonElement; label: string } = $props()
</script>

<button type="button" class="q" bind:this={el} aria-label={label} {...rest}>?</button>

<style>
  .q {
    flex: none; box-sizing: border-box; width: 15px; height: 15px; padding: 0; margin: 0; border-radius: 50%; border: 1px solid var(--line);
    background: transparent; color: var(--muted); font: 600 10px/1 var(--font, system-ui); cursor: help; vertical-align: middle;
    /* centred by the box, not by line height: the font's metrics put the mark low on Linux */
    display: inline-grid; place-items: center;
  }
  /* one that opens a panel is pressed, not just read */
  .q[aria-haspopup] { cursor: pointer; }
  .q:hover, .q:focus-visible { color: var(--text); border-color: var(--accent); }
  .q[aria-expanded='true'] { color: var(--accent); border-color: var(--accent); }
</style>
