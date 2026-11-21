<script lang="ts">
  // What a pane or section shows while it has nothing: its icon in a soft disc, what is missing,
  // and a line on how to get it, with an action under it when there is one.
  import type { Snippet } from 'svelte'
  import Icon from './Icon.svelte'
  import Button from './Button.svelte'
  import { openExternal, isWebUrl } from './lib/openurl'

  // What goes under the hint is `Button`s: they are drawn as they are everywhere else, not restyled here.
  // learn is the address of a page that explains this: a "Learn more" link under the hint (web addresses only).
  // kind is "empty" (there is nothing yet) or "filtered" (there is something, and the filter hides all of it): a
  // filtered one has the search icon and a title that names the filter, and `onclear` adds "Clear filter" before the
  // other actions. It is announced (role status) as the filter changes, which an empty one is not.
  // Before: eight panes drew "Nothing matches" as a plain muted line, with no way to clear what hid everything.
  let {
    kind = 'empty',
    icon,
    title,
    query = '',
    hint = '',
    compact = false,
    learn = '',
    onclear,
    children,
  }: {
    kind?: 'empty' | 'filtered'
    icon?: string
    title?: string
    /** For a filtered one: the words typed, quoted in the default title. */
    query?: string
    hint?: string
    compact?: boolean
    learn?: string
    onclear?: () => void
    children?: Snippet
  } = $props()
  const mark = $derived(icon ?? (kind === 'filtered' ? 'search' : 'list'))
  const heading = $derived(title ?? (kind === 'filtered' ? (query ? `Nothing matches “${query}”` : 'Nothing matches') : ''))
</script>

<div class="empty" class:compact role={kind === 'filtered' ? 'status' : undefined}>
  <span class="disc"><Icon name={mark} size={compact ? 26 : 36} /></span>
  <strong>{heading}</strong>
  {#if hint}<span class="hint">{hint}</span>{/if}
  {#if learn && isWebUrl(learn)}<Button variant="ghost" small onclick={() => openExternal(learn)}>Learn more</Button>{/if}
  {#if onclear || children}<div class="act">{#if onclear}<Button small onclick={onclear}>Clear filter</Button>{/if}{#if children}{@render children()}{/if}</div>{/if}
</div>

<style>
  .empty { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 6px; text-align: center; padding: 24px; min-height: 0; }
  .disc { display: grid; place-items: center; width: 76px; height: 76px; border-radius: 50%; background: color-mix(in srgb, var(--muted) 12%, transparent); color: var(--muted); margin-bottom: 6px; }
  .compact .disc { width: 56px; height: 56px; }
  strong { color: var(--text); font-size: var(--fs-lg); font-weight: 600; }
  .compact strong { font-size: var(--fs-md); }
  .hint { color: var(--muted); font-size: var(--fs-sm); max-width: 320px; line-height: 1.45; }
  .act { margin-top: 6px; display: flex; flex-wrap: wrap; justify-content: center; gap: var(--sp-2); }
</style>
