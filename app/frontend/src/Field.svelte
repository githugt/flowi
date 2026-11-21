<script lang="ts">
  import type { Snippet } from 'svelte'

  // A label above one control (a text box, a list, a path field). The whole thing is a <label>, so a click on the
  // words focuses the control and a screen reader names it.
  // Before: twelve screens each wrote `.f` or `.field` and `.label` with the same three lines of CSS, and the controls
  // in some inherited the label's small size while the others did not.
  let {
    label,
    wide = false,
    children,
  }: {
    label: string
    /** Takes the whole row of a grid. */
    wide?: boolean
    children: Snippet
  } = $props()
</script>

<label class="field" class:wide>
  <span class="label">{label}</span>
  {@render children()}
</label>

<style>
  .field { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
  .label { color: var(--muted); font-size: var(--fs-xs); }
  .wide { grid-column: 1 / -1; }
</style>
