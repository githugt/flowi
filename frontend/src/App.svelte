<script lang="ts">
  import { markerAt, slotAmong } from './lib/reorder'
  import { screenOf } from './lib/screens'
  import { onMount, tick } from 'svelte'
  import { SessionService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { app } from './lib/app.svelte'
  import Sidebar from './Sidebar.svelte'
  import Splitter from './Splitter.svelte'
  import Verb from './Verb.svelte'
  import Icon from './Icon.svelte'
  import { ui } from './lib/ui.svelte'
  import { newMenu, promptNewEnvironment } from './lib/menus'
  import EnvOffers from './EnvOffers.svelte'

  let sidebarWidth = $state(260)
  import FlowEditor from './FlowEditor.svelte'
  import RequestEditor from './RequestEditor.svelte'
  import Overlays from './Overlays.svelte'
  import PasteOffer from './PasteOffer.svelte'
  import HooksEditor from './HooksEditor.svelte'
  import WelcomeScreen from './WelcomeScreen.svelte'
  import TelemetryScreen from './TelemetryScreen.svelte'
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
  import { envSwatch } from './lib/envcolours'

  onMount(() => app.init())

  // The open tabs are kept for the next session, once the last one's are back.
  $effect(() => {
    const rows = app.tabRows
    if (app.restored) SessionService.SaveTabs(rows)
  })

  const tab = $derived(app.tabs.find((t) => t.node === app.active))
  // The picker takes the colour of the environment it shows, so a production one cannot be mistaken for a test one.
  const selectedSwatch = $derived(envSwatch(app.tree?.envColours?.[app.tree?.selected ?? '']))
  const NEW_ENV = ':new' // the picker's "New environment…" entry; ':' is not allowed in an environment name

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

<div class="shell">
  <Sidebar width={sidebarWidth} />
  <Splitter name="sidebar" side="left" min={200} max={520} bind:width={sidebarWidth} />
  <section>
    <Announcements />
    <nav class="tabs">
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
        <div class="dropmark" style:left={`${tabDrag.markX}px`} style:top={`${strip0?.top ?? 0}px`} style:height={`${strip0?.height ?? 0}px`} aria-hidden="true"><span>Drop here</span></div>
        <div class="ghost" style:left={`${tabDrag.x + 12}px`} style:top={`${tabDrag.y + 10}px`} aria-hidden="true">
          {#if screenOf(tabDrag.node)}<Icon name={screenOf(tabDrag.node)!.icon} size={14} />{/if}
          {app.find(tabDrag.node)?.label ?? t?.label}
        </div>
      {/if}
      <div class="fixed">
        <button class="plus" title="New…" aria-label="New" onpointerdown={(e) => e.stopPropagation()} onclick={(e) => { const r = e.currentTarget.getBoundingClientRect(); ui.openMenu(r.left, r.bottom + 4, newMenu()) }}><Icon name="plus" /></button>
        {#if app.tree}
          <!-- Always shown, even with no environments yet: it is where a new user learns they need one. -->
          <select class="envsel" class:none={!app.tree.selected} class:tinted={!!selectedSwatch} style:--env={selectedSwatch} title="Environment: where {'{{env.…}}'} values come from" aria-label="Environment" value={app.tree.selected}
            onchange={(e) => {
              const v = e.currentTarget.value
              if (v === NEW_ENV) {
                e.currentTarget.value = app.tree?.selected ?? ''
                promptNewEnvironment()
              } else app.selectEnvironment(v)
            }}>
            <option value="">No environment</option>
            {#each app.tree.environments as name}<option value={name}>{name}</option>{/each}
            <option disabled>──────────</option>
            <option value={NEW_ENV}>New environment…</option>
          </select>
        {/if}
      </div>
    </nav>
    <!-- Below the tab strip, not above it: the window's top 40 points are its (invisible) title bar. -->
    <EnvOffers />
    <div class="body">
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
      {:else if tab && tab.node === 'screen:welcome'}
        <WelcomeScreen />
      {:else if tab && tab.node === 'screen:telemetry'}
        <TelemetryScreen />
      {:else if tab && tab.node === 'screen:settings'}
        <SettingsScreen />
      {:else if tab && tab.node === 'screen:secrets'}
        <SecretsScreen />
      {:else if tab && tab.node === 'screen:blueprints'}
        <BlueprintsScreen />
      {:else if tab && tab.node === 'screen:trash'}
        <TrashScreen />
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
      {#if app.error}<p class="err">{app.error}</p>{/if}
    </div>
    <Footer />
  </section>
</div>
<Overlays />
<PasteOffer />

<style>
  .shell { display: flex; height: 100%; }
  section { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .tabs {
    height: 44px;
    flex: none;
    display: flex;
    align-items: flex-end;
    background: var(--panel);
    border-bottom: 1px solid var(--line);
    --wails-draggable: drag;
  }
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
    border-bottom: 2px solid transparent;
    border-radius: 8px 8px 0 0;
    --wails-draggable: no-drag;
  }
  .tab.dragging { opacity: 0.35; cursor: grabbing; }
  .ghost {
    position: fixed; z-index: 80; pointer-events: none; display: flex; align-items: center; gap: 6px; padding: 6px 12px;
    background: var(--panel); color: var(--text); border: 1px solid var(--accent); border-radius: var(--r-md); box-shadow: var(--shadow); white-space: nowrap;
  }
  .dropmark { position: fixed; z-index: 79; width: 2px; margin-left: -1px; background: var(--accent); border-radius: 1px; pointer-events: none; }
  .dropmark span {
    /* Left of the marker: the ghost trails the pointer to the right and hid a centred label. */
    position: absolute; top: 100%; right: 0; transform: translate(-4px, 4px); padding: 1px 6px; white-space: nowrap;
    font-size: var(--fs-xs); color: var(--on-accent); background: var(--accent); border-radius: var(--r-sm);
  }
  .tab:hover { background: color-mix(in srgb, var(--accent) 8%, transparent); }
  .tab.active { background: var(--bg); border-color: var(--line); border-bottom-color: var(--accent); }
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
  .envsel.tinted { background: color-mix(in srgb, var(--env) 30%, var(--bg)); border-color: var(--env); }
  .envsel.none { color: var(--warn); border-color: color-mix(in srgb, var(--warn) 60%, var(--line)); }
  .envsel { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); font: inherit; padding: 3px 6px; max-width: 160px; }
  .body { flex: 1; min-height: 0; position: relative; }
  .empty { color: var(--muted); text-align: center; margin-top: 20vh; }
  .err { position: absolute; left: 16px; bottom: 16px; color: var(--fail); }
</style>
