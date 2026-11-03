<script lang="ts">
  // The named ways a request can end, in order. success and failure are built in (the request and its hooks
  // passed, or failed); any other row is a name and a rule over the response. A send has every outcome whose
  // rule holds, and the arrows on the canvas follow the outcomes they name. Rows can be reordered, switched off
  // or removed, the built-in ones too: without success, nothing runs after the request on success. Each can
  // have a colour (on the card's port and its arrows) and a description. New outcomes go on top.
  import type { Snippet } from 'svelte'
  import { tick } from 'svelte'
  import ExprHelp from './ExprHelp.svelte'
  import Icon from './Icon.svelte'
  import { COLOUR_NAMES, outcomeColour } from './lib/card'

  interface Row {
    Name: string
    When: string
    Disabled: boolean
    Color: string
    Description: string
  }

  let {
    outcomes,
    onchange,
    ruleKind = 'http',
    resetLabel = 'Reset to defaults',
    hideReset = false,
    footer,
  }: {
    outcomes: Partial<Row>[] | null | undefined
    onchange: (rows: Row[]) => void
    /** The kind of request the rules are about (http, message or dataset): what the rule can read, and its help. */
    ruleKind?: string
    /** What the reset button says, and whether it is hidden: in a flow it takes the step's own list away. */
    resetLabel?: string
    hideReset?: boolean
    /** Extra actions under the list (the flow's: use the request's, save to the request). */
    footer?: Snippet
  } = $props()

  const DEFAULTS: Row[] = [
    { Name: 'success', When: '', Disabled: false, Color: 'green', Description: '' },
    { Name: 'failure', When: '', Disabled: false, Color: 'red', Description: '' },
  ]
  const BUILTIN = ['success', 'failure']
  const said = {
    success: 'The request and its hooks passed.',
    failure: 'The request, a hook or a check failed, or it could not be sent.',
  } as Record<string, string>
  const helpContext = $derived(ruleKind === 'dataset' ? 'outcome.rule.dataset' : ruleKind === 'message' ? 'outcome.rule.message' : 'outcome.rule')
  const example = $derived(ruleKind === 'dataset' ? 'row_count == 0' : ruleKind === 'message' ? 'received == 0' : 'response.status == 404')
  const second = $derived(ruleKind === 'dataset' ? 'row_count not in [1, 2]' : ruleKind === 'message' ? 'outcome in ["timeout", "error"]' : 'status in [500, 503]')

  const written = $derived(!!outcomes?.length)
  let local = $state<Row[]>([])
  let within = $state(false)
  $effect(() => {
    if (!within) local = (outcomes?.length ? outcomes : DEFAULTS).map((r) => ({ Name: r.Name ?? '', When: r.When ?? '', Disabled: !!r.Disabled, Color: r.Color ?? '', Description: r.Description ?? '' }))
  })
  let box = $state<HTMLDivElement>()
  function leave(e: FocusEvent) {
    if (!box?.contains(e.relatedTarget as Node | null)) within = false
  }

  const clean = (r: Row): Row => ({ ...r, Color: r.Color || '', Description: r.Description || '' })
  const commit = () => onchange(local.map(clean))
  // The row whose description and colour are showing as fields; the others show them as text.
  let opened = $state<number | null>(null)
  function reorder(from: number, to: number) {
    if (to < 0 || to >= local.length || from === to) return
    const [row] = local.splice(from, 1)
    local.splice(to, 0, row)
    if (opened === from) opened = to
    else if (opened !== null && from < opened && to >= opened) opened--
    else if (opened !== null && from > opened && to <= opened) opened++
  }
  function move(i: number, by: number) {
    reorder(i, i + by)
    commit()
  }
  // Dragging the grip moves the row live under the pointer, and the new order is written when it is let go.
  // Pointer events rather than HTML5 drag and drop, which the window's web view does not do reliably.
  let dragging = $state<number | null>(null)
  function grab(e: PointerEvent, i: number) {
    if (e.button !== 0) return
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
    dragging = i
  }
  function drag(e: PointerEvent) {
    if (dragging === null || !box) return
    const cards = [...box.querySelectorAll<HTMLElement>('.card')]
    const over = cards.findIndex((c) => e.clientY >= c.getBoundingClientRect().top && e.clientY <= c.getBoundingClientRect().bottom)
    if (over < 0 || over === dragging) return
    reorder(dragging, over)
    dragging = over
  }
  function drop() {
    if (dragging === null) return
    dragging = null
    commit()
  }
  // The keyboard way: arrows on the grip move the row.
  function gripKeys(e: KeyboardEvent, i: number) {
    if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return
    e.preventDefault()
    move(i, e.key === 'ArrowUp' ? -1 : 1)
    tick().then(() => box?.querySelectorAll<HTMLElement>('.grip')[i + (e.key === 'ArrowUp' ? -1 : 1)]?.focus())
  }
  function remove(i: number) {
    local.splice(i, 1)
    if (opened === i) opened = null
    else if (opened !== null && i < opened) opened--
    commit()
  }
  // A new outcome goes first: it is what is looked for before the general success and failure.
  async function add() {
    local.unshift({ Name: '', When: '', Disabled: false, Color: 'sky', Description: '' })
    // A new outcome shows its description and colour, which the owner is likely to set straight away; the rest stay folded.
    opened = 0
    await tick()
    box?.querySelector<HTMLInputElement>('input.name')?.focus()
  }
  function addBuiltin(name: string) {
    local.push({ Name: name, When: '', Disabled: false, Color: outcomeColour(name), Description: '' })
    commit()
  }
  const missing = $derived(BUILTIN.filter((b) => !local.some((r) => r.Name === b)))
  const nameProblem = (r: Row, i: number) =>
    r.Name === '' ? 'Give it a name' : !/^[A-Za-z0-9_][A-Za-z0-9_-]*$/.test(r.Name) ? 'Letters, digits, - and _' : local.findIndex((x) => x.Name === r.Name) !== i ? 'Already used' : ''
  const colourOf = (r: Row) => outcomeColour(r.Name, r.Color)
  const tone = (r: Row) => `var(--c-${colourOf(r)})`
</script>

<div bind:this={box} class="outcomes" onfocusin={() => (within = true)} onfocusout={leave} role="group" aria-label="Outcomes">
  <p class="muted">
    Ways this request can end. A send has <strong>every</strong> outcome whose rule holds, so a 404 can be both <code>failure</code> and <code>not-found</code>. The arrows on the flow canvas follow the outcomes they name; the order here is the order of the ports on the card.
    {#if !written}Success and failure are the defaults; change anything and the list is written down.{/if}
  </p>
  <div class="actions top">
    <button onclick={add}><Icon name="plus" size={13} /> Add outcome</button>
    <ExprHelp context={helpContext} />
    <span class="hint">New outcomes go first, above the defaults.</span>
  </div>
  {#each local as r, i (i)}
    {@const builtin = BUILTIN.includes(r.Name)}
    {@const problem = nameProblem(r, i)}
    <div class="card" class:off={r.Disabled} class:lifted={dragging === i} style:--tone={tone(r)}>
      <div class="head">
        <button class="grip" aria-label={`Move ${r.Name || 'outcome'}`} title="Drag to reorder (or use the up and down arrows)" onpointerdown={(e) => grab(e, i)} onpointermove={drag} onpointerup={drop} onpointercancel={drop} onkeydown={(e) => gripKeys(e, i)}>
          <svg width="10" height="16" viewBox="0 0 10 16" aria-hidden="true">{#each [3, 8, 13] as y}<circle cx="2.5" cy={y} r="1.3" /><circle cx="7.5" cy={y} r="1.3" />{/each}</svg>
        </button>
        <label class="on" title={r.Disabled ? 'Switched off: it never matches' : 'Switched on'}>
          <input type="checkbox" checked={!r.Disabled} onchange={(e) => { r.Disabled = !e.currentTarget.checked; commit() }} />
        </label>
        {#if builtin}
          <span class="name built" style:color={tone(r)}>{r.Name}</span>
        {:else}
          <input class="name" style:width={`${Math.max(r.Name.length, 4) + 3}ch`} class:bad={!!problem} placeholder="name" bind:value={r.Name} oninput={() => !nameProblem(r, i) && commit()} spellcheck="false" aria-label="Outcome name" title={problem} />
        {/if}
        <span class="grow"></span>
        <button class="x" aria-label={`Remove ${r.Name || 'outcome'}`} title={local.length === 1 ? 'A request has at least one outcome' : 'Remove'} disabled={local.length === 1} onclick={() => remove(i)}>×</button>
      </div>
      {#if builtin}
        <p class="said">{said[r.Name]}</p>
      {:else}
        <input class="rule" placeholder={example} bind:value={r.When} oninput={() => commit()} spellcheck="false" aria-label={`Rule of ${r.Name || 'the outcome'}`} title={r.When} />
      {/if}
      {#if opened === i}
        <input class="desc" placeholder="What this outcome means (optional)" bind:value={r.Description} oninput={() => commit()} aria-label={`Description of ${r.Name || 'the outcome'}`} />
        <div class="swatches" role="radiogroup" aria-label={`Colour of ${r.Name || 'the outcome'}`}>
          {#each COLOUR_NAMES as c (c)}
            <button class="swatch" class:chosen={colourOf(r) === c} style:background={`var(--c-${c})`} role="radio" aria-checked={colourOf(r) === c} aria-label={c} title={c} onclick={() => { r.Color = c; commit() }}></button>
          {/each}
        </div>
      {/if}
      <button class="more" aria-expanded={opened === i} onclick={() => (opened = opened === i ? null : i)}>
        <span class="chev" aria-hidden="true">{opened === i ? '▾' : '▸'}</span>
        {#if opened === i}Hide description and colour{:else if r.Description}<span class="said">{r.Description}</span>{:else}Description and colour{/if}
      </button>
      {#if problem}<p class="problem">{problem}</p>{/if}
    </div>
  {/each}
  <div class="actions">
    {#each missing as m (m)}<button class="quiet" onclick={() => addBuiltin(m)}>Add {m} back</button>{/each}
    {#if written && !hideReset}<button class="quiet" onclick={() => onchange([])}>{resetLabel}</button>{/if}
    {@render footer?.()}
  </div>
  <p class="muted small">A rule is a test written without braces, over what the request got: <code>{example}</code>, <code>{second}</code>. It sees the response after the hooks have run. Press <strong>?</strong> for what it can read and the operators, <code>in</code> and <code>not in</code> included.</p>
</div>

<style>
  .outcomes { display: flex; flex-direction: column; gap: 6px; }
  .muted { color: var(--muted); font-size: var(--fs-md); margin: 0 0 4px; line-height: 1.45; }
  .small { font-size: var(--fs-sm); margin-top: 6px; }
  .hint { color: var(--muted); font-size: var(--fs-sm); }
  .card { border: 1px solid var(--line); border-left: 4px solid var(--tone); border-radius: 8px; background: var(--panel); padding: 6px 8px 8px; display: flex; flex-direction: column; gap: 6px; }
  .head { display: flex; align-items: center; gap: 8px; }
  .grow { flex: 1; }
  .card.off .name, .card.off .rule, .card.off .desc, .card.off .said { opacity: 0.45; }
  .grip { all: unset; box-sizing: border-box; display: grid; place-items: center; padding: 4px 3px; border-radius: 4px; color: var(--muted); fill: currentColor; cursor: grab; touch-action: none; }
  .grip:hover { color: var(--text); background: var(--tint-hover); }
  .grip:focus-visible { outline: 2px solid var(--accent); }
  .card.lifted { box-shadow: 0 6px 18px rgb(0 0 0 / 0.35); }
  .card.lifted .grip { cursor: grabbing; }
  .more { all: unset; box-sizing: border-box; display: flex; align-items: center; gap: 6px; min-width: 0; cursor: pointer; color: var(--muted); font-size: var(--fs-sm); border-radius: 4px; padding: 1px 2px; }
  .more:hover { color: var(--text); }
  .more:focus-visible { outline: 2px solid var(--accent); }
  .more .said { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .chev { flex: none; width: 10px; font-size: 9px; }
  .on { display: flex; align-items: center; }
  input.name, input.rule, input.desc { min-width: 0; width: 100%; box-sizing: border-box; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: 6px; padding: 5px 8px; font-size: var(--fs-md); }
  /* The name is a pill in the outcome's colour, the same as the built-in ones, and grows with what is typed. */
  input.name { width: auto; max-width: 260px; font-weight: 600; color: var(--tone); background: color-mix(in srgb, var(--tone) 16%, transparent); border-color: transparent; border-radius: 6px; padding: 3px 8px; }
  input.name::placeholder { color: var(--muted); font-weight: 400; }
  input.name, input.rule { font-family: var(--mono, ui-monospace, monospace); }
  input.desc { font-size: var(--fs-sm); }
  input.bad { border-color: var(--fail) !important; }
  .built { font-weight: 600; padding: 3px 8px; border-radius: 6px; background: color-mix(in srgb, var(--tone) 16%, transparent); }
  .said { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .swatches { display: flex; gap: 7px; flex-wrap: wrap; align-items: center; }
  .swatch { all: unset; box-sizing: border-box; width: 18px; height: 18px; border-radius: 50%; cursor: pointer; border: 2px solid transparent; }
  .swatch:hover { transform: scale(1.12); }
  .swatch.chosen { border-color: var(--text); }
  .x { all: unset; cursor: pointer; padding: 0 6px; color: var(--muted); font-size: var(--fs-lg); }
  .x:hover:not(:disabled) { color: var(--fail); }
  .x:disabled { opacity: 0.3; cursor: default; }
  .problem { margin: 0; color: var(--fail); font-size: var(--fs-sm); }
  .actions { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; margin-top: 2px; }
  .actions button { display: inline-flex; gap: 5px; align-items: center; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: 8px; padding: 5px 10px; cursor: pointer; font-size: var(--fs-md); }
  .actions button:hover { background: var(--tint-hover); }
  .actions .quiet { color: var(--muted); border-style: dashed; }
</style>
