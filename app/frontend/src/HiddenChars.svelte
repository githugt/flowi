<script lang="ts">
  import Popover from './Popover.svelte'
  import Button from './Button.svelte'
  // A button that shows up inside a field only when its text has characters
  // that cannot be seen (a zero-width space, a non-breaking space, a space at
  // the end). It lists them and can remove them.
  import { findHidden, revealHidden, stripHidden } from './lib/hidden'

  let { value, onclean }: { value: string; onclean: (text: string) => void } = $props()

  let open = $state(false)
  let at = $state({ x: 0, y: 0 })
  const found = $derived(findHidden(value))
  $effect(() => {
    if (!found.length) open = false
  })
</script>

{#if found.length}
  <span class="host">
    <button type="button" class="hid" title="This text has characters you cannot see" aria-label="Hidden characters" aria-haspopup="dialog" aria-expanded={open} onclick={(e) => { e.preventDefault(); const r = e.currentTarget.getBoundingClientRect(); at = { x: r.right, y: r.bottom }; open = !open }}>
      ⚠ {found.length}
    </button>
  </span>
  {#if open}
    <Popover x={at.x} y={at.y} end width={300} height={220} label="Hidden characters" onclose={() => (open = false)}>
      <p>Hidden characters in this text:</p>
      <ul>{#each found as h}<li><code>{h.code}</code> {h.name}, at {h.index}</li>{/each}</ul>
      <code class="shown">{revealHidden(value)}</code>
      <Button type="button" variant="primary" small onclick={(e) => { e.preventDefault(); open = false; onclean(stripHidden(value)) }}>Remove them</Button>
    </Popover>
  {/if}
{/if}

<style>
  /* The popover is outside .host: a transformed parent would make its fixed position relative to the parent. */
  .host { position: absolute; right: 4px; top: 50%; transform: translateY(-50%); z-index: 5; }
  .hid { background: color-mix(in srgb, var(--warn) 18%, var(--panel)); color: var(--warn); border: 1px solid var(--warn); border-radius: var(--r-pill); padding: 0 7px; font: 600 10px/16px inherit; cursor: pointer; }
  p { margin: 0 0 4px; }
  ul { margin: 0 0 6px; padding-left: 16px; max-height: 120px; overflow: auto; }
  code { font: var(--fs-xs) var(--mono); }
  .shown { display: block; background: var(--bg); border-radius: var(--r-sm); padding: 3px 6px; margin-bottom: 8px; word-break: break-all; -webkit-user-select: text; user-select: text; }
</style>
