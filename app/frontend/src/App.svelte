<script lang="ts">
  import { markerAt, slotAmong } from './lib/reorder'
  import { screenOf } from './lib/screens'
  import { onMount, tick } from 'svelte'
  import { EnvironmentService, SessionService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { app, type Tab } from './lib/app.svelte'
  import { edgeOf, panesOf, sharesOf, type Dir, type Layout, type Side } from './lib/layout'
  import Sidebar from './Sidebar.svelte'
  import Splitter from './Splitter.svelte'
  import Rail from './Rail.svelte'
  import { collapsedOnOpen, loadPins, savePins, type TabKind } from './lib/sidebar'
  import Verb from './Verb.svelte'
  import Icon from './Icon.svelte'
  import { ui, type MenuItem } from './lib/ui.svelte'
  import { newMenu, promptNewEnvironment } from './lib/menus'
  import EnvOffers from './EnvOffers.svelte'

  let sidebarWidth = $state(260)

  // The sidebar folds to a rail when a flow opens (the graph wants the room) unless pinned open; a request leaves it
  // as it was. The pin is kept for each kind of tab.
  const store = (() => {
    try {
      return localStorage
    } catch {
      return { getItem: () => null, setItem: () => {} }
    }
  })()
  let pins = $state(loadPins(store))
  let collapsed = $state(false)
  const kindOf = (node: string): TabKind => {
    const k = app.tabs.find((t) => t.node === node)?.kind
    return k === 'flow' || k === 'request' ? k : 'other'
  }
  let lastActive = ''
  $effect(() => {
    const active = app.active
    if (active === lastActive) return
    lastActive = active
    collapsed = collapsedOnOpen(kindOf(active), collapsed, pins)
  })
  // Pinning is for the kind of tab in front (a screen counts as a request tab).
  function pinOpen() {
    const kind = kindOf(app.active) === 'flow' ? 'flow' : 'request'
    pins[kind] = true
    savePins(store, pins)
    collapsed = false
  }
  function fold() {
    const kind = kindOf(app.active) === 'flow' ? 'flow' : 'request'
    pins[kind] = false
    savePins(store, pins)
    collapsed = true
  }
  import FlowEditor from './FlowEditor.svelte'
  import RequestEditor from './RequestEditor.svelte'
  import Overlays from './Overlays.svelte'
  import PasteOffer from './PasteOffer.svelte'
  import HooksEditor from './HooksEditor.svelte'
  import HomeScreen from './HomeScreen.svelte'
  import Logo from './Logo.svelte'
  import WelcomeScreen from './WelcomeScreen.svelte'
  import TelemetryScreen from './TelemetryScreen.svelte'
  import TracingScreen from './TracingScreen.svelte'
  import Announcements from './Announcements.svelte'
  import EnvEditor from './EnvEditor.svelte'
  import Footer from './Footer.svelte'
  import SettingsScreen from './SettingsScreen.svelte'
  import SecretsScreen from './SecretsScreen.svelte'
  import ManagerScreen from './ManagerScreen.svelte'
  import ImportScreen from './ImportScreen.svelte'
  import SchedulesScreen from './SchedulesScreen.svelte'
  import ReplayerScreen from './ReplayerScreen.svelte'
  import BlueprintsScreen from './BlueprintsScreen.svelte'
  import TrashScreen from './TrashScreen.svelte'
  import ChecksScreen from './ChecksScreen.svelte'
  import { envSwatch } from './lib/envcolours'
  import { createAltTap, isWindows } from './lib/altmenu'
  import { Events } from '@wailsio/runtime'

  onMount(() => app.init())

  // On Windows the menu bar stays hidden until Alt is tapped (the Go side hides it and answers this event).
  onMount(() => {
    if (!isWindows(navigator.userAgent)) return
    const alt = createAltTap(() => Events.Emit('toggle-menu-bar'))
    const down = (e: KeyboardEvent) => alt.keydown(e)
    const up = (e: KeyboardEvent) => alt.keyup(e)
    const cancel = () => alt.cancel()
    window.addEventListener('keydown', down)
    window.addEventListener('keyup', up)
    window.addEventListener('pointerdown', cancel)
    window.addEventListener('blur', cancel)
    return () => {
      window.removeEventListener('keydown', down)
      window.removeEventListener('keyup', up)
      window.removeEventListener('pointerdown', cancel)
      window.removeEventListener('blur', cancel)
    }
  })

  // The open tabs are kept for the next session, once the last one's are back.
  $effect(() => {
    const rows = app.tabRows
    if (app.restored) SessionService.SaveTabs(rows)
  })

  const tab = $derived(app.tabs.find((t) => t.node === app.active))

  // The editor is a tree of panes, each showing one tab; the pane last pressed is the one the tab strip, Save and Send act on.
  const panes = $derived(panesOf(app.layout))
  const multiple = $derived(panes.length > 1)

  // A pane follows the step selected in the flow it was opened from.
  let followed = ''
  $effect(() => {
    const flow = app.follow
    const step = flow ? (app.flows[flow]?.selected ?? '') : ''
    if (!step || step === followed) return
    followed = step
    void app.showStepBeside(flow, step)
  })

  // The layout is kept for the workspace: read back once the session's tabs are back, and written whenever it changes.
  $effect(() => app.readLayouts())
  $effect(() => app.keepLayout())

  // The picker takes the colour of the environment it shows, so a production one cannot be mistaken for a test one.
  const selectedSwatch = $derived(envSwatch(app.tree?.envColours?.[app.tree?.selected ?? '']))
  // How many variables the environment in use changes about the one it extends: a chip beside the picker says so,
  // and opens the comparison. Asked again whenever the environments or the choice change.
  let overrides = $state<{ extends: string; count: number }>({ extends: '', count: 0 })
  $effect(() => {
    const name = app.tree?.selected ?? ''
    void app.tree?.environments
    if (!name) {
      overrides = { extends: '', count: 0 }
      return
    }
    EnvironmentService.Overrides(name).then((o) => {
      if (name === app.tree?.selected) overrides = o as unknown as { extends: string; count: number }
    })
  })
  // The environment chip's menu: "No environment", each environment (the current one ticked), then a way to make one.
  function environmentMenu(): MenuItem[] {
    const selected = app.tree?.selected ?? ''
    const choices = ['', ...(app.tree?.environments ?? [])]
    const items: MenuItem[] = choices.map((name) => ({ label: name || 'No environment', checked: name === selected, run: () => app.selectEnvironment(name) }))
    return [...items, { label: '', separator: true, run: () => {} }, { label: 'New environment…', run: promptNewEnvironment }]
  }

  // The layouts menu: the arrangements the person named for this workspace, to put back or forget, a way to name the
  // one in use, and a way back to one pane.
  function layoutsMenu(): MenuItem[] {
    const names = Object.keys(app.named).sort()
    const items: MenuItem[] = [{ label: 'Save this layout as…', run: promptSaveLayout }]
    if (multiple) items.push({ label: 'Close all splits', run: () => app.closeSplit() })
    if (names.length) items.push({ label: '', separator: true, run: () => {} }, ...names.map((name) => ({ label: name, run: () => void app.applyLayout(name) })))
    if (names.length) items.push({ label: '', separator: true, run: () => {} }, ...names.map((name) => ({ label: `Forget “${name}”`, run: () => app.deleteLayout(name) })))
    return items
  }
  function promptSaveLayout() {
    ui.prompt({
      title: 'Save this layout',
      label: multiple ? 'Name (the tabs in each pane are kept)' : 'Name (one pane: drag a tab to an edge to split first)',
      initial: '',
      done: (text) => {
        const name = text.trim()
        if (name) app.saveLayout(name)
      },
    })
  }

  // Tabs are reordered by dragging with the pointer (the browser's own drag and drop
  // is not reliable in the window's web view). A press only becomes a drag after a few pixels.
  // The drag listens on the window, not with a pointer capture on the tab: moving the tab
  // in the page drops its capture, and the rest of the drag went astray.
  // While a tab is dragged it stays in place, dimmed; a copy follows the pointer (x, y) and a bar
  // marks where it would drop (markX, between tabs). The tabs move once, on release: moving them
  // under the pointer as it went left nothing to show where the tab would land.
  /** The edge of a pane a dragged tab would split: which pane, which side, and where the pane is. */
  type DropTarget = { pane: string; side: Side; rect: { left: number; top: number; right: number; bottom: number } }
  let tabDrag = $state<{ node: string; startX: number; active: boolean; x: number; y: number; slot: number; markX: number; zone: DropTarget | null; overBody: boolean } | null>(null)
  let bodyEl = $state<HTMLDivElement>()
  let strip = $state<HTMLDivElement>()
  let dragged = false // the click that ends a drag is not a click on the tab

  function tabDown(e: PointerEvent, node: string) {
    if (e.button !== 0 || (e.target as HTMLElement).closest('.x')) return
    tabDrag = { node, startX: e.clientX, active: false, x: e.clientX, y: e.clientY, slot: -1, markX: 0, zone: null, overBody: false }
    window.addEventListener('pointermove', tabMove)
    window.addEventListener('pointerup', tabUp)
    window.addEventListener('pointercancel', tabUp)
  }

  // A divider between panes: dragged with the pointer, or moved a little with the arrow keys. The position is the
  // pointer's place along the split that holds the divider, as a fraction of it.
  function dividerDown(e: PointerEvent, dir: Dir, path: number[], boundary: number) {
    const box = (e.currentTarget as HTMLElement).parentElement
    if (e.button !== 0 || !box) return
    e.preventDefault() // no text is selected by the drag
    const move = (m: PointerEvent) => {
      const r = box.getBoundingClientRect()
      const along = dir === 'row' ? (m.clientX - r.left) / r.width : (m.clientY - r.top) / r.height
      app.resizeDivider(path, boundary, along)
    }
    const stop = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', stop)
      window.removeEventListener('pointercancel', stop)
      document.body.classList.remove('resizing')
    }
    document.body.classList.add('resizing')
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', stop)
    window.addEventListener('pointercancel', stop)
  }
  const KEY_STEP = 0.02
  function dividerKey(e: KeyboardEvent, dir: Dir, path: number[], boundary: number, shares: number[]) {
    const keys: Record<string, number> = dir === 'row' ? { ArrowLeft: -1, ArrowRight: 1 } : { ArrowUp: -1, ArrowDown: 1 }
    const way = keys[e.key]
    if (!way) return
    e.preventDefault()
    const at = shares.slice(0, boundary + 1).reduce((a, b) => a + b, 0)
    app.resizeDivider(path, boundary, at + way * KEY_STEP)
  }

  // The pane under the pointer, and the edge of it the pointer is near.
  function zoneUnder(x: number, y: number): DropTarget | null {
    for (const el of bodyEl?.querySelectorAll<HTMLElement>('[data-pane]') ?? []) {
      const r = el.getBoundingClientRect()
      const side = edgeOf(x, y, r)
      if (side) return { pane: el.dataset.pane!, side, rect: { left: r.left, top: r.top, right: r.right, bottom: r.bottom } }
    }
    return null
  }

  function tabMove(e: PointerEvent) {
    if (!tabDrag || !strip) return
    if (!(e.buttons & 1)) return tabUp() // released where no pointerup reached us
    if (!tabDrag.active && Math.abs(e.clientX - tabDrag.startX) < 5) return
    const node = tabDrag.node
    const others = [...strip.querySelectorAll<HTMLElement>('[data-tab]')].filter((t) => t.dataset.tab !== node)
    const spans = others.map((t) => ({ start: t.getBoundingClientRect().left, end: t.getBoundingClientRect().right }))
    const slot = slotAmong(spans, e.clientX)
    // Over the editor a drop splits (at an edge) or does nothing (in the middle); only over the strip does it reorder.
    const body = bodyEl?.getBoundingClientRect()
    const overBody = !!body && e.clientY >= body.top
    const zone = overBody ? zoneUnder(e.clientX, e.clientY) : null
    tabDrag = { ...tabDrag, active: true, x: e.clientX, y: e.clientY, slot, markX: markerAt(spans, slot, strip.getBoundingClientRect().left), zone, overBody }
  }

  // The tabs scroll sideways when there are more than fit. A tab opened or picked elsewhere (the
  // sidebar, a menu) is scrolled into view: it opened past the edge, out of sight, and it seemed
  // that no tab had opened.
  let overflow = $state({ left: false, right: false })
  function measureOverflow() {
    if (!strip) return
    overflow = { left: strip.scrollLeft > 0, right: strip.scrollLeft + strip.clientWidth < strip.scrollWidth - 1 }
  }
  $effect(() => {
    void app.active
    void app.tabs.length
    tick().then(() => {
      strip?.querySelector<HTMLElement>('[data-tab].active')?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
      measureOverflow()
    })
  })
  // A mouse wheel turns vertically; over the tabs it scrolls them sideways.
  function stripWheel(e: WheelEvent) {
    if (!strip || Math.abs(e.deltaX) > Math.abs(e.deltaY)) return
    strip.scrollLeft += e.deltaY
    e.preventDefault()
  }

  function tabUp() {
    if (tabDrag?.active && tabDrag.zone) app.splitWith(tabDrag.node, tabDrag.zone.pane, tabDrag.zone.side)
    else if (tabDrag?.active && !tabDrag.overBody) app.moveTabTo(tabDrag.node, tabDrag.slot)
    dragged = !!tabDrag?.active
    tabDrag = null
    window.removeEventListener('pointermove', tabMove)
    window.removeEventListener('pointerup', tabUp)
    window.removeEventListener('pointercancel', tabUp)
  }
</script>

<div class="shell">
  {#if collapsed}
    <Rail width={sidebarWidth} onexpand={() => (collapsed = false)} onpin={pinOpen} />
  {:else}
    <Sidebar width={sidebarWidth} onfold={fold} />
    <Splitter name="sidebar" side="left" min={200} max={520} bind:width={sidebarWidth} />
  {/if}
  <section class:folded={collapsed}>
    <Announcements />
    <nav class="tabs">
      <!-- With the sidebar folded, Home is the top of the rail instead. -->
      {#if !collapsed}
        <button class="home" class:on={app.active === 'screen:home'} title="Home" aria-label="Home" aria-current={app.active === 'screen:home' ? 'page' : undefined} onpointerdown={(e) => e.stopPropagation()} onclick={() => app.openScreen('screen:home', 'Home')}><Logo size={22} /></button>
      {/if}
      <div class="strip" class:fade-left={overflow.left} class:fade-right={overflow.right} bind:this={strip} onscroll={measureOverflow} onwheel={stripWheel}>
        {#each app.tabs as t (t.node)}
          {@const verb = t.kind === 'flow' ? 'FLOW' : app.requests[t.node]?.verb || app.find(t.node)?.method || ''}
          <div
            class="tab"
            class:active={t.node === app.active}
            class:dragging={tabDrag?.active && tabDrag.node === t.node}
            data-tab={t.node}
            role="presentation"
            onpointerdown={(e) => tabDown(e, t.node)}
          >
            <button class="name" onclick={() => { if (!dragged) app.active = t.node; dragged = false }}>
              <span class="dirty" class:on={app.isDirty(t.node)}>●</span>
              {#if verb}<Verb {verb} pill={false} short />{:else if screenOf(t.node)}<Icon name={screenOf(t.node)!.icon} size={14} />{/if}
              <span class="title">{app.find(t.node)?.label ?? t.label}</span>
            </button>
            <button class="x" aria-label="Close" onclick={() => app.requestClose(t.node)}>×</button>
          </div>
        {/each}
      </div>
      {#if tabDrag?.active}
        {@const t = app.tabs.find((x) => x.node === tabDrag!.node)}
        {@const strip0 = strip?.getBoundingClientRect()}
        {#if !tabDrag.overBody}<div class="dropmark" style:left={`${tabDrag.markX}px`} style:top={`${strip0?.top ?? 0}px`} style:height={`${strip0?.height ?? 0}px`} aria-hidden="true"><span>Drop here</span></div>{/if}
        <div class="ghost" style:left={`${tabDrag.x + 12}px`} style:top={`${tabDrag.y + 10}px`} aria-hidden="true">
          {#if screenOf(tabDrag.node)}<Icon name={screenOf(tabDrag.node)!.icon} size={14} />{/if}
          {app.find(tabDrag.node)?.label ?? t?.label}
        </div>
      {/if}
      <div class="fixed">
        <button class="plus" title="Layouts: split the editor, save how it is divided" aria-label="Layouts" onpointerdown={(e) => e.stopPropagation()} onclick={(e) => { const r = e.currentTarget.getBoundingClientRect(); ui.openMenu(r.left, r.bottom + 4, layoutsMenu()) }}><Icon name="panes" /></button>
        <button class="plus" title="New…" aria-label="New" onpointerdown={(e) => e.stopPropagation()} onclick={(e) => { const r = e.currentTarget.getBoundingClientRect(); ui.openMenu(r.left, r.bottom + 4, newMenu()) }}><Icon name="plus" /></button>
        {#if app.tree}
          <!-- Always shown, even with no environments yet: it is where a new user learns they need one. -->
          <button class="envsel" class:none={!app.tree.selected} class:tinted={!!selectedSwatch} style:--env={selectedSwatch} title="Environment: where {'{{env.…}}'} values come from" aria-label="Environment: {app.tree.selected || 'none'}" aria-haspopup="menu"
            onpointerdown={(e) => e.stopPropagation()} onclick={(e) => { const r = e.currentTarget.getBoundingClientRect(); ui.openMenu(r.left, r.bottom + 4, environmentMenu()) }}>
            <span class="envdot" aria-hidden="true"></span>
            <span class="envname">{app.tree.selected || 'No environment'}</span>
            <svg class="envchev" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 9l6 6 6-6" /></svg>
          </button>
          {#if overrides.count > 0}
            <button class="ovr" title="What {app.tree.selected} changes about {overrides.extends}: click to compare" aria-label="{overrides.count} overrides of {overrides.extends}"
              onpointerdown={(e) => e.stopPropagation()} onclick={() => (ui.envCompare = { a: overrides.extends, b: app.tree?.selected ?? '' })}>{overrides.count} override{overrides.count === 1 ? '' : 's'}</button>
          {/if}
        {/if}
      </div>
    </nav>
    <!-- Below the tab strip, not above it: the window's top 40 points are its (invisible) title bar. -->
    <EnvOffers />
    <div class="body" class:split={multiple} bind:this={bodyEl}>
      {#if tabDrag?.active && tabDrag.zone}
        {@const z = tabDrag.zone}
        {@const w = z.rect.right - z.rect.left}
        {@const h = z.rect.bottom - z.rect.top}
        <div class="zone" style:left={`${z.side === 'right' ? z.rect.left + w / 2 : z.rect.left}px`} style:top={`${z.side === 'bottom' ? z.rect.top + h / 2 : z.rect.top}px`}
          style:width={`${z.side === 'left' || z.side === 'right' ? w / 2 : w}px`} style:height={`${z.side === 'top' || z.side === 'bottom' ? h / 2 : h}px`} aria-hidden="true">
          <span>Open {z.side === 'left' || z.side === 'right' ? `on the ${z.side}` : z.side === 'top' ? 'above' : 'below'}</span>
        </div>
      {/if}
      {@render arranged(app.layout, [])}
      {#if app.error}<p class="err">{app.error}</p>{/if}
    </div>
    <Footer />
  </section>
</div>
{#snippet arranged(l: Layout, path: number[])}
  {#if l.kind === 'split'}
    {@const shares = sharesOf(l)}
    <div class="arrange {l.dir}" data-split={path.join('.')}>
      {#each l.children as child, i (child.kind === 'pane' ? child.id : `s${i}`)}
        {#if i > 0}
          <!-- A focusable separator is the ARIA window splitter: interactive, which Svelte's check does not know. -->
          <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
          <div
            class="divider {l.dir}"
            role="separator"
            tabindex="0"
            aria-orientation={l.dir === 'row' ? 'vertical' : 'horizontal'}
            aria-label="Resize the panes"
            aria-valuenow={Math.round(shares.slice(0, i).reduce((a, b) => a + b, 0) * 100)}
            aria-valuemin="0"
            aria-valuemax="100"
            onpointerdown={(e) => dividerDown(e, l.dir, path, i - 1)}
            onkeydown={(e) => dividerKey(e, l.dir, path, i - 1, shares)}
          ></div>
        {/if}
        <div class="cell" style:flex={`${shares[i]} 1 0`}>{@render arranged(child, [...path, i])}</div>
      {/each}
    </div>
  {:else}
    {@const tab = app.tabs.find((t) => t.node === l.node)}
    <div class="pane" class:focus={!multiple || app.focus === l.id} data-pane={l.id} onpointerdowncapture={() => (app.focus = l.id)} role="presentation">
      {#if multiple}
        <div class="sidehead">
          <span class="sidetitle" title={app.followPane === l.id ? 'Shows the request of the step selected in the flow' : ''}>{tab ? (app.find(tab.node)?.label ?? tab.label) : ''}{#if app.followPane === l.id} <span class="following">· follows the selected step</span>{/if}</span>
          <button class="x" aria-label="Close the pane" title="Close the pane: this tab stays open" onclick={() => app.closePaneById(l.id)}>×</button>
        </div>
      {/if}
      <div class="panebody">{@render editor(tab)}</div>
    </div>
  {/if}
{/snippet}
{#snippet editor(tab: Tab | undefined)}
  {#if tab && tab.kind === 'flow' && app.flows[tab.node]}
    {#key tab.node}
      <FlowEditor view={app.flows[tab.node]} />
    {/key}
  {:else if tab && tab.kind === 'request' && app.requests[tab.node]}
    {#key tab.node}
      <RequestEditor view={app.requests[tab.node]} />
    {/key}
  {:else if tab && tab.kind === 'hooks' && app.hooks[tab.node.slice(6)]}
    {#key tab.node}
      <HooksEditor view={app.hooks[tab.node.slice(6)]} />
    {/key}
  {:else if tab && tab.kind === 'environment' && app.envs[tab.node]}
    {#key tab.node}
      <EnvEditor view={app.envs[tab.node]} />
    {/key}
  {:else if tab && tab.node === 'screen:home'}
    <HomeScreen />
  {:else if tab && tab.node === 'screen:welcome'}
    <WelcomeScreen />
  {:else if tab && tab.node === 'screen:telemetry'}
    <TelemetryScreen />
  {:else if tab && tab.node === 'screen:tracing'}
    <TracingScreen />
  {:else if tab && tab.node === 'screen:settings'}
    <SettingsScreen />
  {:else if tab && tab.node === 'screen:secrets'}
    <SecretsScreen />
  {:else if tab && tab.node === 'screen:blueprints'}
    <BlueprintsScreen />
  {:else if tab && tab.node === 'screen:trash'}
    <TrashScreen />
  {:else if tab && tab.node === 'screen:checks'}
    <ChecksScreen />
  {:else if tab && tab.node === 'screen:replayer'}
    <ReplayerScreen />
  {:else if tab && tab.node === 'screen:schedules'}
    <SchedulesScreen />
  {:else if tab && tab.node === 'screen:import'}
    <ImportScreen />
  {:else if tab && tab.node === 'screen:manager'}
    <ManagerScreen />
  {:else if tab}
    <p class="empty">{tab.label}: this kind of document is not ported to the new window yet.</p>
  {:else}
    <p class="empty">Open a request or a flow from the sidebar.</p>
  {/if}
{/snippet}

<Overlays />
<PasteOffer />

<style>
  .shell { display: flex; height: 100%; }
  section { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  /* The window's buttons sit over the top-left corner, which the rail is too narrow to clear. */
  .folded .tabs { padding-left: 48px; }
  .tabs {
    height: 44px;
    flex: none;
    display: flex;
    align-items: flex-end;
    background: var(--chrome);
    --wails-draggable: drag;
  }
  /* The way back to Home, ahead of the tabs and outside their scrolling strip, so it is always there. */
  .home { flex: none; align-self: center; --wails-draggable: no-drag; margin: 0 0 0 8px; padding: 4px; display: grid; place-items: center; background: none; border: 0; border-radius: var(--r-md); opacity: 0.85; }
  .home:hover { background: var(--tint-hover); opacity: 1; }
  .home.on { background: var(--tint-sel); opacity: 1; }
  .strip { flex: 1; min-width: 0; display: flex; align-items: flex-end; gap: 2px; padding: 0 8px; overflow-x: auto; scrollbar-width: none; align-self: stretch; }
  .strip::-webkit-scrollbar { display: none; }
  /* A soft edge where more tabs are scrolled out of sight. A mask reads only alpha, so black is opaque, not a colour. */
  .strip.fade-right { mask-image: linear-gradient(to right, black calc(100% - 32px), transparent); }
  .strip.fade-left { mask-image: linear-gradient(to left, black calc(100% - 32px), transparent); }
  .strip.fade-left.fade-right { mask-image: linear-gradient(to right, transparent, black 32px, black calc(100% - 32px), transparent); }
  .tab {
    flex: none;
    display: flex;
    align-items: center;
    max-width: 240px;
    border: 1px solid transparent;
    border-bottom: 0;
    border-radius: var(--r-lg) var(--r-lg) 0 0;
    --wails-draggable: no-drag;
  }
  .tab.dragging { opacity: 0.35; cursor: grabbing; }
  .ghost {
    position: fixed; z-index: 80; pointer-events: none; display: flex; align-items: center; gap: 6px; padding: 6px 12px;
    background: var(--raised); color: var(--text); border: 1px solid var(--accent); border-radius: var(--r-md); box-shadow: var(--shadow); white-space: nowrap;
  }
  .zone { position: fixed; z-index: 78; display: grid; place-items: center; background: color-mix(in srgb, var(--accent) 14%, transparent); box-shadow: inset 0 0 0 2px var(--accent); border-radius: var(--r-lg); pointer-events: none; }
  .zone span { padding: 4px 10px; border-radius: var(--r-md); background: var(--accent); color: var(--bg); font-size: var(--fs-sm); font-weight: 600; }
  .dropmark { position: fixed; z-index: 79; width: 2px; margin-left: -1px; background: var(--accent); border-radius: 1px; pointer-events: none; }
  .dropmark span {
    /* Left of the marker: the ghost trails the pointer to the right and hid a centred label. */
    position: absolute; top: 100%; right: 0; transform: translate(-4px, 4px); padding: 1px 6px; white-space: nowrap;
    font-size: var(--fs-xs); color: var(--on-accent); background: var(--accent); border-radius: var(--r-sm);
  }
  .tab:hover { background: var(--tint-hover); }
  .tab.active { background: var(--bg); border-color: var(--line); margin-bottom: -1px; position: relative; z-index: 2; }
  .tab button { background: none; border: 0; color: var(--text); font: inherit; }
  .tab .name { display: flex; align-items: center; gap: 6px; padding: 8px 6px 8px 8px; min-width: 0; }
  .tab:not(.active) .name { color: var(--muted); }
  .tab .title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  /* The edited dot's room is always kept, so editing never resizes a tab. */
  .dirty { width: 8px; font-size: 8px; color: var(--dirty); visibility: hidden; }
  .dirty.on { visibility: visible; }
  .tab .x { color: var(--muted); padding: 8px 8px 8px 2px; }
  .tab .x:hover { color: var(--text); }
  .fixed { flex: none; display: flex; align-items: center; gap: 6px; padding: 0 10px 8px 6px; --wails-draggable: no-drag; }
  .plus { background: none; border: 1px solid var(--line); border-radius: var(--r-md); color: var(--muted); padding: 3px 5px; display: grid; place-items: center; }
  .plus:hover { color: var(--text); border-color: var(--accent); }
  .ovr { background: transparent; color: var(--muted); border: 1px solid var(--line); border-radius: var(--r-pill); font: inherit; font-size: var(--fs-xs); padding: 2px 8px; }
  .ovr:hover { color: var(--text); background: var(--raised-3); }
  .envsel.tinted { background: color-mix(in srgb, var(--env) 30%, var(--bg)); border-color: var(--env); }
  .envsel.none { color: var(--warn); border-color: color-mix(in srgb, var(--warn) 60%, var(--line)); }
  .envsel { display: flex; align-items: center; gap: 6px; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-pill); font: inherit; padding: 3px 8px 3px 10px; max-width: 180px; }
  .envdot { flex: none; width: 8px; height: 8px; border-radius: 50%; background: var(--env, var(--muted)); }
  .envsel.none .envdot { background: var(--warn); }
  .envname { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .envchev { flex: none; color: var(--muted); }
  .body { flex: 1; min-height: 0; position: relative; background: var(--bg); border-top: 1px solid var(--line); border-left: 1px solid var(--line); display: flex; }
  /* One pane fills the body; a split gives each half, with the focused one marked by a line along its top. */
  .pane { flex: 1; min-width: 0; min-height: 0; position: relative; display: flex; flex-direction: column; overflow: hidden; }
  .split .pane { border-top: 2px solid transparent; }
  .split .pane.focus { border-top-color: var(--accent); }
  .arrange { flex: 1; min-width: 0; min-height: 0; display: flex; }
  .arrange.row { flex-direction: row; }
  .arrange.col { flex-direction: column; }
  /* A cell holds a pane or a split and takes the share it was given; min-width/height 0 lets it shrink below its content. */
  .cell { min-width: 0; min-height: 0; display: flex; }
  .divider { flex: none; position: relative; background: var(--line); }
  .divider.row { width: 1px; cursor: col-resize; }
  .divider.col { height: 1px; cursor: row-resize; }
  /* The line is one pixel; the hit area around it is wider so it can be grabbed. */
  .divider::after { content: ''; position: absolute; }
  .divider.row::after { top: 0; bottom: 0; left: -3px; right: -3px; }
  .divider.col::after { left: 0; right: 0; top: -3px; bottom: -3px; }
  .divider:hover, .divider:focus-visible { background: var(--accent); outline: none; }
  :global(body.resizing) { user-select: none; cursor: col-resize; }
  /* The editors fill their parent with position: absolute, so the body is the box that sits below the pane header. */
  .panebody { flex: 1; min-height: 0; position: relative; }
  .sidehead { flex: none; display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 2px 8px; font-size: var(--fs-xs); color: var(--muted); border-bottom: 1px solid var(--line); }
  .sidetitle { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .following { opacity: 0.75; }
  .sidehead .x { background: none; border: 0; color: inherit; font-size: 16px; line-height: 1; padding: 0 4px; cursor: pointer; }
  .sidehead .x:hover { color: var(--text); }
  .empty { color: var(--muted); text-align: center; margin-top: 20vh; }
  .err { position: absolute; left: 16px; bottom: 16px; color: var(--fail); }
</style>
