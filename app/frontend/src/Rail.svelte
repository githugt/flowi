<script lang="ts">
  // The sidebar folded to a narrow rail: icons for the tree's three groups and the tools that stay (Secrets, Import,
  // Settings). Pointing at a group opens the whole tree as a floating panel over the canvas (nothing moves); it
  // closes when the pointer leaves, unless pinned open, and stays while a button is held (a request being dragged out
  // of it). "Pin open" puts the sidebar back for this kind of tab.
  import { tick } from 'svelte'
  import { app } from './lib/app.svelte'
  import { screens } from './lib/screens'
  import Icon from './Icon.svelte'
  import Logo from './Logo.svelte'
  import Sidebar from './Sidebar.svelte'
  import type { RailGroup } from './lib/railgroups'

  let { width, onexpand, onpin }: { width: number; onexpand: () => void; onpin: () => void } = $props()

  let peek = $state(false)
  let shown = $state<RailGroup | ''>('')
  let closing: ReturnType<typeof setTimeout> | undefined
  const tools = screens.filter((s) => s.rail)
  const groups = [
    { icon: 'folder', title: 'Collections', id: 'collections' as RailGroup },
    { icon: 'variables', title: 'Environments', id: 'environments' as RailGroup },
    { icon: 'blueprint', title: 'Flows', id: 'flows' as RailGroup },
  ]

  // Only a group icon (or Cmd+K) opens the panel. Entering the rest of the rail, the panel itself included, just
  // cancels a pending close: opening from there showed whichever group was last looked at (the flows alone, say) when
  // the pointer only passed over Secrets.
  function enter(group: RailGroup | '' = '') {
    clearTimeout(closing)
    if (!group) return
    peek = true
    shown = group
  }
  // The Jump to field has the focus: the panel stays when the pointer leaves, and goes with a click elsewhere or Escape.
  const typing = () => !!wrap?.contains(document.activeElement) && document.activeElement instanceof HTMLInputElement
  function leave(e: PointerEvent) {
    // A held button means a drag from the tree is under way: the tree stays until it is dropped.
    if (e.buttons || typing()) return
    clearTimeout(closing)
    closing = setTimeout(() => (peek = false), 250)
  }
  // After a drag out of the tree the pointer is wherever it was dropped; if that is still over the panel, leaving it
  // closes it as usual (a release over the workspace menu must not).
  let wrap: HTMLElement | undefined
  function dropped(e: PointerEvent) {
    if (!peek || wrap?.contains(e.target as Node)) return
    closing = setTimeout(() => (peek = false), 600)
  }
  // A click on a row opens it and puts the panel away; the workspace button and its menu in the head, and the Jump to
  // field, are not rows.
  function opened(e: MouseEvent) {
    if ((e.target as HTMLElement).closest('.head, .jump')) return
    peek = false
  }
  function pressedOutside(e: PointerEvent) {
    if (peek && !wrap?.contains(e.target as Node)) peek = false
  }
  function escaped(e: KeyboardEvent) {
    // The Jump to field takes the first Escape to clear itself (and says so by cancelling the event).
    if (peek && e.key === 'Escape' && !e.defaultPrevented) peek = false
  }

  // Cmd/Ctrl+K while the sidebar is folded shows the panel with its Jump to field focused, without pinning it.
  let seenJump = app.jumpRequests
  $effect(() => {
    if (app.jumpRequests === seenJump) return
    seenJump = app.jumpRequests
    enter(shown || 'collections')
    tick().then(() => {
      const field = wrap?.querySelector<HTMLInputElement>('.jump input')
      field?.focus()
      field?.select()
    })
  })
  // Whatever opened a tab (Enter in the field, a row) puts the panel away.
  let lastActive = app.active
  $effect(() => {
    if (app.active === lastActive) return
    lastActive = app.active
    peek = false
  })
</script>

<svelte:window onpointerup={dropped} onpointerdown={pressedOutside} onkeydown={escaped} />

<div class="wrap" bind:this={wrap} onpointerenter={() => enter()} onpointerleave={leave} role="presentation">
  <nav class="rail" aria-label="Sidebar">
    <div class="spacer"></div>
    <button class="homebtn" onpointerenter={leave} class:on={app.active === 'screen:home'} title="Home" aria-label="Home" aria-current={app.active === 'screen:home' ? 'page' : undefined} onclick={() => app.openScreen('screen:home', 'Home')}><Logo size={22} /></button>
    <button class="expand" title="Show the sidebar" aria-label="Show the sidebar" onpointerenter={leave} onclick={onexpand}><Icon name="list" /></button>
    {#each groups as g}
      <button title={`${g.title}: show the tree`} aria-label={g.title} aria-haspopup="tree" aria-expanded={peek} onpointerenter={() => enter(g.id)} onclick={() => enter(g.id)} onfocus={() => enter(g.id)}><Icon name={g.icon} /></button>
    {/each}
    <span class="grow"></span>
    {#each tools as t}
      <button title={t.title} aria-label={t.title} class:on={app.active === t.id} onpointerenter={leave} onclick={() => app.openScreen(t.id, t.title)}><Icon name={t.icon} /></button>
    {/each}
  </nav>
  {#if peek}
    <div class="peek" role="presentation" onclick={opened}>
      <Sidebar {width} peek group={shown} onpin={onpin} />
    </div>
  {/if}
</div>

<style>
  .wrap { position: relative; flex: none; }
  .rail { width: 40px; height: 100%; box-sizing: border-box; display: flex; flex-direction: column; align-items: center; gap: 2px; padding: 0 4px 8px; background: var(--chrome); }
  .spacer { height: 44px; flex: none; --wails-draggable: drag; width: 100%; }
  .grow { flex: 1; }
  .rail button { background: none; border: 0; border-radius: var(--r-md); color: var(--muted); padding: 6px; display: grid; place-items: center; }
  .rail button:hover { color: var(--text); background: var(--tint-hover); }
  .rail button.on { color: var(--accent); background: var(--tint-sel); }
  .expand { margin-bottom: 6px; }
  .rail .homebtn { margin-bottom: 8px; padding: 5px; }
  .rail .homebtn.on { background: var(--tint-sel); }
  /* Floating, over the canvas: the layout does not move. */
  .peek { position: absolute; z-index: 50; top: 0; bottom: 0; left: 40px; box-shadow: var(--shadow); display: flex; }
</style>
