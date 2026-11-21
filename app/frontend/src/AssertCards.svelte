<script lang="ts">
  import IconButton from './IconButton.svelte'
  // A request's checks as cards, one per check, each ticked or crossed by the latest send, with a bulk
  // edit for pasting a list. Like KVRows, it keeps what is being typed while Go round-trips the edit.
  import { clamp } from './lib/clamp'
  import { tick } from 'svelte'
  import Button from './Button.svelte'
  import EmptyState from './EmptyState.svelte'
  import TemplateField from './TemplateField.svelte'
  import type { Item } from './lib/exprcomplete'
  import { checkNotes, checkStates, triedStates, type CheckOutcome, type CheckTry } from './lib/checks'
  import { debounced } from './lib/debounce'
  import type { Starter } from './lib/assertstarters'

  let {
    checks,
    outcome,
    onchange,
    example,
    vocabulary,
    starters = [],
    evaluate,
  }: {
    checks: string[]
    outcome?: CheckOutcome
    onchange: (checks: string[]) => void
    /** A sample check shown as the placeholder. */
    example: string
    /** What a check can name, for completing the word being typed. */
    vocabulary?: () => Promise<Item[]>
    /** Checks the latest response suggests; a click adds one. */
    starters?: Starter[]
    /** Tries checks on the response on screen, with no send; without it a new check stays untried until the next send. */
    evaluate?: (lines: string[]) => Promise<{ at: string; checks: CheckTry[] }>
  } = $props()

  let local = $state<string[]>([])
  let within = $state(false)
  $effect(() => {
    if (!within) local = [...checks]
  })

  let box = $state<HTMLDivElement>()
  // Focus moving between this form's own controls is not leaving it (see KVRows).
  function leave(e: FocusEvent) {
    if (!box?.contains(e.relatedTarget as Node | null)) within = false
  }

  const sentStates = $derived(checkStates(local, outcome))
  // The checks the send did not judge (added or edited since) are tried on the response on screen as they are typed.
  let tried = $state<CheckTry[]>([])
  let triedAt = $state('')
  let ask = 0
  const tryThem = debounced(async (lines: string[]) => {
    const mine = ++ask
    try {
      const r = await evaluate!(lines)
      if (mine === ask) [tried, triedAt] = [r.checks, r.at]
    } catch {
      if (mine === ask) [tried, triedAt] = [[], '']
    }
  }, 250)
  $effect(() => {
    if (!evaluate) return
    const untried = local.filter((c, i) => c.trim() !== '' && sentStates[i] === 'idle')
    if (untried.length) tryThem(untried)
    else {
      tryThem.cancel()
      ask++
      ;[tried, triedAt] = [[], '']
    }
  })
  const shown = $derived(triedStates(local, sentStates, tried))
  const states = $derived(shown.map((x) => x.state))
  const mark = { idle: '○', passed: '✓', failed: '✗', error: '!' }
  const hint = { idle: 'Not run yet', passed: 'Passed', failed: 'Failed', error: 'Could not be evaluated: see the notes on the response' }

  function commit() {
    onchange(local.filter((c) => c.trim() !== ''))
  }

  async function add() {
    local.push('')
    await tick()
    const inputs = box?.querySelectorAll<HTMLInputElement>('.c input')
    inputs?.[inputs.length - 1]?.focus()
  }

  // The suggestions not already among the checks.
  const offered = $derived(starters.filter((s) => !local.includes(s.line)))
  function addStarter(line: string) {
    local = [...local.filter((c) => c.trim() !== ''), line]
    commit()
  }

  let bulk = $state(false)
  let bulkText = $state('')
  async function startBulk() {
    bulkText = local.filter((c) => c.trim() !== '').join('\n')
    bulk = true
    await tick()
    box?.querySelector<HTMLTextAreaElement>('textarea')?.focus()
  }
  function bulkInput(text: string) {
    bulkText = text
    onchange(text.split('\n').filter((l) => l.trim() !== ''))
  }
</script>

{#if offered.length && !bulk}
  <div class="starters" role="group" aria-label="Checks from the last response">
    <span class="muted">From the last response:</span>
    {#each offered as s (s.line)}<Button small title={`Add ${s.line}`} onclick={() => addStarter(s.line)}>+ {s.label}</Button>{/each}
  </div>
{/if}
{#if !local.length && !bulk}
  <EmptyState icon="assert" title="No checks" hint="A failing check fails the send." compact>
    <Button onclick={add}>Add check</Button><Button onclick={startBulk}>Bulk edit</Button>
  </EmptyState>
{/if}
<div class="cards" bind:this={box} hidden={!local.length && !bulk} onfocusin={() => (within = true)} onfocusout={leave}>
  {#if bulk}
    <textarea
      rows={clamp(bulkText.split('\n').length + 1, 6, 24)}
      value={bulkText}
      placeholder={example}
      spellcheck="false"
      aria-label="Checks, one per line"
      oninput={(e) => bulkInput(e.currentTarget.value)}
    ></textarea>
    <div class="tools">
      <span class="hint">One check per line</span>
      <Button variant="link" onclick={() => (bulk = false)}>Card edit</Button>
    </div>
  {:else}
    {#each local as check, i}
      {@const why = shown[i].tried ? `${hint[states[i]]} on the response from ${triedAt} (not sent again)` : hint[states[i]]}
      <div class="card {states[i]}">
        <span class="tick" title={why} aria-label={why}>{mark[states[i]]}</span>
        <div class="c"><TemplateField plain {vocabulary} placeholder={example} aria="Check {i + 1}" value={local[i]} oninput={(t) => { local[i] = t; commit() }} /></div>
        <IconButton icon="close" size={12} danger label="Remove check" onclick={() => { local.splice(i, 1); commit() }} />
      </div>
      {#if shown[i].tried && shown[i].note}
        <p class="reason" role="note">Saw {shown[i].note}</p>
      {:else if states[i] === 'failed' || states[i] === 'error'}
        {#each checkNotes(check, outcome) as note}<p class="reason" role="note">{note}</p>{/each}
      {/if}
      {#if shown[i].note && !shown[i].tried}
        <p class="reason soft" role="note">Needs a send: {shown[i].note}</p>
      {/if}
    {/each}
    <div class="tools">
      <Button variant="link" onclick={add}>+ Add</Button>
      <Button variant="link" onclick={startBulk}>Bulk edit</Button>
    </div>
  {/if}
</div>

<style>
  .starters { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; margin: 0 0 8px; }
  .cards[hidden] { display: none; }
  .cards { display: flex; flex-direction: column; gap: 6px; }
  .card {
    display: flex; align-items: center; gap: 8px; padding: 6px 8px;
    background: var(--bg); border: 1px solid var(--line); border-left-width: 3px; border-radius: var(--r-md);
  }
  .reason { margin: -2px 0 2px 27px; font-size: var(--fs-xs); color: var(--fail); overflow-wrap: anywhere; -webkit-user-select: text; user-select: text; }
  .reason.soft { color: var(--muted); }
  .card.passed { border-left-color: var(--ok); }
  .card.failed, .card.error { border-left-color: var(--fail); }
  .tick { flex: none; width: 16px; text-align: center; color: var(--muted); }
  .passed .tick { color: var(--ok); }
  .failed .tick, .error .tick { color: var(--fail); }
  .c { flex: 1; min-width: 0; }
  .c :global(input) { outline: 0; font: var(--fs-sm) ui-monospace, SFMono-Regular, Menlo, monospace; }
  .card:focus-within { border-color: var(--accent); }
  .x:hover { color: var(--fail); }
  .tools { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .hint { color: var(--muted); font-size: var(--fs-xs); }
  textarea {
    resize: vertical; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md);
    padding: 6px 8px; font: var(--fs-sm)/1.5 ui-monospace, SFMono-Regular, Menlo, monospace; -webkit-user-select: text; user-select: text;
  }
  textarea:focus { border-color: var(--accent); }
</style>
