<script lang="ts">
  import type { Snippet } from 'svelte'

  // A full-width strip under a header that says something about the whole screen and offers what to do about it:
  // "this file changed on disk", "the run failed", "an environment is missing". The tone is the colour and the role:
  // a failure is announced at once (alert), the rest wait their turn (status).
  // Before: six strips each wrote the layout, a tinted background, the border and the text and buttons row.
  let {
    tone = 'info',
    icon,
    children,
    actions,
  }: {
    tone?: 'info' | 'warn' | 'fail'
    /** A mark before the text, in the tone's colour. */
    icon?: string
    children: Snippet
    /** Buttons at the far end. */
    actions?: Snippet
  } = $props()
</script>

<div class="banner {tone}" role={tone === 'fail' ? 'alert' : 'status'}>
  {#if icon}<span class="icon" aria-hidden="true">{icon}</span>{/if}
  <div class="text">{@render children()}</div>
  {#if actions}{@render actions()}{/if}
</div>

<style>
  .banner { flex: none; display: flex; align-items: center; gap: var(--sp-2); padding: 6px 12px; font-size: var(--fs-sm); border-bottom: 1px solid var(--line); }
  .info { background: color-mix(in srgb, var(--accent) 14%, var(--panel)); }
  .warn { background: color-mix(in srgb, var(--warn) 15%, var(--panel)); }
  .fail { background: color-mix(in srgb, var(--fail) 10%, var(--panel)); color: var(--fail); }
  .icon { color: var(--warn); }
  .fail .icon { color: var(--fail); }
  .text { flex: 1; min-width: 0; }
</style>
