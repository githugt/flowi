<script lang="ts">
  // The window's main menu as a button, for Linux, where the system's own menu button cannot be put at the left.
  // Elsewhere there is nothing to draw: the system shows its own menu.
  import { app } from './lib/app.svelte'
  import Menu, { type MenuItem } from './Menu.svelte'
  import { anchorAt, type MenuAt } from './lib/menuanchor'
  import { isLinux, mainItems, opensMenu } from './lib/mainmenu'

  let open = $state(false)
  let at = $state<MenuAt>({ x: 0, y: 0 })
  let button = $state<HTMLButtonElement>()
  function toggle() {
    if (!open && button) at = anchorAt(button.getBoundingClientRect())
    open = !open
  }
  const items = $derived<MenuItem[]>(mainItems.map((i) => ({ label: i.label, hint: i.shortcut, separated: i.separated, onselect: () => app.onMenu(i.action) })))
</script>

<svelte:window
  onkeydown={(e) => {
    if (!isLinux(navigator.userAgent) || !opensMenu(e)) return
    e.preventDefault()
    toggle()
  }}
/>

{#if isLinux(navigator.userAgent)}
  <span class="mm">
    <button class="btn" bind:this={button} aria-haspopup="menu" aria-expanded={open} title="Main menu" aria-label="Main menu" onclick={toggle}>
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" aria-hidden="true"><path d="M2 3.5h12M2 8h12M2 12.5h6" /></svg>
    </button>
    {#if open}<Menu {at} minWidth={230} label="Main menu" {items} onclose={() => (open = false)} />{/if}
  </span>
{/if}

<style>
  .mm { flex: none; --wails-draggable: no-drag; }
  .btn { display: grid; place-items: center; padding: 5px; background: none; border: 0; border-radius: var(--r-md); color: var(--muted); }
  .btn:hover, .btn[aria-expanded='true'] { color: var(--text); background: var(--tint-hover); }
</style>
