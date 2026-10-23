<script lang="ts">
  import ExprHelp from './ExprHelp.svelte'
  import HiddenChars from './HiddenChars.svelte'
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

  // ---- suggestions
  let open = $state<Suggest | null>(null)
  let choice = $state(0)
  let asked = 0
  async function ask() {
    if (!suggest || !input) return
    const mine = ++asked
    const s = await suggest(local, input.selectionStart ?? local.length)
    if (mine !== asked) return
    open = s.items.length ? s : null
    choice = 0
  }

  function apply(i: number) {
    if (!open) return
    const s = open
    const rest = local.slice(s.to)
    const closing = rest.startsWith('}}') ? '' : '}}'
    const next = local.slice(0, s.from) + s.items[i].insert + closing + rest
    const caret = s.from + s.items[i].insert.length + 2
    local = next
    oninput?.(next)
    open = null
    queueMicrotask(() => input?.setSelectionRange(caret, caret))
  }

  function onkeydown(e: KeyboardEvent) {
    if (open) {
      if (e.key === 'ArrowDown') (e.preventDefault(), (choice = (choice + 1) % open.items.length))
      else if (e.key === 'ArrowUp') (e.preventDefault(), (choice = (choice - 1 + open.items.length) % open.items.length))
      else if (e.key === 'Enter' || e.key === 'Tab') (e.preventDefault(), apply(choice))
      else if (e.key === 'Escape') (e.stopPropagation(), (open = null))
      return
    }
    if (e.key === 'Enter') onenter?.(local)
  }

  // The pair colours cycle, as in the URL bar of the other window.
  // Theme colours, so each query pair stays readable in light themes too.
  const PAIRS = ['var(--accent)', 'var(--warn)', 'var(--ok)', 'var(--literal)', 'var(--info)', 'var(--fail)']
  const colour = (s: Span) => (s.pair >= 0 && ['key', 'value', 'equals', 'variable', 'unknown-variable'].includes(s.part) ? PAIRS[s.pair % PAIRS.length] : undefined)
</script>

<label class="field">
  {#if label}<span class="label">{label}{#if help}<ExprHelp context={help} />{/if}</span>{/if}
  <div class="wrap">
    {#if inSync}
      <div class="hl" class:mono aria-hidden="true" style:transform={`translateX(${-scroll}px)`}>{#each coloured!.spans as s}<span class="p-{s.part}" style:color={colour(s)}>{s.text}</span>{/each}</div>
    {/if}
    <input
      bind:this={input}
      class:mono
      class:hide={inSync}
      value={local}
      {placeholder}
      spellcheck="false"
      autocomplete="off"
      onfocus={() => (focused = true)}
      onblur={() => ((focused = false), (open = null))}
      oninput={(e) => {
        local = e.currentTarget.value
        oninput?.(local)
        ask()
      }}
      onscroll={(e) => (scroll = e.currentTarget.scrollLeft)}
      onkeyup={(e) => ['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key) && ask()}
      onclick={ask}
      {onkeydown}
    />
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
  .mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: var(--fs-sm); }
  /* With colours drawn behind, the input's own text is clear but its caret stays. */
  .hide { background: transparent; color: transparent; caret-color: var(--text); }
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
  .p-variable { color: var(--literal); }
  .p-unknown-variable { color: var(--fail); text-decoration: underline wavy; }
  .p-fragment { color: var(--muted); }
  .menu { position: absolute; top: 100%; left: 0; right: 0; z-index: 30; margin: 2px 0 0; padding: 4px; list-style: none; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); box-shadow: 0 8px 24px rgb(0 0 0 / 0.35); max-height: 220px; overflow: auto; }
  .menu li.on button { background: color-mix(in srgb, var(--accent) 22%, transparent); }
  .menu button { display: flex; gap: 10px; width: 100%; text-align: left; background: none; border: 0; color: var(--text); font: inherit; padding: 3px 8px; border-radius: var(--r-sm); }
  .menu code { font-family: var(--mono); font-size: var(--fs-sm); }
  .menu span { color: var(--muted); font-size: var(--fs-xs); margin-left: auto; }
</style>
