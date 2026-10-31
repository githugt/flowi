<script lang="ts">
  // A coloured code box for a body or a message: AreaField's place and props, with the code editor
  // inside. Bodies were plain text areas, so JSON and its {{templates}} showed uncoloured.
  import CodeEditor from './CodeEditor.svelte'
  import EmptyState from './EmptyState.svelte'
  import type { CodeLang } from './lib/highlight'

  let {
    value,
    oninput,
    lang = 'json',
    label = '',
    minHeight = 140,
    empty,
  }: {
    value: string
    oninput: (text: string) => void
    lang?: CodeLang
    label?: string
    minHeight?: number
    /** Drawn inside the box while it is empty and not focused; a click goes through to the editor. */
    empty?: { icon: string; title: string; hint: string }
  } = $props()

  let focused = $state(false)
  let editor = $state<CodeEditor>()
</script>

<div class="f">
  {#if label}<span class="label">{label}</span>{/if}
  <div
    class="box"
    class:focus={focused}
    style:min-height={`${minHeight}px`}
    onfocusin={() => (focused = true)}
    onfocusout={() => (focused = false)}
    role="presentation"
  >
    <CodeEditor bind:this={editor} {value} {lang} holdWhileFocused onchange={oninput} />
    {#if empty && !value && !focused}
      <button class="inside" type="button" tabindex="-1" aria-label={empty.title} onclick={() => editor?.focus()}>
        <EmptyState icon={empty.icon} title={empty.title} hint={empty.hint} compact />
      </button>
    {/if}
  </div>
</div>

<style>
  .f { display: flex; flex-direction: column; gap: 3px; flex: 1; min-height: 0; }
  .label { color: var(--muted); font-size: var(--fs-xs); }
  .box { position: relative; flex: 1; min-height: 0; display: flex; flex-direction: column; border: 1px solid var(--line); border-radius: var(--r-md); overflow: hidden; }
  .box.focus { border-color: var(--accent); }
  .inside { position: absolute; inset: 0; display: flex; background: var(--bg); border: 0; padding: 0; color: inherit; font: inherit; cursor: text; }
</style>
