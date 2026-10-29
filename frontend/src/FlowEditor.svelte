<script lang="ts">
  import { confirmDeleteSteps } from './lib/flowDelete'
  import { FlowService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import type { FlowView, Slot, StepChoice, WireChoices } from './lib/types'
  import Canvas, { type DragInfo } from './Canvas.svelte'
  import SidePanel from './SidePanel.svelte'
  import RunMenu from './RunMenu.svelte'
  import CodeView from './CodeView.svelte'
  import FileBanner from './FileBanner.svelte'
  import Chips from './Chips.svelte'
  import SkipChip from './SkipChip.svelte'
  import DebugBar from './DebugBar.svelte'
  import FlowTools from './FlowTools.svelte'
  import Problem from './Problem.svelte'
  import { Clipboard } from '@wailsio/runtime'
  import { ui } from './lib/ui.svelte'
  import { app } from './lib/app.svelte'
  import VarsBar from './VarsBar.svelte'
  import StepPopup from './StepPopup.svelte'
  import IOEditor from './IOEditor.svelte'
  import { withRouteByStatus } from './lib/popup'
  import CaseEditor from './CaseEditor.svelte'
  import RowEditor from './RowEditor.svelte'
  import WirePanel from './WirePanel.svelte'
  import OutputDock from './OutputDock.svelte'
  import { errorText } from './lib/errors'

  let { view, embedded = false }: { view: FlowView; embedded?: boolean } = $props()
  let canvas = $state<Canvas>()
  let code = $state<CodeView>()
  let mode = $state<'canvas' | 'code'>('canvas')
  let slot = $state<Slot | null>(null)

  const node = $derived(view.node)

  // A value wire dropped from one step onto another: the choices of what to take and where to put it, or why not.
  let wire = $state<{ from: string; to: string; x: number; y: number; choices: WireChoices } | null>(null)
  async function onData(from: string, to: string, x: number, y: number) {
    try {
      const choices = (await FlowService.WireChoices(node, from, to)) as unknown as WireChoices
      if (choices.reason) ui.toast(choices.reason)
      else wire = { from, to, x, y, choices }
    } catch (e) {
      ui.toast(errorText(e))
    }
  }

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
    if (e.key === '/' && !mod && !typing && editing && !popup) {
      e.preventDefault()
      openPopup(window.innerWidth / 2 - 180, 120, null)
    } else if (mod && e.key.toLowerCase() === 'z' && !typing) {
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

  // The popup that adds a step: opened by the + on either side of a card (near that card), the Add step button
  // and the / key (at the end, or after the selected step). It shows only in Edit.
  let popup = $state<{ x: number; y: number; near: { step: string; before: boolean } | null } | null>(null)
  function openPopup(x: number, y: number, near: { step: string; before: boolean } | null) {
    popup = { x, y, near }
  }
  function openPopupAtButton(e: MouseEvent) {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    openPopup(r.left, r.bottom, null)
  }
  // A Route's case port label opened for editing: select the Route (the edit applies to the selected step), then show the test.
  let caseAt = $state<{ index: number; x: number; y: number } | null>(null)
  async function openCase(step: string, index: number, e: MouseEvent) {
    const at = { index, x: e.clientX, y: e.clientY }
    await FlowService.Select(node, step)
    caseAt = at
  }

  // A Start or End row opened for editing: the input or output behind it, and where it was clicked.
  let io = $state<{ role: 'start' | 'end'; index: number; x: number; y: number } | null>(null)
  function openIO(role: 'start' | 'end', key: string, e: MouseEvent) {
    if (!editing) return
    const list = role === 'start' ? view.inputs : view.outputs
    const index = list.findIndex((i) => key === (role === 'start' ? 'input:' : 'output:') + i.name)
    if (index >= 0) io = { role, index, x: e.clientX, y: e.clientY }
  }
  async function addInput() {
    try {
      const name = await FlowService.AddInput(node, '')
      const index = view.inputs.findIndex((i) => i.name === name)
      io = { role: 'start', index: index >= 0 ? index : view.inputs.length, x: window.innerWidth / 3, y: 140 }
    } catch (e) {
      ui.toast(errorText(e))
    }
  }
  async function addOutput() {
    try {
      const name = await FlowService.AddOutput(node, '')
      const index = view.outputs.findIndex((i) => i.name === name)
      io = { role: 'end', index: index >= 0 ? index : view.outputs.length, x: window.innerWidth / 2, y: 140 }
    } catch (e) {
      ui.toast(errorText(e))
    }
  }

  // The selected card's toolbar: add opens the popup after the step; the rest act on it at once.
  function onTool(tool: 'add' | 'skip' | 'break' | 'duplicate' | 'delete', step: string, e: MouseEvent) {
    if (tool === 'add') openPopup(e.clientX, e.clientY, { step, before: false })
    else if (tool === 'skip') FlowService.ToggleSkip(node, step)
    else if (tool === 'break') FlowService.ToggleBreak(node, step)
    else if (tool === 'duplicate') FlowService.Duplicate(node, step)
    else confirmDeleteSteps(node, [step])
  }
  const onColour = (step: string, colour: string) => FlowService.SetColor(node, step, colour).catch((e) => ui.toast(errorText(e)))
  // The step a new one would follow, when it gives a status to route by: a request, a URL call or a webhook.
  const routeAfter = $derived.by(() => {
    if (!popup) return ''
    const after = popup.near ? (popup.near.before ? '' : popup.near.step) : view.selected
    const u = view.diagram.units.find((x) => x.nodes.includes(after))
    const type = u?.types[u.nodes.indexOf(after)]
    return type && ['request', 'fetch', 'webhook'].includes(type) ? after : ''
  })
  // The popup's list: the ten kinds, and after Route the shortcut that routes by the status of the step before.
  const popupSteps = $derived(withRouteByStatus(view.steps, routeAfter))
  function addPicked(s: StepChoice) {
    const near = popup?.near
    const after = routeAfter
    popup = null
    if (s.mode === 'status' && after) FlowService.AddRouteByStatus(node, after).catch((e) => ui.toast(errorText(e)))
    else if (near) FlowService.AddStepNear(node, s.type, near.step, near.before)
    else FlowService.AddStep(node, s.type, 0, 0, 0, false)
  }
</script>

{#snippet outputDock(step: string, pinned: boolean, pin: () => void)}
  <OutputDock {view} {step} {pinned} onpin={pin} />
{/snippet}

{#snippet rowPanel(row: string, close: () => void)}
  <RowEditor {view} {row} onclose={close} />
{/snippet}

<svelte:window {onkeydown} />

<div class="editor" bind:this={root}>
  <header>
    <strong>{view.name}</strong>
    {#if !embedded}<Chips {view} /><SkipChip {view} />{/if}
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
      <FlowTools {view} />
    {/if}
    {#if editing}
      <button title="Add a step (/)" onclick={openPopupAtButton}>+ Add step</button>
      <button disabled={!view.canUndo} onclick={() => FlowService.Undo(node)}>Undo</button>
      <button disabled={!view.canRedo} onclick={() => FlowService.Redo(node)}>Redo</button>
    {/if}
    {#if !embedded}
      <button class="edit" class:on={editMode} aria-pressed={editMode} title={editMode ? 'Stop editing: more room for the graph' : 'Edit the flow: add, move and link steps'} onclick={toggleEdit}>{editMode ? 'Done' : 'Edit'}</button>
    {/if}
    {#if !embedded}<button class="primary" disabled={!view.dirty} onclick={() => FlowService.Save(node)}>Save</button>{/if}
    {#if !embedded}<button class="more" aria-label="More" title="More" onpointerdown={(e) => e.stopPropagation()} onclick={moreMenu}>⋯</button>{/if}
  </header>
  <DebugBar {view} />
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
        onadd={editing ? openPopup : undefined}
        ontoolbar={editing ? onTool : undefined}
        oncolour={editing ? onColour : undefined}
        oncase={editing ? openCase : undefined}
        start={view.start}
        end={view.end}
        onedgerow={openIO}
        onmoveinput={editing ? (from, to) => FlowService.MoveInput(node, from, to).catch((e) => ui.toast(errorText(e))) : undefined}
        onaddinput={editing ? addInput : undefined}
        onaddoutput={editing ? addOutput : undefined}
        ondrag={onDrag}
        ondrop={onDrop}
        oncancel={() => (slot = null)}
        initialView={app.canvasViews[node] ?? null}
        onviewchange={(v) => (app.canvasViews[node] = v)}
        panel={editing ? rowPanel : undefined}
        ondata={editing ? onData : undefined}
        dock={outputDock}
      />
    </div>
    <VarsBar vars={view.vars} />
    </div>
    <SidePanel {view} {editing} />
  </div>
</div>

{#if wire}
  <WirePanel {node} from={wire.from} to={wire.to} x={wire.x} y={wire.y} choices={wire.choices} onclose={() => (wire = null)} />
{/if}

{#if caseAt}
  <CaseEditor {view} index={caseAt.index} x={caseAt.x} y={caseAt.y} onclose={() => (caseAt = null)} />
{/if}

{#if io}
  {@const item = (io.role === 'start' ? view.inputs : view.outputs)[io.index]}
  {#if item}<IOEditor {node} role={io.role} index={io.index} {item} x={io.x} y={io.y} onclose={() => (io = null)} />{/if}
{/if}

{#if popup}
  <StepPopup steps={popupSteps} x={popup.x} y={popup.y} onpick={addPicked} onclose={() => (popup = null)} />
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
