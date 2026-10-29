<script lang="ts">
  // One step drawn as a calm card inside the canvas SVG: a header with the step's tile and name, a sub-line
  // that says what it does, at most four summary rows (an icon and a short phrase, opened in place), and
  // control ports on its right edge. It draws what internal/card decided; it never reads a step's fields.
  import Icon from './Icon.svelte'
  import type { Face, Unit } from './lib/types'
  import { CARD, COLOUR_NAMES, clipText, colourToken, rowIcon, toneToken, typeIcon } from './lib/card'

  let {
    u,
    face,
    title,
    selected = false,
    status = '',
    result = '',
    colour,
    skipped = false,
    breakpoint = false,
    note = '',
    toolbar = false,
    ontool,
    oncolour,
    oncase,
    onrow,
    onport,
    ondataport,
  }: {
    u: Unit
    face: Face
    title: string
    selected?: boolean
    status?: string
    result?: string
    colour?: string // a colour the person gave the step
    skipped?: boolean // this run leaves the step out: the header is hatched
    note?: string // why the step has no value (it reads from a skipped one)
    toolbar?: boolean // the step is the one selected, in Edit: its toolbar floats above it
    breakpoint?: boolean // a debug session pauses before this step: a red dot in the header
    ontool?: (tool: 'add' | 'skip' | 'break' | 'duplicate' | 'delete', e: MouseEvent) => void
    oncolour?: (colour: string) => void // '' for the category's own
    oncase?: (index: number, e: MouseEvent) => void // a case's port label was clicked: edit its test in place
    onrow?: (key: string) => void
    onport?: (e: PointerEvent) => void // the pointer went down on the done port: a needs link starts
    ondataport?: (e: PointerEvent) => void // …on the data port: a value wire starts
  } = $props()

  const w = $derived(u.headW || u.w)
  const h = $derived(u.headH || u.h)
  const type = $derived(u.types[0] ?? '')
  const tile = $derived(colourToken(type, colour))
  const badgeW = $derived(face.badge ? 10 + face.badge.length * 7.4 : 0)
  const subX = $derived(14 + (face.badge ? badgeW + 8 : 0))
  const subChars = $derived(Math.max(8, Math.floor((w - subX - 14) / 7.2)))
  const NAMES: Record<string, string> = {
    inputs: 'Inputs', send: 'Message', receive: 'Receive', connection: 'Connection', hooks: 'Hooks',
    settings: 'Settings', values: 'Values', checks: 'Checks', steps: 'Steps', message: 'Message',
  }
  const nameOf = (key: string) => NAMES[key] ?? key
  const rowsTop = CARD.header + CARD.subline

  // The toolbar floats above the card (above the note, when there is one); the swatches open beside it.
  const TOOLS = [
    { id: 'colour', icon: 'palette', label: 'Colour' },
    { id: 'add', icon: 'plus', label: 'Add a step after this one' },
    { id: 'skip', icon: 'skip', label: 'Skip this step in the next runs' },
    { id: 'break', icon: 'breakpoint', label: 'Pause before this step when debugging' },
    { id: 'duplicate', icon: 'copy', label: 'Duplicate this step' },
    { id: 'delete', icon: 'trash', label: 'Delete this step' },
  ] as const
  const BTN = 32
  const barTop = $derived(note ? -80 : -48)
  let swatches = $state(false)
  $effect(() => {
    if (!toolbar) swatches = false
  })
  function tool(id: (typeof TOOLS)[number]['id'], e: MouseEvent) {
    if (id === 'colour') swatches = !swatches
    else ontool?.(id, e)
  }
</script>

<g class="stepcard {status}" class:selected class:skipped role="group" aria-label={`${title}, ${type}`}>
  <rect class="body" width={w} height={h} rx="14" />

  {#if skipped}<path class="hatch" d={`M0,${h > CARD.header ? CARD.header : h} V14 a14,14 0 0 1 14,-14 H${w - 14} a14,14 0 0 1 14,14 V${CARD.header} Z`} />{/if}
  <rect class="tile" x="12" y="9" width="28" height="28" rx="8" style:fill={`var(${tile})`} />
  <g class="tileicon" transform="translate(16 13)" style:color={colour ? 'var(--on-tile)' : 'var(--text)'}><Icon name={typeIcon(type)} size={20} /></g>
  <text class="title" x="50" y="28">{clipText(title, Math.floor((w - 50 - 30) / 7.6))}<title>{title}</title></text>
  {#if status}<circle class="dot {status}" cx={w - 16} cy="23" r="5" />{/if}

  <g transform={`translate(0 ${CARD.header})`}>
    {#if face.badge}
      <rect class="badge" x="14" y="4" width={badgeW} height="19" rx="5" />
      <text class="badgetext" x={14 + badgeW / 2} y="17.5" text-anchor="middle">{face.badge}</text>
    {/if}
    <text class="subtext" x={subX} y="18">{clipText(face.subline, subChars)}<title>{face.subline}</title></text>
  </g>

  {#each face.rows as row, i (row.key + i)}
    {@const y = rowsTop + i * CARD.row}
    <g class="row" role="button" tabindex="-1" aria-label={`${nameOf(row.key)}: ${row.summary}`}
      onpointerdown={(e) => e.stopPropagation()} onclick={() => onrow?.(row.key)} onkeydown={() => {}}>
      <title>{`${nameOf(row.key)}: ${row.summary}`}</title>
      <rect class="rowhit" x="6" {y} width={w - 12} height={CARD.row} rx="8" />
      <g class="rowicon" transform={`translate(16 ${y + 9})`}><Icon name={rowIcon(row.key)} size={16} /></g>
      <text class="rowtext" x="42" y={y + 22}>{clipText(row.summary, Math.floor((w - 42 - 34) / 7.2))}</text>
      <path class="chev" d={`M${w - 26},${y + 13} l5,4 l-5,4`} />
    </g>
  {/each}

  {#each face.ports as p, i (p.id)}
    {@const py = CARD.inPort + i * CARD.portGap}
    <circle class="port {p.tone}" cx={w} cy={py} r="6" style:stroke={`var(${toneToken(p.tone)})`}
      role="button" tabindex="-1" aria-label={p.label} onpointerdown={(e) => p.id === 'done' && onport?.(e)}>
      <title>{p.id === 'done' ? 'Drag onto another step to make it wait for this one' : p.label}</title>
    </circle>
    {#if p.tone === 'case' && oncase}
      <text class="portlabel edit" x={w + 12} y={py + 4} style:fill={`var(${toneToken(p.tone)})`} role="button" tabindex="-1" aria-label={`Edit the test ${p.label}`}
        onpointerdown={(e) => e.stopPropagation()} onclick={(e) => oncase(Number(p.id.replace('case-', '')), e)} onkeydown={() => {}}>{clipText(p.label, 22)}<title>Edit this test</title></text>
    {:else if p.tone !== 'done'}<text class="portlabel" x={w + 12} y={py + 4} style:fill={`var(${toneToken(p.tone)})`}>{clipText(p.label, 22)}</text>{/if}
  {/each}

  <!-- The data port: a square at the foot of the card, seen on hover, while selected, or while a wire is being dragged. -->
  {#if ondataport}
    <rect class="dataport" x={w / 2 - 6} y={h - 6} width="12" height="12" rx="3" role="button" tabindex="-1" aria-label="Drag onto another step to pass a value to it"
      onpointerdown={(e) => ondataport(e)}><title>Drag onto another step to pass a value to it</title></rect>
  {/if}

  {#if note}
    <g class="note"><rect x="10" y="-34" width={w - 20} height="26" rx="8" /><text x="20" y="-17">{clipText(note, Math.floor((w - 40) / 6.2))}<title>{note}</title></text></g>
  {/if}
  {#if toolbar}
    <g class="toolbar" transform={`translate(0 ${barTop})`} role="toolbar" aria-label="Step tools">
      <rect class="bar" width={TOOLS.length * BTN + 12} height="40" rx="12" />
      {#each TOOLS as t, i (t.id)}
        <g class="tool" class:on={(t.id === 'skip' && skipped) || (t.id === 'break' && breakpoint) || (t.id === 'colour' && swatches)} role="button" tabindex="-1" aria-label={t.label}
          onpointerdown={(e) => e.stopPropagation()} onclick={(e) => tool(t.id, e)} onkeydown={() => {}}>
          <title>{t.label}</title>
          <rect x={6 + i * BTN} y="4" width={BTN} height="32" rx="8" />
          <g class="toolicon" transform={`translate(${6 + i * BTN + 8} 12)`}><Icon name={t.icon} size={16} /></g>
        </g>
      {/each}
      {#if swatches}
        <g class="swatches" transform={`translate(${TOOLS.length * BTN + 20} 0)`}>
          <rect class="bar" width={(COLOUR_NAMES.length + 1) * 28 + 12} height="40" rx="12" />
          <g role="button" tabindex="-1" aria-label="The category's own colour" onpointerdown={(e) => e.stopPropagation()} onclick={() => oncolour?.('')} onkeydown={() => {}}>
            <title>The category's own colour</title>
            <circle class="swatch none" class:chosen={!colour} cx="26" cy="20" r="9" style:stroke={`var(${tile})`} />
          </g>
          {#each COLOUR_NAMES as c, i (c)}
            <g role="button" tabindex="-1" aria-label={c} onpointerdown={(e) => e.stopPropagation()} onclick={() => oncolour?.(c)} onkeydown={() => {}}>
              <title>{c}</title>
              <circle class="swatch" class:chosen={colour === c} cx={26 + (i + 1) * 28} cy="20" r="9" style:fill={`var(--c-${c})`} />
            </g>
          {/each}
        </g>
      {/if}
    </g>
  {/if}
  {#if breakpoint}<circle class="bpdot" cx="-5" cy={CARD.header / 2} r="6"><title>A debug run pauses before this step</title></circle>{/if}
  {#if skipped}<text class="skiptag" x={w - 36} y="28" text-anchor="end">Skipped</text>{/if}

  {#if result}<text class="result {status}" x={w - 12} y={h - 9} text-anchor="end">{result}</text>{/if}
</g>

<style>
  .body { fill: var(--panel); stroke: var(--line); stroke-width: 1.2; filter: drop-shadow(0 2px 8px rgb(0 0 0 / 0.28)); transition: stroke 0.15s; }
  .selected .body { stroke: var(--accent); stroke-width: 2.2; }
  .passed .body { stroke: var(--ok); }
  .failed .body, .error .body, .aborted .body { stroke: var(--fail); stroke-width: 2.2; }
  .running .body { stroke: var(--accent); }
  .hatch { fill: url(#skiphatch); pointer-events: none; }
  .skipped .body { stroke-dasharray: 5 4; }
  .bpdot { fill: var(--fail); stroke: var(--bg); stroke-width: 2; pointer-events: none; }
  .skiptag { fill: var(--muted); font-size: 12px; font-weight: 600; pointer-events: none; }
  .note rect { fill: color-mix(in srgb, var(--warn, var(--fail)) 18%, var(--panel)); stroke: var(--warn, var(--fail)); stroke-width: 1; }
  .note text { fill: var(--text); font-size: 12.5px; pointer-events: none; }
  .bar { fill: var(--panel); stroke: var(--line); stroke-width: 1.2; filter: drop-shadow(0 2px 8px rgb(0 0 0 / 0.28)); }
  .tool { cursor: pointer; }
  .tool rect { fill: transparent; }
  .tool:hover rect, .tool.on rect { fill: var(--tint-hover); }
  .toolicon { color: var(--text); pointer-events: none; }
  .swatch { cursor: pointer; stroke: transparent; stroke-width: 2.4; }
  .swatch.none { fill: transparent; stroke-width: 2; }
  .swatch.chosen { stroke: var(--text); }
  .tile { stroke: none; }
  .tileicon { pointer-events: none; }
  .title { fill: var(--text); font-size: 14px; font-weight: 600; pointer-events: none; }
  .badge { fill: color-mix(in srgb, var(--accent) 24%, transparent); }
  .badgetext { fill: var(--accent); font-size: 11px; font-weight: 700; letter-spacing: 0.04em; pointer-events: none; }
  .subtext { fill: var(--muted); font-size: 13px; pointer-events: none; }
  .row { cursor: pointer; }
  .rowhit { fill: transparent; }
  .row:hover .rowhit { fill: var(--tint-hover); }
  .rowicon { color: var(--muted); pointer-events: none; }
  .rowtext { fill: var(--text); font-size: 13.5px; pointer-events: none; }
  .chev { fill: none; stroke: var(--muted); stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; opacity: 0; pointer-events: none; }
  .row:hover .chev { opacity: 1; }
  .port { fill: var(--bg); stroke-width: 2; cursor: crosshair; }
  .port:hover { fill: var(--tint-sel); }
  .dataport { fill: var(--bg); stroke: var(--data); stroke-width: 2; cursor: crosshair; opacity: 0; transition: opacity 0.15s; }
  .stepcard:hover .dataport, .selected .dataport { opacity: 1; }
  .dataport:hover { fill: var(--data); }
  .portlabel { font-size: 12px; font-weight: 600; pointer-events: none; }
  .portlabel.edit { pointer-events: auto; cursor: text; }
  .portlabel.edit:hover { text-decoration: underline; }
  .dot { fill: var(--muted); }
  .dot.passed { fill: var(--ok); }
  .dot.failed, .dot.error, .dot.aborted { fill: var(--fail); }
  .dot.running { fill: var(--accent); }
  .result { fill: var(--muted); font-size: 12px; pointer-events: none; }
  .result.failed, .result.error { fill: var(--fail); }
</style>
