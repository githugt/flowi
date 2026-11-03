<script lang="ts">
  // A compact feed of messages: one 28px row each (direction, time, preview, size), opened in place.
  // The live view keeps adding rows; Pause freezes the list and counts what arrived meanwhile.
  import Code from './Code.svelte'
  import Icon from './Icon.svelte'
  import SearchField from './SearchField.svelte'
  import { Clipboard } from '@wailsio/runtime'
  import { typeChip } from './lib/messagetype'

  type Message = { direction: string; at: string; body: string; key?: string; kind?: string; type?: string }
  let { messages, live = false }: { messages: Message[]; live?: boolean } = $props()

  let query = $state('')
  let only = $state<'all' | 'sent' | 'received'>('all')
  let open = $state<number | null>(null)
  // While paused the list shows the first `frozen` messages; the rest wait behind the "N new" pill.
  let frozen = $state<number | null>(null)
  // Clear hides what has arrived so far. The messages belong to the run, so this only hides them here; a list that
  // becomes shorter than what was cleared is a new run, and shows whole again.
  let cleared = $state(0)

  const list = $derived(cleared > 0 && cleared <= messages.length ? messages.slice(cleared) : messages)
  const visible = $derived(frozen === null ? list : list.slice(0, frozen))
  const waiting = $derived(list.length - visible.length)
  const shown = $derived(
    visible
      .map((m, i) => ({ m, i }))
      .filter(({ m }) => only === 'all' || m.direction === only)
      .filter(({ m }) => !query || m.body.toLowerCase().includes(query.toLowerCase()) || (m.key ?? '').toLowerCase().includes(query.toLowerCase()) || (m.type ?? '').toLowerCase().includes(query.toLowerCase())),
  )

  function togglePause() {
    frozen = frozen === null ? list.length : null
  }
  function clear() {
    cleared += list.length
    open = null
    if (frozen !== null) frozen = 0
  }
  function preview(body: string): string {
    return body.replace(/\s+/g, ' ').trim()
  }
  function size(body: string): string {
    const n = new TextEncoder().encode(body).length
    return n < 1024 ? `${n} B` : `${(n / 1024).toFixed(1)} KB`
  }
  function copy(body: string) {
    Clipboard.SetText(body)
  }
</script>

<div class="feed">
  <div class="tb">
    {#if live}<span class="live"><i></i>Live</span>{/if}
    <div class="find"><SearchField bind:value={query} placeholder="Filter by text or field…" label="Filter messages" /></div>
    <div class="seg" role="group" aria-label="Direction">
      {#each ['all', 'sent', 'received'] as d}
        <button class:on={only === d} onclick={() => (only = d as typeof only)}>{d === 'all' ? 'All' : d === 'sent' ? 'Sent' : 'Received'}</button>
      {/each}
    </div>
    {#if live}
      <span class="grow"></span>
      <button class="mini" onclick={togglePause}>{frozen === null ? 'Pause' : 'Resume'}</button>
      <button class="mini" disabled={!list.length} title="Hide what has arrived so far" onclick={clear}>Clear</button>
    {/if}
  </div>
  {#if waiting > 0}<button class="newpill" onclick={() => (frozen = null)}>{waiting} new</button>{/if}
  <div class="rows">
    {#each shown as { m, i } (i)}
      <button class="r" class:open={open === i} onclick={() => (open = open === i ? null : i)} aria-expanded={open === i}>
        <span class="d {m.direction}" title={m.direction === 'sent' ? 'Sent' : 'Received'}><Icon name={m.direction === 'sent' ? 'up' : 'down'} size={14} /></span>
        <span class="t">{m.at}</span>
        {#if m.kind}<span class="kind {m.kind}" title="What the body is">{m.kind}</span>{/if}
        {#if typeChip(m.type, m.kind)}<span class="ty" title="The protocol's name for this message">{typeChip(m.type, m.kind)}</span>{/if}
        {#if m.key}<span class="k">{m.key}</span>{/if}
        <span class="p">{preview(m.body)}</span>
        <span class="z">{size(m.body)}</span>
      </button>
      {#if open === i}
        <div class="det"><button class="mini cp" onclick={() => copy(m.body)}>Copy</button><Code text={m.body} /></div>
      {/if}
    {:else}
      <p class="muted none">{list.length ? 'No message matches.' : 'Nothing yet.'}</p>
    {/each}
  </div>
</div>

<style>
  .feed { display: flex; flex-direction: column; min-height: 0; position: relative; }
  .tb { display: flex; align-items: center; gap: 8px; padding: 8px 0; flex: none; }
  .live { display: flex; align-items: center; gap: 7px; font-weight: 600; font-size: var(--fs-sm); color: var(--ok); margin-right: 4px; }
  .live i { width: 8px; height: 8px; border-radius: 50%; background: var(--ok); box-shadow: 0 0 0 4px var(--ok-t); }
  .find { width: 260px; max-width: 100%; }
  .seg { display: flex; padding: 2px; border-radius: var(--r-md); background: var(--sunk); }
  .seg button { border: 0; background: none; color: var(--muted); height: 24px; padding: 0 10px; border-radius: var(--r-sm); font-size: var(--fs-sm); cursor: pointer; }
  .seg button.on { background: var(--raised); color: var(--text); }
  .grow { flex: 1; }
  .newpill { position: absolute; left: 50%; top: 44px; transform: translateX(-50%); z-index: 3; border: 0; border-radius: var(--r-pill); height: 26px; padding: 0 12px; background: var(--accent); color: var(--on-accent); font-weight: 600; font-size: var(--fs-sm); cursor: pointer; box-shadow: 0 6px 18px rgb(0 0 0 / 0.4); }
  .rows { overflow: auto; min-height: 0; }
  .r { all: unset; box-sizing: border-box; display: flex; align-items: center; gap: 12px; width: 100%; height: var(--row-h-lg); padding: 0 10px; border-radius: var(--r-sm); font-size: var(--fs-sm); cursor: pointer; }
  .rows > :nth-child(odd) { background: color-mix(in srgb, var(--text) 2.5%, transparent); }
  .r:hover, .r.open { background: var(--hover, color-mix(in srgb, var(--text) 7%, transparent)); }
  .r:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }
  .d { display: grid; place-items: center; width: 16px; flex: none; }
  .d.received { color: var(--ok); }
  .d.sent { color: var(--accent); }
  .t { width: 92px; flex: none; font: var(--fs-xs) var(--mono, ui-monospace, monospace); color: var(--muted); font-variant-numeric: tabular-nums; }
  .k { flex: none; padding: 0 6px; border-radius: var(--r-pill); background: color-mix(in srgb, var(--muted) 20%, transparent); font-size: var(--fs-xs); line-height: 18px; }
  .ty { flex: none; max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; padding: 0 8px; border-radius: var(--r-pill); font: var(--fs-xs) var(--mono, ui-monospace, monospace); line-height: 18px; border: 1px solid var(--line); color: var(--text); }
  .kind { flex: none; width: 50px; text-align: center; border-radius: var(--r-pill); font-size: var(--fs-xs); line-height: 18px; background: color-mix(in srgb, var(--muted) 18%, transparent); color: var(--muted); }
  .kind.json { background: var(--tint-accent); color: var(--accent); }
  .kind.binary { background: var(--warn-t); color: var(--warn); }
  .p { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font: var(--fs-sm) var(--mono, ui-monospace, monospace); color: var(--text); opacity: 0.85; }
  .z { flex: none; font: var(--fs-xs) var(--mono, ui-monospace, monospace); color: var(--muted); }
  .det { position: relative; margin: 0 0 6px; padding: 10px 16px 12px 40px; border-radius: 0 0 var(--r-md) var(--r-md); background: var(--sunk); }
  .cp { position: absolute; right: 10px; top: 8px; }
  .none { padding: 12px 10px; }
</style>
