<script lang="ts">
  import type { Snippet } from 'svelte'

  // A tick box with its words: the box and the label are one target, set on one line, and the box takes the accent
  // colour (app.css). The words are what goes between the tags; a box that stands in a table cell has none and is named
  // by `label` instead. Text size and colour are the surroundings': put the Checkbox in an element that sets them.
  // `bind:checked` for a box that owns a value; `onchange` for one whose value is worked out (a list that holds it,
  // a switch stored the other way round).
  // Before: sixteen boxes each wrote their own label, gap and alignment, with gaps of 4, 6 and 8 px.
  let {
    checked = $bindable(false),
    onchange,
    label,
    title,
    disabled = false,
    children,
  }: {
    checked?: boolean
    onchange?: (checked: boolean) => void
    /** The name for a screen reader, when there are no words beside the box. */
    label?: string
    title?: string
    disabled?: boolean
    children?: Snippet
  } = $props()
</script>

<label class="check" class:off={disabled} {title}>
  <input
    type="checkbox"
    {checked}
    {disabled}
    aria-label={label}
    onchange={(e) => {
      checked = e.currentTarget.checked
      onchange?.(checked)
    }} />
  {#if children}<span>{@render children()}</span>{/if}
</label>

<style>
  .check { display: inline-flex; align-items: center; gap: var(--sp-2); font: inherit; color: inherit; cursor: pointer; }
  .check.off { cursor: default; }
  input { margin: 0; flex: none; cursor: inherit; }
</style>
