<script lang="ts">
  // What a pane or section shows while it has nothing: its icon in a soft disc, what is missing,
  // and a line on how to get it, with an action under it when there is one.
  import type { Snippet } from 'svelte'
  import Icon from './Icon.svelte'

  let { icon, title, hint = '', compact = false, children }: { icon: string; title: string; hint?: string; compact?: boolean; children?: Snippet } = $props()
</script>

<div class="empty" class:compact>
  <span class="disc"><Icon name={icon} size={compact ? 26 : 36} /></span>
  <strong>{title}</strong>
  {#if hint}<span class="hint">{hint}</span>{/if}
  {#if children}<div class="act">{@render children()}</div>{/if}
</div>

<style>
  .empty { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 6px; text-align: center; padding: 24px; min-height: 0; }
  .disc { display: grid; place-items: center; width: 76px; height: 76px; border-radius: 50%; background: color-mix(in srgb, var(--muted) 12%, transparent); color: var(--muted); margin-bottom: 6px; }
  .compact .disc { width: 56px; height: 56px; }
  strong { color: var(--muted); font-size: var(--fs-lg); font-weight: 600; }
  .compact strong { font-size: var(--fs-md); }
  .hint { color: var(--muted); font-size: var(--fs-sm); max-width: 320px; line-height: 1.45; }
  .act { margin-top: 6px; }
  .act :global(button) { background: transparent; color: var(--accent); border: 1px solid color-mix(in srgb, var(--accent) 45%, transparent); border-radius: var(--r-md); padding: 4px 12px; font: inherit; }
  .act :global(button:hover) { border-color: var(--accent); }
</style>
