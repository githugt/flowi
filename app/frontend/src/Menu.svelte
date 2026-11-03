<script module lang="ts">
  export interface MenuItem {
    label: string
    onselect: () => void
    icon?: string
    disabled?: boolean
    title?: string
    /** The item for the screen that is open now. */
    active?: boolean
    /** A rule above this item, to set a group apart. */
    separated?: boolean
    /** Shown with a check: the current choice in a list of options (every row then keeps room for the tick). */
    checked?: boolean
  }
</script>

<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from './Icon.svelte'

  // A menu that opens from a button. The parent decides when it is shown and where (position it with `class`
  // or `style`); this draws the items and moves between them with the arrow keys. Escape and a chosen item call onclose.
  let { items, onclose, label = '', class: cls = '', style = '' }: { items: MenuItem[]; onclose: () => void; label?: string; class?: string; style?: string } = $props()

  function keys(e: KeyboardEvent) {
    const rows = [...(e.currentTarget as HTMLElement).querySelectorAll<HTMLButtonElement>('button:not(:disabled)')]
    const i = rows.indexOf(document.activeElement as HTMLButtonElement)
    if (e.key === 'ArrowDown') rows[(i + 1) % rows.length]?.focus()
    else if (e.key === 'ArrowUp') rows[(i - 1 + rows.length) % rows.length]?.focus()
    else if (e.key === 'Escape') onclose()
    else return
    e.preventDefault()
  }

  let el: HTMLElement
  const hasChecks = $derived(items.some((i) => i.checked !== undefined))
  // Focus moves into the menu when it opens, so the arrow keys work at once.
  onMount(() => el.querySelector<HTMLButtonElement>('button:not(:disabled)')?.focus({ preventScroll: true }))

  function choose(item: MenuItem) {
    onclose()
    item.onselect()
  }
</script>

<!-- Stops pointer-down here so a window-level "click outside closes" does not close it before the click lands. -->
<div bind:this={el} class="menu {cls}" {style} role="menu" aria-label={label || undefined} tabindex="-1" onkeydown={keys} onpointerdown={(e) => e.stopPropagation()}>
  {#each items as item}
    {#if item.separated}<hr />{/if}
    <button role={item.checked === undefined ? 'menuitem' : 'menuitemradio'} aria-checked={item.checked} class:on={item.active} disabled={item.disabled} title={item.title} onclick={() => choose(item)}>
      {#if hasChecks}<span class="tick" aria-hidden="true">{item.checked ? '✓' : ''}</span>{/if}{#if item.icon}<Icon name={item.icon} size={15} />{/if}{item.label}
    </button>
  {/each}
</div>

<style>
  .menu { position: absolute; z-index: 40; min-width: 170px; width: max-content; max-width: calc(100vw - 16px); max-height: calc(100vh - 24px); overflow: auto; display: flex; flex-direction: column; padding: 4px; background: var(--raised); border: 1px solid var(--line); border-radius: var(--r-lg); box-shadow: var(--shadow); --wails-draggable: no-drag; }
  button { display: flex; align-items: center; gap: 8px; background: none; border: 0; border-radius: var(--r-sm); color: var(--text); font: inherit; text-align: left; padding: 5px 10px; white-space: nowrap; }
  .tick { flex: none; width: 1em; color: var(--accent); }
  button:hover:not(:disabled), button:focus-visible { background: var(--tint-sel); outline: none; }
  button:disabled { opacity: 0.4; }
  button.on { color: var(--accent); }
  hr { width: 100%; border: 0; border-top: 1px solid var(--line); margin: 4px 0; }
</style>
