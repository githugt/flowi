<script lang="ts">
  import Menu, { type MenuItem } from './Menu.svelte'
  import { anchorAt, type MenuAt } from './lib/menuanchor'
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
  let button = $state<HTMLButtonElement>()
  // The menu is placed by the Menu from the button's edges: under it, or over it when there is more room above, and
  // as wide as the button at least.
  let at = $state<MenuAt>({ x: 0, y: 0 })
  let minW = $state(0)
  const current = $derived(options.find((o) => o.value === value))
  // A separator option is a rule above the option after it.
  const items = $derived.by<MenuItem[]>(() => {
    const out: MenuItem[] = []
    let rule = false
    for (const o of options) {
      if (o.separator) {
        rule = out.length > 0
        continue
      }
      out.push({ label: o.label, icon: o.icon, tone: o.tone, dot: o.dot, count: o.count, detail: o.detail, disabled: o.disabled, checked: current ? o.value === value : undefined, separated: rule, onselect: () => onpick(o.value) })
      rule = false
    }
    return out
  })

  function toggle() {
    if (!button) return
    const r = button.getBoundingClientRect()
    at = anchorAt(r, { flip: true })
    minW = r.width
    open = !open
  }
</script>

<svelte:window onresize={() => (open = false)} />

<div class="dd" style:width {title}>
  <button type="button" class="btn" class:pill aria-haspopup="menu" aria-expanded={open} bind:this={button} onclick={toggle}>
    {#if icon && current?.icon}<Icon name={current.icon} size={14} />{/if}
    <!-- The button shows the bare verb (AMQP), which fits its pill; the menu names it in full (AMQP (RabbitMQ)). -->
    {#if current?.tone}<Verb verb={current.tone} {pill} />{:else}<span class="t">{current?.label ?? label}</span>{/if}
    <svg class="chev" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 9l6 6 6-6" /></svg>
  </button>
  {#if open}<Menu {at} minWidth={minW} label={title ?? label} {items} onclose={() => (open = false)} />{/if}
</div>

<style>
  .dd { display: inline-block; }
  .btn {
    display: inline-flex; align-items: center; gap: 6px; width: 100%; box-sizing: border-box;
    background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); font: inherit; padding: 5px 8px;
  }
  .btn:hover { background: var(--raised-3); }
  .t { flex: 1; text-align: left; white-space: nowrap; }
  .chev { color: var(--muted); margin-left: auto; flex: none; }
</style>
