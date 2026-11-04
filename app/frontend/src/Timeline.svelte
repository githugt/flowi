<script lang="ts">
  // The finished run in time: one bar per step, one row per step that ran alongside another. Dragging across it
  // (or the arrow keys) moves the playhead; the canvas shows the flow as it was at that moment.
  import { layoutBars, layoutGhosts, nextBoundary, spanOf } from './lib/timeline'
  import type { GhostView, HistoryRow, TimelineView } from './lib/types'

  let {
    timeline,
    playhead = null,
    onseek,
    onselect,
    runs = [],
    ghosts = [],
    oncompare,
  }: {
    timeline: TimelineView
    playhead?: number | null
    onseek: (ms: number | null) => void
    onselect?: (step: string) => void
    /** The saved runs to compare with, newest first. */
    runs?: HistoryRow[]
    /** The earlier run's bars, drawn behind the current ones. */
    ghosts?: GhostView[]
    oncompare?: (runId: string) => void
  } = $props()
  let compareWith = $state('')

  const LANE_H = 14
  const PAD = 6
  let width = $state(600)
  const lanes = $derived(Math.max(1, ...timeline.bars.map((b) => b.lane + 1)))
  // With an earlier run behind, both share one scale: a longer earlier run would otherwise be cut off.
  const total = $derived(spanOf(timeline, ghosts))
  const rects = $derived(layoutBars(timeline, width - PAD * 2, LANE_H, total))
  const behind = $derived(layoutGhosts(ghosts, total, width - PAD * 2, LANE_H))
  const slowest = $derived(ghosts.filter((g) => g.deltaMS > 0).sort((a, b) => b.deltaMS - a.deltaMS)[0])
  const headX = $derived(playhead === null ? null : PAD + (playhead / total) * (width - PAD * 2))

  function msAt(e: PointerEvent): number {
    const box = (e.currentTarget as SVGElement).getBoundingClientRect()
    const frac = (e.clientX - box.left - PAD) / Math.max(box.width - PAD * 2, 1)
    return Math.round(Math.min(Math.max(frac, 0), 1) * total)
  }

  let dragging = false
  function down(e: PointerEvent) {
    dragging = true
    ;(e.currentTarget as SVGElement).setPointerCapture(e.pointerId)
    onseek(msAt(e))
  }
  function move(e: PointerEvent) {
    if (dragging) onseek(msAt(e))
  }
  function up() {
    dragging = false
  }

  function key(e: KeyboardEvent) {
    const now = playhead ?? total
    if (e.key === 'ArrowRight') onseek(nextBoundary(timeline.bars, now, 1))
    else if (e.key === 'ArrowLeft') onseek(nextBoundary(timeline.bars, now, -1))
    else if (e.key === 'Escape' || e.key === 'End') onseek(null)
    else if (e.key === 'Home') onseek(0)
    else return
    e.preventDefault()
  }
</script>

{#if timeline.bars.length}
  <div class="timeline" bind:clientWidth={width}>
    <div class="head">
      <span class="label">Run timeline</span>
      <span class="muted">{playhead === null ? `${timeline.totalMS} ms · drag to look back` : `at ${playhead} ms of ${timeline.totalMS}`}</span>
      {#if runs.length && oncompare}
        <select class="compare" aria-label="Compare with an earlier run" bind:value={compareWith} onchange={() => compareWith && oncompare(compareWith)}>
          <option value="">Compare with…</option>
          {#each runs as r (r.id)}<option value={r.id}>{r.label}</option>{/each}
        </select>
      {/if}
      {#if ghosts.length && slowest}<span class="muted">slowest vs then: <b>{slowest.id}</b> +{slowest.deltaMS} ms</span>{/if}
      {#if playhead !== null}<button class="live" onclick={() => onseek(null)}>Back to the end</button>{/if}
    </div>
    <svg
      role="slider"
      tabindex="0"
      aria-label="Run timeline: arrow keys step to the next start or end, Escape returns to the end"
      aria-valuemin="0"
      aria-valuemax={timeline.totalMS}
      aria-valuenow={playhead ?? timeline.totalMS}
      {width}
      height={lanes * LANE_H + PAD * 2}
      onpointerdown={down}
      onpointermove={move}
      onpointerup={up}
      onkeydown={key}
    >
      {#each behind as g (g.id + g.x)}
        <rect class="ghost" class:slower={g.slower} class:faster={g.faster} x={PAD + g.x} y={PAD + g.y + 1} width={g.w} height={LANE_H - 3} rx="2" aria-hidden="true"><title>{g.id}: earlier run</title></rect>
      {/each}
      {#each rects as r (r.id + r.x)}
        <rect class="bar {r.status}" x={PAD + r.x} y={PAD + r.y + 1} width={r.w} height={LANE_H - 3} rx="2" onclick={() => onselect?.(r.id)} role="presentation">
          <title>{r.id}</title>
        </rect>
      {/each}
      {#if headX !== null}<line class="playhead" x1={headX} x2={headX} y1="0" y2={lanes * LANE_H + PAD * 2} />{/if}
    </svg>
  </div>
{/if}

<style>
  .timeline { flex: none; border-top: 1px solid var(--line); background: var(--panel); padding: 4px 0 0; }
  .head { display: flex; align-items: center; gap: var(--sp-2); padding: 0 10px; font-size: var(--fs-xs); }
  .label { font-weight: 600; }
  .muted { color: var(--muted); }
  .live { margin-left: auto; background: none; border: 1px solid var(--line); border-radius: var(--r-pill); color: var(--muted); font: inherit; padding: 0 8px; cursor: pointer; }
  .live:hover { border-color: var(--accent); color: var(--text); }
  svg { display: block; cursor: ew-resize; outline-offset: -2px; touch-action: none; }
  .bar { fill: var(--muted); opacity: 0.7; }
  .bar.passed { fill: var(--ok); }
  .bar.failed, .bar.error, .bar.aborted { fill: var(--fail); }
  .bar.skipped { fill: var(--muted); opacity: 0.35; }
  .ghost { fill: none; stroke: var(--muted); stroke-width: 1; stroke-dasharray: 3 2; pointer-events: none; }
  .ghost.slower { stroke: var(--warn); }
  .ghost.faster { stroke: var(--ok); }
  .compare { margin-left: auto; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-pill); font: inherit; padding: 0 6px; }
  .compare + .muted, .muted + .live { margin-left: 0; }
  .playhead { stroke: var(--accent); stroke-width: 2; pointer-events: none; }
</style>
