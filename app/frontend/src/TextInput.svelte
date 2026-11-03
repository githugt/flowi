<script lang="ts">
  // The one-line text field the forms share: the same height (it follows the density setting), border, radius and
  // accent ring everywhere. `mono` is for values that are code (paths, addresses, rules); `invalid` marks a value
  // that cannot be used. The parent places it (width, flex) with `style`; `el` is the input, for focusing it.
  let {
    value = $bindable(''),
    el = $bindable(),
    mono = false,
    invalid = false,
    placeholder = '',
    label,
    disabled = false,
    inputmode,
    type = 'text',
    readonly = false,
    list,
    id,
    title,
    style = '',
    oninput,
    onkeydown,
    onblur,
  }: {
    value?: string
    el?: HTMLInputElement
    mono?: boolean
    invalid?: boolean
    placeholder?: string
    /** The accessible name, for a field that has no visible label of its own. */
    label?: string
    disabled?: boolean
    inputmode?: 'text' | 'numeric' | 'decimal'
    /** text, or password for a secret that is typed. */
    type?: 'text' | 'password'
    readonly?: boolean
    /** The id of a datalist offering suggestions. */
    list?: string
    id?: string
    title?: string
    style?: string
    oninput?: (e: Event) => void
    onkeydown?: (e: KeyboardEvent) => void
    onblur?: (e: FocusEvent) => void
  } = $props()
</script>

<input bind:this={el} class="field" class:mono class:invalid bind:value {type} {readonly} {list} {id} {placeholder} {disabled} {inputmode} {title} {style} {oninput} {onkeydown} {onblur} aria-label={label} aria-invalid={invalid || undefined} spellcheck="false" autocomplete="off" />

<style>
  .field { box-sizing: border-box; width: 100%; min-width: 0; height: var(--ctl-h); padding: 0 8px; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); font: inherit; -webkit-user-select: text; user-select: text; }
  .mono { font: var(--fs-sm) var(--mono, ui-monospace, monospace); }
  .field:focus { border-color: var(--accent); }
  .field.invalid { border-color: var(--fail); }
  .field:disabled { opacity: 0.55; }
  .field::placeholder { color: var(--muted); }
</style>
