<script lang="ts">
  // The two things a file can tell an editor: it changed on disk under
  // unsaved edits, or saving would drop its comments.
  let {
    onDisk,
    needsDrop,
    revert,
    saveDropping,
    declineDrop,
  }: { onDisk: boolean; needsDrop: boolean; revert: () => void; saveDropping: () => void; declineDrop: () => void } = $props()
</script>

{#if onDisk}
  <p class="bar warn">This file changed on disk. <button onclick={revert}>Load from disk</button></p>
{/if}
{#if needsDrop}
  <p class="bar warn">
    Saving rewrites this file in Flowi's own format, which drops its comments.
    <button onclick={saveDropping}>Save and drop comments</button>
    <button onclick={declineDrop}>Cancel</button>
  </p>
{/if}

<style>
  .bar { margin: 0; padding: 6px 12px; font-size: var(--fs-sm); background: color-mix(in srgb, var(--warn) 15%, transparent); display: flex; gap: 8px; align-items: center; }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 2px 10px; font: inherit; }
  button:hover { border-color: var(--accent); }
</style>
