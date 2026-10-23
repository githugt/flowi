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
  import { untrack } from 'svelte'
  import type { Card, DataEdge, Diagram, Edge, Slot, Unit } from './lib/types'

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
    oncollapse,
    onaddcase,
    onadd,
    ondrag,
    ondrop,
    oncancel,
    initialView = null,
    onviewchange,
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
    oncollapse: (unit: string, collapsed: boolean) => void
    onaddcase?: (condition: string) => void // the "+ else if" under a condition (editing only)
    /** A + was clicked: a new step goes right before or after `near.step`; near is null for the + of an empty flow. */
    onadd?: (clientX: number, clientY: number, near: { step: string; before: boolean } | null) => void
    ondrag: (info: DragInfo) => void
    ondrop: (info: DragInfo) => void
    oncancel: () => void
    initialView?: CanvasView | null // where the canvas was the last time this flow was shown
    onviewchange?: (v: CanvasView) => void
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
  // A needs link being drawn from a unit's port to the pointer.
  let link = $state<{ from: string; x: number; y: number } | null>(null)
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
  const byId = $derived(new Map(units.map((u) => [u.id, u])))

  function at(u: Unit) {
    const m = moving
    if (m && m.id === u.id) return { x: u.x + m.dx, y: u.y + m.dy }
    return pending[u.id] ?? { x: u.x, y: u.y }
  }

  function portY(u: Unit): number {
    return at(u).y + (u.kind === 'container' ? (u.headW ? 32 : 20) : u.kind === 'request' ? (u.above ?? 0) + 32 : 24)
  }

  // A container drawn as a card has an arrow to each of its lanes, labelled with what takes it.
  const branchArrows = $derived.by(() => {
    const out: { d: string; label: string; lx: number; ly: number }[] = []
    for (const l of diagram.lanes ?? []) {
      const c = byId.get(l.container)
      if (!c?.headW) continue
      const x1 = at(c).x + c.headW
      const y1 = portY(c)
      const x2 = l.x
      const y2 = l.portY
      const mid = Math.max(20, Math.abs(x2 - x1) / 2)
      out.push({ d: `M${x1},${y1} C${x1 + mid},${y1} ${x2 - mid},${y2} ${x2},${y2}`, label: l.label.length > 22 ? l.label.slice(0, 21) + '…' : l.label, lx: x1 + 8, ly: y2 - 8 })
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

  function edgePath(e: Edge): string {
    const a = byId.get(e.from)
    const b = byId.get(e.to)
    if (!a || !b) return ''
    const x1 = at(a).x + a.w
    const y1 = portY(a)
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
  }
  let press: Press | null = null

  function onbackground(e: PointerEvent) {
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

  function onunit(e: PointerEvent, u: Unit) {
    e.stopPropagation()
    const c = toCanvas(e.clientX, e.clientY)!
    const step = stepAt(u, c.x, c.y)
    if (e.shiftKey) {
      onselect(step, true)
      return // shift-click only changes the selection
    }
    if (!selection.includes(step)) onselect(step, false)
    if (!editable) {
      press = { kind: 'pan', sx: e.clientX, sy: e.clientY, vx: view.x, vy: view.y, dragging: false }
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

  function onport(e: PointerEvent, u: Unit) {
    e.stopPropagation()
    if (!editable) return
    const c = toCanvas(e.clientX, e.clientY)!
    link = { from: u.id, x: c.x, y: c.y }
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
      if (target && target.id !== link.from) onconnect(link.from, target.id)
      link = null
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
    const minX = Math.min(...units.map((u) => u.x)) - (onadd ? ADD_GAP + 14 : 0)
    const minY = Math.min(...units.map((u) => u.y))
    const maxX = Math.max(...units.map((u) => u.x + u.w)) + (onadd ? ADD_GAP + 14 : 0)
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
    <marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto">
      <path d="M0,0 L10,5 L0,10 z" fill="var(--muted)" />
    </marker>
    <marker id="dataarrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto">
      <path d="M0,0 L10,5 L0,10 z" fill="var(--data)" />
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

    {#each diagram.edges as e}
      {#if !e.into}
        <path class="edge" class:fail={e.fail} class:needs={e.needs} d={edgePath(e)} marker-end="url(#arrow)" />
        <path class="flow" class:fail={e.fail} class:live={live(e)} d={edgePath(e)} />
        {#if e.needs && edgeMid(e)}{@const m = edgeMid(e)!}<text class="edgelabel" x={m.x} y={m.y} text-anchor="middle">waits for</text>{/if}
      {/if}
    {/each}

    {#each units as u (u.id)}
      {@const p = at(u)}
      {@const card = cards[u.id]}
      {@const status = statusOf(u)}
      <g class="unit {u.kind} {status}" class:selected={selectedUnits.has(u.id)} class:pulse={pulse === u.id} class:dragging={moving?.id === u.id}
        transform={`translate(${p.x} ${p.y})`} onpointerdown={(e) => onunit(e, u)} ondblclick={() => u.kind === 'strip' && (u.nodes.length > 1 || u.collapsed) && oncollapse(u.id, !u.collapsed)} role="button" tabindex="-1">
        {#if u.headW}
          <rect width={u.headW} height="40" y="12" rx="10" />
        {:else if u.above || u.nodes.some((_, i) => stacked(u, i))}
          <!-- a request row, with its stacked actions as cards above and below it -->
          <rect y={u.above ?? 0} width={u.w} height={cell(u, u.core ?? 0).h} rx="10" />
        {:else}
          <rect width={u.w} height={u.h} rx="10" />
        {/if}
        {#if u.kind === 'container' && u.headW}
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
        {#if u.kind === 'request' && card?.hooks?.length}
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
        {#if status}<circle class="dot {status}" cx={u.w - 14} cy={(u.above ?? 0) + 14} r="5" />{/if}
        {#if !u.headW}<circle class="port" cx={u.w} cy={portY(u) - at(u).y} r="6" role="button" tabindex="-1" aria-label="Drag to make another step wait for this one" onpointerdown={(e) => onport(e, u)}><title>Drag onto another step to make it wait for this one</title></circle>{/if}
        {#if resultOf(u)}<text class="result {status}" x={u.w - 10} y={(u.above ?? 0) + (isBlock(u) ? cell(u, u.core ?? 0).h : u.h) - 8} text-anchor="end">{resultOf(u)}</text>{/if}
      </g>
    {/each}

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
        <path class="linking" d={`M${at(from).x + from.w},${portY(from)} L${link.x},${link.y}`} />
        <text class="slotlabel" x={link.x + 10} y={link.y - 8}>Release on a step to make it wait for {from.nodes[from.core ?? 0]}</text>
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
  .data { fill: none; stroke: var(--data); stroke-width: 1.3; stroke-dasharray: 2 4; opacity: 0.55; pointer-events: none; }
  .data.hot { opacity: 1; stroke-width: 1.8; }
  .datalabel { fill: var(--data); font-size: 10px; opacity: 0.7; pointer-events: none; }
  .datalabel.hot { opacity: 1; font-weight: 600; }
  .edgelabel { fill: var(--muted); font-size: 10px; font-style: italic; pointer-events: none; }
  .branchlabel { fill: var(--accent); font-size: 11px; font-weight: 600; pointer-events: none; }
  .edge.branch { stroke-dasharray: none; }
  .lane { fill: none; stroke: var(--line); stroke-dasharray: 6 5; }
  .edge { fill: none; stroke: var(--muted); stroke-width: 1.6; }
  .edge.needs { stroke-dasharray: 4 4; }
  .edge.fail, .flow.fail { stroke: var(--fail); }
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
  .linking { stroke: var(--accent); stroke-width: 2; stroke-dasharray: 5 4; fill: none; pointer-events: none; }
  .kind { fill: var(--muted); font-size: 11px; }
  .id { fill: var(--text); font-size: 13px; font-weight: 600; }
  .slot { fill: var(--accent); pointer-events: none; }
  .slotlabel { fill: var(--accent); font-size: 10px; pointer-events: none; }
  .ghost { pointer-events: none; opacity: 0.85; }
  .ghost rect { fill: var(--accent); }
  .ghost text { fill: var(--on-accent); font-size: 11px; font-weight: 600; }
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
