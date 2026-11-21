<script lang="ts">
  // A set of radio buttons for one choice, each with its words: the button and its words are one target, the buttons
  // share a name, and the arrow keys move between them (the platform does that). `legend` is the visible question and
  // names the group for a screen reader; `label` names a group that has no visible question. `row` sets the options on
  // one line, `boxed` draws the edge round them (a form section).
  // `bind:value` for a group that owns its value; `onchange(value)` for one that does something when it changes.
  // Before: two places each wrote their own labels, gaps and fieldset.
  type Option = { value: string; label: string; hint?: string }

  let {
    value = $bindable(''),
    options,
    legend,
    label,
    row = false,
    boxed = false,
    disabled = false,
    onchange,
  }: {
    value?: string
    options: readonly Option[]
    legend?: string
    label?: string
    row?: boolean
    boxed?: boolean
    disabled?: boolean
    onchange?: (value: string) => void
  } = $props()

  const id = $props.id()
</script>

<fieldset class:row class:boxed {disabled} aria-label={legend ? undefined : label}>
  {#if legend}<legend>{legend}</legend>{/if}
  {#each options as o (o.value)}
    <label title={o.hint}>
      <input
        type="radio"
        name={`radio-${id}`}
        {disabled}
        value={o.value}
        checked={value === o.value}
        onchange={() => {
          value = o.value
          onchange?.(o.value)
        }} />
      {o.label}
    </label>
  {/each}
</fieldset>

<style>
  fieldset { display: flex; flex-direction: column; gap: var(--sp-2); min-width: 0; margin: 0; padding: 0; border: 0; }
  fieldset.row { flex-direction: row; flex-wrap: wrap; align-items: center; gap: var(--sp-2) var(--sp-4); }
  fieldset.boxed { padding: var(--sp-2) var(--sp-3); border: 1px solid var(--line); border-radius: var(--r-lg); }
  legend { padding: 0 6px; color: var(--muted); font-size: var(--fs-sm); }
  label { display: flex; align-items: center; gap: var(--sp-2); cursor: pointer; }
  fieldset:disabled label { cursor: default; }
  input { margin: 0; flex: none; cursor: inherit; }
</style>
