<script lang="ts">
  import { describedBy, errorId } from './lib/fielderror'
  import { isEnter } from './lib/enterkey'
  import { REVEAL_MS, hasTemplate, maskField } from './lib/masked'
  import Button from './Button.svelte'
  import HiddenChars from './HiddenChars.svelte'
  // The one-line text field the forms share: the same height (it follows the density setting), border, radius and
  // accent ring everywhere. `mono` is for values that are code (paths, addresses, rules); `invalid` marks a value
  // that cannot be used. The parent places it (width, flex) with `style`; `el` is the input, for focusing it.
  // Two ways to use it. Bound (`bind:value`): the text is the parent's. Reported (`ontext`): the value comes from
  // Go and each edit goes back to it, and since Go answers a moment later, the text being typed is kept while the
  // input has the focus. A reported field also warns about characters that cannot be seen, and a `secret` one shows
  // dots with an eye to reveal them. A label above it is `Field`'s.
  // Before: TextField was a second component with its own height rule, background and focus colour.
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
    describedby,
    error = '',
    title,
    style = '',
    oninput,
    onkeydown,
    onblur,
    ontext,
    onenter,
    secret = false,
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
    /** The id of the element that says what is wrong with the value. */
    describedby?: string
    /** What is wrong with the value: shown under the field, announced once, and the field is marked invalid. */
    error?: string
    title?: string
    style?: string
    oninput?: (e: Event) => void
    onkeydown?: (e: KeyboardEvent) => void
    onblur?: (e: FocusEvent) => void
    /** Reported mode: called with the text after each edit; `value` is then only what Go last said. */
    ontext?: (text: string) => void
    /** Called with the text when Enter is pressed (not while an input method is composing). */
    onenter?: (text: string) => void
    /** Shows dots for a typed value, with an eye to reveal it; a {{secret.x}} reference is shown as it is. */
    secret?: boolean
  } = $props()
  const messageId = errorId()

  let typed = $state<string | undefined>()
  let revealed = $state(false)
  let hideTimer: ReturnType<typeof setTimeout> | undefined
  const shown = $derived(typed ?? value)
  const masked = $derived(maskField(shown, secret, revealed))
  const boxed = $derived(ontext !== undefined || secret)

  function edit(text: string) {
    if (ontext) {
      typed = text
      ontext(text)
    } else value = text
  }
  function toggleReveal() {
    revealed = !revealed
    clearTimeout(hideTimer)
    if (revealed) hideTimer = setTimeout(() => (revealed = false), REVEAL_MS)
  }
</script>

{#snippet input()}
  <input bind:this={el} class="field" class:mono class:invalid={invalid || !!error} value={shown} type={masked ? 'password' : type} {readonly} {list} {id} {placeholder} {disabled} {inputmode} {title} {style}
    oninput={(e) => { edit(e.currentTarget.value); oninput?.(e) }}
    onkeydown={(e) => { if (isEnter(e)) onenter?.(shown); onkeydown?.(e) }}
    onblur={(e) => { typed = undefined; onblur?.(e) }}
    aria-label={label} aria-invalid={invalid || !!error || undefined} aria-describedby={describedBy(describedby, error, messageId)} spellcheck="false" autocomplete="off" />
{/snippet}
{#if boxed}
  <div class="box">
    {@render input()}
    {#if secret && !hasTemplate(shown)}
      <Button small aria-pressed={revealed} title={revealed ? 'Hide the value' : 'Show the value'} onclick={toggleReveal}>{revealed ? 'Hide' : 'Show'}</Button>
    {/if}
    <HiddenChars value={shown} onclean={(t) => (ontext ? ontext(t) : (value = t))} />
  </div>
{:else}
  {@render input()}
{/if}
{#if error}<span class="errmsg" id={messageId} role="alert">{error}</span>{/if}

<style>
  .box { position: relative; display: flex; gap: 4px; width: 100%; }
  .box .field { flex: 1; }
  .field { box-sizing: border-box; width: 100%; min-width: 0; height: var(--ctl-h); padding: 0 8px; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); font: inherit; -webkit-user-select: text; user-select: text; }
  .mono { font: var(--fs-sm) var(--mono, ui-monospace, monospace); }
  .field:focus { border-color: var(--accent); }
  .field.invalid { border-color: var(--fail); }
  .errmsg { display: block; margin-top: 2px; color: var(--fail); font-size: var(--fs-xs); }
  .field:disabled { opacity: var(--op-disabled-field); }
  .field::placeholder { color: var(--muted); }
</style>
