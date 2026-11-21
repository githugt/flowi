<script lang="ts">
  import { isEnter } from './lib/enterkey'
  import ExprHelp from './ExprHelp.svelte'
  import HiddenChars from './HiddenChars.svelte'
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { app } from './lib/app.svelte'
  import { canSet, chipAt, exprOfChip, type ChipBox } from './lib/chiphint'
  import type { VariableHint } from './lib/types'
  import { inTemplate, suggestFor, type Item } from './lib/exprcomplete'
  // A single-line input for text with {{templates}}: it colours the text when
  // Go can (spans), and offers what a template can be completed with.
  // The input keeps what the user types while Go round-trips the edit.
  interface Span {
    text: string
    part: string
    pair: number
  }
  interface Suggest {
    from: number
    to: number
    items: { insert: string; detail: string }[]
  }

  let {
    value,
    oninput,
    onenter,
    label = '',
    help = '',
    placeholder = '',
    mono = true,
    spans,
    suggest,
    vocabulary,
    plain = false,
    aria = '',
  }: {
    value: string
    oninput?: (text: string) => void
    onenter?: (text: string) => void
    label?: string
    help?: string // an ExprHelp context: shows a ? beside the label
    placeholder?: string
    mono?: boolean
    spans?: (text: string) => Promise<Span[]>
    suggest?: (text: string, caret: number) => Promise<Suggest>
    /** What a bare expression (no braces) can name; completes the word before the caret. Read on first use. */
    vocabulary?: () => Promise<Item[]>
    /** Without its own border and background, for a field that sits in a card. */
    plain?: boolean
    /** The accessible name when there is no visible label. */
    aria?: string
  } = $props()

  let local = $state('')
  let focused = $state(false)
  let input = $state<HTMLInputElement>()
  let scroll = $state(0)
  $effect(() => {
    if (!focused) local = value
  })

  // Coloured spans, trusted only while they are for the text shown.
  let coloured = $state<{ text: string; spans: Span[] } | null>(null)
  let token = 0
  $effect(() => {
    const text = local
    if (!spans || !text) {
      coloured = null
      return
    }
    const mine = ++token
    spans(text).then((s) => {
      if (mine === token) coloured = { text, spans: s }
    })
  })
  const inSync = $derived(coloured !== null && coloured.text === local)

  // ---- what a {{variable}} chip says when hovered: its value and where it comes from; a click on an unset one adds it
  // to the selected environment. The chips are drawn behind the input, so the mouse is matched to their boxes.
  let hl = $state<HTMLDivElement>()
  let wrap = $state<HTMLDivElement>()
  let tip = $state<{ hint: VariableHint; left: number } | null>(null)
  let tipKey = ''
  let hovered = 0
  function chipsDrawn(): ChipBox[] {
    if (!hl) return []
    return [...hl.querySelectorAll<HTMLElement>('.p-variable, .p-unknown-variable')].map((el) => {
      const r = el.getBoundingClientRect()
      return { left: r.left, right: r.right, top: r.top, bottom: r.bottom, text: el.textContent ?? '', unknown: el.classList.contains('p-unknown-variable') }
    })
  }
  async function overChip(e: MouseEvent) {
    const chip = inSync ? chipAt(chipsDrawn(), e.clientX, e.clientY) : undefined
    const key = chip ? `${chip.left}:${chip.text}` : ''
    if (key === tipKey) return
    tipKey = key
    const mine = ++hovered
    if (!chip) {
      tip = null
      return
    }
    const hint = (await RequestService.VariableHint(exprOfChip(chip.text))) as unknown as VariableHint
    if (mine !== hovered) return
    tip = { hint, left: Math.max(0, chip.left - (wrap?.getBoundingClientRect().left ?? 0)) }
  }
  function leaveChip() {
    tipKey = ''
    hovered++
    tip = null
  }
  function setFromChip() {
    if (tip && canSet(tip.hint)) void app.addEnvironmentRows(tip.hint.environment, [{ name: tip.hint.name, secret: tip.hint.secret }])
  }

  // ---- suggestions
  let open = $state<(Suggest & { bare: boolean }) | null>(null)
  let choice = $state(0)
  let asked = 0
  let words: Item[] | null = null // the vocabulary, fetched once the field is first used
  async function ask() {
    if (!input) return
    const mine = ++asked
    const caret = input.selectionStart ?? local.length
    const found = await suggestions(caret)
    if (mine !== asked) return
    open = found && found.items.length ? found : null
    choice = 0
  }

  // Inside {{ }} the template completion answers; elsewhere a field that reads a bare expression
  // completes the word before the caret.
  async function suggestions(caret: number): Promise<(Suggest & { bare: boolean }) | null> {
    // Go answers nothing outside an unclosed {{, so it is not asked: Before: every key typed in an address made a call.
    if (suggest && inTemplate(local, caret)) return { ...(await suggest(local, caret)), bare: false }
    if (!vocabulary) return null
    words ??= await vocabulary().catch(() => [])
    const s = suggestFor(local, caret, words)
    return s && { ...s, bare: true }
  }

  function apply(i: number) {
    if (!open) return
    const s = open
    const insert = s.items[i].insert
    const rest = local.slice(s.to)
    const closing = s.bare || rest.startsWith('}}') ? '' : '}}'
    const next = local.slice(0, s.from) + insert + closing + rest
    const caret = s.from + insert.length + (s.bare ? 0 : 2)
    local = next
    oninput?.(next)
    open = null
    queueMicrotask(() => {
      input?.setSelectionRange(caret, caret)
      // "body." and "len(" are the start of a longer word: keep offering what follows.
      if (s.bare && /[.(]$/.test(insert)) ask()
    })
  }

  function onkeydown(e: KeyboardEvent) {
    if (open) {
      if (e.key === 'ArrowDown') (e.preventDefault(), (choice = (choice + 1) % open.items.length))
      else if (e.key === 'ArrowUp') (e.preventDefault(), (choice = (choice - 1 + open.items.length) % open.items.length))
      else if (isEnter(e) || e.key === 'Tab') (e.preventDefault(), apply(choice))
      else if (e.key === 'Escape') (e.stopPropagation(), (open = null))
      return
    }
    if (isEnter(e)) onenter?.(local)
  }

  // The pair colours cycle, as in the URL bar of the other window.
  // Theme colours, so each query pair stays readable in light themes too.
  const PAIRS = ['var(--accent)', 'var(--warn)', 'var(--ok)', 'var(--literal)', 'var(--info)', 'var(--fail)']
  const colour = (s: Span) => (s.pair >= 0 && ['key', 'value', 'equals', 'variable', 'unknown-variable'].includes(s.part) ? PAIRS[s.pair % PAIRS.length] : undefined)
</script>

<label class="field">
  {#if label}<span class="label">{label}{#if help}<ExprHelp context={help} />{/if}</span>{/if}
  <div class="wrap" bind:this={wrap}>
    {#if inSync}
      <div class="hl" bind:this={hl} class:mono aria-hidden="true" style:transform={`translateX(${-scroll}px)`}>{#each coloured!.spans as s}<span class="p-{s.part}" style:color={colour(s)}>{s.text}</span>{/each}</div>
    {/if}
    <input
      bind:this={input}
      class:mono
      class:plain
      class:hide={inSync}
      class:setchip={tip && canSet(tip.hint)}
      value={local}
      {placeholder}
      aria-label={aria || undefined}
      spellcheck="false"
      autocomplete="off"
      onfocus={() => ((focused = true), (words = null))}
      onblur={() => ((focused = false), (open = null))}
      oninput={(e) => {
        local = e.currentTarget.value
        oninput?.(local)
        ask()
      }}
      onscroll={(e) => (scroll = e.currentTarget.scrollLeft)}
      onkeyup={(e) => ['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key) && ask()}
      onclick={() => (setFromChip(), ask())}
      onmousemove={overChip}
      onmouseleave={leaveChip}
      {onkeydown}
    />
    {#if tip}
      <div class="tip" role="tooltip" class:unset={!tip.hint.known} style:left={`${tip.left}px`}>{tip.hint.text}</div>
    {/if}
    <HiddenChars value={local} onclean={(t) => { local = t; oninput?.(t) }} />
    {#if open}
      <ul class="menu" role="listbox">
        {#each open.items as it, i}
          <li role="option" aria-selected={i === choice} class:on={i === choice}>
            <button type="button" onpointerdown={(e) => (e.preventDefault(), apply(i))}>
              <code>{it.insert}</code><span>{it.detail}</span>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</label>

<style>
  .field { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
  .label { color: var(--muted); font-size: var(--fs-xs); }
  .wrap { position: relative; }
  input {
    width: 100%;
    box-sizing: border-box;
    background: var(--bg);
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    padding: 5px 8px;
    font: inherit;
    -webkit-user-select: text;
    user-select: text;
    position: relative;
  }
  input:focus { border-color: var(--accent); }
  input.plain { background: transparent; border: 0; padding: 2px 0; }
  .mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: var(--fs-sm); }
  /* With colours drawn behind, the input's own text is clear but its caret stays. */
  .hide { background: transparent; color: transparent; caret-color: var(--text); }
  /* Selected text is drawn by the input itself: WebKit's selection is opaque and covered the colours behind, so a
     selected address looked empty. */
  .hide::selection { color: var(--text); background: color-mix(in srgb, var(--accent) 35%, var(--bg)); }
  .wrap:has(.hide)::before { content: ''; position: absolute; inset: 0; background: var(--bg); border-radius: var(--r-md); }
  .hl {
    position: absolute;
    inset: 0;
    padding: 6px 9px;
    white-space: pre;
    overflow: hidden;
    pointer-events: none;
    z-index: 1;
    line-height: normal;
    font: inherit;
  }
  .hl.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: var(--fs-sm); }
  .p-scheme { color: var(--accent); }
  .p-separator, .p-query-mark, .p-equals, .p-fragment-mark { color: var(--muted); }
  .p-subdomain { color: var(--info); }
  .p-host { color: var(--ok); }
  .p-port { color: var(--warn); }
  /* A template value reads as a chip: its text on a tint of its own colour. */
  .p-variable { color: var(--literal); background: color-mix(in srgb, var(--literal) 16%, transparent); border-radius: var(--r-sm); }
  .p-unknown-variable { color: var(--fail); text-decoration: underline wavy; }
  .p-fragment { color: var(--muted); }
  .setchip { cursor: pointer; }
  .tip { position: absolute; top: 100%; margin-top: 2px; max-width: min(520px, 90vw); padding: 4px 8px; background: var(--raised); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); box-shadow: var(--shadow); font-size: var(--fs-xs); overflow-wrap: anywhere; pointer-events: none; }
  .tip.unset { border-color: var(--fail); }
  .menu, .tip { z-index: 30; }
  .menu { position: absolute; top: 100%; left: 0; right: 0; margin: 2px 0 0; padding: 4px; list-style: none; background: var(--raised); border: 1px solid var(--line); border-radius: var(--r-lg); box-shadow: var(--shadow); max-height: 220px; overflow: auto; }
  .menu li.on button { background: var(--tint-sel); }
  .menu button { display: flex; gap: 10px; width: 100%; text-align: left; background: none; border: 0; color: var(--text); font: inherit; padding: 3px 8px; border-radius: var(--r-sm); }
  .menu code { font-family: var(--mono); font-size: var(--fs-sm); }
  .menu span { color: var(--muted); font-size: var(--fs-xs); margin-left: auto; }
</style>
