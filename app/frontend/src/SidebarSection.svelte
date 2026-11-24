<script lang="ts">
  // A section of the sidebar (Requests, Flows, Environments): a heading that folds it away, with how many rows it holds.
  // It stays as the person left it (lib/foldstate), and `open` is how it starts the first time. The sections are one
  // scrolling list and their headings are pinned: those above the rows in view stay at the top and those below stay at
  // the foot, so a long list of requests gets the whole height and Flows and Environments are still one click away.
  // Before: the tree had no sections; Environments sat among the collections and Flows were inside each one.
  import type { Snippet } from 'svelte'
  import Icon from './Icon.svelte'
  import Button from './Button.svelte'
  import { readFold, writeFold, type FoldStorage } from './lib/foldstate'

  let { name, title, count, open = false, index = 0, total = 1, children }: { name: string; title: string; count?: number; open?: boolean; index?: number; total?: number; children: Snippet } = $props()

  function store(): FoldStorage | undefined {
    try {
      return localStorage
    } catch {
      return undefined
    }
  }
  // svelte-ignore state_referenced_locally
  let shown = $state(readFold(store(), `sidebar:${name}`, open))
  const toggle = () => {
    shown = !shown
    writeFold(store(), `sidebar:${name}`, shown)
  }
  const body = $derived(`sidebar-${name}`)
</script>

<section class="section" class:open={shown}>
  <div class="head" style:--i={index} style:--after={total - 1 - index}>
    <Button variant="ghost" small aria-expanded={shown} aria-controls={body} onclick={toggle}>
      <span class="twist" class:open={shown} aria-hidden="true"><Icon name="chevr" size={12} /></span>
      <span class="title">{title}</span>
      {#if count !== undefined}<span class="n" aria-label="{count} rows">{count}</span>{/if}
    </Button>
  </div>
  {#if shown}<div class="body" id={body} role="group" aria-label={title}>{@render children()}</div>{/if}
</section>

<style>
  /* The section has no box: its heading and rows are children of the scrolling list, so the heading can stay pinned to
     its edges for the whole list (a sticky element cannot leave its parent). */
  .section { display: contents; }
  .head { --h: calc(var(--row-h-lg) + 1px); position: sticky; top: calc(var(--i) * var(--h)); bottom: calc(var(--after) * var(--h)); z-index: var(--z-handle); background: var(--panel); border-top: 1px solid var(--line); }
  .section:first-child > .head { border-top: 0; }
  .head :global(.ghost) { display: flex; align-items: center; width: 100%; justify-content: flex-start; gap: 6px; min-height: var(--row-h-lg); padding: 0 8px; border-radius: 0; font-size: var(--fs-xs); font-weight: 700; letter-spacing: 0.05em; text-transform: uppercase; }
  .twist { display: inline-flex; transition: transform var(--t-fast) var(--ease-out); }
  .twist.open { transform: rotate(90deg); }
  .n { margin-left: auto; font-weight: 600; letter-spacing: 0; }
  .body { padding: 0 6px 6px; }
</style>
