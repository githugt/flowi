<script lang="ts">
  import { tick } from 'svelte'
  import { app } from './lib/app.svelte'
  import { ui } from './lib/ui.svelte'
  import { errorText } from './lib/errors'
  import { WorkspaceService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import TreeItem from './TreeItem.svelte'
  import SearchField from './SearchField.svelte'
  import Menu, { type MenuItem } from './Menu.svelte'
  import Icon from './Icon.svelte'
  import { screens } from './lib/screens'
  import type { TreeNode } from './lib/types'
  import { rootsForGroup, type RailGroup } from './lib/railgroups'
  import { filterTree, firstLeaf } from './lib/treefilter'

  // peek: shown floating from the rail, with a Pin open button; otherwise it has the fold button.
  // group: when shown from a rail icon, the part of the tree that icon stands for.
  let { width, peek = false, group = '', onpin, onfold }: { width: number; peek?: boolean; group?: RailGroup | ''; onpin?: () => void; onfold?: () => void } = $props()

  // Drafts go last, after the collections and environments: they come and go,
  // and showing them first pushed everything else down the moment one was made.
  const ordered = $derived([...rootsForGroup(app.tree?.roots ?? [], group)].sort((a, b) => Number(!!a.draft) - Number(!!b.draft)))

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
  const roots = $derived(filterTree(ordered, query))
  const noMatches = $derived(!!query.trim() && roots.length === 0)

  // Enter opens the first hit, the down arrow moves into the list, Escape empties the field.
  function jumpKeys(e: KeyboardEvent) {
    if (e.key === 'Escape' && query) query = ''
    else if (e.key === 'Enter') {
      const hit = firstLeaf(roots)
      if (hit) app.open(hit)
    } else if (e.key === 'ArrowDown') (e.currentTarget as HTMLElement).closest('aside')?.querySelector<HTMLButtonElement>('.tree .row')?.focus()
    else return
    e.preventDefault()
  }

  // Arrow keys walk the visible rows; right and left open and close a group.
  function treeKeys(e: KeyboardEvent) {
    const rows = [...(e.currentTarget as HTMLElement).querySelectorAll<HTMLButtonElement>('.row')]
    const i = rows.indexOf(document.activeElement as HTMLButtonElement)
    if (i < 0) return
    const open = rows[i].parentElement?.getAttribute('aria-expanded')
    if (e.key === 'ArrowDown') rows[Math.min(i + 1, rows.length - 1)].focus()
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
  const moreItems = $derived<MenuItem[]>(more.map((t) => ({ label: t.title, icon: t.icon, active: app.active === t.id, onselect: () => app.openScreen(t.id, t.title) })))
</script>

<svelte:window onpointerdown={() => ((wsOpen = false), (moreOpen = false))} />

<aside style:width={`${width}px`}>
  <div class="head" class:peeking={peek}>
    <button class="ws" aria-haspopup="menu" aria-expanded={wsOpen} title="Workspace" onpointerdown={(e) => e.stopPropagation()} onclick={() => (wsOpen = !wsOpen)}>
      <span class="avatar" aria-hidden="true">{(app.tree?.name ?? '?').slice(0, 1).toUpperCase()}</span><span class="wsname">{app.tree?.name ?? ''}</span><span class="caret" aria-hidden="true">▾</span>
    </button>
    {#if wsOpen}
      <Menu class="wsmenu" label="Workspace" items={wsItems} onclose={() => (wsOpen = false)} />
    {/if}
    {#if peek}
      <button class="pin" title="Keep the sidebar open for this kind of tab" onclick={(e) => { e.stopPropagation(); onpin?.() }}>Pin open</button>
    {:else if onfold}
      <button class="pin" title="Fold the sidebar to a rail" aria-label="Fold the sidebar" onclick={onfold}>«</button>
    {/if}
  </div>
  <div class="jump"><SearchField bind:value={query} bind:el={jumpField} placeholder="Jump to…" label="Jump to a request, flow or environment" onkeydown={jumpKeys} /></div>
  <div class="tree" role="tree" tabindex="-1" onkeydown={treeKeys}>
    {#each roots as node, i (node.id)}
      <TreeItem {node} depth={0} tint={app.rainbow.length ? app.rainbow[i % app.rainbow.length] : ''} {query} />
    {/each}
    {#if noMatches}<p class="none">Nothing matches “{query.trim()}”.</p>{/if}
  </div>
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
  <nav class="rail" aria-label="Tools">
    {#each tools as t}
      <button title={t.title} aria-label={t.title} class:on={app.active === t.id} onclick={() => app.openScreen(t.id, t.title)}><Icon name={t.icon} /></button>
    {/each}
    <button class="morebtn" class:on={moreActive || moreOpen} title="More tools" aria-label="More tools" aria-haspopup="menu" aria-expanded={moreOpen} onpointerdown={(e) => e.stopPropagation()} onclick={() => (moreOpen = !moreOpen)}>⋯</button>
    {#if settings}
      <button class="end" title={settings.title} aria-label={settings.title} class:on={app.active === settings.id} onclick={() => app.openScreen(settings.id, settings.title)}><Icon name={settings.icon} /></button>
    {/if}
    {#if moreOpen}
      <Menu class="moremenu" label="More tools" items={moreItems} onclose={() => (moreOpen = false)} />
    {/if}
  </nav>
</aside>

<style>
  aside {
    flex: none;
    display: flex;
    flex-direction: column;
    background: var(--chrome);
    min-height: 0;
  }
  .head { position: relative; height: 44px; display: flex; align-items: center; gap: 8px; padding: 0 12px 0 84px; --wails-draggable: drag; }
  .head.peeking { padding-left: 52px; }
  .ws {
    --wails-draggable: no-drag; min-width: 0; display: flex; align-items: center; gap: 4px; padding: 3px 6px; margin-left: -6px;
    background: none; border: 0; border-radius: var(--r-md); color: var(--text); font: inherit; font-weight: 600;
  }
  .ws:hover, .ws[aria-expanded='true'] { background: var(--tint-hover); }
  .avatar { flex: none; width: 22px; height: 22px; border-radius: 7px; display: grid; place-items: center; background: linear-gradient(135deg, var(--accent), var(--data)); color: var(--on-accent); font-size: var(--fs-xs); font-weight: 700; }
  .wsname { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .caret { color: var(--muted); font-size: var(--fs-xs); }
  /* Menu.svelte draws the menu; these say where it opens from. */
  aside :global(.wsmenu) { top: calc(100% - 4px); left: 78px; min-width: 200px; }
  .pin { --wails-draggable: no-drag; margin-left: auto; white-space: nowrap; flex: none; background: none; border: 1px solid var(--line); border-radius: var(--r-md); color: var(--muted); font: inherit; font-size: var(--fs-sm); padding: 1px 8px; }
  .pin:hover { color: var(--text); border-color: var(--accent); }
  .jump { margin: 0 8px 6px; }
  .none { margin: 12px 8px; color: var(--muted); font-size: var(--fs-sm); }
  /* The tools sit in a strip at the foot, out of the way of the tree; their menu opens upward. */
  .rail { position: relative; flex: none; display: flex; gap: 2px; padding: 6px 8px; border-top: 1px solid var(--line); }
  .rail .end { margin-left: auto; }
  .rail .morebtn { font-size: var(--fs-md, 15px); line-height: 1; min-width: 28px; }
  aside :global(.moremenu) { bottom: calc(100% - 2px); right: 8px; }
  .rail button { background: none; border: 0; border-radius: var(--r-md); color: var(--muted); padding: 5px 7px; display: grid; place-items: center; }
  .rail button:hover { color: var(--text); background: var(--tint-hover); }
  .rail button.on { color: var(--accent); background: var(--tint-sel); }
  .tree { flex: 1; overflow: auto; padding: 4px 6px 12px; }
  .selbar {
    display: flex; align-items: center; gap: 6px; padding: 6px 10px; border-top: 1px solid var(--line);
    font-size: var(--fs-sm); background: color-mix(in srgb, var(--accent) 8%, var(--panel));
  }
  .selbar .grow { flex: 1; }
  .selbar button { background: none; border: 1px solid var(--line); border-radius: var(--r-md); color: var(--text); font: inherit; padding: 2px 10px; }
  .selbar button:hover { background: var(--raised-3); }
</style>
