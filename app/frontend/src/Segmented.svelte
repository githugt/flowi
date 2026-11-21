<script lang="ts">
  // One choice out of a few that are all shown at once (Canvas or Code, All, Sent or Received), as joined buttons.
  // A radio group: one tab stop, the arrow keys move and choose. Not for tabs that switch a panel (Tabs), and not for
  // an on or off setting (Toggle).
  // Before: four hand-made copies, each with its own border, radius and selected colour; two had no radio roles.
  import { radiogroup } from './lib/radiogroup'

  export interface SegmentItem {
    id: string
    label: string
    title?: string
    disabled?: boolean
  }

  let {
    items,
    value,
    onpick,
    label,
    size = 'sm',
  }: {
    items: SegmentItem[]
    value: string
    onpick: (id: string) => void
    /** What the group chooses, said to a screen reader. */
    label: string
    size?: 'sm' | 'xs'
  } = $props()
</script>

<div class="seg {size}" role="radiogroup" use:radiogroup aria-label={label}>
  {#each items as s (s.id)}
    <button type="button" role="radio" aria-checked={value === s.id} class:on={value === s.id} disabled={s.disabled} title={s.title} onclick={() => onpick(s.id)}>{s.label}</button>
  {/each}
</div>

<style>
  .seg { display: inline-flex; flex: none; border: 1px solid var(--line); border-radius: var(--r-md); overflow: hidden; }
  button { background: none; border: 0; border-radius: 0; color: var(--muted); font: inherit; font-size: var(--fs-sm); padding: 0 10px; cursor: pointer; }
  .sm button { height: var(--ctl-h-sm); }
  .xs button { height: 20px; font-size: var(--fs-xs); }
  button + button { border-left: 1px solid var(--line); }
  button:hover:not(:disabled):not(.on) { color: var(--text); background: var(--tint-hover); }
  button.on { background: var(--tint-sel); color: var(--text); }
  button:disabled { opacity: var(--op-disabled); cursor: default; }
</style>
