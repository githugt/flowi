<script lang="ts">
  import ExprHelp from './ExprHelp.svelte'
  import HiddenChars from './HiddenChars.svelte'
  // A text input that keeps what the user is typing while Go round-trips the
  // edit: the prop only replaces the text when the field is not focused.
  let {
    value,
    oninput,
    onenter,
    label = '',
    help = '',
    placeholder = '',
    mono = false,
    list,
  }: {
    value: string
    oninput?: (text: string) => void
    onenter?: (text: string) => void
    label?: string
    help?: string // an ExprHelp context: shows a ? beside the label
    placeholder?: string
    mono?: boolean
    list?: string
  } = $props()

  let local = $state('')
  let focused = $state(false)
  $effect(() => {
    if (!focused) local = value
  })
</script>

<label class="field">
  {#if label}<span class="label">{label}{#if help}<ExprHelp context={help} />{/if}</span>{/if}
  <div class="box">
    <input
      class:mono
      value={local}
      {placeholder}
      {list}
      spellcheck="false"
      onfocus={() => (focused = true)}
      onblur={() => (focused = false)}
      oninput={(e) => {
        local = e.currentTarget.value
        oninput?.(local)
      }}
      onkeydown={(e) => e.key === 'Enter' && onenter?.(local)}
    />
    <HiddenChars value={local} onclean={(t) => { local = t; oninput?.(t) }} />
  </div>
</label>

<style>
  .field { display: flex; flex-direction: column; gap: 3px; }
  .label { color: var(--muted); font-size: var(--fs-xs); }
  .box { position: relative; display: flex; }
  .box input { flex: 1; }
  input {
    background: var(--bg);
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    padding: 5px 8px;
    font: inherit;
    min-width: 0;
    -webkit-user-select: text;
    user-select: text;
  }
  input:focus { border-color: var(--accent); }
  .mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: var(--fs-sm); }
</style>
