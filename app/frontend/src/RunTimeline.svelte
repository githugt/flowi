<script lang="ts">
  import Button from './Button.svelte'
  import { axisTicks, formatSpan, runLength, slowestStep, timelineRows } from './lib/timeline'
  import type { TimelineBar } from './lib/types'

  // The last run as a waterfall: one bar per step on a shared time axis, so steps that ran side by side line up and the
  // slow one stands out. A step's name selects it on the canvas.
  let { bars, onselect }: { bars: TimelineBar[]; onselect: (step: string) => void } = $props()

  const rows = $derived(timelineRows(bars))
  const total = $derived(runLength(bars))
  const ticks = $derived(axisTicks(total))
  const slow = $derived(slowestStep(bars))
</script>

<section class="timeline" aria-label="Run timeline">
  <p class="head">
    <strong>Timeline</strong>
    <span class="muted">{formatSpan(total)} in all{slow ? ` · ${slow.step} took ${slow.share}% of it` : ''}</span>
  </p>
  <div class="axis" aria-hidden="true">
    <span></span>
    <div class="ticks">{#each ticks as t (t.at)}<span class="tick" style:left="{t.at}%">{t.label}</span>{/each}</div>
  </div>
  <ul>
    {#each rows as r (r.step)}
      <li class:overlaps={r.overlaps}>
        <Button variant="ghost" small title="Show {r.step} on the canvas" onclick={() => onselect(r.step)}>{r.step}</Button>
        <div class="track">
          <span class="bar {r.status}" style:left="{r.left}%" style:width="{r.width}%" title="{r.step}: {r.status}, {r.duration}"></span>
        </div>
        <span class="dur">{r.duration}</span>
      </li>
    {/each}
  </ul>
</section>

<style>
  .timeline { padding: var(--sp-2) var(--sp-3); border-bottom: 1px solid var(--line); max-height: 40%; overflow: auto; flex: none; }
  .head { margin: 0 0 var(--sp-2); display: flex; gap: var(--sp-2); align-items: baseline; font-size: var(--fs-sm); }
  .muted { color: var(--muted); }
  ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
  li, .axis { display: grid; grid-template-columns: 140px 1fr 72px; align-items: center; gap: var(--sp-2); }
  li :global(button) { justify-self: start; max-width: 100%; overflow: hidden; text-overflow: ellipsis; }
  .track { position: relative; height: 12px; background: color-mix(in srgb, var(--muted) 10%, transparent); border-radius: var(--r-sm); }
  .bar { position: absolute; top: 0; bottom: 0; border-radius: var(--r-sm); background: var(--muted); }
  .bar.passed { background: var(--ok); }
  .bar.running { background: var(--info); }
  .bar.skipped, .bar.cancelled { background: var(--muted); opacity: 0.6; }
  .bar.failed, .bar.error, .bar.aborted { background: var(--fail); }
  .dur { color: var(--muted); font-size: var(--fs-xs); text-align: right; font-variant-numeric: tabular-nums; }
  .ticks { position: relative; height: 1.4em; font-size: var(--fs-xs); color: var(--muted); }
  .tick { position: absolute; transform: translateX(-50%); white-space: nowrap; }
  .tick:first-child { transform: none; }
  .tick:last-child { transform: translateX(-100%); }
</style>
