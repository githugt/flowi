<script lang="ts">
  // The sidebar folded to a narrow rail: icons for the tree's three groups and the tools that stay (Secrets, Import,
  // Settings). Pointing at a group opens the whole tree as a floating panel over the canvas (nothing moves); it
  // closes when the pointer leaves, unless pinned open, and stays while a button is held (a request being dragged out
  // of it). "Pin open" puts the sidebar back for this kind of tab.
  import { app } from './lib/app.svelte'
  import { screens } from './lib/screens'
  import Icon from './Icon.svelte'
  import Sidebar from './Sidebar.svelte'

  let { width, onexpand, onpin }: { width: number; onexpand: () => void; onpin: () => void } = $props()

  let peek = $state(false)
  let closing: ReturnType<typeof setTimeout> | undefined
  const tools = screens.filter((s) => s.rail)
  const groups = [
    { icon: 'folder', title: 'Collections' },
    { icon: 'variables', title: 'Environments' },
    { icon: 'blueprint', title: 'Flows' },
  ]

  function enter() {
    clearTimeout(closing)
    peek = true
  }
  function leave(e: PointerEvent) {
    // A held button means a drag from the tree is under way: the tree stays until it is dropped.
    if (e.buttons) return
    clearTimeout(closing)
    closing = setTimeout(() => (peek = false), 250)
  }
  function opened() {
    peek = false
  }
</script>

<svelte:window onpointerup={() => !peek || (closing = setTimeout(() => (peek = false), 600))} />

<div class="wrap" onpointerenter={enter} onpointerleave={leave} role="presentation">
  <nav class="rail" aria-label="Sidebar">
    <div class="spacer"></div>
    <button class="expand" title="Show the sidebar" aria-label="Show the sidebar" onclick={onexpand}><Icon name="list" /></button>
    {#each groups as g}
      <button title={g.title} aria-label={g.title} onpointerenter={enter}><Icon name={g.icon} /></button>
    {/each}
    <span class="grow"></span>
    {#each tools as t}
      <button title={t.title} aria-label={t.title} class:on={app.active === t.id} onclick={() => app.openScreen(t.id, t.title)}><Icon name={t.icon} /></button>
    {/each}
  </nav>
  {#if peek}
    <div class="peek" role="presentation" onclick={opened}>
      <Sidebar {width} peek onpin={onpin} />
    </div>
  {/if}
</div>

<style>
  .wrap { position: relative; flex: none; }
  .rail { width: 40px; height: 100%; box-sizing: border-box; display: flex; flex-direction: column; align-items: center; gap: 2px; padding: 0 4px 8px; background: var(--panel); border-right: 1px solid var(--line); }
  .spacer { height: 44px; flex: none; --wails-draggable: drag; width: 100%; }
  .grow { flex: 1; }
  .rail button { background: none; border: 0; border-radius: var(--r-md); color: var(--muted); padding: 6px; display: grid; place-items: center; }
  .rail button:hover { color: var(--text); background: var(--tint-hover); }
  .rail button.on { color: var(--accent); background: var(--tint-sel); }
  .expand { margin-bottom: 6px; }
  /* Floating, over the canvas: the layout does not move. */
  .peek { position: absolute; z-index: 50; top: 0; bottom: 0; left: 40px; box-shadow: var(--shadow); display: flex; }
</style>
