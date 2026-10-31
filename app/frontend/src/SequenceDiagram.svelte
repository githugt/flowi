<script lang="ts">
  // A run drawn as a sequence diagram: Flowi on the left, one lifeline for each host it called, an arrow out and an
  // arrow back for each attempt. Drawn here from data, so it needs no diagram library.
  import type { SequenceView } from './lib/types'

  let { seq }: { seq: SequenceView } = $props()
  const COL = 190
  const ROW = 30
  const TOP = 46
  const x = (i: number) => 90 + i * COL // 0 is Flowi, 1.. the hosts
  const width = $derived(x(seq.hosts.length) + 90)
  const height = $derived(TOP + seq.steps.length * ROW + 24)
  const columns = $derived(['Flowi', ...seq.hosts])
</script>

<svg viewBox={`0 0 ${width} ${height}`} width={width} {height} role="img" aria-label="Sequence diagram of the run">
  <defs>
    <marker id="seqarrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" fill="currentColor" /></marker>
  </defs>
  {#each columns as name, i}
    <rect class="head" x={x(i) - 70} y="6" width="140" height="26" rx="8" />
    <text class="headtext" x={x(i)} y="24" text-anchor="middle">{name.length > 22 ? name.slice(0, 21) + '…' : name}<title>{name}</title></text>
    <line class="life" x1={x(i)} y1="34" x2={x(i)} y2={height - 8} />
  {/each}
  {#each seq.steps as s, i}
    {@const y = TOP + i * ROW + 14}
    {#if s.kind === 'note'}
      <g class="note" class:failed={s.failed}>
        <rect x={x(0) - 70} y={y - 14} width={x(seq.hosts.length) - x(0) + 140} height="24" rx="6" />
        <text x={x(0) - 60} y={y + 3}>{s.text.length > 110 ? s.text.slice(0, 109) + '…' : s.text}<title>{s.text}</title></text>
      </g>
    {:else}
      {@const to = s.kind === 'call' ? x(s.host + 1) : x(0)}
      {@const from = s.kind === 'call' ? x(0) : x(s.host + 1)}
      <g class="arrow" class:reply={s.kind === 'reply'} class:failed={s.failed}>
        <line x1={from} y1={y} x2={to} y2={y} marker-end="url(#seqarrow)" />
        <text x={(from + to) / 2} y={y - 4} text-anchor="middle">{s.text.length > 40 ? s.text.slice(0, 39) + '…' : s.text}<title>{s.text}</title></text>
      </g>
    {/if}
  {/each}
</svg>

<style>
  svg { color: var(--text); display: block; }
  .head { fill: var(--panel); stroke: var(--line); }
  .headtext { fill: var(--text); font-size: 12px; font-weight: 600; }
  .life { stroke: var(--line); stroke-dasharray: 4 4; }
  .arrow line { stroke: currentColor; stroke-width: 1.4; }
  .arrow.reply line { stroke-dasharray: 5 3; color: var(--muted); }
  .arrow text { fill: var(--text); font-size: 11px; }
  .arrow.reply text { fill: var(--muted); }
  .arrow.failed line { color: var(--fail); stroke: var(--fail); }
  .arrow.failed text { fill: var(--fail); }
  .note rect { fill: color-mix(in srgb, var(--warn) 12%, transparent); stroke: var(--warn); }
  .note.failed rect { fill: color-mix(in srgb, var(--fail) 12%, transparent); stroke: var(--fail); }
  .note text { fill: var(--text); font-size: 11px; }
</style>
