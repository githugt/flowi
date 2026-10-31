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
  function move(i: number, by: number) {
    const j = i + by
    if (j < 0 || j >= local.length) return
    ;[local[i], local[j]] = [local[j], local[i]]
    commit()
  }
  function remove(i: number) {
    local.splice(i, 1)
    commit()
  }
  // A new outcome goes first: it is what is looked for before the general success and failure.
  async function add() {
    local.unshift({ Name: '', When: '', Disabled: false, Color: 'sky', Description: '' })
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
    <div class="card" class:off={r.Disabled} style:--tone={tone(r)}>
      <div class="head">
        <span class="order">
          <button class="mv" aria-label="Move up" title="Move up" disabled={i === 0} onclick={() => move(i, -1)}>▲</button>
          <button class="mv" aria-label="Move down" title="Move down" disabled={i === local.length - 1} onclick={() => move(i, 1)}>▼</button>
        </span>
        <label class="on" title={r.Disabled ? 'Switched off: it never matches' : 'Switched on'}>
          <input type="checkbox" checked={!r.Disabled} onchange={(e) => { r.Disabled = !e.currentTarget.checked; commit() }} />
        </label>
        {#if builtin}
          <span class="name built" style:color={tone(r)}>{r.Name}</span>
        {:else}
          <input class="name" class:bad={!!problem} placeholder="name" bind:value={r.Name} oninput={() => !nameProblem(r, i) && commit()} spellcheck="false" aria-label="Outcome name" title={problem} />
        {/if}
        <span class="grow"></span>
        <button class="x" aria-label={`Remove ${r.Name || 'outcome'}`} title={local.length === 1 ? 'A request has at least one outcome' : 'Remove'} disabled={local.length === 1} onclick={() => remove(i)}>×</button>
      </div>
      {#if builtin}
        <p class="said">{said[r.Name]}</p>
      {:else}
        <input class="rule" placeholder={example} bind:value={r.When} oninput={() => commit()} spellcheck="false" aria-label={`Rule of ${r.Name || 'the outcome'}`} title={r.When} />
      {/if}
      <input class="desc" placeholder="What this outcome means (optional)" bind:value={r.Description} oninput={() => commit()} aria-label={`Description of ${r.Name || 'the outcome'}`} />
      <div class="swatches" role="radiogroup" aria-label={`Colour of ${r.Name || 'the outcome'}`}>
        {#each COLOUR_NAMES as c (c)}
          <button class="swatch" class:chosen={colourOf(r) === c} style:background={`var(--c-${c})`} role="radio" aria-checked={colourOf(r) === c} aria-label={c} title={c} onclick={() => { r.Color = c; commit() }}></button>
        {/each}
      </div>
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
  .muted { color: var(--muted); font-size: 13px; margin: 0 0 4px; line-height: 1.45; }
  .small { font-size: 12px; margin-top: 6px; }
  .hint { color: var(--muted); font-size: 12px; }
  .card { border: 1px solid var(--line); border-left: 4px solid var(--tone); border-radius: 8px; background: var(--panel); padding: 6px 8px 8px; display: flex; flex-direction: column; gap: 6px; }
  .head { display: flex; align-items: center; gap: 8px; }
  .grow { flex: 1; }
  .card.off .name, .card.off .rule, .card.off .desc, .card.off .said { opacity: 0.45; }
  .order { display: flex; flex-direction: column; }
  .mv { all: unset; cursor: pointer; font-size: 8px; line-height: 1; padding: 1px 3px; color: var(--muted); border-radius: 3px; }
  .mv:hover:not(:disabled) { color: var(--text); background: var(--tint-hover); }
  .mv:disabled { opacity: 0.25; cursor: default; }
  .on { display: flex; align-items: center; }
  input.name, input.rule, input.desc { min-width: 0; width: 100%; box-sizing: border-box; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: 6px; padding: 5px 8px; font-size: 13px; }
  input.name { width: auto; flex: 0 1 180px; }
  input.name, input.rule { font-family: var(--mono, ui-monospace, monospace); }
  input.desc { font-size: 12.5px; }
  input.bad { border-color: var(--fail); }
  .built { font-weight: 600; padding: 3px 8px; border-radius: 6px; background: color-mix(in srgb, var(--tone) 16%, transparent); }
  .said { color: var(--muted); font-size: 12.5px; margin: 0; }
  .swatches { display: flex; gap: 7px; flex-wrap: wrap; align-items: center; }
  .swatch { all: unset; box-sizing: border-box; width: 18px; height: 18px; border-radius: 50%; cursor: pointer; border: 2px solid transparent; }
  .swatch:hover { transform: scale(1.12); }
  .swatch.chosen { border-color: var(--text); }
  .x { all: unset; cursor: pointer; padding: 0 6px; color: var(--muted); font-size: 16px; }
  .x:hover:not(:disabled) { color: var(--fail); }
  .x:disabled { opacity: 0.3; cursor: default; }
  .problem { margin: 0; color: var(--fail); font-size: 12px; }
  .actions { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; margin-top: 2px; }
  .actions button { display: inline-flex; gap: 5px; align-items: center; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: 8px; padding: 5px 10px; cursor: pointer; font-size: 13px; }
  .actions button:hover { background: var(--tint-hover); }
  .actions .quiet { color: var(--muted); border-style: dashed; }
</style>
