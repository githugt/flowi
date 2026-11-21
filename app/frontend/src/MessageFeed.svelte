<script lang="ts">
  import EmptyState from './EmptyState.svelte'
  // A compact feed of messages: one 28px row each (direction, time, preview, size), opened in place.
  // The live view keeps adding rows; Pause freezes the list and counts what arrived meanwhile.
  import Button from './Button.svelte'
  import Segmented from './Segmented.svelte'
  import MessageDetail from './MessageDetail.svelte'
  import Icon from './Icon.svelte'
  import SearchField from './SearchField.svelte'
  import { positionChip, typeChip } from './lib/messagetype'
  import { atEnd, droppedRows, isNewRun, messageMatches, toNdjson } from './lib/feed'
  import { debounced } from './lib/debounce'
  import { sizeText } from './lib/bytesize'
  import { bodyBytes, type DetailMessage } from './lib/messagedetail'
  import { copy } from './lib/clip'
  import { ui } from './lib/ui.svelte'

  type Message = DetailMessage
  let { messages, live = false }: { messages: Message[]; live?: boolean } = $props()

  let query = $state('')
  // The filter is applied once typing pauses, so a long feed is not searched on every key.
  let applied = $state('')
  const apply = debounced((q: string) => (applied = q), 250)
  $effect(() => {
    if (query === '') applied = ''
    else apply(query)
  })
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
      .filter(({ m }) => messageMatches(m, applied)),
  )

  // A shorter list is a new run: the open row, the pause and the clear belonged to the last one, and index i is another message now.
  let seen = 0
  $effect(() => {
    const n = messages.length
    if (isNewRun(seen, n)) {
      open = null
      frozen = null
      cleared = 0
    }
    seen = n
  })

  // Only the newest rows are drawn; the note says how many older ones are left out.
  const dropped = $derived(droppedRows(shown.length))
  const drawn = $derived(dropped ? shown.slice(dropped) : shown)

  // Follow keeps the newest row in view while the feed is live; scrolling up lets go, and the end takes hold again.
  let follow = $state(true)
  let rows = $state<HTMLElement>()
  $effect(() => {
    void drawn.length
    if (live && follow && rows) rows.scrollTop = rows.scrollHeight
  })
  function scrolled() {
    if (rows) follow = atEnd(rows.scrollTop, rows.clientHeight, rows.scrollHeight)
  }
  function copyAll() {
    void copy(toNdjson(shown.map(({ m }) => m)), `${shown.length} ${shown.length === 1 ? 'message' : 'messages'}`)
  }

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
  const size = (m: Message): string => sizeText(bodyBytes(m))
</script>

<div class="feed">
  <div class="tb">
    {#if live}<span class="live"><i></i>Live</span>{/if}
    <div class="find"><SearchField bind:value={query} placeholder="Filter by text or field…" label="Filter messages" /></div>
    <Segmented label="Direction" items={[{ id: 'all', label: 'All' }, { id: 'sent', label: 'Sent' }, { id: 'received', label: 'Received' }]} value={only} onpick={(id) => (only = id as typeof only)} />
    {#if live}
      <span class="grow"></span>
      <Button small variant={follow ? 'primary' : 'secondary'} aria-pressed={follow} title="Keep the newest message in view" onclick={() => { follow = !follow; if (follow && rows) rows.scrollTop = rows.scrollHeight }}>Follow</Button>
      <Button small onclick={togglePause}>{frozen === null ? 'Pause' : 'Resume'}</Button>
      <Button small disabled={!list.length} title="Hide what has arrived so far" onclick={clear}>Clear</Button>
    {/if}
    {#if !live}<span class="grow"></span>{/if}
    <Button small disabled={!shown.length} title="Copy the messages shown, one JSON object per line" onclick={copyAll}>Copy all</Button>
  </div>
  {#if waiting > 0}<button class="newpill" onclick={() => (frozen = null)}>{waiting} new</button>{/if}
  <div class="rows" bind:this={rows} onscroll={scrolled}>
    {#if dropped}<p class="muted none">The oldest {dropped} {dropped === 1 ? 'message is' : 'messages are'} not shown. Copy all still has them.</p>{/if}
    {#each drawn as { m, i } (i)}
      <button class="r" class:open={open === i} onclick={() => (open = open === i ? null : i)} aria-expanded={open === i}>
        <span class="d {m.direction}" title={m.direction === 'sent' ? 'Sent' : 'Received'}><Icon name={m.direction === 'sent' ? 'up' : 'down'} size={14} /></span>
        <span class="t">{m.at}</span>
        {#if m.kind}<span class="kind {m.kind}" title="What the body is">{m.kind}</span>{/if}
        {#if typeChip(m.type, m.kind)}<span class="ty" title="The protocol's name for this message">{typeChip(m.type, m.kind)}</span>{/if}
        {#if positionChip(m)}<span class="ty" title="Where the broker put this message: partition @ offset, or stream sequence">{positionChip(m)}</span>{/if}
        {#if m.key}<span class="k">{m.key}</span>{/if}
        <span class="p">{preview(m.body)}</span>
        <span class="z">{size(m)}</span>
      </button>
      {#if open === i}
        <MessageDetail message={m} />
      {/if}
    {:else}
      {#if list.length}<EmptyState kind="filtered" compact {query} onclear={() => { query = ''; only = 'all' }} />{:else}<EmptyState icon="stream" title="No messages yet" compact />{/if}
    {/each}
  </div>
</div>

<style>
  .feed { display: flex; flex-direction: column; min-height: 0; position: relative; }
  .tb { display: flex; align-items: center; gap: 8px; padding: 8px 0; flex: none; }
  .live { display: flex; align-items: center; gap: 7px; font-weight: 600; font-size: var(--fs-sm); color: var(--ok); margin-right: 4px; }
  .live i { width: 8px; height: 8px; border-radius: 50%; background: var(--ok); box-shadow: 0 0 0 4px var(--ok-t); }
  .find { width: 260px; max-width: 100%; }
  .grow { flex: 1; }
  .newpill { position: absolute; left: 50%; bottom: 12px; transform: translateX(-50%); z-index: 3; border: 0; border-radius: var(--r-pill); height: 26px; padding: 0 12px; background: var(--accent); color: var(--on-accent); font-weight: 600; font-size: var(--fs-sm); cursor: pointer; box-shadow: var(--shadow-lift); }
  .rows { overflow: auto; min-height: 0; }
  .r { all: unset; box-sizing: border-box; display: flex; align-items: center; gap: 12px; width: 100%; height: var(--row-h-lg); padding: 0 10px; border-radius: var(--r-sm); font-size: var(--fs-sm); cursor: pointer; }
  .rows > :nth-child(odd) { background: color-mix(in srgb, var(--text) 2.5%, transparent); }
  .r:hover, .r.open { background: var(--tint-hover); }
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
  .none { padding: 12px 10px; }
</style>
