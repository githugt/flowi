<script lang="ts">
  // The window's main menu as a button, for Linux, where the system's own menu button cannot be put at the left.
  // Elsewhere there is nothing to draw: the system shows its own menu.
  import { app } from './lib/app.svelte'
  import Menu, { type MenuItem } from './Menu.svelte'
  import { isLinux, mainItems } from './lib/mainmenu'

  let open = $state(false)
  const items = $derived<MenuItem[]>(mainItems.map((i) => ({ label: i.label, hint: i.shortcut, separated: i.separated, onselect: () => app.onMenu(i.action) })))
</script>

<svelte:window onpointerdown={() => (open = false)} />

{#if isLinux(navigator.userAgent)}
  <span class="mm">
    <button class="btn" aria-haspopup="menu" aria-expanded={open} title="Main menu" aria-label="Main menu" onpointerdown={(e) => e.stopPropagation()} onclick={() => (open = !open)}>
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" aria-hidden="true"><path d="M2 3.5h12M2 8h12M2 12.5h6" /></svg>
    </button>
    {#if open}<Menu class="mainmenu" label="Main menu" {items} onclose={() => (open = false)} />{/if}
  </span>
{/if}

<style>
  .mm { position: relative; flex: none; --wails-draggable: no-drag; }
  .btn { display: grid; place-items: center; padding: 5px; background: none; border: 0; border-radius: var(--r-md); color: var(--muted); }
  .btn:hover, .btn[aria-expanded='true'] { color: var(--text); background: var(--tint-hover); }
  .mm :global(.mainmenu) { top: calc(100% + 4px); left: 0; min-width: 230px; }
</style>
