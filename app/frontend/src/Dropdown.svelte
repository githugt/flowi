<script lang="ts">
  import Icon from './Icon.svelte'
  import Verb from './Verb.svelte'

  import type { DropOption as Option } from './lib/types'

  // A button showing the chosen option, opening a menu of all of them:
  // icons beside the names, verbs in their colour, a dot on those that have content.
  let {
    options,
    value,
    onpick,
    label = '',
    pill = false,
    icon = false,
    width,
    title,
  }: { options: Option[]; value: string; onpick: (v: string) => void; label?: string; pill?: boolean; icon?: boolean; width?: string; title?: string } = $props()

  let open = $state(false)
  let root = $state<HTMLElement>()
  let button = $state<HTMLButtonElement>()
  // The menu is fixed to the window, not to the button: inside the panels it was cut off by their edges (a body
  // type picked at the bottom of the window could not be reached). It opens upward when there is more room above.
  let place = $state<{ left: number; top?: number; bottom?: number; maxH: number; minW: number }>({ left: 0, maxH: 300, minW: 0 })
  const MARGIN = 12 // kept clear of the window's edge
  const ROW_H = 30 // an option's height, to guess whether the menu fits below

  function toggle() {
    open = !open
    if (open) place = placeMenu()
  }
  function placeMenu() {
    const r = button!.getBoundingClientRect()
    const below = window.innerHeight - r.bottom - MARGIN
    const above = r.top - MARGIN
    const wanted = options.length * ROW_H + 12
    const up = below < wanted && above > below
    return up
      ? { left: r.left, bottom: window.innerHeight - r.top + 4, maxH: above, minW: r.width }
      : { left: r.left, top: r.bottom + 4, maxH: below, minW: r.width }
  }
  const current = $derived(options.find((o) => o.value === value))

  function pick(o: Option) {
    open = false
    if (!o.disabled) onpick(o.value)
  }
  function away(e: PointerEvent) {
    if (open && root && !root.contains(e.target as Node)) open = false
  }
</script>

<svelte:window onpointerdown={away} onresize={() => (open = false)} onkeydown={(e) => open && e.key === 'Escape' && (open = false)} />

<div class="dd" bind:this={root} style:width {title}>
  <button type="button" class="btn" class:pill aria-haspopup="menu" aria-expanded={open} bind:this={button} onclick={toggle}>
    {#if icon && current?.icon}<Icon name={current.icon} size={14} />{/if}
    <!-- The button shows the bare verb (AMQP), which fits its pill; the menu names it in full (AMQP (RabbitMQ)). -->
    {#if current?.tone}<Verb verb={current.tone} {pill} />{:else}<span class="t">{current?.label ?? label}</span>{/if}
    <svg class="chev" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 9l6 6 6-6" /></svg>
  </button>
  {#if open}
    <div class="menu" role="menu" style:left={`${place.left}px`} style:top={place.top === undefined ? 'auto' : `${place.top}px`} style:bottom={place.bottom === undefined ? 'auto' : `${place.bottom}px`} style:max-height={`${place.maxH}px`} style:min-width={`${place.minW}px`}>
      {#each options as o}
        {#if o.separator}
          <hr />
        {:else}
          <button type="button" role="menuitemradio" aria-checked={o.value === value} class:cur={o.value === value} disabled={o.disabled} onclick={() => pick(o)}>
            {#if o.icon}<Icon name={o.icon} size={15} />{/if}
            <span class="grow">
              {#if o.tone}<Verb verb={o.tone} text={o.label} pill={false} />{:else}{o.label}{/if}
              {#if o.detail}<small>{o.detail}</small>{/if}
            </span>
            {#if o.count}<span class="count">{o.count}</span>{:else if o.dot}<span class="dot"></span>{/if}
          </button>
        {/if}
      {/each}
    </div>
  {/if}
</div>

<style>
  .dd { position: relative; display: inline-block; }
  .btn {
    display: inline-flex; align-items: center; gap: 6px; width: 100%; box-sizing: border-box;
    background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); font: inherit; padding: 5px 8px;
  }
  .btn:hover { border-color: var(--accent); }
  .t { flex: 1; text-align: left; white-space: nowrap; }
  .chev { color: var(--muted); margin-left: auto; flex: none; }
  .menu {
    position: fixed; z-index: 30; overflow: auto;
    display: flex; flex-direction: column; padding: 4px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg);
    box-shadow: var(--shadow);
  }
  .menu button { display: flex; align-items: center; gap: 8px; border: 0; background: transparent; color: var(--text); font: inherit; text-align: left; padding: 5px 10px; border-radius: var(--r-sm); white-space: nowrap; }
  .menu button:hover:not(:disabled) { background: color-mix(in srgb, var(--accent) 16%, transparent); }
  .menu button.cur { background: color-mix(in srgb, var(--accent) 24%, transparent); }
  .menu button:disabled { opacity: 0.4; }
  .grow { flex: 1; }
  small { display: block; color: var(--muted); font-size: var(--fs-xs); }
  hr { border: 0; border-top: 1px solid var(--line); margin: 4px 0; width: 100%; }
  .dot { width: 6px; height: 6px; border-radius: 50%; background: var(--ok); }
</style>
