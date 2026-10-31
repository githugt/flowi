<script lang="ts" module>
  export interface DragInfo {
    unit: string // the dragged unit
    step: string // the dragged step: the lifted card, or the unit's first step
    lift: boolean // one card lifted out of a strip
    x: number // pointer, canvas units
    y: number
    snap: number // how close a drop must be to a connector, canvas units
    atX: number // where the unit's top-left corner would be put
    atY: number
  }
</script>

<script lang="ts">
  // The canvas renders a laid-out Diagram as SVG. Pan, pinch-zoom and drag
  // happen here, locally; the Go side hears of a drop once, on release, and
  // answers every pointer move of a drag with the slot to mark.
  import { untrack, type Snippet } from 'svelte'
  import type { Card, DataEdge, Diagram, Edge, Face, Slot, Unit } from './lib/types'
  import StepCard from './StepCard.svelte'
  import EdgeCard from './EdgeCard.svelte'
  import { edgeCards, keepStart } from './lib/edgecards'
  import { arrowEdges, arrowKey, arrowMarker, arrowOutcomes, outcomeOfPort, outcomesOfCard, toggled } from './lib/arrows'
  import { CARD, COLOUR_NAMES, openAfterClick, portLine, type OpenRow } from './lib/card'

  export interface CanvasView {
    x: number
    y: number
    k: number
  }

  let {
    diagram,
    cards,
    data = [],
    selection,
    statuses,
    results,
    slot,
    editable = true,
    onselect,
    onconnect,
    onfail,
    onoutcome,
    onaddon,
    onarrow,
    onlocked,
    oncollapse,
    onaddcase,
    onadd,
    ontoolbar,
    oncolour,
    oncase,
    ondrag,
    ondrop,
    oncancel,
    initialView = null,
    onviewchange,
    panel,
    ondata,
    dock,
    start = null,
    end = null,
    onedgerow,
    onmoveinput,
    onaddinput,
    onaddoutput,
  }: {
    diagram: Diagram
    cards: Record<string, Card>
    data?: DataEdge[] // values that pass between units, drawn under the cards
    selection: string[]
    statuses: Record<string, string>
    results: Record<string, string>
    slot: Slot | null
    editable?: boolean // structure changes (move, lift, connect) need Edit; pan and select do not
    onselect: (step: string, additive: boolean) => void
    onconnect: (from: string, to: string) => void
    /** A Fail port was dragged onto a step (or an arrow from it deleted, target ""): `step` goes on at `target` when it fails. */
    onfail?: (step: string, target: string) => void
    /** An outcome port (success, failure or a named outcome) was dragged onto a step: `to` runs when `from` ends with it. */
    onoutcome?: (from: string, outcome: string, to: string) => void
    /** The + beside an outcome port was clicked: a step is to be added that runs on that outcome of `step`. */
    onaddon?: (step: string, outcome: string, clientX: number, clientY: number) => void
    /** The person switched the outcomes that take the arrow from step `from` to step `to`: its success, its failure, both or neither. */
    onarrow?: (from: string, to: string, outcomes: string[]) => void
    /** The person tried to change the flow (move a step, open a row) while Edit is off. */
    onlocked?: () => void
    oncollapse: (unit: string, collapsed: boolean) => void
    onaddcase?: (condition: string) => void // the "+ else if" under a condition (editing only)
    /** A + was clicked: a new step goes right before or after `near.step`; near is null for the + of an empty flow. */
    onadd?: (clientX: number, clientY: number, near: { step: string; before: boolean } | null) => void
    ontoolbar?: (tool: 'add' | 'skip' | 'break' | 'duplicate' | 'delete', step: string, e: MouseEvent) => void // the selected card's toolbar
    oncolour?: (step: string, colour: string) => void
    oncase?: (step: string, index: number, e: MouseEvent) => void // a Route's case port label (Edit only)
    ondrag: (info: DragInfo) => void
    ondrop: (info: DragInfo) => void
    oncancel: () => void
    initialView?: CanvasView | null // where the canvas was the last time this flow was shown
    onviewchange?: (v: CanvasView) => void
    /** What opens under a card when one of its rows is clicked (Edit only); given the row and how to close it. */
    panel?: Snippet<[string, () => void]>
    /** A value wire was dropped from one step onto another (Edit only); where it was released, in client points. */
    ondata?: (from: string, to: string, clientX: number, clientY: number) => void
    /** What a step did in the last run, shown under its card while it is selected or pinned; given the step and its pin. */
    dock?: Snippet<[string, boolean, () => void]>
    /** The Start card (the flow's inputs) and the End card (its outputs; null without outputs). */
    start?: Face | null
    end?: Face | null
    onedgerow?: (role: 'start' | 'end', key: string, e: MouseEvent) => void
    onmoveinput?: (from: number, to: number) => void
    onaddinput?: () => void
    onaddoutput?: () => void
  } = $props()

  const MIN_K = 0.15
  const MAX_K = 3
  // A flow opens no further out than this, so its step names stay readable; Fit still shows all of it.
  const READABLE_K = 0.8
  const SNAP_PX = 24 // screen points
  const DRAG_THRESHOLD = 4 // screen points before a press becomes a drag
  const ACTION_W = 120 // keep in step with canvas.ActionW / StripPad
  const STRIP_PAD = 20
  const ADD_GAP = 34 // from a card's right edge to the + after it, clear of its port
  const HOOK_X = 36 // where a card's hook dots start, after their "hooks" label
  const ATTACH_W = 100 // canvas.attachedW
  const REQUEST_W = 220
  // A step name that fits its cell.
  // Text that fits a cell this wide (about 6.5 units a character).
  const fitText = (t: string, w: number) => (t.length * 6.5 > w - 16 ? t.slice(0, Math.max(3, Math.floor((w - 16) / 6.5) - 1)) + '…' : t)
  const clip = (id: string) => (id.length > 14 ? id.slice(0, 13) + '…' : id)
  const selectedSteps = $derived(new Set(selection))

  let view = $state<CanvasView>(untrack(() => initialView) ?? { x: 40, y: 60, k: 1 })
  $effect(() => onviewchange?.({ x: view.x, y: view.y, k: view.k }))
  let svg = $state<SVGSVGElement>() as SVGSVGElement
  // The unit being moved (not a lifted card) and how far.
  let moving = $state<{ id: string; dx: number; dy: number } | null>(null)
  // A lifted card follows the pointer as a ghost.
  let ghost = $state<{ step: string; x: number; y: number } | null>(null)
  // A needs link being drawn from a unit's port to the pointer; a value wire (kind data) is drawn the same way.
  let link = $state<{ from: string; x: number; y: number; data?: boolean; port?: string; ports?: string[] } | null>(null)
  // Outcome ports picked together (shift-click) on one card: dragging any of them takes them all to the step it is dropped on.
  let pickedPorts = $state<{ unit: string; ports: string[] }>({ unit: '', ports: [] })
  // Where a unit was dropped, until Go's new layout arrives. The layout may put the unit somewhere else (a drop
  // that reorders, or into a lane, is placed by the layout, not by the pointer), so any newer layout replaces it.
  let pending = $state<Record<string, { x: number; y: number }>>({})
  let pendingBase: unknown = null
  let pendingTimer: ReturnType<typeof setTimeout> | undefined
  function holdDrop(id: string, at: { x: number; y: number }) {
    pending[id] = at
    pendingBase = diagram
    clearTimeout(pendingTimer)
    pendingTimer = setTimeout(() => (pending = {}), 1000) // a drop Go refused changes no layout
  }
  $effect(() => {
    const d = diagram
    if (pendingBase !== null && d !== pendingBase) {
      pending = {}
      pendingBase = null
    }
  })

  const units = $derived(diagram.units)
  // Start and End are views of the flow, placed beside its first and last card.
  const placed = $derived(edgeCards(units, start?.rows.length ?? 0, end?.rows.length ?? 0, editable && !!onaddinput))
  // Start stays where it was shown while the first step is dragged about (see keepStart); it is remembered here.
  let startKept = $state<{ x: number; y: number } | null>(null)
  const edge = $derived({ ...placed, start: keepStart(startKept, placed.start, placed.first) })
  $effect(() => {
    const s = edge.start
    const kept = untrack(() => startKept)
    if (!kept || kept.x !== s.x || kept.y !== s.y) startKept = { x: s.x, y: s.y }
  })
  const curve = (x1: number, y1: number, x2: number, y2: number) => {
    const mid = (x1 + x2) / 2
    return `M${x1},${y1} C${mid},${y1} ${mid},${y2} ${x2},${y2}`
  }
  const byId = $derived(new Map(units.map((u) => [u.id, u])))

  function at(u: Unit) {
    const m = moving
    if (m && m.id === u.id) return { x: u.x + m.dx, y: u.y + m.dy }
    return pending[u.id] ?? { x: u.x, y: u.y }
  }

  function portY(u: Unit): number {
    if (u.card) return at(u).y + CARD.inPort
    return at(u).y + (u.kind === 'container' ? (u.headW ? 32 : 20) : u.kind === 'request' ? (u.above ?? 0) + 32 : 24)
  }

  /** The y of a card's port `id` (the in port's line for a unit that is not a card or has no such port). */
  function portYOf(u: Unit, id?: string): number {
    return u.card && id ? portLine(at(u).y, u.ports, id) : portY(u)
  }

  // A container drawn as a card has an arrow to each of its lanes, labelled with what takes it.
  const branchArrows = $derived.by(() => {
    const out: { d: string; label: string; lx: number; ly: number }[] = []
    for (const l of diagram.lanes ?? []) {
      const c = byId.get(l.container)
      if (!c?.headW) continue
      const x1 = at(c).x + c.headW
      const y1 = portYOf(c, l.port)
      const x2 = l.x
      const y2 = l.portY
      const mid = Math.max(20, Math.abs(x2 - x1) / 2)
      out.push({ d: `M${x1},${y1} C${x1 + mid},${y1} ${x2 - mid},${y2} ${x2},${y2}`, label: c.card ? '' : l.label.length > 22 ? l.label.slice(0, 21) + '…' : l.label, lx: x1 + 8, ly: y2 - 8 })
    }
    return out
  })

  // Data edges: a value one card makes and another reads, drawn as a dotted
  // arc under the cards so it is not mistaken for the run order.
  let showData = $state(true)
  const dataArcs = $derived.by(() => {
    const out: { d: string; label: string; lx: number; ly: number; hot: boolean }[] = []
    for (const e of showData ? data : []) {
      const a = byId.get(e.from)
      const b = byId.get(e.to)
      if (!a || !b) continue
      const x1 = at(a).x + a.w / 2
      const y1 = at(a).y + a.h
      const x2 = at(b).x + b.w / 2
      const y2 = at(b).y + b.h
      const drop = 36 + Math.min(60, Math.abs(x2 - x1) / 8)
      const hot = a.nodes.some((n) => selectedSteps.has(n)) || b.nodes.some((n) => selectedSteps.has(n))
      out.push({ d: `M${x1},${y1} C${x1},${y1 + drop} ${x2},${y2 + drop} ${x2},${y2 + 4}`, label: e.label, lx: (x1 + x2) / 2, ly: Math.max(y1, y2) + drop * 0.75 + 4, hot })
    }
    return out
  })

  // Where the label of an edge goes: halfway along it.
  function edgeMid(e: Edge): { x: number; y: number } | null {
    const a = byId.get(e.from)
    const b = byId.get(e.to)
    if (!a || !b) return null
    return { x: (at(a).x + a.w + at(b).x) / 2, y: (portY(a) + portY(b)) / 2 - 7 }
  }

  // Cards with a failure route show a port at the bottom edge; the route leaves from it, so it is not
  // taken for a step that follows.
  const failSources = $derived(new Set(diagram.edges.filter((e) => e.fail && !byId.get(e.from)?.card).map((e) => e.from)))

  function failPath(e: Edge): string {
    const a = byId.get(e.from)
    const b = byId.get(e.to)
    if (!a || !b) return ''
    const x1 = at(a).x + a.w / 2
    const y1 = at(a).y + a.h
    const x2 = at(b).x
    const y2 = portY(b)
    const drop = Math.max(36, Math.abs(y2 - y1) / 2)
    const lead = Math.max(30, Math.abs(x2 - x1) / 3)
    return `M${x1},${y1} C${x1},${y1 + drop} ${x2 - lead},${y2} ${x2},${y2}`
  }

  function edgePath(e: Edge): string {
    const a = byId.get(e.from)
    const b = byId.get(e.to)
    if (!a || !b) return ''
    if (e.fail && !a.card) return failPath(e)
    const x1 = at(a).x + a.w
    const y1 = portYOf(a, e.fromPort)
    const x2 = at(b).x
    const y2 = portY(b)
    const mid = Math.max(30, Math.abs(x2 - x1) / 2)
    return `M${x1},${y1} C${x1 + mid},${y1} ${x2 - mid},${y2} ${x2},${y2}`
  }

  function zoomAt(cx: number, cy: number, factor: number) {
    const k = Math.min(MAX_K, Math.max(MIN_K, view.k * factor))
    const f = k / view.k
    view = { k, x: cx - (cx - view.x) * f, y: cy - (cy - view.y) * f }
  }

  function local(e: { clientX: number; clientY: number }) {
    const r = svg.getBoundingClientRect()
    return { x: e.clientX - r.left, y: e.clientY - r.top }
  }

  /** Canvas coordinates of a client point, or null when it is outside the canvas. */
  export function toCanvas(clientX: number, clientY: number): { x: number; y: number; snap: number } | null {
    const r = svg.getBoundingClientRect()
    if (clientX < r.left || clientX > r.right || clientY < r.top || clientY > r.bottom) return null
    return { x: (clientX - r.left - view.x) / view.k, y: (clientY - r.top - view.y) / view.k, snap: SNAP_PX / view.k }
  }

  // Trackpad pinch arrives as a wheel event with ctrlKey set (Chromium,
  // WebView2); two-finger scroll is a plain wheel event.
  function onwheel(e: WheelEvent) {
    e.preventDefault()
    if (e.ctrlKey) {
      const p = local(e)
      zoomAt(p.x, p.y, Math.exp(-e.deltaY * 0.01))
    } else {
      view = { ...view, x: view.x - e.deltaX, y: view.y - e.deltaY }
    }
  }

  // WebKit (macOS WKWebView) sends gesture events for pinch instead.
  let gestureStart = 1
  function ongesturestart(e: any) {
    e.preventDefault()
    gestureStart = view.k
  }
  function ongesturechange(e: any) {
    e.preventDefault()
    const p = local(e)
    zoomAt(p.x, p.y, (gestureStart * e.scale) / view.k)
  }

  type Press = {
    kind: 'pan' | 'unit'
    sx: number
    sy: number
    vx: number
    vy: number
    unit?: Unit
    step?: string
    lift?: boolean
    dragging: boolean
    locked?: boolean // Edit is off: dragging pans, and says so once
    told?: boolean
  }
  let press: Press | null = null

  // The row of a card that is open, and so the card it opens under: one at a time.
  let open = $state<OpenRow | null>(null)
  function rowClicked(u: Unit, key: string) {
    onselect(u.nodes[0], false)
    if (!editable) onlocked?.()
    open = openAfterClick(open, u.id, key, !!panel)
  }
  // Steps whose dock stays open although another step is selected.
  let pins = $state<string[]>([])
  const togglePin = (step: string) => (pins = pins.includes(step) ? pins.filter((s) => s !== step) : [...pins, step])
  // The docks to show: for each card that has run and is selected or pinned (not the one a row editor is open under).
  const docked = $derived(
    !dock
      ? []
      : units.filter((u) => u.card && !!statuses[u.nodes[0]] && (selectedUnits.has(u.id) || pins.includes(u.nodes[0])) && open?.unit !== u.id),
  )
  const openUnit = $derived(open ? byId.get(open.unit) : undefined)
  // The panel sits right under its card, as wide as the card is (at least 300 points), and follows it as the view moves.
  function boxUnder(u: Unit) {
    const p = at(u)
    return { left: Math.max(8, view.x + p.x * view.k), top: view.y + (p.y + (u.headH || u.h)) * view.k + 8, width: Math.max(300, (u.headW || u.w) * view.k) }
  }
  const panelBox = $derived(openUnit ? boxUnder(openUnit) : null)
  // The panel belongs to the card's step: it closes when the selection moves to another step (not before the
  // selection has arrived: a click selects and opens at once, and the selection comes back from Go).
  let openSeen = false
  $effect(() => {
    if (!open) {
      openSeen = false
      return
    }
    if (!openUnit) open = null // the card went away (deleted, undone)
    else if (selectedUnits.has(open.unit)) openSeen = true
    else if (openSeen) open = null
  })

  function onbackground(e: PointerEvent) {
    open = null
    pickedEdge = ''
    pickedPorts = { unit: '', ports: [] }
    if (!e.shiftKey) onselect('', false)
    press = { kind: 'pan', sx: e.clientX, sy: e.clientY, vx: view.x, vy: view.y, dragging: true }
    svg.setPointerCapture(e.pointerId)
  }

  // The cell of node i in a request block with attached actions, as the layout put it: beside the
  // request, or in a stack above or below it.
  function cell(u: Unit, i: number): { x: number; y: number; w: number; h: number } {
    const c = u.cells?.[i]
    if (c) return c
    const core = u.core ?? 0
    if (i < core) return { x: i * ATTACH_W, y: 0, w: ATTACH_W, h: u.h }
    if (i === core) return { x: core * ATTACH_W, y: 0, w: REQUEST_W, h: u.h }
    return { x: core * ATTACH_W + REQUEST_W + (i - core - 1) * ATTACH_W, y: 0, w: ATTACH_W, h: u.h }
  }
  // A block with a stack above or below its request is drawn as the request's row and a card for each stacked action.
  const stacked = (u: Unit, i: number) => isBlock(u) && i !== (u.core ?? 0) && cell(u, i).h < (u.cells?.[u.core ?? 0]?.h ?? u.h)
  const isBlock = (u: Unit) => u.kind === 'request' && u.nodes.length > 1

  // Which card of a strip the pointer is on.
  function stepAt(u: Unit, canvasX: number, canvasY = 0): string {
    if (isBlock(u)) {
      const x = canvasX - u.x
      const y = canvasY - u.y
      const i = u.nodes.findIndex((_, k) => {
        const c = cell(u, k)
        return x >= c.x && x < c.x + c.w && y >= c.y && y < c.y + c.h
      })
      return u.nodes[i < 0 ? (u.core ?? 0) : i]
    }
    if (u.kind !== 'strip' || u.collapsed) return u.nodes[0]
    const i = Math.floor((canvasX - u.x - STRIP_PAD / 2) / ACTION_W)
    return u.nodes[Math.min(Math.max(i, 0), u.nodes.length - 1)]
  }

  /** The step a unit's card is for (the request of a block, else its first step). */
  const coreOf = (u: Unit) => u.nodes[u.core ?? 0]

  // The arrow picked by a click says when its step runs: when the one before succeeds, when it fails, or either
  // way. Two switches on it change that; switching both off (or Delete) takes the arrow away, which Go refuses
  // for the arrow from the step before (the step would follow it again by itself). An on_error goto route is
  // not an arrow written on a step: it only has its ×.
  let pickedEdge = $state('')
  const pickedEdges = $derived(arrowEdges(diagram.edges, pickedEdge))
  const pickedE = $derived(pickedEdges[0])
  const pickedOutcomes = $derived(pickedE ? arrowOutcomes(diagram.edges, pickedE.from, pickedE.to) : [])
  // The switches: every outcome the card the arrow leaves can end with, and any the arrow still follows that the card no longer has.
  const pickedOffered = $derived.by(() => {
    if (!pickedE) return []
    const offered = outcomesOfCard(cards[pickedE.from]?.face)
    return [...offered, ...pickedOutcomes.filter((o) => !offered.includes(o))]
  })
  // The colour an outcome was given shows on its arrows and switches; without one they keep their tone's.
  const portIdOf = (o: string) => (o === 'success' ? 'done' : o === 'failure' ? 'fail' : `outcome-${o}`)
  function portColourName(unit: string, port: string | undefined): string | undefined {
    const p = cards[unit]?.face?.ports?.find((x) => x.id === port)
    return p?.color && (COLOUR_NAMES as readonly string[]).includes(p.color) ? p.color : undefined
  }
  const portColour = (unit: string, port: string | undefined) => {
    const name = portColourName(unit, port)
    return name ? `var(--c-${name})` : undefined
  }
  const edgeTone = (e: Edge) => portColour(e.from, e.fromPort)
  const markerOf = (e: Edge) => `url(#${arrowMarker(e, portColourName(e.from, e.fromPort))})`
  const outcomeLabel = (o: string) => (o === 'success' ? 'On success' : o === 'failure' ? 'On failure' : o)
  const chipWidth = (o: string) => 36 + outcomeLabel(o).length * 6.8
  // The row of switches is centred on the arrow.
  const chipRow = $derived.by(() => {
    let x = 0
    const items = pickedOffered.map((o) => {
      const item = { o, x, w: chipWidth(o) }
      x += item.w + 6
      return item
    })
    return { items, shift: -(x + 30 - 6) / 2 }
  })
  function pickEdge(ev: PointerEvent, e: Edge) {
    ev.stopPropagation()
    open = null
    onselect('', false)
    pickedEdge = arrowKey(e)
  }
  /** The steps an arrow joins: the one it leaves (the one an after_failure_of names, else the last of the card) and the first of the one it enters. */
  function arrowSteps(): { from: string; to: string } | null {
    const e = pickedE
    const a = e && byId.get(e.from)
    const b = e && byId.get(e.to)
    if (!e || !a || !b) return null
    const named = pickedEdges.find((x) => x.fromStep)?.fromStep
    return { from: named ?? a.nodes[a.nodes.length - 1], to: b.nodes[0] }
  }
  function switchOutcome(outcome: string) {
    if (!editable) return onlocked?.()
    const steps = arrowSteps()
    if (!steps) return
    const next = toggled(pickedOutcomes, outcome)
    onarrow?.(steps.from, steps.to, next)
    if (!next.length) pickedEdge = ''
  }
  function removeEdge() {
    const steps = arrowSteps()
    if (!steps || !editable) return
    if (pickedE?.goto) onfail?.(steps.from, '')
    else onarrow?.(steps.from, steps.to, [])
    pickedEdge = ''
  }
  function onkeydown(ev: KeyboardEvent) {
    if (!pickedE) return
    if (ev.key === 'Escape') pickedEdge = ''
    if ((ev.key === 'Delete' || ev.key === 'Backspace') && !(ev.target as Element | null)?.closest?.('input, textarea, [contenteditable]')) {
      ev.preventDefault()
      editable ? removeEdge() : onlocked?.()
    }
  }
  // The middle of an arrow, where its switches go: a curve with level ends passes through the average of them.
  function edgeCentre(e: Edge): { x: number; y: number } | null {
    const a = byId.get(e.from)
    const b = byId.get(e.to)
    if (!a || !b) return null
    const failFromStrip = e.fail && !a.card
    const x1 = failFromStrip ? at(a).x + a.w / 2 : at(a).x + a.w
    const y1 = failFromStrip ? at(a).y + a.h : portYOf(a, e.fromPort)
    return { x: (x1 + at(b).x) / 2, y: (y1 + portY(b)) / 2 }
  }

  function onunit(e: PointerEvent, u: Unit) {
    e.stopPropagation()
    if (open && open.unit !== u.id) open = null
    pickedEdge = ''
    pickedPorts = { unit: '', ports: [] }
    const c = toCanvas(e.clientX, e.clientY)!
    const step = stepAt(u, c.x, c.y)
    if (e.shiftKey) {
      onselect(step, true)
      return // shift-click only changes the selection
    }
    if (!selection.includes(step)) onselect(step, false)
    if (!editable) {
      press = { kind: 'pan', sx: e.clientX, sy: e.clientY, vx: view.x, vy: view.y, dragging: false, locked: true }
      svg.setPointerCapture(e.pointerId)
      return
    }
    press = {
      kind: 'unit', sx: e.clientX, sy: e.clientY, vx: view.x, vy: view.y, unit: u, step,
      // One card of a strip (or an action of a request block) is lifted out; a single card, the request itself and Alt move the whole unit.
      lift: !e.altKey && ((u.kind === 'strip' && !u.collapsed && u.nodes.length > 1) || (isBlock(u) && step !== u.nodes[u.core ?? 0])), dragging: false,
    }
    svg.setPointerCapture(e.pointerId)
  }

  function info(e: PointerEvent, p: Press): DragInfo {
    const c = toCanvas(e.clientX, e.clientY) ?? { x: 0, y: 0, snap: SNAP_PX / view.k }
    const dx = (e.clientX - p.sx) / view.k
    const dy = (e.clientY - p.sy) / view.k
    return {
      unit: p.unit!.id, step: p.step!, lift: !!p.lift, x: c.x, y: c.y, snap: c.snap,
      atX: Math.round(p.unit!.x + dx), atY: Math.round(p.unit!.y + dy),
    }
  }

  function onport(e: PointerEvent, u: Unit, port = 'done') {
    e.stopPropagation()
    if (!editable) return
    if (e.shiftKey) {
      const same = pickedPorts.unit === u.id ? pickedPorts.ports : []
      pickedPorts = { unit: u.id, ports: same.includes(port) ? same.filter((p) => p !== port) : [...same, port] }
      return
    }
    const together = pickedPorts.unit === u.id && pickedPorts.ports.includes(port) ? pickedPorts.ports : [port]
    pickedPorts = { unit: '', ports: [] }
    const c = toCanvas(e.clientX, e.clientY)!
    link = { from: u.id, x: c.x, y: c.y, port, ports: together }
    svg.setPointerCapture(e.pointerId)
  }

  function ondataport(e: PointerEvent, u: Unit) {
    e.stopPropagation()
    if (!editable || !ondata) return
    const c = toCanvas(e.clientX, e.clientY)!
    link = { from: u.id, x: c.x, y: c.y, data: true }
    svg.setPointerCapture(e.pointerId)
  }

  function unitAt(x: number, y: number): Unit | undefined {
    return units.find((u) => x >= u.x && x <= u.x + u.w && y >= u.y && y <= u.y + u.h)
  }

  function onmovePointer(e: PointerEvent) {
    if (link) {
      const c = toCanvas(e.clientX, e.clientY)
      if (c) link = { ...link, x: c.x, y: c.y }
      return
    }
    const p = press
    if (!p) return
    const dx = e.clientX - p.sx
    const dy = e.clientY - p.sy
    if (p.kind === 'pan') {
      if (p.locked && !p.told && Math.hypot(dx, dy) >= DRAG_THRESHOLD) {
        p.told = true
        onlocked?.()
      }
      view = { ...view, x: p.vx + dx, y: p.vy + dy }
      return
    }
    if (!p.dragging && Math.hypot(dx, dy) < DRAG_THRESHOLD) return
    p.dragging = true
    const i = info(e, p)
    if (p.lift) ghost = { step: p.step!, x: i.x, y: i.y }
    else moving = { id: p.unit!.id, dx: dx / view.k, dy: dy / view.k }
    ondrag(i)
  }

  function onrelease(e: PointerEvent) {
    if (link) {
      const target = unitAt(link.x, link.y)
      const wire = link.data
      const from = link.from
      const port = link.port
      const ports = link.ports ?? (port ? [port] : [])
      link = null
      if (target && target.id !== from) {
        if (wire) ondata?.(byId.get(from)!.nodes[0], target.nodes[0], e.clientX, e.clientY)
        else if ((port && port !== 'done') || ports.length > 1) for (const p of ports) onoutcome?.(coreOf(byId.get(from)!), outcomeOfPort(p), target.nodes[0])
        else onconnect(from, target.id)
      }
      return
    }
    const p = press
    press = null
    if (p?.kind === 'unit' && p.dragging) {
      const i = info(e, p)
      if (!p.lift) holdDrop(p.unit!.id, { x: i.atX, y: i.atY })
      ondrop(i)
    }
    moving = null
    ghost = null
  }

  function oncancelPointer() {
    link = null
    press = null
    moving = null
    ghost = null
    oncancel()
  }

  /** Shows the whole flow, or with minK, as much of it as fits at that zoom, starting from its left end. */
  export function fit(minK = MIN_K) {
    if (!units.length) return
    const r = svg.getBoundingClientRect()
    // The + on each side of the flow is part of the picture: room for them on the left and right.
    const minX = Math.min(...units.map((u) => u.x), start ? edge.start.x : Infinity) - (onadd ? ADD_GAP + 14 : 0)
    const minY = Math.min(...units.map((u) => u.y))
    const maxX = Math.max(...units.map((u) => u.x + u.w), edge.end ? edge.end.x + edge.end.w : -Infinity) + (onadd ? ADD_GAP + 14 : 0)
    const maxY = Math.max(...units.map((u) => u.y + u.h))
    const whole = Math.min((r.width - 80) / (maxX - minX), (r.height - 80) / (maxY - minY))
    const k = Math.min(MAX_K, Math.max(minK, whole))
    const width = (maxX - minX) * k
    const x = width <= r.width - 80 ? (r.width - width) / 2 - minX * k : 40 - minX * k
    view = { k, x, y: (r.height - (maxY - minY) * k) / 2 - minY * k }
  }

  // The unit whose step is being pointed at, for a moment, so the eye finds it.
  let pulse = $state('')
  let pulseTimer: ReturnType<typeof setTimeout> | undefined

  /** Pans so the unit holding step is in the middle (zooming in when the view is far out) and pulses it. */
  export function reveal(step: string) {
    const u = units.find((x) => x.nodes.includes(step))
    if (!u) return
    const r = svg.getBoundingClientRect()
    const k = Math.max(view.k, 0.6)
    view = { k, x: r.width / 2 - (u.x + u.w / 2) * k, y: r.height / 2 - (u.y + u.h / 2) * k }
    pulse = u.id
    clearTimeout(pulseTimer)
    pulseTimer = setTimeout(() => (pulse = ''), 2400)
  }

  // A flow opens fitted to the canvas (at a readable zoom); after that the view is the user's,
  // and it is kept when the tab is shown again.
  let fitted = untrack(() => initialView) !== null
  $effect(() => {
    if (!fitted && units.length && svg.clientWidth > 0) {
      fitted = true
      fit(READABLE_K)
    }
  })

  $effect(() => {
    svg.addEventListener('gesturestart', ongesturestart)
    svg.addEventListener('gesturechange', ongesturechange)
    return () => {
      svg.removeEventListener('gesturestart', ongesturestart)
      svg.removeEventListener('gesturechange', ongesturechange)
    }
  })

  const RANK = ['', 'skipped', 'passed', 'running', 'cancelled', 'failed', 'error', 'aborted']
  // A unit shows the worst status of the steps in it.
  function statusOf(u: Unit): string {
    let worst = ''
    for (const n of u.nodes) {
      const s = statuses[n] ?? ''
      if (RANK.indexOf(s) > RANK.indexOf(worst)) worst = s
    }
    return worst
  }
  function resultOf(u: Unit): string {
    if (isBlock(u)) return results[u.nodes[u.core ?? 0]] ?? ''
    return u.nodes.length === 1 ? (results[u.nodes[0]] ?? '') : ''
  }
  const live = (e: Edge) => {
    const a = byId.get(e.from)
    return !!a && ['passed', 'running'].includes(statusOf(a))
  }

  // The minimap: the flow's extent, and the part of it the canvas shows.
  const bounds = $derived.by(() => {
    const minX = Math.min(...units.map((u) => u.x))
    const minY = Math.min(...units.map((u) => u.y))
    return { x: minX, y: minY, w: Math.max(...units.map((u) => u.x + u.w)) - minX, h: Math.max(...units.map((u) => u.y + u.h)) - minY }
  })
  let size = $state({ w: 800, h: 500 })
  $effect(() => {
    const ro = new ResizeObserver(() => (size = { w: svg.clientWidth, h: svg.clientHeight }))
    ro.observe(svg)
    return () => ro.disconnect()
  })
  const shown = $derived({ x: -view.x / view.k, y: -view.y / view.k, w: size.w / view.k, h: size.h / view.k })
  // The minimap only earns its place when part of the flow is out of view.
  const clipped = $derived(bounds.x < shown.x || bounds.y < shown.y || bounds.x + bounds.w > shown.x + shown.w || bounds.y + bounds.h > shown.y + shown.h)

  // Clicking or dragging on the minimap moves the view there.
  function minimapPan(e: PointerEvent) {
    const m = e.currentTarget as SVGSVGElement
    const move = (ev: PointerEvent) => {
      const r = m.getBoundingClientRect()
      const vb = m.viewBox.baseVal
      const cx = vb.x + ((ev.clientX - r.left) / r.width) * vb.width
      const cy = vb.y + ((ev.clientY - r.top) / r.height) * vb.height
      view = { ...view, x: size.w / 2 - cx * view.k, y: size.h / 2 - cy * view.k }
    }
    move(e)
    m.setPointerCapture(e.pointerId)
    m.onpointermove = move
    m.onpointerup = () => (m.onpointermove = m.onpointerup = null)
    e.stopPropagation()
  }

  const selectedUnits = $derived(new Set(units.filter((u) => u.nodes.some((n) => selection.includes(n))).map((u) => u.id)))
  // A + on each side of a card: on the selected one, else the right + after the last card (the one
  // reaching furthest right) and the left + before the first. There was only the right one, so a step
  // could not be added in front of another.
  const picked = $derived(units.find((u) => selectedUnits.has(u.id) && !u.headW))
  const addAfter = $derived.by(() => {
    if (picked) return picked
    const cards = units.filter((u) => !u.headW)
    return cards.reduce<Unit | null>((best, u) => (!best || at(u).x + u.w > at(best).x + best.w ? u : best), null)
  })
  // Where a + goes: on the arrow joining the card to its neighbour, half-way along it, so it reads as
  // "a step here" and never covers an arrowhead. Without a neighbour it stands clear of the card, joined by a stub.
  function plusOnEdge(u: Unit, outgoing: boolean): { x: number; y: number } | null {
    const e = diagram.edges.find((e) => !e.into && !e.needs && (outgoing ? e.from === u.id : e.to === u.id))
    const a = e && byId.get(e.from)
    const b = e && byId.get(e.to)
    if (!a || !b) return null
    return { x: (at(a).x + a.w + at(b).x) / 2, y: (portY(a) + portY(b)) / 2 }
  }

  const addBefore = $derived.by(() => {
    if (picked) return picked
    const cards = units.filter((u) => !u.headW)
    return cards.reduce<Unit | null>((best, u) => (!best || at(u).x < at(best).x ? u : best), null)
  })
</script>

<!-- A + that adds a step; the dashed stub joins it to its card (from x1 to x2, none when equal). -->
{#snippet plus(p: { x: number; y: number }, title: string, x1: number, x2: number, near: { step: string; before: boolean } | null)}
  <g class="addstep" role="button" tabindex="-1" aria-label={title}
    onpointerdown={(e) => e.stopPropagation()} onclick={(e) => onadd?.(e.clientX, e.clientY, near)} onkeydown={() => {}}>
    <title>{title}</title>
    {#if x1 !== x2}<line class="addstub" {x1} y1={p.y} {x2} y2={p.y} />{/if}
    <circle cx={p.x} cy={p.y} r="13" />
    <path d={`M${p.x - 5},${p.y} h10 M${p.x},${p.y - 5} v10`} />
  </g>
{/snippet}

<svelte:window {onkeydown} />

<svg
  bind:this={svg}
  class:readonly={!editable}
  role="application"
  aria-label="Flow canvas"
  onwheel={onwheel}
  onpointerdown={onbackground}
  onpointermove={onmovePointer}
  onpointerup={onrelease}
  onpointercancel={oncancelPointer}
>
  <defs>
    <pattern id="grid" width="24" height="24" patternUnits="userSpaceOnUse"
      patternTransform={`translate(${view.x} ${view.y}) scale(${view.k})`}>
      <circle cx="1" cy="1" r="1" fill="var(--line)" />
    </pattern>
    <!-- the header of a skipped step -->
    <pattern id="skiphatch" width="8" height="8" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
      <rect width="4" height="8" fill="var(--muted)" opacity="0.28" />
    </pattern>
    <!-- Where an arrow lands is a ring on the card's edge, in the arrow's own colour: a triangle was hard to read as a
         direction. One marker per colour, since a marker cannot take the colour of its line everywhere (context-stroke). -->
    {#each [{ id: 'arrow', colour: 'var(--muted)' }, { id: 'arrow-fail', colour: 'var(--fail)' }, ...COLOUR_NAMES.map((c) => ({ id: `arrow-c-${c}`, colour: `var(--c-${c})` }))] as m (m.id)}
      <marker id={m.id} viewBox="0 0 10 10" refX="5" refY="5" markerWidth="9" markerHeight="9" markerUnits="userSpaceOnUse" orient="auto">
        <circle cx="5" cy="5" r="3.6" fill="var(--bg)" stroke={m.colour} stroke-width="1.8" />
      </marker>
    {/each}
    <marker id="dataarrow" viewBox="0 0 10 10" refX="5" refY="5" markerWidth="9" markerHeight="9" markerUnits="userSpaceOnUse" orient="auto">
      <circle cx="5" cy="5" r="3.6" fill="var(--bg)" stroke="var(--data)" stroke-width="1.8" />
    </marker>
  </defs>
  <rect width="100%" height="100%" fill="url(#grid)" />

  <g transform={`translate(${view.x} ${view.y}) scale(${view.k})`}>
    {#each diagram.lanes as l}
      <rect class="lane" x={l.x} y={l.y} width={l.w} height={l.h} rx="10" />
      {#if l.label && !byId.get(l.container)?.headW}<text class="lanelabel" x={l.x + 10} y={l.y - 7}>{l.label.length > 40 ? l.label.slice(0, 39) + '…' : l.label}</text>{/if}
    {/each}

    {#each dataArcs as a}
      <path class="data" class:hot={a.hot} d={a.d} marker-end="url(#dataarrow)" />
      {#if a.hot || dataArcs.length <= 6}<text class="datalabel" class:hot={a.hot} x={a.lx} y={a.ly} text-anchor="middle">{a.label}</text>{/if}
    {/each}

    {#if editable && onaddcase}
      {#each units.filter((u) => u.kind === 'container' && u.types?.[0] === 'condition') as u (u.id)}
        {@const lanes = (diagram.lanes ?? []).filter((l) => l.container === u.id)}
        {#if lanes.length}
          {@const last = lanes[lanes.length - 1]}
          <g class="addcase" role="button" tabindex="-1" onpointerdown={(e) => e.stopPropagation()} onclick={() => onaddcase(u.id)} onkeydown={() => {}}>
            <rect x={last.x} y={last.y + last.h + 10} width="92" height="22" rx="11" />
            <text x={last.x + 46} y={last.y + last.h + 25} text-anchor="middle">+ else if</text>
          </g>
        {/if}
      {/each}
    {/if}

    {#each branchArrows as b}
      <path class="edge branch" d={b.d} marker-end="url(#arrow)" />
      {#if b.label}<text class="branchlabel" x={b.lx} y={b.ly}>{b.label}</text>{/if}
    {/each}

    {#if start && edge.first}
      <path class="edge" d={curve(edge.start.x + edge.start.w, edge.start.y + CARD.inPort, edge.first.x, edge.first.y + CARD.inPort)} marker-end="url(#arrow)" />
    {/if}
    {#if edge.end && !edge.end.ghost && edge.last}
      <path class="edge" d={curve(edge.last.x + edge.last.w, edge.last.y + CARD.inPort, edge.end.x, edge.end.y + CARD.inPort)} marker-end="url(#arrow)" />
    {/if}
    {#if start}
      <EdgeCard role="start" face={start} place={edge.start} {editable} k={view.k} onrow={(key, e) => onedgerow?.('start', key, e)} onmove={onmoveinput} onadd={onaddinput} />
    {/if}
    {#if edge.end && (end || edge.end.ghost)}
      <EdgeCard role="end" face={end} place={edge.end} {editable} k={view.k} onrow={(key, e) => onedgerow?.('end', key, e)} onadd={onaddoutput} />
    {/if}

    {#each diagram.edges as e}
      {#if !e.into}
        {@const either = !e.fail && !e.outcome && diagram.edges.some((x) => (x.fail || !!x.outcome) && !x.goto && x.from === e.from && x.to === e.to)}
        <path class="edge" class:fail={e.fail} class:named={!!e.outcome} style:stroke={edgeTone(e)} d={edgePath(e)} marker-end={markerOf(e)} />
        <path class="flow" class:fail={e.fail} class:named={!!e.outcome} class:live={live(e)} style:stroke={edgeTone(e)} d={edgePath(e)} />
        <path class="edgepick" class:on={pickedEdges.includes(e)} d={edgePath(e)} />
        <path class="edgehit" d={edgePath(e)} role="button" tabindex="-1" aria-label="Select this arrow" onpointerdown={(ev) => pickEdge(ev, e)}><title>{e.fail ? 'Runs when the step fails' : 'Runs when the step succeeds'}</title></path>
        {#if e.fail && edgeMid(e)}{@const m = edgeMid(e)!}<text class="edgelabel fail" x={m.x} y={m.y + 16} text-anchor="middle">if it fails</text>{/if}
        {#if e.outcome && edgeMid(e)}{@const m = edgeMid(e)!}<text class="edgelabel named" style:fill={edgeTone(e)} x={m.x} y={m.y + 16} text-anchor="middle">{e.outcome}</text>{/if}
        {#if either && edgeMid(e)}{@const m = edgeMid(e)!}<text class="edgelabel" x={m.x} y={m.y} text-anchor="middle">if it succeeds</text>{/if}
      {/if}
    {/each}

    {#each units as u (u.id)}
      {@const p = at(u)}
      {@const card = cards[u.id]}
      {@const status = statusOf(u)}
      <g class="unit {u.kind} {status}" class:selected={selectedUnits.has(u.id)} class:pulse={pulse === u.id} class:dragging={moving?.id === u.id}
        transform={`translate(${p.x} ${p.y})`} onpointerdown={(e) => onunit(e, u)} ondblclick={() => u.kind === 'strip' && (u.nodes.length > 1 || u.collapsed) && oncollapse(u.id, !u.collapsed)} role="button" tabindex="-1">
        {#if u.card && card?.face}
          <StepCard {u} face={card.face} title={card.title} selected={selectedUnits.has(u.id)} {status} result={resultOf(u)} skipped={card.skipped} breakpoint={card.break} note={card.skipNote} colour={card.color || undefined}
            toolbar={!!ontoolbar && editable && selection.length === 1 && selectedUnits.has(u.id) && !moving && !link}
            ontool={(tool, e) => ontoolbar?.(tool, u.nodes[u.core ?? 0], e)} oncolour={(c) => oncolour?.(u.nodes[u.core ?? 0], c)}
            oncase={oncase ? (i, e) => oncase(u.nodes[u.core ?? 0], i, e) : undefined}
            onrow={(key) => rowClicked(u, key)} onport={(e, port) => onport(e, u, port)} pickedPorts={pickedPorts.unit === u.id ? pickedPorts.ports : []} onaddon={onaddon && editable ? (port, e) => onaddon(coreOf(u), outcomeOfPort(port), e.clientX, e.clientY) : undefined} ondataport={ondata ? (e) => ondataport(e, u) : undefined} />
        {:else if u.headW}
          <rect width={u.headW} height="40" y="12" rx="10" />
        {:else if u.above || u.nodes.some((_, i) => stacked(u, i))}
          <!-- a request row, with its stacked actions as cards above and below it -->
          <rect y={u.above ?? 0} width={u.w} height={cell(u, u.core ?? 0).h} rx="10" />
        {:else}
          <rect width={u.w} height={u.h} rx="10" />
        {/if}
        {#if u.card}
          <!-- a card draws itself -->
        {:else if u.kind === 'container' && u.headW}
          <text x={u.headW / 2} y="30" class="id" text-anchor="middle">{u.types[0]}</text>
          <text x={u.headW / 2} y="45" class="kind" text-anchor="middle">{fitText(card?.title ?? '', u.headW)}</text>
        {:else if u.kind === 'container'}
          <text x="14" y="25" class="id">{card?.title} <tspan class="kind">{u.types[0]}</tspan></text>
        {:else if isBlock(u)}
          <!-- a request with the actions shown with it: cells in list order, the request in the middle of its own -->
          {#each u.nodes as id, i}
            {@const c = cell(u, i)}
            {@const stack = stacked(u, i)}
            {#if stack}
              <rect class="chip stack" class:cellsel={selectedSteps.has(id)} x={c.x} y={c.y} width={c.w} height={c.h} rx="8" />
              <text x={c.x + 12} y={c.y + c.h / 2 + 4} class="id">{clip(id)}</text>
              <text x={c.x + c.w - 12} y={c.y + c.h / 2 + 4} class="kind" text-anchor="end">{u.types[i] ?? ''}</text>
            {:else}
              {#if selectedSteps.has(id)}<rect class="cellsel" x={c.x + 2} y={c.y + 3} width={c.w - 4} height={c.h - 6} rx="8" />{/if}
              {#if i > 0 && !(u.cells && cell(u, i - 1).y !== c.y)}<line class="cellsep" x1={c.x} x2={c.x} y1={c.y + 10} y2={c.y + c.h - 10} />{/if}
              {#if i === (u.core ?? 0)}
                <text x={c.x + c.w / 2} y={c.y + c.h / 2 - 4} class="id" text-anchor="middle">{fitText(card?.title ?? '', c.w)}</text>
                <text x={c.x + c.w / 2} y={c.y + c.h / 2 + 14} class="kind" text-anchor="middle">{fitText(card?.subtitle ?? '', c.w)}</text>
              {:else}
                <rect class="chip" x={c.x + 3} y={c.y + 5} width={c.w - 6} height={c.h - 10} rx="8" />
                <text x={c.x + c.w / 2} y={c.y + c.h / 2 - 4} class="id" text-anchor="middle">{clip(id)}</text>
                <text x={c.x + c.w / 2} y={c.y + c.h / 2 + 14} class="kind" text-anchor="middle">{u.types[i] ?? ''}</text>
              {/if}
            {/if}
          {/each}
        {:else if u.kind === 'strip' && u.nodes.length > 1 && !u.collapsed}
          <!-- one cell per action, ACTION_W wide: the drop marker and the lifted card follow these cells -->
          {#each u.nodes as id, i}
            {@const x0 = STRIP_PAD / 2 + i * ACTION_W}
            {#if selectedSteps.has(id)}<rect class="cellsel" x={x0 + 2} y="3" width={ACTION_W - 4} height={u.h - 6} rx="8" />{/if}
            {#if i > 0}<line class="cellsep" x1={x0} x2={x0} y1="10" y2={u.h - 10} />{/if}
            <text x={x0 + ACTION_W / 2} y={u.h / 2 - 4} class="id" text-anchor="middle">{clip(id)}</text>
            <text x={x0 + ACTION_W / 2} y={u.h / 2 + 14} class="kind" text-anchor="middle">{u.types[i] ?? ''}</text>
          {/each}
        {:else}
          <text x={u.w / 2} y={u.h / 2 - 4} class="id" text-anchor="middle">{fitText(card?.title ?? '', u.w)}</text>
          <text x={u.w / 2} y={u.h / 2 + 14} class="kind" text-anchor="middle">{fitText(card?.subtitle ?? '', u.w)}</text>
        {/if}
        {#if u.kind === 'request' && !u.card && card?.hooks?.length}
          {@const core = isBlock(u) ? cell(u, u.core ?? 0) : { x: 0, y: 0, w: u.w, h: u.h }}
          {@const shownHooks = card.hooks.slice(0, 12)}
          {@const pre = shownHooks.filter((t) => t.phase === 'pre').length}
          <!-- A dot for each hook that runs around the request, filled as it passes or fails: the ones before
               it, a bar standing for the request, the ones after. Unlabelled, the dots read as ports. -->
          <g class="hooks" transform={`translate(${core.x + 14} ${core.y + core.h - 10})`}>
            <title>{`${card.hooks.length} ${card.hooks.length === 1 ? 'hook runs' : 'hooks run'} with this request: ${pre} before, ${card.hooks.length - pre} after`}</title>
            <text class="hklabel" x="0" y="3">hooks</text>
            {#each shownHooks as t, i}
              <circle class="hk {t.state}" cx={HOOK_X + i * 10 + (t.phase === 'post' ? 10 : 0)} cy="0" r="3.5"><title>{`${t.phase === 'pre' ? 'before' : 'after'} the request · ${t.level} · ${t.label}${t.state ? ' · ' + t.state : ''}`}</title></circle>
            {/each}
            <line class="hkbar" x1={HOOK_X + pre * 10} x2={HOOK_X + pre * 10} y1="-5" y2="5" />
            {#if card.hooks.length > 12}<text x={HOOK_X + 12 * 10 + 12} y="3" class="kind">+{card.hooks.length - 12}</text>{/if}
          </g>
        {/if}
        {#if status && !u.card}<circle class="dot {status}" cx={u.w - 14} cy={(u.above ?? 0) + 14} r="5" />{/if}
        {#if !u.headW && !u.card}<circle class="port" cx={u.w} cy={portY(u) - at(u).y} r="6" role="button" tabindex="-1" aria-label="Drag to make another step wait for this one" onpointerdown={(e) => onport(e, u)}><title>Drag onto another step to make it wait for this one</title></circle>{/if}
        {#if resultOf(u) && !u.card}<text class="result {status}" x={u.w - 10} y={(u.above ?? 0) + (isBlock(u) ? cell(u, u.core ?? 0).h : u.h) - 8} text-anchor="end">{resultOf(u)}</text>{/if}
      </g>
    {/each}

    <!-- The port of a failure route sits on the card's bottom edge, over the card. -->
    {#each units as u (u.id)}
      {#if failSources.has(u.id)}
        {@const p = at(u)}
        <circle class="failport" cx={p.x + u.w / 2} cy={p.y + u.h} r="5"><title>Where the failure route leaves this step</title></circle>
      {/if}
    {/each}

    <!-- The switches of the picked arrow, over the cards so the narrow gap between two of them is no limit. -->
    {#if pickedE && edgeCentre(pickedE)}
      {@const m = edgeCentre(pickedE)!}
      {#if pickedE.goto}
        {#if editable}
          <g class="edgex" role="button" tabindex="-1" aria-label="Delete this route" onpointerdown={(ev) => ev.stopPropagation()} onclick={removeEdge} onkeydown={() => {}}>
            <title>Delete this failure route</title>
            <circle cx={m.x} cy={m.y} r="12" />
            <path d={`M${m.x - 4},${m.y - 4} l8,8 M${m.x + 4},${m.y - 4} l-8,8`} />
          </g>
        {/if}
      {:else}
        <g class="arrowswitch" transform={`translate(${m.x + chipRow.shift} ${m.y})`}>
          {#each chipRow.items as sw (sw.o)}
            {@const on = pickedOutcomes.includes(sw.o)}
            {@const tone = sw.o === 'success' ? 'success' : sw.o === 'failure' ? 'failure' : 'named'}
            <g class="switch {tone}" class:on style:--chip={portColour(pickedE.from, portIdOf(sw.o))} role="switch" aria-checked={on} aria-label={`Run when the step ends with ${sw.o}`} tabindex="-1"
              onpointerdown={(ev) => ev.stopPropagation()} onclick={() => switchOutcome(sw.o)} onkeydown={() => {}}>
              <title>{on ? `Runs on ${sw.o}: click to stop` : `Click to also run on ${sw.o}`}</title>
              <rect x={sw.x} y="-13" width={sw.w} height="26" rx="13" />
              <path class="tick" d={`M${sw.x + 11},0 l4,4 l8,-8`} />
              <text x={sw.x + 29} y="4">{outcomeLabel(sw.o)}</text>
            </g>
          {/each}
          {#if editable}
            <g class="edgex" role="button" tabindex="-1" aria-label="Remove this arrow" onpointerdown={(ev) => ev.stopPropagation()} onclick={removeEdge} onkeydown={() => {}}>
              <title>Remove this arrow: the step is then not started by it</title>
              <circle cx={(chipRow.items.at(-1)?.x ?? 0) + (chipRow.items.at(-1)?.w ?? 0) + 18} cy="0" r="12" />
              <path d={`M${(chipRow.items.at(-1)?.x ?? 0) + (chipRow.items.at(-1)?.w ?? 0) + 14},-4 l8,8 M${(chipRow.items.at(-1)?.x ?? 0) + (chipRow.items.at(-1)?.w ?? 0) + 22},-4 l-8,8`} />
            </g>
          {/if}
        </g>
      {/if}
    {/if}

    {#if onadd && !moving && !link}
      {#if addAfter}
        {@const joined = plusOnEdge(addAfter, true)}
        {@const after = joined ?? { x: at(addAfter).x + addAfter.w + ADD_GAP, y: portY(addAfter) }}
        {@render plus(after, 'Add a step after this one', joined ? 0 : after.x - ADD_GAP + 8, joined ? 0 : after.x - 13, { step: addAfter.nodes[addAfter.nodes.length - 1], before: false })}
      {/if}
      {#if addBefore}
        {@const joined = plusOnEdge(addBefore, false)}
        {@const before = joined ?? { x: at(addBefore).x - ADD_GAP, y: portY(addBefore) }}
        {@render plus(before, 'Add a step before this one', joined ? 0 : before.x + 13, joined ? 0 : before.x + ADD_GAP - 8, { step: addBefore.nodes[0], before: true })}
      {/if}
      {#if !addAfter && !addBefore}
        {@render plus({ x: 40, y: 40 }, 'Add the first step', 0, 0, null)}
      {/if}
    {/if}

    {#if link}
      {@const from = byId.get(link.from)}
      {#if from}
        <path class="linking" class:wire={link.data} class:failing={link.port === 'fail'} class:named={!!link.port && link.port.startsWith('outcome-')} d={link.data ? `M${at(from).x + from.w / 2},${at(from).y + (from.headH || from.h)} L${link.x},${link.y}` : `M${at(from).x + from.w},${link.port && link.port !== 'done' ? portYOf(from, link.port) : portY(from)} L${link.x},${link.y}`} />
        <text class="slotlabel" x={link.x + 10} y={link.y - 8}>{link.data ? `Release on a step to pass it a value from ${from.nodes[0]}` : link.port === 'fail' ? `Release on a step to run it when ${coreOf(from)} fails` : link.port?.startsWith('outcome-') ? `Release on a step to run it on ${outcomeOfPort(link.port)} of ${coreOf(from)}` : `Release on a step to make it wait for ${coreOf(from)}`}</text>
      {/if}
    {/if}
    {#if slot?.ok}
      {#if slot.horizontal}
        <rect class="slot" x={slot.x} y={slot.y - 2} width={slot.w ?? 0} height="4" rx="2" />
        <text class="slotlabel" x={slot.x + (slot.w ?? 0) / 2} y={slot.y - 6} text-anchor="middle">{`Attach ${slot.side} ${slot.attach}`}</text>
      {:else}
        <rect class="slot" x={slot.x - 2} y={slot.y} width="4" height={slot.h} rx="2" />
        <text class="slotlabel" x={slot.x} y={slot.y - 6} text-anchor="middle">{slot.attach ? `Attach to ${slot.attach}` : 'Drop here'}</text>
      {/if}
    {/if}
    {#if ghost}
      <g class="ghost" transform={`translate(${ghost.x - 54} ${ghost.y - 17})`}>
        <rect width="108" height="34" rx="6" />
        <text x="10" y="21">{ghost.step}</text>
      </g>
    {/if}
  </g>
</svg>

{#if panel && open && panelBox}
  <div class="rowpanel" style:left={`${panelBox.left}px`} style:top={`${panelBox.top}px`} style:width={`${panelBox.width}px`}
    onpointerdown={(e) => e.stopPropagation()} onwheel={(e) => e.stopPropagation()} onkeydown={(e) => e.key === 'Escape' && (open = null)} role="presentation">
    {@render panel(open.key, () => (open = null))}
  </div>
{/if}

{#if dock}
  {#each docked as u (u.id)}
    {@const b = boxUnder(u)}
    <div class="dockbox" style:left={`${b.left}px`} style:top={`${b.top}px`} style:width={`${b.width}px`} onpointerdown={(e) => e.stopPropagation()} onwheel={(e) => e.stopPropagation()} role="presentation">
      {@render dock(u.nodes[0], pins.includes(u.nodes[0]), () => togglePin(u.nodes[0]))}
    </div>
  {/each}
{/if}

{#if units.length > 1 && clipped}
  <svg class="minimap" viewBox={`${bounds.x - 40} ${bounds.y - 40} ${bounds.w + 80} ${bounds.h + 80}`} role="presentation" onpointerdown={minimapPan}>
    {#each units as u (u.id)}<rect x={u.x} y={u.y} width={u.w} height={u.h} class={u.kind} />{/each}
    <rect class="view" x={shown.x} y={shown.y} width={shown.w} height={shown.h} />
  </svg>
{/if}

<div class="bar">
  <button onclick={() => zoomAt(svg.clientWidth / 2, svg.clientHeight / 2, 1 / 1.25)}>−</button>
  <span>{Math.round(view.k * 100)}%</span>
  <button onclick={() => zoomAt(svg.clientWidth / 2, svg.clientHeight / 2, 1.25)}>+</button>
  <button onclick={() => fit()}>Fit</button>
  {#if data.length}<button class:on={showData} title="Show where values pass from one step to another" onclick={() => (showData = !showData)}>Data</button>{/if}
</div>

<style>
  svg {
    width: 100%;
    height: 100%;
    display: block;
    cursor: grab;
    touch-action: none;
  }
  .lanelabel { fill: var(--accent); font-size: 11px; font-weight: 600; pointer-events: none; }
  .hk { fill: none; stroke: var(--muted); stroke-width: 1.5; }
  .hklabel { fill: var(--muted); font-size: 9.5px; letter-spacing: 0.03em; }
  .hkbar { stroke: var(--muted); stroke-width: 2; stroke-linecap: round; }
  .hk.passed { fill: var(--ok); stroke: var(--ok); }
  .hk.failed, .hk.error { fill: var(--fail); stroke: var(--fail); }
  .hk.running { stroke: var(--accent); }
  .addstep { cursor: pointer; }
  .addstep circle { fill: var(--panel); stroke: var(--accent); stroke-width: 1.5; stroke-dasharray: 3 3; }
  .addstep path { stroke: var(--accent); stroke-width: 2; stroke-linecap: round; }
  .addstep:hover circle { fill: color-mix(in srgb, var(--accent) 22%, var(--panel)); stroke-dasharray: none; }
  .addstub { stroke: var(--muted); stroke-width: 1.5; stroke-dasharray: 3 4; }
  .addcase { cursor: pointer; }
  .addcase rect { fill: var(--panel); stroke: var(--accent); stroke-dasharray: 3 3; }
  .addcase text { fill: var(--accent); font-size: 11px; pointer-events: none; }
  .addcase:hover rect { fill: color-mix(in srgb, var(--accent) 18%, var(--panel)); }
  /* Faint, thin dots were not seen as an arrow at all after a value was connected. */
  .data { fill: none; stroke: var(--data); stroke-width: 2; stroke-dasharray: 6 4; opacity: 0.9; pointer-events: none; }
  .data.hot { opacity: 1; stroke-width: 2.6; }
  .datalabel { fill: var(--data); font-size: 11px; opacity: 0.9; pointer-events: none; }
  .datalabel.hot { opacity: 1; font-weight: 600; }
  .edgelabel { fill: var(--muted); font-size: 10px; font-style: italic; pointer-events: none; }
  .branchlabel { fill: var(--accent); font-size: 11px; font-weight: 600; pointer-events: none; }
  .edge.branch { stroke-dasharray: none; }
  .lane { fill: none; stroke: var(--line); stroke-dasharray: 6 5; }
  .edge { fill: none; stroke: var(--muted); stroke-width: 1.6; }
  .edge.fail, .flow.fail { stroke: var(--fail); }
  .edgehit { fill: none; stroke: transparent; stroke-width: 16; pointer-events: stroke; cursor: pointer; }
  .edgepick { fill: none; stroke: var(--accent); stroke-width: 3; opacity: 0; pointer-events: none; }
  .edgepick.on { opacity: 0.9; }
  .readonly .edgehit { cursor: default; }
  .edgex { cursor: pointer; }
  .edgex circle { fill: var(--panel); stroke: var(--accent); stroke-width: 2; }
  .edgex:hover circle { fill: var(--fail); stroke: var(--fail); }
  .edgex path { fill: none; stroke: var(--text); stroke-width: 2; stroke-linecap: round; pointer-events: none; }
  .switch { cursor: pointer; }
  .switch rect { fill: var(--panel); stroke: var(--line); stroke-width: 1.4; filter: drop-shadow(0 2px 6px rgb(0 0 0 / 0.3)); }
  .switch text { fill: var(--muted); font-size: 12px; font-weight: 600; pointer-events: none; }
  .switch .tick { fill: none; stroke: transparent; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; pointer-events: none; }
  .switch.on text { fill: var(--text); }
  .switch.on rect { stroke: var(--chip, var(--accent)); }
  .switch.success.on rect { stroke: var(--chip, var(--ok)); fill: color-mix(in srgb, var(--chip, var(--ok)) 18%, var(--panel)); }
  .switch.named.on rect { stroke: var(--chip, var(--info)); fill: color-mix(in srgb, var(--chip, var(--info)) 18%, var(--panel)); }
  .switch.named.on .tick { stroke: var(--chip, var(--info)); }
  .switch.failure.on rect { stroke: var(--chip, var(--fail)); fill: color-mix(in srgb, var(--chip, var(--fail)) 18%, var(--panel)); }
  .switch.success.on .tick { stroke: var(--chip, var(--ok)); }
  .switch.failure.on .tick { stroke: var(--chip, var(--fail)); }
  .switch:hover rect { stroke: var(--accent); }
  .linking.failing { stroke: var(--fail); }
  .linking.named, .edge.named, .flow.named { stroke: var(--info); }
  .edgelabel.named { fill: var(--info); }
  .edgelabel.fail { fill: var(--fail); }
  .failport { fill: var(--bg); stroke: var(--fail); stroke-width: 2; pointer-events: none; }
  .flow {
    fill: none;
    stroke: var(--accent);
    stroke-width: 2.4;
    stroke-dasharray: 3 21;
    stroke-linecap: round;
    animation: drift 1.2s linear infinite;
    pointer-events: none;
    opacity: 0.3;
    transition: opacity 0.3s;
  }
  .flow.live { opacity: 1; stroke-width: 3; }
  @keyframes drift { to { stroke-dashoffset: -24; } }
  .unit { cursor: pointer; }
  .unit rect {
    stroke: var(--line);
    stroke-width: 1.2;
    transition: stroke 0.2s, filter 0.2s;
    filter: drop-shadow(0 2px 6px rgb(0 0 0 / 0.25));
  }
  .unit.request rect { fill: var(--request); }
  .unit.strip rect { fill: var(--strip); }
  .unit.action rect { fill: var(--action); }
  .unit.container rect { fill: var(--container); }
  .unit.selected rect { stroke: var(--accent); stroke-width: 2.5; filter: drop-shadow(0 0 10px var(--accent)); }
  .unit.dragging rect { stroke: var(--accent); }
  .unit text { pointer-events: none; }
  .unit rect.chip { fill: var(--action); stroke: none; filter: none; }
  .unit rect.cellsel, .unit.selected rect.cellsel { fill: color-mix(in srgb, var(--accent) 28%, transparent); stroke: none; filter: none; }
  .unit .cellsep { stroke: color-mix(in srgb, var(--text) 25%, transparent); stroke-width: 1; }
  .unit.passed rect { stroke: var(--ok); }
  .unit.failed rect, .unit.error rect, .unit.aborted rect { stroke: var(--fail); stroke-width: 2.4; filter: drop-shadow(0 0 5px var(--fail)); }
  .unit.pulse rect { animation: pulse 0.7s ease-in-out 3; }
  @keyframes pulse { 50% { filter: drop-shadow(0 0 14px var(--fail)); stroke-width: 4; } }
  .unit.running rect { stroke: var(--accent); animation: pulse 1.1s ease-in-out infinite; }
  @keyframes pulse { 50% { stroke-opacity: 0.35; } }
  .dot { fill: var(--muted); }
  .dot.passed { fill: var(--ok); }
  .dot.failed, .dot.error, .dot.aborted { fill: var(--fail); }
  .dot.running { fill: var(--accent); animation: pulse 1.1s ease-in-out infinite; }
  .result { fill: var(--muted); font-size: 10px; }
  .result.failed, .result.error { fill: var(--fail); }
  .port { fill: var(--bg); stroke: var(--muted); stroke-width: 1.6; opacity: 0; cursor: crosshair; transition: opacity 0.15s; }
  .unit:hover .port, .unit.selected .port { opacity: 1; }
  .readonly .port { display: none; }
  .port:hover { fill: var(--accent); stroke: var(--accent); }
  .linking.wire { stroke: var(--data); }
  .linking { stroke: var(--accent); stroke-width: 2; stroke-dasharray: 5 4; fill: none; pointer-events: none; }
  .kind { fill: var(--muted); font-size: 11px; }
  .id { fill: var(--text); font-size: 13px; font-weight: 600; }
  .slot { fill: var(--accent); pointer-events: none; }
  .slotlabel { fill: var(--accent); font-size: 10px; pointer-events: none; }
  .ghost { pointer-events: none; opacity: 0.85; }
  .ghost rect { fill: var(--accent); }
  .ghost text { fill: var(--on-accent); font-size: 11px; font-weight: 600; }
  .rowpanel {
    position: absolute;
    z-index: 5;
    box-sizing: border-box;
    max-height: 60%;
    overflow: auto;
    padding: 12px;
    background: var(--panel);
    border: 1px solid var(--accent);
    border-radius: 14px;
    box-shadow: var(--shadow);
  }
  .dockbox { position: absolute; z-index: 4; box-sizing: border-box; }
  .minimap {
    position: absolute;
    left: 16px;
    bottom: 16px;
    width: 150px;
    height: 90px;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: var(--r-lg);
    cursor: pointer;
  }
  .minimap rect { fill: var(--muted); opacity: 0.55; rx: 6; }
  .minimap rect.view { fill: color-mix(in srgb, var(--accent) 18%, transparent); stroke: var(--accent); stroke-width: 6; opacity: 1; rx: 0; }
  .bar {
    position: absolute;
    right: 16px;
    bottom: 16px;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 10px;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: var(--r-lg);
  }
  .bar button.on { color: var(--accent); }
  .bar span { min-width: 40px; text-align: center; color: var(--muted); }
  button {
    background: transparent;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    padding: 3px 10px;
    font: inherit;
  }
  button:hover:not(:disabled) { border-color: var(--accent); }
</style>
