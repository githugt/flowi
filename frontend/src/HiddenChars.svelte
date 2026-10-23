<script lang="ts">
  // A button that shows up inside a field only when its text has characters
  // that cannot be seen (a zero-width space, a non-breaking space, a space at
  // the end). It lists them and can remove them.
  import { findHidden, revealHidden, stripHidden } from './lib/hidden'

  let { value, onclean }: { value: string; onclean: (text: string) => void } = $props()

  let open = $state(false)
  const found = $derived(findHidden(value))
  $effect(() => {
    if (!found.length) open = false
  })
</script>

<svelte:window onpointerdown={() => (open = false)} />

{#if found.length}
  <span class="host">
    <button type="button" class="hid" title="This text has characters you cannot see" aria-label="Hidden characters" aria-expanded={open} onpointerdown={(e) => e.stopPropagation()} onclick={(e) => { e.preventDefault(); open = !open }}>
      ⚠ {found.length}
    </button>
    {#if open}
      <div class="pop" role="dialog" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
        <p>Hidden characters in this text:</p>
        <ul>{#each found as h}<li><code>{h.code}</code> {h.name}, at {h.index}</li>{/each}</ul>
        <code class="shown">{revealHidden(value)}</code>
        <button type="button" class="fix" onclick={(e) => { e.preventDefault(); open = false; onclean(stripHidden(value)) }}>Remove them</button>
      </div>
    {/if}
  </span>
{/if}

<style>
  .host { position: absolute; right: 4px; top: 50%; transform: translateY(-50%); z-index: 5; }
  .hid { background: color-mix(in srgb, var(--warn) 18%, var(--panel)); color: var(--warn); border: 1px solid var(--warn); border-radius: var(--r-pill); padding: 0 7px; font: 600 10px/16px inherit; cursor: pointer; }
  .pop { position: absolute; right: 0; top: calc(100% + 6px); width: 300px; padding: 10px 12px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); box-shadow: 0 8px 24px rgb(0 0 0 / 0.35); font-size: var(--fs-sm); color: var(--text); transform: none; }
  p { margin: 0 0 4px; }
  ul { margin: 0 0 6px; padding-left: 16px; max-height: 120px; overflow: auto; }
  code { font: var(--fs-xs) var(--mono); }
  .shown { display: block; background: var(--bg); border-radius: var(--r-sm); padding: 3px 6px; margin-bottom: 8px; word-break: break-all; -webkit-user-select: text; user-select: text; }
  .fix { background: var(--accent); color: var(--on-accent); border: 0; border-radius: var(--r-md); padding: 3px 12px; font: inherit; cursor: pointer; }
</style>
