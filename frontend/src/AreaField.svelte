<script lang="ts">
  import EmptyState from './EmptyState.svelte'
  // A multi-line input that keeps what the user is typing while Go round-trips
  // the edit: the prop only replaces the text when the field is not focused.
  let {
    value,
    oninput,
    placeholder = '',
    mono = true,
    label = '',
    rows = 8,
    empty,
  }: {
    value: string
    oninput: (text: string) => void
    placeholder?: string
    mono?: boolean
    label?: string
    rows?: number
    /** Drawn inside the box while it is empty and not focused; a click goes through to the box. */
    empty?: { icon: string; title: string; hint: string }
  } = $props()

  let local = $state('')
  let focused = $state(false)
  $effect(() => {
    if (!focused) local = value
  })
</script>

<label class="f">
  {#if label}<span>{label}</span>{/if}
  <div class="box">
  {#if empty && !local && !focused}
    <div class="inside" aria-hidden="true"><EmptyState icon={empty.icon} title={empty.title} hint={empty.hint} compact /></div>
  {/if}
  <textarea
    class:mono
    {rows}
    {placeholder}
    spellcheck="false"
    value={local}
    onfocus={() => (focused = true)}
    onblur={() => (focused = false)}
    oninput={(e) => {
      local = e.currentTarget.value
      oninput(local)
    }}
  ></textarea>
  </div>
</label>

<style>
  .f { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); flex: 1; min-height: 0; }
  .box { position: relative; flex: 1; min-height: 0; display: flex; }
  .inside { position: absolute; inset: 0; display: flex; pointer-events: none; overflow: hidden; }
  textarea {
    flex: 1;
    min-height: 120px;
    resize: none;
    background: var(--bg);
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    padding: 8px;
    font: inherit;
    font-size: var(--fs-md);
    -webkit-user-select: text;
    user-select: text;
  }
  .mono { font: 12.5px/1.45 ui-monospace, Menlo, monospace; }
  textarea:focus { border-color: var(--accent); }
</style>
