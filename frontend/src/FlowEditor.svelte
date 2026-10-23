<script lang="ts">
  import { confirmDeleteSteps } from './lib/flowDelete'
  import { FlowService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import type { FlowView, Slot } from './lib/types'
  import Canvas, { type DragInfo } from './Canvas.svelte'
  import SidePanel from './SidePanel.svelte'
  import RunMenu from './RunMenu.svelte'
  import CodeView from './CodeView.svelte'
  import FileBanner from './FileBanner.svelte'
  import Chips from './Chips.svelte'
  import Problem from './Problem.svelte'
  import { Clipboard } from '@wailsio/runtime'
  import { ui } from './lib/ui.svelte'
  import { app } from './lib/app.svelte'
  import VarsBar from './VarsBar.svelte'
  import Palette from './Palette.svelte'

  let { view, embedded = false }: { view: FlowView; embedded?: boolean } = $props()
  let canvas = $state<Canvas>()
  let code = $state<CodeView>()
  let mode = $state<'canvas' | 'code'>('canvas')
  let slot = $state<Slot | null>(null)
  // A step type dragged out of the palette.
  let fromPalette = $state<{ type: string; x: number; y: number; moved: boolean } | null>(null)

  const node = $derived(view.node)

  // Viewing leaves the whole window to the graph; Edit brings the palette and lets steps be moved and linked.
  // A flow without steps starts in Edit, the hook editors (embedded) are always editing.
  let editMode = $state(false)
  let seen = ''
  $effect(() => {
    if (seen !== view.node) {
      seen = view.node
      editMode = view.diagram.units.length === 0
    }
  })
  const editing = $derived(embedded || editMode)

  // Steps the last run failed at; the chip in the header points at them, and a run that ends in a failure shows the first.
  const failedSteps = $derived(
    Object.entries(view.run.statuses)
      .filter(([, s]) => ['failed', 'error', 'aborted'].includes(s))
      .map(([id]) => id),
  )
  let failIndex = 0
  let dismissedError = $state('')
  function showFailure() {
    if (!failedSteps.length) return
    const id = failedSteps[failIndex++ % failedSteps.length]
    FlowService.Select(node, id)
    mode = 'canvas'
    canvas?.reveal(id)
  }
  let wasRunning = false
  $effect(() => {
    const running = view.run.running
    if (wasRunning && !running && failedSteps.length) {
      failIndex = 0
      showFailure()
    }
    wasRunning = running
  })

  const help = 'Drag a step onto a line or between steps to move it there, or into empty space to place it freely · drag one card out of a strip to reorder it · drag from a port to a step to add needs · Shift-click selects several · Delete removes · double-click a strip to collapse it · pinch to zoom'

  function moreMenu(e: MouseEvent) {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    ui.openMenu(r.left - 160, r.bottom + 4, [
      { label: 'Fit to window', run: () => canvas?.fit() },
      { label: 'Re-layout', run: () => FlowService.Relayout(node) },
      { separator: true, label: '', run: () => {} },
      {
        label: 'Copy as Mermaid',
        run: async () => {
          Clipboard.SetText((await FlowService.Mermaid(node)) as string)
          ui.toast('Copied as a Mermaid flowchart.')
        },
      },
      {
        label: 'Save as blueprint…',
        disabled: view.dirty,
        run: () =>
          ui.prompt({
            title: 'Save as blueprint',
            label: 'Blueprint name',
            initial: view.name,
            done: (name) => FlowService.SaveAsBlueprint(node, name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'blueprint', name),
          }),
      },
      { separator: true, label: '', run: () => {} },
      { label: 'Keys and gestures', run: () => ui.toast(help) },
    ])
  }

  let root = $state<HTMLDivElement>()
  function onkeydown(e: KeyboardEvent) {
    // Every open flow listens on the window: only the one on screen answers.
    if (!root?.offsetParent) return
    const mod = e.metaKey || e.ctrlKey
    const target = e.target as HTMLElement
    const typing = target?.matches?.('input, textarea, select') || target?.isContentEditable
    if (mod && e.key.toLowerCase() === 'z' && !typing) {
      e.preventDefault()
      e.shiftKey ? FlowService.Redo(node) : FlowService.Undo(node)
    } else if (mod && e.key.toLowerCase() === 's' && !embedded) {
      e.preventDefault()
      FlowService.Save(node)
    } else if ((e.key === 'Delete' || e.key === 'Backspace') && view.selection.length && !typing) {
      // It deleted only in Edit, and silently did nothing otherwise; now it asks, or says why it cannot.
      e.preventDefault()
      if (editing) confirmDeleteSteps(node, view.selection)
      else ui.toast('Turn on Edit to delete steps.')
    }
  }

  // One preview in flight at a time; the newest request waits its turn.
  let previewing = false
  let queued: (() => Promise<Slot>) | null = null
  async function preview(ask: () => Promise<Slot>) {
    if (previewing) {
      queued = ask
      return
    }
    previewing = true
    try {
      slot = (await ask()) as Slot
    } finally {
      previewing = false
    }
    if (queued) {
      const next = queued
      queued = null
      preview(next)
    }
  }

  const onDrag = (i: DragInfo) =>
    preview(() => FlowService.Preview(node, i.unit, i.step, i.lift, Math.round(i.x), Math.round(i.y), i.snap) as Promise<Slot>)

  async function onDrop(i: DragInfo) {
    slot = null
    queued = null
    await FlowService.Drop(node, i.unit, i.step, i.lift, Math.round(i.x), Math.round(i.y), i.snap, i.atX, i.atY)
  }

  function startPaletteDrag(type: string, e: PointerEvent) {
    fromPalette = { type, x: e.clientX, y: e.clientY, moved: false }
  }

  function movePaletteDrag(e: PointerEvent) {
    const d = fromPalette
    if (!d) return
    d.x = e.clientX
    d.y = e.clientY
    d.moved ||= Math.hypot(e.movementX, e.movementY) > 0
    const c = canvas!.toCanvas(e.clientX, e.clientY)
    if (!c) {
      slot = null
      return
    }
    preview(() => FlowService.Preview(node, '', '', false, Math.round(c.x), Math.round(c.y), c.snap) as Promise<Slot>)
  }

  async function endPaletteDrag(e: PointerEvent) {
    const d = fromPalette
    fromPalette = null
    slot = null
    queued = null
    // A press without a move is a click: the button's own click adds the step at the end.
    if (!d?.moved) return
    const c = canvas!.toCanvas(e.clientX, e.clientY)
    if (c) await FlowService.AddStep(node, d.type, Math.round(c.x), Math.round(c.y), c.snap, true)
    suppressClick = true
    setTimeout(() => (suppressClick = false))
  }

  // Done in the code view applies what was typed first; with problems, Edit stays on to fix them.
  async function toggleEdit() {
    if (editMode && mode === 'code' && !(await code!.settle())) return
    editMode = !editMode
  }

  async function show(next: 'canvas' | 'code') {
    if (next === mode) return
    // Typed code is applied first; with problems the user stays to fix them.
    if (mode === 'code' && !(await code!.settle())) return
    mode = next
  }

  let suppressClick = false
  const add = (type: string) => !suppressClick && FlowService.AddStep(node, type, 0, 0, 0, false)

  // The canvas's + on either side of a card: every step type by group, picked from a menu where it was
  // clicked, added right before or after that card. It shows only in Edit, with the palette, which offers
  // the same steps for dragging into place: shown while viewing, it offered an edit the view did not allow.
  function addMenu(x: number, y: number, near: { step: string; before: boolean } | null) {
    const items: { label: string; run: () => void; disabled?: boolean; separator?: boolean }[] = []
    let group = ''
    for (const p of view.palette) {
      if (p.group !== group) {
        if (group) items.push({ separator: true, label: '', run: () => {} })
        group = p.group
        items.push({ label: group.toUpperCase(), run: () => {}, disabled: true })
      }
      items.push({ label: p.type, run: () => (near ? FlowService.AddStepNear(node, p.type, near.step, near.before) : add(p.type)) })
    }
    ui.openMenu(x, y + 8, items)
  }
</script>

<svelte:window {onkeydown} onpointermove={movePaletteDrag} onpointerup={endPaletteDrag} />

<div class="editor" bind:this={root}>
  <header>
    <strong>{view.name}</strong>
    {#if !embedded}<Chips {view} />{/if}
    {#if failedSteps.length && !view.run.running}
      <button class="failed" title="Show the step the run failed at" onclick={showFailure}>Failed: {failedSteps[0]}{failedSteps.length > 1 ? ` +${failedSteps.length - 1}` : ''}</button>
    {/if}
    {#if !embedded}
      <div class="views" role="radiogroup" aria-label="View">
        <button class:on={mode === 'canvas'} onclick={() => show('canvas')}>Canvas</button>
        <button class:on={mode === 'code'} onclick={() => show('code')}>Code</button>
      </div>
    {/if}
    <span class="spacer"></span>
    {#if !embedded}
      {#if view.run.running}<span class="status">Running…</span>
      {:else if view.run.summary && !view.run.error}<span class="status">{view.run.summary}</span>{/if}
      <RunMenu {view} />
    {/if}
    {#if editing}
      <button disabled={!view.canUndo} onclick={() => FlowService.Undo(node)}>Undo</button>
      <button disabled={!view.canRedo} onclick={() => FlowService.Redo(node)}>Redo</button>
    {/if}
    {#if !embedded}
      <button class="edit" class:on={editMode} aria-pressed={editMode} title={editMode ? 'Stop editing: more room for the graph' : 'Edit the flow: add, move and link steps'} onclick={toggleEdit}>{editMode ? 'Done' : 'Edit'}</button>
    {/if}
    {#if !embedded}<button class="primary" disabled={!view.dirty} onclick={() => FlowService.Save(node)}>Save</button>{/if}
    {#if !embedded}<button class="more" aria-label="More" title="More" onpointerdown={(e) => e.stopPropagation()} onclick={moreMenu}>⋯</button>{/if}
  </header>
  <!-- A run's error gets its own line: in the header it was cut to a few words. -->
  {#if !embedded && view.run.error && !view.run.running && dismissedError !== view.run.error}
    <div class="runerr" role="alert">
      <span class="msg"><strong>Run failed.</strong> <Problem text={view.run.error ?? ""} /></span>
      {#if failedSteps.length}<button onclick={showFailure}>Show step</button>{/if}
      <button class="x" aria-label="Dismiss" onclick={() => (dismissedError = view.run.error ?? "")}>×</button>
    </div>
  {/if}
  {#if mode === 'code'}
    <div class="work"><div class="center"><CodeView bind:this={code} {view} editable={editing} /></div></div>
  {/if}
  <FileBanner onDisk={view.onDisk} needsDrop={view.needsDrop} revert={() => FlowService.Revert(node)} saveDropping={() => FlowService.SaveDroppingComments(node)} declineDrop={() => FlowService.DeclineDrop(node)} />
  <div class="work" hidden={mode === 'code'}>
    {#if editing}<Palette entries={view.palette} onadd={add} ondragstart={startPaletteDrag} />{/if}
    <div class="center">
    <div class="canvas">
      <Canvas
        bind:this={canvas}
        diagram={view.diagram}
        cards={view.cards}
        data={view.data}
        selection={view.selection}
        statuses={view.run.statuses}
        results={view.run.results}
        {slot}
        editable={editing}
        onselect={(step, additive) => (additive ? FlowService.ToggleSelect(node, step) : FlowService.Select(node, step))}
        onconnect={(from, to) => FlowService.Connect(node, from, to)}
        oncollapse={(unit, on) => FlowService.Collapse(node, unit, on)}
        onaddcase={(id) => FlowService.AddCaseTo(node, id)}
        onadd={editing ? addMenu : undefined}
        ondrag={onDrag}
        ondrop={onDrop}
        oncancel={() => (slot = null)}
        initialView={app.canvasViews[node] ?? null}
        onviewchange={(v) => (app.canvasViews[node] = v)}
      />
    </div>
    <VarsBar vars={view.vars} />
    </div>
    <SidePanel {view} {editing} />
  </div>
</div>

{#if fromPalette && fromPalette.moved}
  <div class="ghost" style:left={`${fromPalette.x + 8}px`} style:top={`${fromPalette.y + 8}px`}>{fromPalette.type}</div>
{/if}

<style>
  .failed { border-radius: var(--r-pill); padding: 1px 10px; font: inherit; font-size: var(--fs-xs); background: transparent; border: 1px solid var(--fail); color: var(--fail); white-space: nowrap; }
  .editor { position: absolute; inset: 0; display: flex; flex-direction: column; }
  header {
    height: 40px;
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 12px;
    border-bottom: 1px solid var(--line);
  }
  .runerr { flex: none; display: flex; align-items: center; gap: var(--sp-2); padding: 6px 12px; font-size: var(--fs-sm); color: var(--fail); background: color-mix(in srgb, var(--fail) 10%, transparent); border-bottom: 1px solid var(--line); }
  .runerr .msg { flex: 1; min-width: 0; }
  .runerr button { padding: 1px 10px; }
  .runerr .x { border: 0; color: var(--muted); padding: 0 4px; }
  header strong { flex: 0 1 auto; min-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  header :global(button) { white-space: nowrap; }
  .work { flex: 1; min-height: 0; display: flex; }
  .views { display: flex; margin-left: 12px; }
  .views button { border-radius: 0; }
  .views button:first-child { border-radius: 6px 0 0 6px; }
  .views button:last-child { border-radius: 0 6px 6px 0; border-left: 0; }
  .views .on { background: color-mix(in srgb, var(--accent) 25%, transparent); border-color: var(--accent); }
  .edit.on { background: color-mix(in srgb, var(--accent) 25%, transparent); border-color: var(--accent); }
  .work[hidden] { display: none; }
  .spacer { flex: 1; }
  .center { flex: 1; min-width: 0; display: flex; flex-direction: column; position: relative; }
  .canvas { flex: 1; min-height: 0; position: relative; }
  .status { color: var(--muted); font-size: var(--fs-sm); max-width: 360px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .ghost {
    position: fixed;
    pointer-events: none;
    background: var(--accent);
    color: var(--on-accent);
    padding: 4px 10px;
    border-radius: var(--r-md);
    font-weight: 600;
    z-index: 10;
  }
  button {
    background: transparent;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    padding: 3px 12px;
    font: inherit;
  }
  button:hover:not(:disabled) { border-color: var(--accent); }
  button:disabled { opacity: 0.4; }
  .more { padding: 3px 9px; }
  /* the same strong blue as a request's Send, not the paler accent */
  .primary:not(:disabled) { background: var(--act-send); color: var(--on-act); border-color: var(--act-send); font-weight: 600; }
  .primary:hover:not(:disabled) { border-color: var(--act-send); background: color-mix(in srgb, var(--act-send) 88%, white); }
</style>
