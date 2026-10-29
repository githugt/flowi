<script lang="ts">
  // The Start card (the flow's inputs) or the End card (what it hands back), drawn in the canvas SVG beside the first
  // and last step. They are views of the flow, not steps: they have no ports to wire and cannot be moved. While
  // editing, a Start row can be dragged to a new place in the list and each card ends in an Add row.
  import Icon from './Icon.svelte'
  import type { Face } from './lib/types'
  import { CARD, clipText } from './lib/card'
  import { dropIndex, type Placed } from './lib/edgecards'

  let {
    role,
    face,
    place,
    editable = false,
    k = 1,
    onrow,
    onmove,
    onadd,
  }: {
    role: 'start' | 'end'
    face: Face | null
    place: Placed
    editable?: boolean
    k?: number // the canvas zoom: a drag is measured in screen pixels
    onrow?: (key: string, e: MouseEvent) => void
    onmove?: (from: number, to: number) => void
    onadd?: () => void
  } = $props()

  const isStart = $derived(role === 'start')
  const title = $derived(isStart ? 'Start' : 'End')
  const addLabel = $derived(isStart ? '+ Add input' : '+ Add output')
  const rows = $derived(face?.rows ?? [])
  const rowsTop = CARD.header + CARD.subline
  const badgeW = $derived(face?.badge ? 10 + face.badge.length * 7.4 : 0)

  let drag = $state<{ from: number; y0: number; dy: number; moved: boolean } | null>(null)
  const target = $derived(drag?.moved ? dropIndex(drag.from, drag.dy, k, rows.length) : -1)

  function down(i: number, e: PointerEvent) {
    e.stopPropagation()
    if (isStart && editable) drag = { from: i, y0: e.clientY, dy: 0, moved: false }
  }
  function move(e: PointerEvent) {
    if (!drag) return
    drag.dy = e.clientY - drag.y0
    drag.moved ||= Math.abs(drag.dy) > 4
  }
  function up(e: PointerEvent) {
    const d = drag
    if (!d) return
    drag = null
    if (d.moved) {
      const to = dropIndex(d.from, d.dy, k, rows.length)
      if (to !== d.from) onmove?.(d.from, to)
    } else {
      onrow?.(rows[d.from].key, e as unknown as MouseEvent)
    }
  }
</script>

<svelte:window onpointermove={move} onpointerup={up} />

{#if place.ghost}
  <g transform={`translate(${place.x} ${place.y})`} class="ghost" role="button" tabindex="-1" aria-label="Add an output: the flow will end with a card that shows it"
    onpointerdown={(e) => e.stopPropagation()} onclick={() => onadd?.()} onkeydown={() => {}}>
    <title>Add an output: the flow will end with a card that shows it</title>
    <rect width={place.w} height={CARD.header} rx="14" />
    <text x={place.w / 2} y="29" text-anchor="middle">+ Add output</text>
  </g>
{:else if face}
  <g transform={`translate(${place.x} ${place.y})`} class="edgecard {role}" role="group" aria-label={title}>
    <rect class="body" width={place.w} height={place.h} rx="14" />
    <rect class="tile" x="12" y="9" width="28" height="28" rx="8" style:fill={`var(${isStart ? '--c-teal' : '--c-purple'})`} />
    <g class="tileicon" transform="translate(16 13)"><Icon name={isStart ? 'stream' : 'extract'} size={20} /></g>
    <text class="title" x="50" y="28">{title}</text>
    <g transform={`translate(0 ${CARD.header})`}>
      {#if face.badge}
        <rect class="badge" x="14" y="4" width={badgeW} height="19" rx="5" />
        <text class="badgetext" x={14 + badgeW / 2} y="17.5" text-anchor="middle">{face.badge}</text>
      {/if}
      <text class="subtext" x={14 + (face.badge ? badgeW + 8 : 0)} y="18">{clipText(face.subline, 28)}<title>{face.subline}</title></text>
    </g>

    {#each rows as row, i (row.key)}
      {@const y = rowsTop + i * CARD.row}
      {@const lifted = drag?.moved && drag.from === i}
      <g class="row" class:lifted role="button" tabindex="-1" aria-label={row.summary} transform={lifted ? `translate(0 ${drag!.dy / k})` : undefined}
        onpointerdown={(e) => down(i, e)} onclick={(e) => !isStart && onrow?.(row.key, e)} onkeydown={() => {}}>
        <title>{row.summary}</title>
        <rect class="rowhit" x="6" {y} width={place.w - 12} height={CARD.row} rx="8" />
        <g class="rowicon" transform={`translate(16 ${y + 9})`}><Icon name={row.icon || 'variables'} size={16} /></g>
        <text class="rowtext" x="42" y={y + 22}>{clipText(row.summary, Math.floor((place.w - 42 - 34) / 7.2))}</text>
        {#if isStart && editable}<text class="grip" x={place.w - 24} y={y + 22}>⋮⋮</text>{/if}
      </g>
    {/each}
    {#if target >= 0 && target !== drag?.from}
      {@const ly = rowsTop + (target > drag!.from ? target + 1 : target) * CARD.row}
      <line class="drop" x1="12" x2={place.w - 12} y1={ly} y2={ly} />
    {/if}

    {#if editable}
      {@const y = rowsTop + rows.length * CARD.row}
      <g class="row add" role="button" tabindex="-1" aria-label={addLabel} onpointerdown={(e) => e.stopPropagation()} onclick={() => onadd?.()} onkeydown={() => {}}>
        <rect class="rowhit" x="6" {y} width={place.w - 12} height={CARD.row} rx="8" />
        <text class="rowtext" x="16" {y} dy="22">{addLabel}</text>
      </g>
    {/if}

    {#if isStart}<circle class="port" cx={place.w} cy={CARD.inPort} r="6" />{:else}<circle class="port" cx="0" cy={CARD.inPort} r="6" />{/if}
  </g>
{/if}

<style>
  .body { fill: var(--panel); stroke: var(--line); stroke-width: 1.2; filter: drop-shadow(0 2px 8px rgb(0 0 0 / 0.28)); }
  .tile { stroke: none; }
  .tileicon { color: var(--on-tile); pointer-events: none; }
  .title { fill: var(--text); font-size: 14px; font-weight: 600; pointer-events: none; }
  .badge { fill: color-mix(in srgb, var(--accent) 24%, transparent); }
  .badgetext { fill: var(--accent); font-size: 11px; font-weight: 700; letter-spacing: 0.04em; pointer-events: none; }
  .subtext { fill: var(--muted); font-size: 13px; pointer-events: none; }
  .row { cursor: pointer; }
  .row.lifted { opacity: 0.85; }
  .rowhit { fill: transparent; }
  .row:hover .rowhit, .row.lifted .rowhit { fill: var(--tint-hover); }
  .rowicon { color: var(--muted); pointer-events: none; }
  .rowtext { fill: var(--text); font-size: 13.5px; pointer-events: none; }
  .add .rowtext { fill: var(--muted); }
  .grip { fill: var(--muted); font-size: 12px; opacity: 0; pointer-events: none; }
  .row:hover .grip { opacity: 1; }
  .drop { stroke: var(--accent); stroke-width: 2.4; stroke-linecap: round; pointer-events: none; }
  .port { fill: var(--bg); stroke: var(--muted); stroke-width: 2; pointer-events: none; }
  .ghost { cursor: pointer; }
  .ghost rect { fill: transparent; stroke: var(--line); stroke-width: 1.4; stroke-dasharray: 5 4; }
  .ghost text { fill: var(--muted); font-size: 13.5px; font-weight: 600; pointer-events: none; }
  .ghost:hover rect { stroke: var(--accent); }
  .ghost:hover text { fill: var(--text); }
</style>
