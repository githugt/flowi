<script lang="ts">
  import type { Snippet } from 'svelte'
  import { modal } from './lib/modal'
  import { scrimClose } from './lib/scrim'

  // One dialog for every screen: the dim layer, the box, the heading and a row of buttons. It names itself for a
  // screen reader, keeps Tab inside, gives focus back on the way out (lib/modal), and closes on Escape or on a press
  // on the layer outside, unless the person changed a field in it (lib/scrim).
  // Before: eighteen dialogs each wrote their own scrim, box, heading and buttons row, with widths of 420, 440, 480
  // and 520 px, three stacking levels chosen by hand, and no two agreeing on whether Escape closed them.
  //
  // title is the heading and the dialog's name (label, when the name should say something else). Put the content in
  // the body and the buttons in `actions`. onclose, when given, is what Escape and a press outside do; leave it out
  // for a dialog that must be answered with a button. onsubmit makes the box a form (Enter in a field submits it).
  // placement "top" with `bare` is the command palette's shape: near the top of the window, no heading (title is only
  // the name for a screen reader) and a tight box; `guard` false lets a press outside close it even after typing (its field is a
  // query, not a form); `badge` is something after the heading's words, such as a Hint.
  // layer: "modal" for a dialog; "above" for one opened from another dialog; "top" for one over everything.
  let {
    title,
    label,
    onclose,
    onsubmit,
    onkeydown,
    width = 420,
    maxHeight = '80vh',
    scroll = false,
    layer = 'modal',
    alert = false,
    placement = 'center',
    bare = false,
    guard = true,
    badge,
    actions,
    children,
  }: {
    title: string
    label?: string
    onclose?: () => void
    onsubmit?: (e: SubmitEvent) => void
    onkeydown?: (e: KeyboardEvent) => void
    width?: number | string
    maxHeight?: string
    scroll?: boolean
    layer?: 'modal' | 'above' | 'top'
    alert?: boolean
    placement?: 'center' | 'top'
    bare?: boolean
    guard?: boolean
    badge?: Snippet
    actions?: Snippet
    children: Snippet
  } = $props()

  const keys = (e: KeyboardEvent) => {
    onkeydown?.(e)
    if (e.key === 'Escape' && !e.defaultPrevented && onclose) {
      e.preventDefault()
      onclose()
    }
  }
  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    onsubmit?.(e)
  }
</script>

<div class="scrim {layer}" class:high={placement === 'top'} role="presentation" use:modal use:scrimClose={{ onclose: () => onclose?.(), guard }}>
  <svelte:element this={onsubmit ? 'form' : 'div'} class="dialog" class:scroll class:bare role={alert ? 'alertdialog' : 'dialog'} aria-label={label ?? title} tabindex="-1" style:width={typeof width === 'number' ? `${width}px` : width} style:max-height={maxHeight}
    onpointerdown={(e: PointerEvent) => e.stopPropagation()} onkeydown={keys} onsubmit={onsubmit ? submit : undefined}>
    {#if !bare}<h3>{title}{#if badge}{@render badge()}{/if}</h3>{/if}
    {@render children()}
    {#if actions}<div class="actions">{@render actions()}</div>{/if}
  </svelte:element>
</div>

<style>
  .scrim { position: fixed; inset: 0; background: var(--scrim); display: grid; place-items: center; z-index: var(--z-modal); }
  .scrim.high { place-items: start center; padding-top: 14vh; }
  .scrim.above { z-index: var(--z-modal-above); }
  .scrim.top { z-index: var(--z-top); }
  .dialog { max-width: calc(100vw - 32px); display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); outline: none; }
  .dialog.bare { gap: 6px; padding: 8px; box-shadow: var(--shadow); }
  .dialog.scroll { overflow: auto; }
  h3 { margin: 0; }
  .dialog > :global(p) { margin: 0; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
</style>
