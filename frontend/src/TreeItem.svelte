<script lang="ts">
  import { app } from './lib/app.svelte'
  import type { TreeNode } from './lib/types'
  import TreeItem from './TreeItem.svelte'
  import { rowMenu } from './lib/menus'
  import { ui } from './lib/ui.svelte'
  import Verb from './Verb.svelte'
  import { WorkspaceService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { moved } from './lib/reorder'
  import { moveRequests, movingWith } from './lib/move'

  let { node, depth, parent }: { node: TreeNode; depth: number; parent?: TreeNode } = $props()
  let open = $state(true)
  const branch = $derived(!['request', 'environment', 'flow', 'missing'].includes(node.kind))
  const active = $derived(app.active === node.id)

  const markable = $derived(['request', 'flow', 'environment'].includes(node.kind))
  const marked = $derived(app.marked.includes(node.id))

  // Requests, flows and environments are marked for actions on several at once. Cmd/Ctrl-click marks or
  // unmarks a row, and Shift-click marks the rows from the last one clicked. The first Cmd-click used to
  // mark only itself, leaving out the row that was open, so two clicks seemed to select one row.
  const markableRows = () => [...document.querySelectorAll<HTMLElement>('[data-tree-row][data-markable]')].map((r) => r.dataset.treeRow!)

  function rangeTo(id: string): string[] {
    const rows = markableRows()
    const from = rows.indexOf(app.markAnchor || app.active)
    const to = rows.indexOf(id)
    if (from < 0 || to < 0) return [id]
    return rows.slice(Math.min(from, to), Math.max(from, to) + 1)
  }

  function click(e: MouseEvent) {
    if (justDragged) {
      justDragged = false
      return
    }
    if (e.shiftKey && markable) {
      app.marked = rangeTo(node.id)
      return
    }
    if ((e.metaKey || e.ctrlKey) && markable) {
      const withOpen = !app.marked.length && app.active && app.active !== node.id && markableRows().includes(app.active)
      app.marked = marked ? app.marked.filter((m) => m !== node.id) : withOpen ? [app.active, node.id] : [...app.marked, node.id]
      app.markAnchor = node.id
      return
    }
    app.marked = []
    app.markAnchor = node.id
    if (branch) open = !open
    else app.open(node)
  }

  // Dragging a row among its siblings changes the order they are shown in, kept in the folder's _folder.yaml.
  // Dragging requests onto a collection or a request folder, in any collection, moves them there (with the
  // other marked rows when the dragged one is marked): before, a drag could only reorder.
  // A press only becomes a drag after a few pixels; the click that ends a drag is not a click.
  const reorderable = $derived(!!parent && !!node.name && ['request', 'flow', 'folder'].includes(node.kind) && ['requests', 'flows', 'folder'].includes(parent.kind))
  const movable = $derived(node.kind === 'request')
  // Rows requests can be dropped into.
  const container = $derived(node.kind === 'collection' || (node.kind === 'requests' || node.kind === 'folder') && node.tree === 'requests')
  let drag = $state<{ startY: number; active: boolean; over: string; after: boolean; into: string } | null>(null)
  let justDragged = false

  function down(e: PointerEvent) {
    if (!(reorderable || movable) || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey) return
    drag = { startY: e.clientY, active: false, over: '', after: false, into: '' }
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  }

  function move(e: PointerEvent) {
    if (!drag) return
    if (!drag.active && Math.abs(e.clientY - drag.startY) < 5) return
    drag.active = true
    drag.over = drag.into = ''
    app.treeDrop = null
    const row = document.elementFromPoint(e.clientX, e.clientY)?.closest<HTMLElement>('[data-tree-row]')
    if (!row || row.dataset.treeRow === node.id) return
    const sibling = reorderable && parent ? (parent.children ?? []).find((c) => c.id === row.dataset.treeRow) : undefined
    if (sibling) {
      const r = row.getBoundingClientRect()
      drag.over = sibling.id
      drag.after = e.clientY > r.top + r.height / 2
      app.treeDrop = { id: sibling.id, after: drag.after }
    } else if (movable && row.dataset.drop !== undefined) {
      drag.into = row.dataset.treeRow!
      app.treeDrop = { id: drag.into, after: false, into: true }
    }
  }

  function up() {
    const d = drag
    drag = null
    app.treeDrop = null
    if (!d?.active) return
    justDragged = true
    if (d.into) return void moveRequests(movingWith(node.id), d.into)
    if (!d.over || !parent) return
    const names = (parent.children ?? []).map((c) => c.name ?? '').filter(Boolean)
    const from = names.indexOf(node.name!)
    const target = names.indexOf((parent.children ?? []).find((c) => c.id === d.over)?.name ?? '')
    if (from < 0 || target < 0) return
    // Dropped after the row under it: its place, unless the dragged row was above it (the list shifts up).
    const to = d.after ? (from < target ? target : target + 1) : from < target ? target - 1 : target
    const next = moved(names, from, Math.min(Math.max(to, 0), names.length - 1))
    if (next.join('/') !== names.join('/')) WorkspaceService.Reorder(parent.id, next, false)
  }

  async function contextmenu(e: MouseEvent) {
    e.preventDefault()
    ui.openMenu(e.clientX, e.clientY, await rowMenu(node.id))
  }
</script>

<div role="treeitem" aria-selected={active} aria-expanded={branch ? open : undefined}>
  <button class="row" class:active class:marked class:group={branch} class:dragging={drag?.active}
    class:dropBefore={app.treeDrop?.id === node.id && !app.treeDrop.into && !app.treeDrop.after}
    class:dropAfter={app.treeDrop?.id === node.id && !app.treeDrop.into && app.treeDrop.after}
    class:dropInto={app.treeDrop?.id === node.id && app.treeDrop.into}
    data-tree-row={node.id} data-markable={markable ? '' : undefined} data-drop={container ? '' : undefined} style:padding-left={`${8 + depth * 14}px`}
    onclick={click} oncontextmenu={contextmenu} onpointerdown={down} onpointermove={move} onpointerup={up} onpointercancel={up}>
    <span class="twist" class:open aria-hidden="true">{#if branch}<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M9 6l6 6-6 6" /></svg>{/if}</span>
    {#if node.kind === 'request' && node.method}<span class="method"><Verb verb={node.method} short /></span>{:else if node.kind === 'flow'}<span class="method"><Verb verb="FLOW" /></span>{/if}
    <span class="label">{node.label}</span>
  </button>
  {#if branch && open}
    {#each node.children ?? [] as child (child.id)}
      <TreeItem node={child} depth={depth + 1} parent={node} />
    {/each}
  {/if}
</div>

<style>
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    text-align: left;
    background: none;
    border: 0;
    border-radius: var(--r-md);
    color: var(--text);
    font: inherit;
    padding-block: 3px;
    cursor: default;
  }
  .row.dragging { opacity: 0.5; }
  .row.dropBefore { box-shadow: inset 0 2px 0 var(--accent); }
  .row.dropAfter { box-shadow: inset 0 -2px 0 var(--accent); }
  .row.dropInto { box-shadow: inset 0 0 0 2px var(--accent); background: color-mix(in srgb, var(--accent) 16%, transparent); }
  .row:hover { background: var(--tint-hover); }
  .row.marked { background: color-mix(in srgb, var(--accent) 32%, transparent); }
  .row.active { background: var(--tint-sel); box-shadow: inset 2px 0 0 var(--accent); }
  .group { color: var(--muted); font-weight: 600; font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: 0.05em; }
  .twist { flex: none; width: 12px; height: 12px; color: var(--muted); display: grid; place-items: center; transition: transform 0.12s; }
  .twist.open { transform: rotate(90deg); }
  .label { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .method { flex: none; min-width: 44px; display: inline-flex; }
</style>
