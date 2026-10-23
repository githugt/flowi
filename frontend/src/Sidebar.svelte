<script lang="ts">
  import { app } from './lib/app.svelte'
  import { ui } from './lib/ui.svelte'
  import { errorText } from './lib/errors'
  import { WorkspaceService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import TreeItem from './TreeItem.svelte'
  import Icon from './Icon.svelte'
  import { screens } from './lib/screens'
  import type { TreeNode } from './lib/types'

  let { width }: { width: number } = $props()

  // Drafts go last, after the collections and environments: they come and go,
  // and showing them first pushed everything else down the moment one was made.
  const roots = $derived([...(app.tree?.roots ?? [])].sort((a, b) => Number(!!a.draft) - Number(!!b.draft)))

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

  const tools = screens.filter((s) => s.rail)

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
  function wsDo(run: () => void) {
    wsOpen = false
    run()
  }
  const renameWorkspace = () =>
    ui.prompt({ title: 'Rename workspace', label: 'Name', initial: app.tree?.name ?? '', done: (name) => WorkspaceService.RenameWorkspace(name).catch(fail) })
  const revealWorkspace = () => WorkspaceService.RevealWorkspace().catch(fail)
  const openSettings = () => app.openScreen('screen:settings', 'Settings')
</script>

<svelte:window onpointerdown={() => (wsOpen = false)} />

<aside style:width={`${width}px`}>
  <div class="head">
    <button class="ws" aria-haspopup="menu" aria-expanded={wsOpen} title="Workspace" onpointerdown={(e) => e.stopPropagation()} onclick={() => (wsOpen = !wsOpen)}>
      <span class="wsname">{app.tree?.name ?? ''}</span><span class="caret" aria-hidden="true">▾</span>
    </button>
    {#if wsOpen}
      <div class="wsmenu" role="menu" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
        <button role="menuitem" onclick={() => wsDo(renameWorkspace)}>Rename workspace…</button>
        <button role="menuitem" disabled={!app.tree?.folder} title={app.tree?.folder} onclick={() => wsDo(revealWorkspace)}>Open folder</button>
        <hr />
        <button role="menuitem" onclick={() => wsDo(openSettings)}>Settings</button>
      </div>
    {/if}
  </div>
  <nav class="rail" aria-label="Tools">
    {#each tools as t}
      <button title={t.title} aria-label={t.title} class:on={app.active === t.id} onclick={() => app.openScreen(t.id, t.title)}><Icon name={t.icon} /></button>
    {/each}
  </nav>
  <div class="tree" role="tree" tabindex="-1" onkeydown={treeKeys}>
    {#each roots as node (node.id)}
      <TreeItem {node} depth={0} />
    {/each}
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
</aside>

<style>
  aside {
    flex: none;
    display: flex;
    flex-direction: column;
    background: var(--panel);
    border-right: 1px solid var(--line);
    min-height: 0;
  }
  .head { position: relative; height: 44px; display: flex; align-items: center; gap: 8px; padding: 0 12px 0 84px; --wails-draggable: drag; }
  .ws {
    --wails-draggable: no-drag; min-width: 0; display: flex; align-items: center; gap: 4px; padding: 3px 6px; margin-left: -6px;
    background: none; border: 0; border-radius: var(--r-md); color: var(--text); font: inherit; font-weight: 600;
  }
  .ws:hover, .ws[aria-expanded='true'] { background: color-mix(in srgb, var(--accent) 12%, transparent); }
  .wsname { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .caret { color: var(--muted); font-size: var(--fs-xs); }
  .wsmenu {
    --wails-draggable: no-drag; position: absolute; top: calc(100% - 4px); left: 78px; z-index: 40; min-width: 200px;
    display: flex; flex-direction: column; padding: 4px;
    background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); box-shadow: var(--shadow);
  }
  .wsmenu button { background: none; border: 0; border-radius: var(--r-sm); color: var(--text); font: inherit; text-align: left; padding: 5px 10px; }
  .wsmenu button:hover:not(:disabled) { background: color-mix(in srgb, var(--accent) 18%, transparent); }
  .wsmenu button:disabled { opacity: 0.4; }
  .wsmenu hr { width: 100%; border: 0; border-top: 1px solid var(--line); margin: 4px 0; }
  .rail { display: flex; gap: 2px; padding: 0 8px 6px; }
  .rail button { background: none; border: 0; border-radius: var(--r-md); color: var(--muted); padding: 5px 7px; display: grid; place-items: center; }
  .rail button:hover { color: var(--text); background: color-mix(in srgb, var(--accent) 12%, transparent); }
  .rail button.on { color: var(--accent); background: color-mix(in srgb, var(--accent) 18%, transparent); }
  .tree { flex: 1; overflow: auto; padding: 4px 6px 12px; }
  .selbar {
    display: flex; align-items: center; gap: 6px; padding: 6px 10px; border-top: 1px solid var(--line);
    font-size: var(--fs-sm); background: color-mix(in srgb, var(--accent) 8%, var(--panel));
  }
  .selbar .grow { flex: 1; }
  .selbar button { background: none; border: 1px solid var(--line); border-radius: var(--r-md); color: var(--text); font: inherit; padding: 2px 10px; }
  .selbar button:hover { border-color: var(--accent); }
</style>
