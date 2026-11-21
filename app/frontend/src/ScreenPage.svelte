<script lang="ts">
  import type { Snippet } from 'svelte'

  // The page of a screen opened in a tab (Secrets, Trash, Settings…): it fills the tab, scrolls by itself, and keeps
  // its text to a readable width, with the same space around it on every screen.
  // Before: ten screens each wrote `.page` with the same padding and their own max-width of 760, 820 or 860 px.
  let {
    width = 820,
    column = false,
    children,
  }: {
    /** The widest the content gets, in px; null for the whole tab. */
    width?: number | null
    /** The content is one column with a gap between its blocks. */
    column?: boolean
    children: Snippet
  } = $props()
</script>

<div class="page" class:column style:max-width={width === null ? undefined : `${width}px`}>{@render children()}</div>

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: 28px 32px 56px; }
  .column { display: flex; flex-direction: column; gap: 10px; }
</style>
