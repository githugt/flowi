<script lang="ts">
  import { ui } from './lib/ui.svelte'

  // A stripe of the status colour on the left, and a glyph for the shape: an error does not rest on red alone.
  const glyph = (level: string): string => (level === 'error' ? '✕' : level === 'warn' ? '!' : '✓')
</script>

<div class="toasts" aria-live="polite">
  {#each ui.toasts as t (t.id)}
    <div class="toast {t.level}"><span class="glyph" aria-hidden="true">{glyph(t.level)}</span>{t.text}</div>
  {/each}
</div>

<style>
  @keyframes rise { from { opacity: 0; transform: translateY(6px) scale(.985); } }
  .toasts { position: fixed; right: 16px; bottom: 16px; display: flex; flex-direction: column; gap: 8px; z-index: 70; }
  .toast { display: flex; align-items: baseline; gap: 8px; max-width: 380px; padding: 9px 14px 9px 12px; border-radius: var(--r-lg); background: var(--raised); border: 1px solid var(--line); border-left: 3px solid var(--accent); box-shadow: var(--shadow); animation: rise var(--t-slow) var(--ease-out); }
  .toast.error { border-left-color: var(--fail); }
  .toast.warn { border-left-color: var(--warn); }
  .glyph { flex: none; width: 1em; text-align: center; font-weight: 700; color: var(--accent); }
  .error .glyph { color: var(--fail); }
  .warn .glyph { color: var(--warn); }
  @media (prefers-reduced-motion: reduce) { .toast { animation: none; } }
</style>
