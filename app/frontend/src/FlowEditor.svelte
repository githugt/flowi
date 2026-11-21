<script lang="ts">
  import Banner from './Banner.svelte'
  import { modalOpen } from './lib/modal'
  import IconButton from './IconButton.svelte'
  import { withShortcut } from './lib/shortcuttip'
  import { isMac } from './lib/commands'
  import Segmented from './Segmented.svelte'
  import { relayoutLabel, relayoutNote } from './lib/relayout'
  import { isLetter } from './lib/keymatch'
  import { openStepTarget } from './lib/opentarget'
  import Button from './Button.svelte'
  import Toggle from './Toggle.svelte'
  import { tick } from 'svelte'
  import { confirmDeleteSteps } from './lib/flowDelete'
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { DiskDiff } from './lib/diskdiff'
  import type { FlowView, Slot, StepChoice, WireChoices } from './lib/types'
  import Canvas, { type DragInfo } from './Canvas.svelte'
  import { copy, paste } from './lib/clip'
  import { clipAction } from './lib/flowkeys'
  import SidePanel from './SidePanel.svelte'
  import RunMenu from './RunMenu.svelte'
  import RunTimeline from './RunTimeline.svelte'
  import StepsPanel from './StepsPanel.svelte'
  import WrapBar from './WrapBar.svelte'
  import { showWrapBar } from './lib/wrapbar'
  import CodeView from './CodeView.svelte'
  import FileBanner from './FileBanner.svelte'
  import Chips from './Chips.svelte'
  import SkipChip from './SkipChip.svelte'
  import DebugBar from './DebugBar.svelte'
  import FlowTools from './FlowTools.svelte'
  import Problem from './Problem.svelte'
  import { ui, type MenuItem } from './lib/ui.svelte'
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
  // The run's waterfall: offered when a finished run has at least two steps to compare.
  let timelineOpen = $state(false)
  // The steps as a list: for a long flow whose canvas is thousands of pixels wide.
  let stepsOpen = $state(false)
  const canShowTimeline = $derived(!view.run.running && (view.run.timeline?.length ?? 0) > 1)
  function showStep(id: string) {
    FlowService.Select(node, id)
    mode = 'canvas'
    canvas?.reveal(id)
  }
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

  const help = 'Drag a step onto a line or between steps to move it there, or into empty space to place it freely · drag one card out of a strip to reorder it · drag from a port to a step to add needs · Shift-click or Ctrl+A selects several · Ctrl+C and Ctrl+V copy and paste steps · Delete removes · pinch to zoom'

  function moreMenu(e: MouseEvent) {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    ui.openMenu(r.left - 160, r.bottom + 4, [
      { label: 'Fit to window', run: () => canvas?.fit() },
      {
        label: relayoutLabel(view.selection.length),
        run: () => {
          ui.toast(relayoutNote(view.selection.length, view.diagram.units.reduce((n, u) => n + u.nodes.length, 0)))
          return FlowService.Relayout(node)
        },
      },
      { separator: true, label: '', run: () => {} },
      {
        label: 'Copy as Mermaid',
        run: async () => {
          void copy((await FlowService.Mermaid(node)) as string, 'as a Mermaid flowchart')
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

  // Select all, copy and paste of steps: the text on the clipboard is the code view's form, so it also goes to a chat.
  async function clipKey(action: 'selectAll' | 'copy' | 'paste') {
    try {
      if (action === 'selectAll') await FlowService.SelectAll(node)
      else if (action === 'copy') void copy((await FlowService.CopySteps(node, view.selection)) as string, view.selection.length === 1 ? 'the step' : 'the steps')
      else {
        const text = await paste()
        if (text === null) return ui.toast('Could not read the clipboard here.')
        const ids = (await FlowService.PasteSteps(node, text)) as string[]
        ui.toast(ids.length === 1 ? 'Pasted 1 step' : `Pasted ${ids.length} steps`)
      }
    } catch (e) {
      ui.toast(errorText(e))
    }
  }

  let root = $state<HTMLDivElement>()
  function onkeydown(e: KeyboardEvent) {
    // Every open flow listens on the window: only the one on screen answers.
    if (!root?.offsetParent) return
    // A dialog is open: the keys belong to it. Before, Delete here still removed the selected step behind the dialog.
    if (modalOpen()) return
    const mod = e.metaKey || e.ctrlKey
    const target = e.target as HTMLElement
    const typing = target?.matches?.('input, textarea, select') || target?.isContentEditable
    const clip = mod ? clipAction(e, { typing: !!typing, editing, selected: view.selection.length, textSelected: !!window.getSelection()?.toString() }) : null
    if (e.key === '/' && !mod && !typing && editing && !popup) {
      e.preventDefault()
      openPopup(window.innerWidth / 2 - 180, 120, null)
    } else if (mod && isLetter(e, 'z') && !typing) {
      e.preventDefault()
      e.shiftKey ? FlowService.Redo(node) : FlowService.Undo(node)
    } else if (clip) {
      e.preventDefault()
      void clipKey(clip)
    } else if (mod && isLetter(e, 's') && !embedded) {
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
  let popup = $state<{ x: number; y: number; near: { step: string; before: boolean } | null; on: { step: string; outcome: string } | null; into?: { step: string; lane: number } } | null>(null)
  function openPopup(x: number, y: number, near: { step: string; before: boolean } | null) {
    popup = { x, y, near, on: null }
  }
  // The + beside an outcome port: the step picked is added to run on that outcome of the step.
  function openPopupOn(step: string, outcome: string, x: number, y: number) {
    popup = { x, y, near: null, on: { step, outcome } }
  }
  // The Add a step button of a lane in the side panel: the step picked goes at the end of that lane of the selected step.
  function openPopupInto(lane: number, e: MouseEvent) {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    popup = { x: r.left, y: r.bottom, near: null, on: null, into: { step: view.selected, lane } }
  }
  // The + Add a step in an empty lane on the canvas: the step picked goes into that lane of its container.
  function openPopupInLane(container: string, lane: number, e: MouseEvent) {
    popup = { x: e.clientX, y: e.clientY, near: null, on: null, into: { step: container, lane } }
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
  // Moving a step or opening a row with Edit off did nothing and said nothing; say so, at most once in a while.
  let lockedAt = 0
  function tellLocked() {
    if (Date.now() - lockedAt < 4000) return
    lockedAt = Date.now()
    ui.toast('Turn on Edit to change the flow.')
  }
  function openIO(role: 'start' | 'end', key: string, e: MouseEvent) {
    if (!editing) return tellLocked()
    const list = role === 'start' ? view.inputs : view.outputs
    const index = list.findIndex((i) => key === (role === 'start' ? 'input:' : 'output:') + i.name)
    if (index >= 0) io = { role, index, x: e.clientX, y: e.clientY }
  }
  // Where the editor of a new Start or End row opens: at the row itself, as when it is clicked, not in the middle
  // of the window. The row is drawn once the view has come back from Go, so wait for it a moment.
  async function whereIsRow(role: 'start' | 'end', name: string): Promise<{ index: number; x: number; y: number }> {
    const list = () => (role === 'start' ? view.inputs : view.outputs)
    for (let tries = 0; tries < 20 && !list().some((i) => i.name === name); tries++) await new Promise((r) => setTimeout(r, 25))
    await tick()
    const index = list().findIndex((i) => i.name === name)
    const row = document.querySelectorAll(`.edgecard.${role} .row:not(.add)`)[Math.max(index, 0)]
    const r = row?.getBoundingClientRect()
    return { index: index >= 0 ? index : list().length, x: r ? r.left + r.width / 2 : window.innerWidth / 3, y: r ? r.bottom : 140 }
  }
  async function addInput() {
    try {
      const name = await FlowService.AddInput(node, '')
      io = { role: 'start', ...(await whereIsRow('start', name)) }
    } catch (e) {
      ui.toast(errorText(e))
    }
  }
  async function addOutput() {
    try {
      const name = await FlowService.AddOutput(node, '')
      io = { role: 'end', ...(await whereIsRow('end', name)) }
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
  // Right-click on a card: what the toolbar and the side panel offer for it, in one place. Skipping and opening the request
  // work on a flow that is not being edited, since neither changes the flow.
  function stepMenu(step: string, e: MouseEvent) {
    const u = view.diagram.units.find((x) => x.nodes.includes(step))
    const type = u?.types[u.nodes.indexOf(step)]
    const calls = type === 'request' || type === 'subflow'
    const skipped = view.skips.steps.includes(step)
    const items: MenuItem[] = []
    if (calls) items.push({ label: type === 'request' ? 'Open request' : 'Open flow', run: () => void openStepTarget(node, step) })
    items.push({ label: 'Skip in the next runs', checked: skipped, run: () => void FlowService.ToggleSkip(node, step) })
    if (editing) {
      items.push({ label: 'Pause before this step', checked: view.debug.breaks.includes(step), run: () => void FlowService.ToggleBreak(node, step) })
      items.push({ label: '', separator: true, run: () => {} })
      items.push({ label: 'Add a step after', run: () => openPopup(e.clientX, e.clientY, { step, before: false }) })
      items.push({ label: 'Duplicate', run: () => void FlowService.Duplicate(node, step) })
      items.push({ label: 'Delete…', run: () => confirmDeleteSteps(node, view.selection.includes(step) && view.selection.length > 1 ? view.selection : [step]) })
    }
    ui.openMenu(e.clientX, e.clientY, items)
  }
  const onColour = (step: string, colour: string) => FlowService.SetColor(node, step, colour).catch(ui.fail)
  // The step a new one would follow, when it gives a status to route by: a request, a URL call or a webhook.
  const routeAfter = $derived.by(() => {
    if (!popup) return ''
    const after = popup.near ? (popup.near.before ? '' : popup.near.step) : view.selected
    const u = view.diagram.units.find((x) => x.nodes.includes(after))
    const type = u?.types[u.nodes.indexOf(after)]
    return type && ['request', 'fetch', 'webhook'].includes(type) ? after : ''
  })
  // The popup's list: the kinds, and after Route the shortcut that routes by the status of the step before.
  const popupSteps = $derived(withRouteByStatus(view.steps, routeAfter))
  function addPicked(s: StepChoice) {
    const near = popup?.near
    const on = popup?.on
    const into = popup?.into
    const after = routeAfter
    popup = null
    if (into) FlowService.AddStepInto(node, s.type, into.step, into.lane).catch(ui.fail)
    else if (on) FlowService.AddStepOnOutcome(node, s.type, on.step, on.outcome).catch(ui.fail)
    else if (s.mode === 'status' && after) FlowService.AddRouteByStatus(node, after).catch(ui.fail)
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
      <span class="views"><Segmented label="View" items={[{ id: 'canvas', label: 'Canvas' }, { id: 'code', label: 'Code' }]} value={mode} onpick={(id) => show(id as 'canvas' | 'code')} /></span>
    {/if}
    <span class="spacer"></span>
    {#if !embedded}
      <span class="live" role="status" aria-live="polite">{#if view.run.running}<span class="status">Running…</span>
      {:else if view.run.summary && !view.run.error}<span class="status">{view.run.summary}</span>{/if}</span>
      <Button small pressed={stepsOpen} title="List the steps: status, result and duration, click one to show it on the canvas" onclick={() => (stepsOpen = !stepsOpen)}>Steps</Button>
      {#if canShowTimeline}<Button small pressed={timelineOpen} title="Show the run as a timeline: when each step started and how long it took" onclick={() => (timelineOpen = !timelineOpen)}>Timeline</Button>{/if}
      <RunMenu {view} />
      <FlowTools {view} />
    {/if}
    {#if editing}
      <Button title="Add a step (/)" onclick={openPopupAtButton}>+ Add step</Button>
      <IconButton icon="undo" size={16} disabled={!view.canUndo} label={withShortcut(view.undoLabel, 'Ctrl+Z', isMac())} onclick={() => FlowService.Undo(node)} />
      <IconButton icon="redo" size={16} disabled={!view.canRedo} label={withShortcut(view.redoLabel, 'Ctrl+Shift+Z', isMac())} onclick={() => FlowService.Redo(node)} />
    {/if}
    {#if !embedded}
      <Toggle framed checked={editMode} onchange={toggleEdit} title={editMode ? 'Editing: switch off for more room for the graph' : 'Switch on to add, move and link steps'}>Edit</Toggle>
    {/if}
    {#if !embedded}<Button variant={view.dirty ? 'primary' : 'secondary'} disabled={!view.dirty} onclick={() => FlowService.Save(node)}>Save</Button>{/if}
    {#if !embedded}<IconButton icon="more" label="More" onpointerdown={(e) => e.stopPropagation()} onclick={moreMenu} />{/if}
  </header>
  <DebugBar {view} />
  {#if !embedded && stepsOpen}<StepsPanel {view} onselect={showStep} />{/if}
  {#if !embedded && timelineOpen && canShowTimeline}<RunTimeline bars={view.run.timeline} onselect={showStep} />{/if}
  <!-- A run's error gets its own line: in the header it was cut to a few words. -->
  {#if !embedded && view.run.error && !view.run.running && dismissedError !== view.run.error}
    <Banner tone="fail">
      <strong>Run failed.</strong> <Problem text={view.run.error ?? ""} />
      {#snippet actions()}
        {#if failedSteps.length}<Button small onclick={showFailure}>Show step</Button>{/if}
        <IconButton icon="close" size={12} label="Dismiss" onclick={() => (dismissedError = view.run.error ?? "")} />
      {/snippet}
    </Banner>
  {/if}
  {#if mode === 'code'}
    <div class="work"><div class="center"><CodeView bind:this={code} {view} editable={editing} /></div></div>
  {/if}
  <FileBanner dirty={view.dirty} onDisk={view.onDisk} needsDrop={view.needsDrop} revert={() => FlowService.Revert(node)} compare={() => FlowService.CompareDisk(node) as Promise<DiskDiff>} keep={() => FlowService.KeepMine(node)} saveDropping={() => FlowService.SaveDroppingComments(node)} declineDrop={() => FlowService.DeclineDrop(node)} />
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
        onconnect={(from, to) => FlowService.Connect(node, from, to).catch(ui.fail)}
        wireTargets={editing ? (from, kind) => FlowService.WireTargets(node, from, kind) : undefined}
        onfail={(step) => FlowService.RouteFailure(node, step, '').catch(ui.fail)}
        onoutcome={(from, outcome, to) => FlowService.ConnectOutcome(node, from, outcome, to).catch(ui.fail)}
        onaddon={openPopupOn}
        onarrow={(from, to, outcomes) => FlowService.SetArrow(node, from, to, outcomes).catch(ui.fail)}
        onlocked={tellLocked}
        oncollapse={(unit, on) => FlowService.Collapse(node, unit, on)}
        onopen={(step) => openStepTarget(node, step)}
        onaddcase={(id) => FlowService.AddCaseTo(node, id)}
        onaddinlane={openPopupInLane}
        oncontext={stepMenu}
        onadd={editing ? openPopup : undefined}
        ontoolbar={editing ? onTool : undefined}
        oncolour={editing ? onColour : undefined}
        oncase={editing ? openCase : undefined}
        start={view.start}
        end={view.end}
        onedgerow={openIO}
        onmoveinput={editing ? (from, to) => FlowService.MoveInput(node, from, to).catch(ui.fail) : undefined}
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
      {#if showWrapBar(editing, view.selection.length)}<WrapBar {node} count={view.selection.length} />{/if}
      {#if editing && view.diagram.units.length === 0}
        <div class="blank" role="note">
          <strong>This flow has no steps yet.</strong>
          <p>Start with one of these:</p>
          <div class="row">
            <Button variant="primary" onclick={openPopupAtButton}>Add a step</Button>
            <Button onclick={() => app.openScreen('screen:blueprints', 'Blueprints')}>From a blueprint</Button>
            <Button onclick={() => app.openScreen('screen:import', 'Import')}>Import</Button>
          </div>
          <p class="muted">Drag from a card's right-hand port to another card to run them in order.</p>
        </div>
      {/if}
    </div>
    <VarsBar vars={view.vars} onselect={showStep} />
    </div>
    <SidePanel {view} {editing} onaddinside={editing ? openPopupInto : undefined} />
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
  .live { display: contents; }
  .failed { border-radius: var(--r-pill); padding: 1px 10px; font: inherit; font-size: var(--fs-xs); background: transparent; border: 1px solid var(--fail); color: var(--fail); white-space: nowrap; }
  .editor { position: absolute; inset: 0; display: flex; flex-direction: column; }
  header {
    height: var(--head-h);
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 12px;
    border-bottom: 1px solid var(--line);
  }
  header strong { flex: 0 1 auto; min-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  header :global(button) { white-space: nowrap; }
  .work { flex: 1; min-height: 0; display: flex; }
  .views { display: flex; margin-left: 12px; }
  .work[hidden] { display: none; }
  .spacer { flex: 1; }
  .center { flex: 1; min-width: 0; display: flex; flex-direction: column; position: relative; }
  .canvas { flex: 1; min-height: 0; position: relative; }
  .blank { position: absolute; left: 50%; top: 50%; transform: translate(-50%, -50%); max-width: 420px; text-align: center; padding: var(--sp-4) var(--sp-6); background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); display: flex; flex-direction: column; gap: var(--sp-2); }
  .blank p { margin: 0; color: var(--muted); font-size: var(--fs-sm); }
  .blank .row { display: flex; gap: var(--sp-2); justify-content: center; flex-wrap: wrap; }
  .status { color: var(--muted); font-size: var(--fs-sm); max-width: 360px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  button {
    background: transparent;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    padding: 3px 12px;
    font: inherit;
  }
  button:hover:not(:disabled) { background: var(--raised-3); }
  button:disabled { opacity: var(--op-disabled); }
  /* the same strong blue as a request's Send, not the paler accent */
</style>
