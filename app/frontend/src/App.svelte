<script lang="ts">
  import { windowPaste } from './lib/paste'
  import { loadActive, rememberable, saveActive } from './lib/activetab'
  import { watermarkHints } from './lib/watermark'
  import { isMac } from './lib/commands'
  import { createTabSaver } from './lib/tabsave'
  import Button from './Button.svelte'
  import { markerAt, slotAmong } from './lib/reorder'
  import { screenOf } from './lib/screens'
  import { onMount, tick } from 'svelte'
  import { SessionService, WorkspaceService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { closeNotice, closePlan, hasFolder, type CloseMode } from './lib/tabmenu'
  import { app } from './lib/app.svelte'
  import Sidebar from './Sidebar.svelte'
  import Splitter from './Splitter.svelte'
  import Rail from './Rail.svelte'
  import { FOLD_NOTE, collapsedOnOpen, loadPins, savePins, shouldExplainFold, type TabKind } from './lib/sidebar'
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
    const was = collapsed
    collapsed = collapsedOnOpen(kindOf(active), collapsed, pins, app.sidebarMode)
    if (!was && collapsed && app.restored && app.sidebarMode === 'auto' && shouldExplainFold(store)) ui.toast(FOLD_NOTE)
  })
  // Ctrl+B and the palette fold or unfold it.
  let seenToggles = app.sidebarToggles
  $effect(() => {
    const n = app.sidebarToggles
    if (n === seenToggles) return
    seenToggles = n
    collapsed = !collapsed
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
  import AboutScreen from './AboutScreen.svelte'
  import TelemetryScreen from './TelemetryScreen.svelte'
  import TracingScreen from './TracingScreen.svelte'
  import Announcements from './Announcements.svelte'
  import Busy from './Busy.svelte'
  import EnvEditor from './EnvEditor.svelte'
  import { crashDetails } from './lib/crashtext'
  import { copy } from './lib/clip'
  import Footer from './Footer.svelte'
  import SettingsScreen from './SettingsScreen.svelte'
  import SecretsScreen from './SecretsScreen.svelte'
  import ManagerScreen from './ManagerScreen.svelte'
  import ImportScreen from './ImportScreen.svelte'
  import SchedulesScreen from './SchedulesScreen.svelte'
  import ReplayerScreen from './ReplayerScreen.svelte'
  import BlueprintsScreen from './BlueprintsScreen.svelte'
  import AuthsScreen from './AuthsScreen.svelte'
  import TrashScreen from './TrashScreen.svelte'
  import EnvCompareScreen from './EnvCompareScreen.svelte'
  import ChecksScreen from './ChecksScreen.svelte'
  import IconButton from './IconButton.svelte'
  import { bandColour, envSwatch } from './lib/envcolours'
  import { createAltTap, isWindows } from './lib/altmenu'
  import { actionForKey, pageAnswersShortcuts } from './lib/mainmenu'
  import { Events } from '@wailsio/runtime'
  import { loadZoom, saveZoom, zoomForKey } from './lib/zoom'
  import { windowTitle } from './lib/wintitle'
  import { windowKeyAction } from './lib/shortcuts'

  onMount(() => app.init())

  // Ctrl or Cmd with + - 0 zooms the whole window; the zoom is kept for the next start.
  onMount(() => {
    let zoom = loadZoom()
    const apply = () => ((document.documentElement.style as CSSStyleDeclaration & { zoom: string }).zoom = String(zoom))
    apply()
    const keys = (e: KeyboardEvent) => {
      const next = zoomForKey(e, zoom)
      if (next === null) return
      e.preventDefault()
      zoom = next
      apply()
      saveZoom(zoom)
    }
    window.addEventListener('keydown', keys)
    return () => window.removeEventListener('keydown', keys)
  })

  // Tab keys that need no menu item, and the "?" sheet. Not while a text field has the keyboard.
  onMount(() => {
    const keys = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement | null
      const typing = !!t && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName) || !!t.closest?.('.cm-editor'))
      const action = windowKeyAction(e, typing)
      if (!action) return
      e.preventDefault()
      app.onMenu(action)
    }
    window.addEventListener('keydown', keys)
    return () => window.removeEventListener('keydown', keys)
  })

  // Linux has no native menu to carry the shortcuts, and on Windows the bar is hidden until Alt: the page answers them
  // (the menu button lists them).
  onMount(() => {
    if (!pageAnswersShortcuts(navigator.userAgent)) return
    const keys = (e: KeyboardEvent) => {
      const action = actionForKey(e)
      if (!action) return
      e.preventDefault()
      app.onMenu(action)
    }
    window.addEventListener('keydown', keys)
    return () => window.removeEventListener('keydown', keys)
  })

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
  // A burst of changes (a drag reorder) is one save, and the same ids again (another tab became active) is none.
  const saveTabs = createTabSaver((rows) => SessionService.SaveTabs(rows))
  $effect(() => {
    const rows = app.tabRows
    if (app.restored) saveTabs(rows)
  })
  // A curl command, an API description or a request file pasted anywhere that does not take text is offered for
  // import: the offer opens with no request to fill, so it only imports.
  function onWindowPaste(e: ClipboardEvent) {
    const text = e.clipboardData?.getData('text/plain') ?? ''
    const kind = windowPaste(e.target, text, e.defaultPrevented || !!app.pasteOffer)
    if (!kind) return
    e.preventDefault()
    app.pasteOffer = { node: '', kind, text, http: false, field: null }
  }

  // Which of them is in front is kept too (a screen in front leaves the last document as it was).
  $effect(() => {
    if (!app.restored) return
    saveActive(rememberable(app.active, app.tabRows, loadActive()))
  })

  // An error shown over the tabs belongs to what was being done: switching tab leaves it behind.
  let shownOn = app.active
  $effect(() => {
    const now = app.active
    if (now === shownOn) return
    shownOn = now
    app.error = ''
  })

  const tab = $derived(app.tabs.find((t) => t.node === app.active))
  // The window title follows the open tab.
  $effect(() => {
    document.title = windowTitle(tab ? (app.find(tab.node)?.label ?? tab.label) : '', tab ? app.isDirty(tab.node) : false)
  })
  // The picker takes the colour of the environment it shows, so a production one cannot be mistaken for a test one.
  const selectedSwatch = $derived(envSwatch(app.tree?.envColours?.[app.tree?.selected ?? '']))
  const band = $derived(bandColour(app.tree))
  // The environment chip's menu: "No environment", each environment (the current one ticked), then a way to make one.
  function environmentMenu(): MenuItem[] {
    const selected = app.tree?.selected ?? ''
    const choices = ['', ...(app.tree?.environments ?? [])]
    const items: MenuItem[] = choices.map((name) => ({ label: name || 'No environment', checked: name === selected, run: () => app.selectEnvironment(name) }))
    return [...items, { label: '', separator: true, run: () => {} }, { label: 'New environment…', run: promptNewEnvironment }, { label: 'Compare environments…', run: () => app.openScreen('screen:envcompare', 'Compare environments') }]
  }

  // Tabs are reordered by dragging with the pointer (the browser's own drag and drop
  // is not reliable in the window's web view). A press only becomes a drag after a few pixels.
  // The drag listens on the window, not with a pointer capture on the tab: moving the tab
  // in the page drops its capture, and the rest of the drag went astray.
  // While a tab is dragged it stays in place, dimmed; a copy follows the pointer (x, y) and a bar
  // marks where it would drop (markX, between tabs). The tabs move once, on release: moving them
  // under the pointer as it went left nothing to show where the tab would land.
  let tabDrag = $state<{ node: string; startX: number; active: boolean; x: number; y: number; slot: number; markX: number } | null>(null)
  let strip = $state<HTMLDivElement>()
  let dragged = false // the click that ends a drag is not a click on the tab

  // Right-click on a tab: close it, the others or those to its right (a tab with unsaved edits stays), show its folder.
  function tabMenu(e: MouseEvent, node: string) {
    e.preventDefault()
    const nodes = app.tabs.map((t) => t.node)
    const bulk = async (mode: CloseMode) => {
      const plan = closePlan(nodes, node, mode, (id) => app.isDirty(id))
      await app.closeAll(plan.close)
      const note = closeNotice(plan.close.length, plan.keep.length)
      if (note) ui.toast(note)
    }
    ui.openMenu(e.clientX, e.clientY, [
      { label: 'Close', run: () => app.requestClose(node) },
      { label: 'Close others', disabled: nodes.length < 2, run: () => bulk('others') },
      { label: 'Close to the right', disabled: nodes.indexOf(node) >= nodes.length - 1, run: () => bulk('right') },
      ...(hasFolder(node) ? [{ separator: true, label: '', run: () => {} }, { label: 'Show folder', run: () => void WorkspaceService.RevealLocation(node).catch(ui.fail) }] : []),
    ])
  }

  function tabDown(e: PointerEvent, node: string) {
    if (e.button !== 0 || (e.target as HTMLElement).closest('.x')) return
    tabDrag = { node, startX: e.clientX, active: false, x: e.clientX, y: e.clientY, slot: -1, markX: 0 }
    window.addEventListener('pointermove', tabMove)
    window.addEventListener('pointerup', tabUp)
    window.addEventListener('pointercancel', tabUp)
  }

  function tabMove(e: PointerEvent) {
    if (!tabDrag || !strip) return
    if (!(e.buttons & 1)) return tabUp() // released where no pointerup reached us
    if (!tabDrag.active && Math.abs(e.clientX - tabDrag.startX) < 5) return
    const node = tabDrag.node
    const others = [...strip.querySelectorAll<HTMLElement>('[data-tab]')].filter((t) => t.dataset.tab !== node)
    const spans = others.map((t) => ({ start: t.getBoundingClientRect().left, end: t.getBoundingClientRect().right }))
    const slot = slotAmong(spans, e.clientX)
    tabDrag = { ...tabDrag, active: true, x: e.clientX, y: e.clientY, slot, markX: markerAt(spans, slot, strip.getBoundingClientRect().left) }
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
    if (tabDrag?.active) app.moveTabTo(tabDrag.node, tabDrag.slot)
    dragged = !!tabDrag?.active
    tabDrag = null
    window.removeEventListener('pointermove', tabMove)
    window.removeEventListener('pointerup', tabUp)
    window.removeEventListener('pointercancel', tabUp)
  }
</script>

<svelte:window onpagehide={() => saveTabs.flush()} onpaste={onWindowPaste} />

<div class="shell">
  {#if collapsed}
    <Rail width={sidebarWidth} onexpand={() => (collapsed = false)} onpin={pinOpen} />
  {:else}
    <Sidebar width={sidebarWidth} onfold={fold} />
    <Splitter name="sidebar" side="left" min={200} max={520} bind:width={sidebarWidth} oncollapse={fold} />
  {/if}
  <section class:folded={collapsed}>
    <Busy />
    <Announcements />
    <nav class="tabs">
      <!-- With the sidebar folded, Home is the top of the rail instead. -->
      {#if !collapsed}
        <button class="home" class:on={app.active === 'screen:home'} title="Home" aria-label="Home" aria-current={app.active === 'screen:home' ? 'page' : undefined} onpointerdown={(e) => e.stopPropagation()} onclick={() => app.openScreen('screen:home', 'Home')}><Logo size={22} /></button>
      {/if}
      <div class="strip" role="tablist" aria-label="Open tabs" class:fade-left={overflow.left} class:fade-right={overflow.right} bind:this={strip} onscroll={measureOverflow} onwheel={stripWheel}>
        {#each app.tabs as t (t.node)}
          {@const verb = t.kind === 'flow' ? 'FLOW' : app.requests[t.node]?.verb || app.find(t.node)?.method || ''}
          <div
            class="tab"
            class:active={t.node === app.active}
            class:dragging={tabDrag?.active && tabDrag.node === t.node}
            data-tab={t.node}
            role="presentation"
            onpointerdown={(e) => tabDown(e, t.node)}
            oncontextmenu={(e) => tabMenu(e, t.node)}
          >
            <button class="name" role="tab" aria-selected={t.node === app.active} title={app.find(t.node)?.label ?? t.label} onclick={() => { if (!dragged) app.active = t.node; dragged = false }}>
              <span class="dirty" class:on={app.isDirty(t.node)}>●</span>
              {#if verb}<Verb {verb} pill={false} short />{:else if screenOf(t.node)}<Icon name={screenOf(t.node)!.icon} size={14} />{/if}
              <span class="title">{app.find(t.node)?.label ?? t.label}</span>
            </button>
            <span class="x"><IconButton icon="close" size={12} label="Close {app.find(t.node)?.label ?? t.label}" onclick={() => app.requestClose(t.node)} /></span>
          </div>
        {/each}
      </div>
      {#if tabDrag?.active}
        {@const t = app.tabs.find((x) => x.node === tabDrag!.node)}
        {@const strip0 = strip?.getBoundingClientRect()}
        <div class="dropmark" style:left={`${tabDrag.markX}px`} style:top={`${strip0?.top ?? 0}px`} style:height={`${strip0?.height ?? 0}px`} aria-hidden="true"><span>Drop here</span></div>
        <div class="ghost" style:left={`${tabDrag.x + 12}px`} style:top={`${tabDrag.y + 10}px`} aria-hidden="true">
          {#if screenOf(tabDrag.node)}<Icon name={screenOf(tabDrag.node)!.icon} size={14} />{/if}
          {app.find(tabDrag.node)?.label ?? t?.label}
        </div>
      {/if}
      <div class="fixed">
        <button class="plus" title="New…" aria-label="New" onpointerdown={(e) => e.stopPropagation()} onclick={(e) => { const r = e.currentTarget.getBoundingClientRect(); ui.openMenu(r.left, r.bottom + 4, newMenu({ x: r.left, y: r.bottom + 4 })) }}><Icon name="plus" /></button>
        {#if app.tree}
          <!-- Always shown, even with no environments yet: it is where a new user learns they need one. -->
          <button class="envsel" class:none={!app.tree.selected && app.envWanted} class:tinted={!!selectedSwatch} style:--env={selectedSwatch} title="Environment: where {'{{env.…}}'} values come from" aria-label="Environment: {app.tree.selected || 'none'}" aria-haspopup="menu"
            onpointerdown={(e) => e.stopPropagation()} onclick={(e) => { const r = e.currentTarget.getBoundingClientRect(); ui.openMenu(r.left, r.bottom + 4, environmentMenu()) }}>
            <span class="envdot" aria-hidden="true"></span>
            <span class="envname">{app.tree.selected || 'No environment'}</span>
            <svg class="envchev" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 9l6 6 6-6" /></svg>
          </button>
        {/if}
      </div>
    </nav>
    {#if band}<div class="envband" style:--env={band} role="presentation" title="The environment in use is tinted"></div>{/if}
    <!-- Below the tab strip, not above it: the window's top 40 points are its (invisible) title bar. -->
    <EnvOffers />
    <main class="body">
      {#key tab?.node}
      <svelte:boundary onerror={(e) => console.error('a tab failed to display', e)}>
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
      {:else if tab && tab.node === 'screen:about'}
        <AboutScreen />
      {:else if tab && tab.node === 'screen:telemetry'}
        <TelemetryScreen />
      {:else if tab && tab.node === 'screen:tracing'}
        <TracingScreen />
      {:else if tab && tab.node === 'screen:settings'}
        <SettingsScreen />
      {:else if tab && tab.node === 'screen:secrets'}
        <SecretsScreen />
      {:else if tab && tab.node === 'screen:auths'}
        <AuthsScreen />
      {:else if tab && tab.node === 'screen:blueprints'}
        <BlueprintsScreen />
      {:else if tab && tab.node === 'screen:trash'}
        <TrashScreen />
      {:else if tab && tab.node === 'screen:envcompare'}
        <EnvCompareScreen />
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
        <div class="empty" role="status"><strong>{tab.label}</strong> cannot be shown in this window.<br /><Button small onclick={() => app.requestClose(tab.node)}>Close tab</Button></div>
      {:else if !app.restored}
        <div class="opening" role="status" aria-busy="true"><Logo size={48} /><span>Opening your workspace…</span></div>
      {:else}
        <div class="watermark">
          <Logo size={56} />
          <p>Open a request or a flow from the sidebar.</p>
          <dl>{#each watermarkHints(isMac()) as h (h.keys)}<dt>{h.label}</dt><dd><kbd>{h.keys}</kbd></dd>{/each}</dl>
        </div>
      {/if}
      {#snippet failed(error, reset)}
        <div class="crashed" role="alert">
          <p><strong>This tab failed to display.</strong> Your files on disk are not affected.</p>
          <div class="row">
            <Button onclick={() => tab && app.requestClose(tab.node)}>Close tab</Button>
            <Button onclick={reset}>Try again</Button>
            <Button onclick={() => void copy(crashDetails(error, tab?.label ?? ''), 'the details')}>Copy details</Button>
          </div>
        </div>
      {/snippet}
      </svelte:boundary>
      {/key}
      {#if app.error}<p class="err" role="alert">{app.error}<Button small variant="ghost" onclick={() => (app.error = '')}>Dismiss</Button></p>{/if}
    </main>
    <Footer />
  </section>
</div>
<Overlays />
<PasteOffer />

<style>
  .shell { display: flex; height: 100%; }
  .crashed { margin: var(--sp-4); padding: var(--sp-3) var(--sp-4); border: 1px solid color-mix(in srgb, var(--fail) 45%, var(--line)); border-radius: var(--r-lg); background: color-mix(in srgb, var(--fail) 8%, transparent); }
  .crashed p { margin: 0 0 var(--sp-2); }
  .crashed .row { display: flex; gap: var(--sp-2); }
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
  .envband { height: 2px; flex: none; background: var(--env); }
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
    position: fixed; z-index: var(--z-drag); pointer-events: none; display: flex; align-items: center; gap: 6px; padding: 6px 12px;
    background: var(--raised); color: var(--text); border: 1px solid var(--accent); border-radius: var(--r-md); box-shadow: var(--shadow); white-space: nowrap;
  }
  .dropmark { position: fixed; z-index: var(--z-drag-mark); width: 2px; margin-left: -1px; background: var(--accent); border-radius: 1px; pointer-events: none; }
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
  .tab .x { display: flex; padding-right: var(--sp-1); }
  .fixed { flex: none; display: flex; align-items: center; gap: 6px; padding: 0 10px 8px 6px; --wails-draggable: no-drag; }
  .plus { background: none; border: 1px solid var(--line); border-radius: var(--r-md); color: var(--muted); padding: 3px 5px; display: grid; place-items: center; }
  .plus:hover { color: var(--text); border-color: var(--accent); }
  .envsel.tinted { background: color-mix(in srgb, var(--env) 30%, var(--bg)); border-color: var(--env); }
  .envsel.none { color: var(--warn); border-color: color-mix(in srgb, var(--warn) 60%, var(--line)); }
  .envsel { display: flex; align-items: center; gap: 6px; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-pill); font: inherit; padding: 3px 8px 3px 10px; max-width: 180px; }
  .envdot { flex: none; width: 8px; height: 8px; border-radius: 50%; background: var(--env, var(--muted)); }
  .envsel.none .envdot { background: var(--warn); }
  .envname { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .envchev { flex: none; color: var(--muted); }
  .body { flex: 1; min-height: 0; position: relative; background: var(--bg); border-top: 1px solid var(--line); border-left: 1px solid var(--line); }
  .empty { color: var(--muted); text-align: center; margin-top: 20vh; }
  .watermark { display: flex; flex-direction: column; align-items: center; gap: var(--sp-2); margin-top: 16vh; color: var(--muted); }
  .watermark :global(svg) { opacity: 0.5; }
  .watermark p { margin: 0 0 var(--sp-3); }
  .watermark dl { display: grid; grid-template-columns: auto auto; gap: var(--sp-1) var(--sp-4); margin: 0; font-size: var(--fs-sm); }
  .watermark dt { text-align: right; }
  .watermark dd { margin: 0; }
  .watermark kbd { font-family: var(--mono); padding: 1px 6px; border-radius: var(--r-sm); box-shadow: inset 0 0 0 1px var(--line); }
  .opening { display: flex; flex-direction: column; align-items: center; gap: var(--sp-3); margin-top: 20vh; color: var(--muted); }
  .err { position: absolute; left: 16px; bottom: 16px; color: var(--fail); display: flex; align-items: center; gap: 8px; }
</style>
