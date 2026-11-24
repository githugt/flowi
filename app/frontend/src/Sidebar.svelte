<script lang="ts">
  import EmptyState from './EmptyState.svelte'
  import { anchorAt, type MenuAt } from './lib/menuanchor'
  import { rowFor, typeAhead, typedChar, JUMP_DELAY_MS } from './lib/typeahead'
  import { debounced } from './lib/debounce'
  import { isMac } from './lib/commands'
  import { isEnter } from './lib/enterkey'
  import { tick } from 'svelte'
  import { app } from './lib/app.svelte'
  import { ui } from './lib/ui.svelte'
  import { errorText } from './lib/errors'
  import { WorkspaceService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import TreeItem from './TreeItem.svelte'
  import SearchField from './SearchField.svelte'
  import Menu, { type MenuItem } from './Menu.svelte'
  import Icon from './Icon.svelte'
  import MainMenuButton from './MainMenuButton.svelte'
  import { screens } from './lib/screens'
  import type { TreeNode } from './lib/types'
  import type { RailGroup } from './lib/railgroups'
  import { filterTree, firstLeaf } from './lib/treefilter'
  import { filterChoices, filterTerms } from './lib/sidefilter'
  import { SECTIONS, countLeaves, leavesOf, loadTab, loadView, saveTab, saveView, sectionRoots, sectionsFor, type PanelTab, type ViewMode } from './lib/sidebarview'
  import IconButton from './IconButton.svelte'
  import Button from './Button.svelte'
  import Tabs from './Tabs.svelte'
  import SidebarSection from './SidebarSection.svelte'
  import SidebarFilter from './SidebarFilter.svelte'
  import HoverCard from './HoverCard.svelte'
  import { hover } from './lib/hovercard.svelte'

  // peek: shown floating from the rail, with a Pin open button; otherwise it has the fold button.
  // group: when shown from a rail icon, the part of the tree that icon stands for.
  let { width, peek = false, group = '', onpin, onfold }: { width: number; peek?: boolean; group?: RailGroup | ''; onpin?: () => void; onfold?: () => void } = $props()

  // "Jump to…" narrows the tree to the rows that match, with the groups leading to them.
  let query = $state('')
  let jumpField = $state<HTMLInputElement>()
  // Cmd/Ctrl+K focuses the field of the sidebar that is open. The peeking one (floating from the rail) is focused
  // by Rail, which opens it first; only a request made since this sidebar appeared counts.
  let seenJump = app.jumpRequests
  $effect(() => {
    if (app.jumpRequests === seenJump) return
    seenJump = app.jumpRequests
    if (!peek) tick().then(() => (jumpField?.focus(), jumpField?.select()))
  })
  // The tree filters 80 ms after the last keystroke, not on each one: in a collection of thousands of rows every
  // letter re-walked the whole tree. Emptying the field and Enter do not wait.
  let filterQuery = $state('')
  const settle = debounced((q: string) => (filterQuery = q), JUMP_DELAY_MS)
  $effect(() => {
    if (query) settle(query)
    else {
      settle.cancel()
      filterQuery = ''
    }
  })
  // The tree is shown as sections (Requests, Flows, Environments) and as a tree or a list; the tab says whether it is all of
  // them or only what was opened lately. The choices are kept for the next start.
  let view = $state<ViewMode>(loadView(safeStore()))
  let tab = $state<PanelTab>(loadTab(safeStore()))
  function safeStore() {
    try {
      return localStorage
    } catch {
      return { getItem: () => null, setItem: () => {} }
    }
  }
  const pickView = (v: ViewMode) => ((view = v), saveView(safeStore(), v))
  const pickTab = (t: string) => ((tab = t as PanelTab), saveTab(safeStore(), tab))
  // A rail icon that shows the sidebar floating stands for one part of it: no tabs there, and only that section.
  const whole = $derived(!group)
  const recentTab = $derived(whole && tab === 'recent')

  const problemRows = $derived(new Set(app.panels.problems.flatMap((p) => (p.node ? [p.node] : []))))
  const filtered = $derived(filterTree(app.tree?.roots ?? [], filterQuery, { problems: problemRows }))
  const searching = $derived(!!filterQuery.trim())
  const sections = $derived(
    sectionsFor(group).map((id) => {
      const meta = SECTIONS.find((s) => s.id === id)!
      const roots = sectionRoots(filtered, id, app.sidebar.favourites, !searching)
      return { ...meta, roots, count: countLeaves(roots) }
    }),
  )
  // The sections on show: while searching, one with nothing to show is left out.
  const shown = $derived(sections.filter((s) => !searching || s.count > 0))
  // Recent: the rows opened last that are still in the tree, narrowed by the field like the rest.
  const recent = $derived(filterTree(app.sidebar.recent.flatMap((r) => app.find(r.id) ?? []), filterQuery, { problems: problemRows }))
  const shownRoots = $derived(recentTab ? recent : sections.flatMap((s) => s.roots))
  const noMatches = $derived(searching && shownRoots.length === 0)
  const listed = $derived(view === 'list')

  // The filter popover (type, status, tag, collection, problems) writes `type:grpc status:review …` into the field.
  let filterOpen = $state(false)
  let filterAt = $state({ x: 0, y: 0 })
  const choices = $derived(filterOpen ? filterChoices(app.tree?.roots ?? []) : { statuses: [], tags: [], types: [], collections: [] })
  const filtersOn = $derived(filterTerms(query).length > 0)
  function openFilter(e: MouseEvent) {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    filterAt = { x: r.right, y: r.bottom }
    filterOpen = !filterOpen
  }
  const clearRecent = () => WorkspaceService.ClearRecent().catch(fail)

  // Enter opens the first hit, the down arrow moves into the list, Escape empties the field.
  function jumpKeys(e: KeyboardEvent) {
    if (e.key === 'Escape' && query) query = ''
    else if (isEnter(e)) {
      settle.flush()
      const hit = firstLeaf(shownRoots)
      if (hit) app.open(hit)
    } else if (e.key === 'ArrowDown') (e.currentTarget as HTMLElement).closest('aside')?.querySelector<HTMLButtonElement>('.tree .row')?.focus()
    else return
    e.preventDefault()
  }

  let typing = { buffer: '', at: -Infinity }
  // Arrow keys walk the visible rows; right and left open and close a group.
  function treeKeys(e: KeyboardEvent) {
    const rows = [...(e.currentTarget as HTMLElement).querySelectorAll<HTMLButtonElement>('.row')]
    const i = rows.indexOf(document.activeElement as HTMLButtonElement)
    if (i < 0) return
    const open = rows[i].closest('[role="treeitem"]')?.getAttribute('aria-expanded')
    // Home, End and the start of a name; the buffer of typed letters empties after a short pause.
    const ch = typedChar(e)
    if (ch) typing = typeAhead(typing, ch, e.timeStamp)
    const jump = e.key === 'Home' || e.key === 'End' || ch ? rowFor(rows.map((r) => r.querySelector('.label')?.textContent ?? ''), i, e.key, ch ? typing.buffer : '') : -1
    if (jump >= 0) rows[jump].focus()
    else if (e.key === 'Home' || e.key === 'End' || ch) return
    else if (e.key === 'ArrowDown') rows[Math.min(i + 1, rows.length - 1)].focus()
    else if (e.key === 'ArrowUp') rows[Math.max(i - 1, 0)].focus()
    else if (e.key === 'ArrowRight' && open === 'false') rows[i].click()
    else if (e.key === 'ArrowLeft' && open === 'true') rows[i].click()
    else if (e.key === 'Escape' && app.marked.length) app.marked = []
    else return
    e.preventDefault()
  }

  // Nine icons did not fit the narrowest sidebar and pushed Settings past its edge, out of reach. The tools used most
  // stay as icons, Settings stays last, and the rest are named in a menu, where there is room for the words too.
  const railScreens = screens.filter((s) => s.rail)
  const pinned = ['screen:import', 'screen:manager', 'screen:secrets', 'screen:replayer', 'screen:checks']
  const tools = railScreens.filter((s) => pinned.includes(s.id))
  const more = railScreens.filter((s) => !pinned.includes(s.id) && s.id !== 'screen:settings')
  const settings = railScreens.find((s) => s.id === 'screen:settings')
  let moreOpen = $state(false)
  let moreAt = $state<MenuAt>({ x: 0, y: 0 })
  const moreActive = $derived(more.some((s) => s.id === app.active))

  // Marked rows get a bar saying how many, with what applies to all of them: marking was invisible beyond
  // the highlight, so it was not clear it had worked or what to do next.
  const requestRows = $derived.by(() => {
    const out = new Set<string>()
    const walk = (nodes: TreeNode[]) => nodes.forEach((n) => (n.kind === 'request' ? out.add(n.id) : walk(n.children ?? [])))
    walk(app.tree?.roots ?? [])
    return out
  })
  const onlyRequests = $derived(app.marked.every((id) => requestRows.has(id)))

  // The workspace's name was a label that did nothing; it opens what can be done to the workspace itself.
  let wsOpen = $state(false)
  let wsAt = $state<MenuAt>({ x: 0, y: 0 })
  const fail = (e: unknown) => ui.toast(errorText(e), 'error')
  const renameWorkspace = () =>
    ui.prompt({ title: 'Rename workspace', label: 'Name', initial: app.tree?.name ?? '', done: (name) => WorkspaceService.RenameWorkspace(name).catch(fail) })
  const revealWorkspace = () => WorkspaceService.RevealWorkspace().catch(fail)
  const openSettings = () => app.openScreen('screen:settings', 'Settings')
  const wsItems = $derived<MenuItem[]>([
    { label: 'Rename workspace…', onselect: renameWorkspace },
    { label: 'Open folder', onselect: revealWorkspace, disabled: !app.tree?.folder, title: app.tree?.folder },
    { label: 'Settings', onselect: openSettings, separated: true },
  ])
  // Only macOS draws its window buttons over the sidebar's top-left corner (a hidden inset title bar): elsewhere the
  // room kept for them is just empty space before the workspace selector.
  const onMac = isMac()
  const moreItems = $derived<MenuItem[]>(more.map((t) => ({ label: t.title, icon: t.icon, active: app.active === t.id, onselect: () => app.openScreen(t.id, t.title) })))
</script>

<aside style:width={`${width}px`}>
  <div class="head" class:peeking={peek} class:mac={onMac}>
    {#if !peek}<MainMenuButton />{/if}
    <button class="ws" aria-haspopup="menu" aria-expanded={wsOpen} title="Workspace" onclick={(e) => { wsAt = anchorAt(e.currentTarget.getBoundingClientRect(), { gap: -4 }); wsOpen = !wsOpen }}>
      <span class="avatar" aria-hidden="true">{(app.tree?.name ?? '?').slice(0, 1).toUpperCase()}</span><span class="wsname">{app.tree?.name ?? ''}</span><span class="caret" aria-hidden="true"><Icon name="chevd" size={12} /></span>
    </button>
    {#if wsOpen}
      <Menu at={wsAt} minWidth={200} label="Workspace" items={wsItems} onclose={() => (wsOpen = false)} />
    {/if}
    {#if peek}
      <button class="pin" title="Keep the sidebar open for this kind of tab" onclick={(e) => { e.stopPropagation(); onpin?.() }}>Pin open</button>
    {:else if onfold}
      <button class="pin" title="Fold the sidebar to a rail" aria-label="Fold the sidebar" onclick={onfold}>«</button>
    {/if}
  </div>
  {#if whole}
    <Tabs size="sm" label="Sidebar" active={tab} onpick={pickTab} items={[{ id: 'all', label: 'All' }, { id: 'recent', label: 'Recent' }]}>
      <span class="views" role="group" aria-label="Show as">
        <span class:on={view === 'tree'}><IconButton icon="tree" size={14} label="Tree view" hint="Show as a tree" aria-pressed={view === 'tree'} onclick={() => pickView('tree')} /></span>
        <span class:on={view === 'list'}><IconButton icon="list" size={14} label="List view" hint="Show as a list, with where each one lives" aria-pressed={view === 'list'} onclick={() => pickView('list')} /></span>
      </span>
    </Tabs>
  {/if}
  <div class="jump">
    <SearchField bind:value={query} bind:el={jumpField} placeholder="Jump to…" label="Jump to a request, flow or environment (type:grpc, tag:x, status:y, in:collection and has:problems narrow it)" onkeydown={jumpKeys} />
    <span class="filterbtn" class:on={filtersOn}>
      <IconButton icon="filter" label="Filter" hint={filtersOn ? 'Filter: some are on' : 'Filter by type, status, tag, collection or problems'} aria-haspopup="dialog" aria-expanded={filterOpen} onclick={openFilter} />
    </span>
    {#if filterOpen}<SidebarFilter bind:query {choices} x={filterAt.x} y={filterAt.y} onclose={() => (filterOpen = false)} />{/if}
  </div>
  <div class="tree" role="tree" tabindex="-1" onkeydown={treeKeys} onscrollcapture={() => hover.leave()}>
    {#if recentTab}
      <div class="scroll">
      <div class="recenthead">
        <span>Recently opened</span>
        {#if app.sidebar.recent.length}<Button variant="link" onclick={clearRecent}>Clear</Button>{/if}
      </div>
      {#each recent as node (node.id)}
        <TreeItem {node} depth={0} flat query={filterQuery} />
      {/each}
      {#if !app.sidebar.recent.length}<p class="empty">Requests and flows you open show up here.</p>{/if}
      </div>
    {:else}
      {#each shown as s, at (s.id)}
        <SidebarSection name={s.id} title={s.title} count={s.count} open={s.open} index={at} total={shown.length}>
          {#each listed || s.id === 'environments' ? leavesOf(s.roots) : s.roots as node, i (node.id)}
            <TreeItem {node} depth={0} section={s.id} flat={listed} tint={!listed && app.rainbow.length ? app.rainbow[i % app.rainbow.length] : ''} query={filterQuery} />
          {/each}
          {#if !s.roots.length && !searching}<p class="empty">{s.id === 'environments' ? 'No environments yet.' : s.id === 'flows' ? 'No flows yet.' : 'No requests yet.'}</p>{/if}
        </SidebarSection>
      {/each}
    {/if}
    {#if noMatches}<div class="scroll"><EmptyState kind="filtered" compact query={filterQuery.trim()} onclear={() => (query = '')} /></div>{/if}
  </div>
  <HoverCard />
  {#if app.marked.length > 1}
    <div class="selbar" role="toolbar" aria-label="Marked rows">
      <span>{app.marked.length} selected</span>
      <span class="grow"></span>
      {#if onlyRequests}
        <button onclick={() => (ui.moving = [...app.marked])}>Move to…</button>
      {/if}
      <button title="Clear the selection (Esc)" onclick={() => (app.marked = [])}>Clear</button>
    </div>
  {/if}
  <!-- Floating from the folded rail, the tools are already on that rail beside it: a second row of them here only
       repeated them, and its More menu never opened (the panel closed first). -->
  {#if !peek}
  <nav class="rail" aria-label="Tools">
    {#each tools as t}
      <button title={t.title} aria-label={t.title} class:on={app.active === t.id} onclick={() => app.openScreen(t.id, t.title)}><Icon name={t.icon} /></button>
    {/each}
    <button class="morebtn" class:on={moreActive || moreOpen} title="More tools" aria-label="More tools" aria-haspopup="menu" aria-expanded={moreOpen} onclick={(e) => { moreAt = anchorAt(e.currentTarget.getBoundingClientRect(), { end: true, above: true, gap: -2 }); moreOpen = !moreOpen }}>⋯</button>
    {#if settings}
      <button class="end" title={settings.title} aria-label={settings.title} class:on={app.active === settings.id} onclick={() => app.openScreen(settings.id, settings.title)}><Icon name={settings.icon} /></button>
    {/if}
    {#if moreOpen}
      <Menu at={moreAt} label="More tools" items={moreItems} onclose={() => (moreOpen = false)} />
    {/if}
  </nav>
  {/if}
</aside>

<style>
  aside {
    flex: none;
    display: flex;
    flex-direction: column;
    background: var(--chrome);
    min-height: 0;
  }
  .head { position: relative; height: 44px; display: flex; align-items: center; gap: 8px; padding: 0 12px; --wails-draggable: drag; }
  .head.mac { padding-left: 84px; }
  .head.mac.peeking { padding-left: 52px; }
  .ws {
    --wails-draggable: no-drag; min-width: 0; display: flex; align-items: center; gap: 4px; padding: 3px 6px; margin-left: -6px;
    background: none; border: 0; border-radius: var(--r-md); color: var(--text); font: inherit; font-weight: 600;
  }
  .ws:hover, .ws[aria-expanded='true'] { background: var(--tint-hover); }
  .avatar { flex: none; width: 22px; height: 22px; border-radius: var(--r-md); display: grid; place-items: center; background: linear-gradient(135deg, var(--accent), var(--data)); color: var(--on-accent); font-size: var(--fs-xs); font-weight: 700; }
  .wsname { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .caret { display: inline-flex; color: var(--muted); }
  .pin { --wails-draggable: no-drag; margin-left: auto; white-space: nowrap; flex: none; background: none; border: 1px solid var(--line); border-radius: var(--r-md); color: var(--muted); font: inherit; font-size: var(--fs-sm); padding: 1px 8px; }
  .pin:hover { color: var(--text); border-color: var(--accent); }
  .jump { margin: 6px 8px; display: flex; align-items: center; gap: 2px; }
  :global(aside > .tabs) { margin: 0 8px; }
  .views { display: inline-flex; gap: 1px; }
  .views > span { display: inline-flex; border-radius: var(--r-md); }
  .views > span.on { background: var(--tint-sel); }
  .views > span.on :global(.iconbtn) { color: var(--text); }
  /* A dot on the filter button while some filter is on, so a narrowed list is not mistaken for the whole. */
  .filterbtn { position: relative; display: inline-flex; }
  .filterbtn.on :global(.iconbtn) { color: var(--accent); }
  .filterbtn.on::after { content: ''; position: absolute; top: 3px; right: 3px; width: 6px; height: 6px; border-radius: 50%; background: var(--accent); pointer-events: none; }
  .recenthead { display: flex; align-items: center; justify-content: space-between; height: var(--row-h-lg); padding: 0 8px; color: var(--muted); font-size: var(--fs-xs); font-weight: 700; letter-spacing: 0.05em; text-transform: uppercase; }
  .recenthead :global(.link) { text-transform: none; letter-spacing: 0; font-weight: 500; font-size: var(--fs-sm); }
  .empty { margin: 2px 8px 6px 26px; color: var(--muted); font-size: var(--fs-sm); }
  .jump > :global(:first-child) { flex: 1; min-width: 0; }
  /* The tools sit in a strip at the foot, out of the way of the tree; their menu opens upward. */
  .rail { position: relative; flex: none; display: flex; gap: 2px; padding: 6px 8px; border-top: 1px solid var(--line); }
  .rail .end { margin-left: auto; }
  .rail .morebtn { font-size: var(--fs-md, 15px); line-height: 1; min-width: 28px; }
  .rail button { background: none; border: 0; border-radius: var(--r-md); color: var(--muted); padding: 5px 7px; display: grid; place-items: center; }
  .rail button:hover { color: var(--text); background: var(--tint-hover); }
  .rail button.on { color: var(--accent); background: var(--tint-sel); }
  /* The sections are one scrolling list with pinned headings (SidebarSection); so is the Recent tab. */
  .tree { flex: 1; min-height: 0; overflow: auto; }
  .scroll { flex: 1; min-height: 0; overflow: auto; padding: 4px 6px 12px; }
  .selbar {
    display: flex; align-items: center; gap: 6px; padding: 6px 10px; border-top: 1px solid var(--line);
    font-size: var(--fs-sm); background: color-mix(in srgb, var(--accent) 8%, var(--panel));
  }
  .selbar .grow { flex: 1; }
  .selbar button { background: none; border: 1px solid var(--line); border-radius: var(--r-md); color: var(--text); font: inherit; padding: 2px 10px; }
  .selbar button:hover { background: var(--raised-3); }
</style>
