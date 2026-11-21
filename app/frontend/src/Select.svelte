<script lang="ts">
  import type { Snippet } from 'svelte'

  // A native select with the app's one look: the field edge, the chevron (app.css) and the control height, so a select
  // sits level with the buttons and inputs beside it. The options go between the tags, as in HTML, and the platform's
  // own list opens, keyboard and screen reader included.
  // `bind:value` for a select that owns a value; `onchange` (it gets the event, so `e.currentTarget.value` is the new
  // value) for one whose value is worked out or that does something when it changes.
  // `tone="raised"` is for a select that stands among buttons in a bar (it is filled like them), `size="sm"` for a dense
  // row, `fill` for one that takes the room it is given.
  // Before: 27 files each wrote their own select rule (padding of 3, 4 and 5 px, three heights, five focus colours).
  let {
    value = $bindable(),
    onchange,
    label,
    title,
    id,
    disabled = false,
    size = 'md',
    tone = 'field',
    fill = false,
    children,
  }: {
    value?: unknown
    onchange?: (e: Event & { currentTarget: HTMLSelectElement }) => void
    /** The name for a screen reader, when no <label> points at the select. */
    label?: string
    title?: string
    id?: string
    disabled?: boolean
    size?: 'md' | 'sm'
    tone?: 'field' | 'raised'
    fill?: boolean
    children?: Snippet
  } = $props()
</script>

<select bind:value {id} {title} {disabled} aria-label={label} class={[size, tone, fill && 'fill']} onchange={(e) => onchange?.(e)}>
  {@render children?.()}
</select>

<style>
  select { min-height: var(--ctl-h); }
  .sm { min-height: 0; height: var(--ctl-h-sm); padding-block: 0; font-size: var(--fs-xs); }
  .raised { background-color: var(--raised); border: 0; box-shadow: inset 0 0 0 1px var(--line); font-weight: 500; }
  .raised:hover:not(:disabled) { background-color: var(--raised-3); }
  .fill { flex: 1; min-width: 0; width: 100%; }
</style>
