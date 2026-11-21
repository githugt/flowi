<script lang="ts">
  // One tab bar for the window: tablist roles, one tab stop with the arrow keys, Home and End moving inside it, and a
  // count badge. The tab in front has the accent line under it.
  // Before: seven hand-made bars with their own padding and badges, most without arrow keys, some without a tab role.
  import type { Snippet } from 'svelte'
  import { tabKeyTarget } from './lib/tabkeys'

  export interface TabItem {
    id: string
    label: string
    /** A number shown after the label (matches, rows, messages). */
    count?: number
    /** 'fail' paints the count as a failure (problems that are errors). */
    tone?: 'fail'
    title?: string
    disabled?: boolean
  }

  let {
    items,
    active,
    onpick,
    label,
    fill = false,
    wrap = false,
    bare = false,
    size = 'md',
    after,
    children,
  }: {
    items: TabItem[]
    /** The tab in front; null when none is (a search is showing every section, a panel is shut). */
    active: string | null
    onpick: (id: string) => void
    label?: string
    /** Tabs share the width equally. */
    fill?: boolean
    /** Tabs go on a second line instead of overflowing. */
    wrap?: boolean
    /** For a bar that sits in a row of its own: no rule under it, the tabs as tall as the row, and it scrolls sideways
     *  when the tabs do not fit. The row sets the height. */
    bare?: boolean
    size?: 'sm' | 'md'
    /** Something after one tab's label that a number cannot say: a status pill, a dot. */
    after?: Snippet<[TabItem]>
    /** What follows the tabs at the far end, such as a pin. */
    children?: Snippet
  } = $props()

  let list = $state<HTMLDivElement>()
  const enabled = $derived(items.filter((i) => !i.disabled))
  // The tab stop is the one in front, else the first that can be chosen.
  const stop = $derived(enabled.find((i) => i.id === active)?.id ?? enabled[0]?.id)

  function keys(e: KeyboardEvent) {
    const at = enabled.findIndex((i) => i.id === (e.target as HTMLElement).dataset.tab)
    const to = tabKeyTarget(e.key, at, enabled.length)
    if (to === null) return
    e.preventDefault()
    const id = enabled[to].id
    onpick(id)
    list?.querySelector<HTMLElement>(`[data-tab="${CSS.escape(id)}"]`)?.focus()
  }
</script>

<div class="tabs {size}" class:fill class:wrap class:bare class:wide={!!children} role="tablist" tabindex="-1" aria-label={label} bind:this={list} onkeydown={keys}>
  {#each items as t (t.id)}
    <button
      type="button"
      role="tab"
      data-tab={t.id}
      aria-selected={active === t.id}
      tabindex={t.id === stop ? 0 : -1}
      class:on={active === t.id}
      disabled={t.disabled}
      title={t.title}
      onclick={() => onpick(t.id)}>{t.label}{#if t.count !== undefined}<span class="count" class:fail={t.tone === 'fail'}>{t.count}</span>{/if}{#if after}{@render after(t)}{/if}</button>
  {/each}
  {#if children}<span class="grow"></span>{@render children()}{/if}
</div>

<style>
  .tabs { display: flex; align-items: center; gap: 2px; border-bottom: 1px solid var(--line); flex: none; }
  .tabs.wrap { flex-wrap: wrap; }
  .tabs.bare { flex: 0 1 auto; min-width: 0; align-self: stretch; border-bottom: 0; overflow-x: auto; scrollbar-width: none; gap: 0; }
  .tabs.bare.wide { flex: 1; }
  button { display: flex; gap: 6px; align-items: baseline; background: none; border: 0; border-bottom: 2px solid transparent; margin-bottom: -1px; color: var(--muted); font: inherit; white-space: nowrap; cursor: pointer; }
  .bare button { align-self: stretch; align-items: center; margin-bottom: 0; }
  .md button { padding: 6px 12px; }
  .sm button { padding: 3px 8px; font-size: var(--fs-sm); }
  /* The strip over a pane has the room of a pane: the narrower padding is what let four tabs fit before. */
  .md.bare button { padding: 0 10px; }
  .fill button { flex: 1; justify-content: center; align-items: center; height: var(--tabbar-h); padding-block: 0; }
  button:hover:not(:disabled) { color: var(--text); }
  button.on { color: var(--text); border-bottom-color: var(--accent); }
  button:disabled { opacity: var(--op-disabled); cursor: default; }
  .count { color: var(--text); font-size: var(--fs-xs); padding: 0 6px; border-radius: var(--r-pill); background: color-mix(in srgb, var(--muted) 22%, transparent); }
  .count.fail { background: var(--fail); color: var(--on-accent); }
  .grow { flex: 1; }
</style>
