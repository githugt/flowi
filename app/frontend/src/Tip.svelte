<script lang="ts">
  import type { Snippet } from 'svelte'

  // An explanation that shows when the pointer rests on something or the keyboard reaches it, and goes on Escape.
  // The words come as text, or as a snippet when they need markup. The panel is fixed to the window, so a scrolling
  // or clipping panel around the trigger cannot cut it off; it opens below and is kept inside the window's edges.
  // The trigger must be reachable by keyboard (a button, or tabindex="0"); a tip is never the only way to learn
  // something a person needs to act.
  // Before: Hint had the only good one, the native title attribute is slow and cannot be reached by keyboard, and a
  // third one sat in TemplateField.
  let {
    text,
    content,
    width = 260,
    children,
  }: {
    text?: string
    content?: Snippet
    width?: number
    /** What the pointer rests on. */
    children: Snippet
  } = $props()

  const edge = 8
  let trigger = $state<HTMLSpanElement>()
  let shown = $state(false)
  let left = $state(0)
  let top = $state(0)
  const id = `tip-${Math.random().toString(36).slice(2, 9)}`

  // The tip is open while the pointer rests on its trigger, which is not where the keyboard focus is, so the Escape that
  // closes it arrives anywhere. It is taken here, before it reaches the dialog or popover the trigger sits in: one Esc
  // closes one thing.
  function escape(e: KeyboardEvent) {
    if (e.key !== 'Escape' || !shown) return
    shown = false
    e.preventDefault()
    e.stopPropagation()
  }

  function show() {
    if (!trigger) return
    const r = trigger.getBoundingClientRect()
    left = Math.max(edge, Math.min(r.right + edge - width, window.innerWidth - width - edge))
    top = r.bottom + 6
    shown = true
  }
</script>

<svelte:window onkeydowncapture={escape} />

<span
  class="tip"
  bind:this={trigger}
  aria-describedby={shown ? id : undefined}
  role="presentation"
  onmouseenter={show}
  onmouseleave={() => (shown = false)}
  onfocusin={show}
  onfocusout={() => (shown = false)}
>
  {@render children()}
  {#if shown}
    <span class="pop" {id} role="tooltip" style="left:{left}px;top:{top}px;width:{width}px">{#if content}{@render content()}{:else}{text}{/if}</span>
  {/if}
</span>

<style>
  .tip { display: inline-flex; vertical-align: middle; font-weight: 400; text-transform: none; letter-spacing: 0; }
  .pop {
    position: fixed; z-index: var(--z-top); box-sizing: border-box; padding: var(--sp-2) 10px; pointer-events: none;
    background: var(--raised); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-lg); box-shadow: var(--shadow);
    font-size: var(--fs-sm); line-height: 1.45; text-align: left; white-space: normal;
  }
</style>
