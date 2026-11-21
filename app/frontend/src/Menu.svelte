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
    /** A keyboard shortcut shown at the right of the label. */
    hint?: string
    /** The item destroys something: drawn in the failure colour. */
    danger?: boolean
    /** A small caption above this item, naming the group that starts here. */
    heading?: string
    /** Choosing it leaves the menu open: an option that is switched on and off, several at a time. */
    multi?: boolean
    /** A line of smaller words under the label. */
    detail?: string
    /** A verb (GET, MQTT…) whose colour the label is drawn in. */
    tone?: string
    /** How many entries it holds, as a badge at the right. */
    count?: number
    /** The "has content" dot at the right, when there is no count. */
    dot?: boolean
  }
</script>

<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from './Icon.svelte'
  import Verb from './Verb.svelte'
  import { menuPlace, type MenuAt } from './lib/menuanchor'

  // A menu that opens at a point (lib/menuanchor: under a button, or wherever a right-click was), over a transparent
  // layer, like Popover: a press anywhere outside closes it, Escape closes it from inside and hands the focus back to
  // what opened it, Tab closes it and carries on. It moves between items with the arrow keys, Home and End, and a
  // chosen item calls onclose first. It is kept inside the window once it has drawn and its size is known.
  // Before: the parent drew it with CSS relative to a wrapper and closed it with a window pointer handler, and the
  // button had to stop its own pointer-down so the handler did not close the menu it was about to toggle.
  let { items, onclose, at, label = '', minWidth = 170 }: { items: MenuItem[]; onclose: () => void; at: MenuAt; label?: string; minWidth?: number } = $props()

  let el = $state<HTMLElement>()
  let w = $state(0)
  let h = $state(0)
  let innerWidth = $state(0)
  let innerHeight = $state(0)
  const place = $derived(menuPlace(at, { w, h }, { w: innerWidth, h: innerHeight }))
  const hasChecks = $derived(items.some((i) => i.checked !== undefined))
  const opener = document.activeElement as HTMLElement | null

  function keys(e: KeyboardEvent) {
    const rows = [...(e.currentTarget as HTMLElement).querySelectorAll<HTMLButtonElement>('button:not(:disabled)')]
    const i = rows.indexOf(document.activeElement as HTMLButtonElement)
    if (e.key === 'ArrowDown') rows[(i + 1) % rows.length]?.focus()
    else if (e.key === 'ArrowUp') rows[(i - 1 + rows.length) % rows.length]?.focus()
    else if (e.key === 'Home') rows[0]?.focus()
    else if (e.key === 'End') rows[rows.length - 1]?.focus()
    else if (e.key === 'Escape') onclose()
    else if (e.key === 'Tab') return onclose()
    else return
    e.preventDefault()
  }

  // Focus moves into the menu when it opens, so the arrow keys work at once, and goes back to the opener when it
  // closes, unless the item chosen has already put it somewhere else (a dialog).
  onMount(() => {
    // The current choice of a list of options takes the focus, so the arrow keys start from it.
    ;(el?.querySelector<HTMLButtonElement>('button[aria-checked="true"]:not(:disabled)') ?? el?.querySelector<HTMLButtonElement>('button:not(:disabled)'))?.focus({ preventScroll: true })
    return () => {
      const active = document.activeElement
      if ((!active || active === document.body || el?.contains(active)) && opener?.isConnected) opener.focus({ preventScroll: true })
    }
  })

  // A right-click outside is the start of another context menu: close this one and pass the click on to what is under it,
  // so the person does not have to click twice.
  function again(e: MouseEvent) {
    e.preventDefault()
    onclose()
    const under = document.elementsFromPoint(e.clientX, e.clientY).find((n) => n !== el && !n.classList.contains('layer'))
    under?.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: e.clientX, clientY: e.clientY }))
  }

  function choose(item: MenuItem) {
    if (!item.multi) onclose()
    item.onselect()
  }
</script>

<svelte:window bind:innerWidth bind:innerHeight />

<div class="layer" role="presentation" onpointerdown={onclose} oncontextmenu={again}></div>
<div bind:this={el} bind:clientWidth={w} bind:clientHeight={h} class="menu pop-in" style:left="{place.left}px" style:top="{place.top}px" style:min-width="{minWidth}px" style:visibility={w ? 'visible' : 'hidden'}
  role="menu" aria-label={label || undefined} tabindex="-1" onkeydown={keys}>
  {#each items as item}
    {#if item.separated}<hr />{/if}
    {#if item.heading}<div class="hd" role="presentation">{item.heading}</div>{/if}
    <button role={item.checked === undefined ? 'menuitem' : item.multi ? 'menuitemcheckbox' : 'menuitemradio'} aria-checked={item.checked} class:on={item.active} class:danger={item.danger} disabled={item.disabled} title={item.title} onclick={() => choose(item)}>
      {#if hasChecks}<span class="tick" aria-hidden="true">{item.checked ? '✓' : ''}</span>{/if}{#if item.icon}<Icon name={item.icon} size={15} />{/if}
      {#if item.detail || item.tone || item.count || item.dot}
        <span class="grow">{#if item.tone}<Verb verb={item.tone} text={item.label} pill={false} />{:else}{item.label}{/if}{#if item.detail}<small>{item.detail}</small>{/if}</span>
        {#if item.count}<span class="count">{item.count}</span>{:else if item.dot}<span class="dot" aria-hidden="true"></span>{/if}
      {:else}{item.label}{/if}{#if item.hint}<span class="hint">{item.hint}</span>{/if}
    </button>
  {/each}
</div>

<style>
  .layer { position: fixed; inset: 0; z-index: var(--z-menu); --wails-draggable: no-drag; }
  .menu { position: fixed; z-index: calc(var(--z-menu) + 1); width: max-content; max-width: calc(100vw - 16px); max-height: calc(100vh - 24px); overflow: auto; display: flex; flex-direction: column; padding: 4px; background: var(--raised); border: 1px solid var(--line); border-radius: var(--r-lg); box-shadow: var(--shadow); --wails-draggable: no-drag; }
  button { display: flex; align-items: center; gap: 8px; background: none; border: 0; border-radius: var(--r-sm); color: var(--text); font: inherit; text-align: left; padding: 5px 10px; white-space: nowrap; }
  .tick { flex: none; width: 1em; color: var(--accent); }
  button:hover:not(:disabled), button:focus-visible { background: var(--tint-sel); outline: none; }
  button:disabled { opacity: var(--op-disabled); }
  button.on { color: var(--accent); }
  button.danger { color: var(--fail); }
  button.danger:hover:not(:disabled), button.danger:focus-visible { background: var(--fail-t); }
  .hint { margin-left: auto; padding-left: 24px; color: var(--muted); font-size: var(--fs-xs); }
  .hd { color: var(--muted); font-size: var(--fs-xs); padding: 6px 10px 2px; }
  .grow { flex: 1; }
  small { display: block; color: var(--muted); font-size: var(--fs-xs); }
  .dot { width: 6px; height: 6px; border-radius: 50%; background: var(--ok); }
  hr { width: 100%; border: 0; border-top: 1px solid var(--line); margin: 4px 0; }
</style>
