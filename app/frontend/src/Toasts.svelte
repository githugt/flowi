<script lang="ts">
  import IconButton from './IconButton.svelte'
  import { ui } from './lib/ui.svelte'
  import Button from './Button.svelte'

  // A stripe of the status colour on the left, and a glyph for the shape: an error does not rest on red alone.
  const glyph = (level: string): string => (level === 'error' ? '✕' : level === 'warn' ? '!' : '✓')
</script>

<div class="toasts" aria-live="polite">
  {#each ui.toasts as t (t.id)}
    <div class="toast {t.level}" role={t.level === 'error' ? 'alert' : undefined} onpointerenter={() => ui.pause(t.id)} onpointerleave={() => ui.resume(t.id)} onfocusin={() => ui.pause(t.id)} onfocusout={() => ui.resume(t.id)}>
      <span class="glyph" aria-hidden="true">{glyph(t.level)}</span><span class="text">{t.text}{#if (t.count ?? 1) > 1} <span class="count" title="The same message came {t.count} times">×{t.count}</span>{/if}</span>
      {#if t.action}<Button small onclick={() => (ui.dismissToast(t.id), t.action?.run())}>{t.action.label}</Button>{/if}
      <IconButton icon="close" size={12} label="Dismiss" onclick={() => ui.dismissToast(t.id)} />
    </div>
  {/each}
</div>

<style>
  @keyframes rise { from { opacity: 0; transform: translateY(6px) scale(.985); } }
  .toasts { position: fixed; right: 16px; bottom: 16px; display: flex; flex-direction: column; gap: 8px; z-index: var(--z-top); }
  .toast { display: flex; align-items: baseline; gap: 8px; max-width: 380px; padding: 9px 14px 9px 12px; border-radius: var(--r-lg); background: var(--raised); border: 1px solid var(--line); border-left: 3px solid var(--accent); box-shadow: var(--shadow); animation: rise var(--t-slow) var(--ease-out); }
  .toast.error { border-left-color: var(--fail); }
  .toast.warn { border-left-color: var(--warn); }
  .text { flex: 1; min-width: 0; overflow-wrap: anywhere; -webkit-user-select: text; user-select: text; }
  .count { color: var(--muted); font-size: var(--fs-xs); white-space: nowrap; }
  .glyph { flex: none; width: 1em; text-align: center; font-weight: 700; color: var(--accent); }
  .error .glyph { color: var(--fail); }
  .warn .glyph { color: var(--warn); }
  @media (prefers-reduced-motion: reduce) { .toast { animation: none; } }
</style>
