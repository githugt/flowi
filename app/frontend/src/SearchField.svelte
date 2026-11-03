<script lang="ts">
  import Icon from './Icon.svelte'

  // The filter field used above lists: a search icon, no browser clear button, an accent ring while it has the focus.
  // The parent sizes it (width, margin) by wrapping it or with a style; `el` is the input, for focusing it.
  let {
    value = $bindable(''),
    el = $bindable(),
    placeholder = 'Filter',
    label,
    onkeydown,
    small = false,
    style = '',
  }: { value?: string; el?: HTMLInputElement; placeholder?: string; label: string; onkeydown?: (e: KeyboardEvent) => void; small?: boolean; style?: string } = $props()
</script>

<label class="search" class:small {style}>
  <Icon name="search" size={14} /><input bind:this={el} type="search" {placeholder} aria-label={label} autocomplete="off" spellcheck="false" bind:value {onkeydown} />
</label>

<style>
  .search { display: flex; align-items: center; gap: 6px; height: var(--ctl-h); padding: 0 8px; box-sizing: border-box; color: var(--muted); background: var(--bg); border: 1px solid var(--line); border-radius: var(--r-md); }
  .small { height: var(--ctl-h-sm); }
  .search:focus-within { border-color: var(--accent); color: var(--text); }
  input { flex: 1; min-width: 0; background: none; border: 0; outline: none; color: var(--text); font: inherit; font-size: var(--fs-sm); }
  input::-webkit-search-cancel-button { display: none; }
</style>
