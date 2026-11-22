<script lang="ts">
  import Modal from './Modal.svelte'
  import { listKeyTarget } from './lib/listkeys'
  import { isEnter } from './lib/enterkey'
  // Cmd/Ctrl+Shift+P: every action of the window's menu by name, filtered as you type. Enter runs the highlighted one,
  // the arrows move, Escape or a click outside closes it. It also opens requests, flows and environments by name.
  import { tick } from 'svelte'
  import { app } from './lib/app.svelte'
  import StatusIcon from './StatusIcon.svelte'
  import { commandList, filterCommands, itemCommands } from './lib/commands'
  import SearchField from './SearchField.svelte'
  import Icon from './Icon.svelte'

  let query = $state('')
  let index = $state(0)
  let field = $state<HTMLInputElement>()
  let list = $state<HTMLDivElement>()
  const commands = $derived(app.paletteOpen ? commandList((a) => app.onMenu(a), app.paletteContext()) : [])
  // The workspace's requests, flows and environments are reached from the same box. With no text, the items that are
  // open lead (a short "recent" list); with text, every match shows. A leading ">" asks for commands only.
  const tabIds = $derived(app.tabs.map((t) => t.node).reverse())
  const items = $derived(app.paletteOpen ? itemCommands(app.tree?.roots ?? [], tabIds, (n) => void app.open(n)) : [])
  const commandsOnly = $derived(query.trimStart().startsWith('>'))
  const text = $derived(commandsOnly ? query.trimStart().slice(1) : query)
  const shown = $derived(
    commandsOnly
      ? filterCommands(commands, text)
      : text.trim()
        ? [...filterCommands(items, text), ...filterCommands(commands, text)]
        : [...items.filter((c) => tabIds.includes(c.id.slice(5))).slice(0, 5), ...commands],
  )

  // Each time it opens: an empty field, the first command highlighted, the cursor in the field.
  $effect(() => {
    if (!app.paletteOpen) return
    query = ''
    index = 0
    tick().then(() => field?.focus())
  })
  $effect(() => {
    void query
    index = 0
  })

  function close() {
    app.paletteOpen = false
  }
  function runAt(i: number) {
    const c = shown[i]
    if (!c) return
    close()
    c.run()
  }
  function keys(e: KeyboardEvent) {
    if (e.key === 'Escape') close()
    else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') index = listKeyTarget(e.key, index, shown.length)!
    else if (isEnter(e)) runAt(index)
    else return
    e.preventDefault()
    tick().then(() => list?.querySelector('.on')?.scrollIntoView({ block: 'nearest' }))
  }
</script>

{#if app.paletteOpen}
  <Modal title="Command palette" bare placement="top" guard={false} width="min(520px, calc(100vw - 32px))" maxHeight="none" onclose={close}>
    <SearchField bind:value={query} bind:el={field} placeholder="Type a command, request, flow or environment… (> for commands only; tag:x or status:y to filter)" label="Command or item" onkeydown={keys} />
    <div class="list" role="listbox" bind:this={list}>
      {#each shown as c, i (c.id)}
        <button class="row" class:on={i === index} role="option" aria-selected={i === index} onpointermove={() => (index = i)} onclick={() => runAt(i)}>
          <Icon name={c.icon} size={14} /><span class="title">{c.title}</span>{#if c.status}<StatusIcon mark={c.status} size={12} />{/if}{#if c.detail}<span class="detail">{c.detail}</span>{/if}{#if c.keys}<kbd>{c.keys}</kbd>{/if}
        </button>
      {:else}
        <p class="none">Nothing matches.</p>
      {/each}
    </div>
  </Modal>
{/if}

<style>
  .list { max-height: 340px; overflow: auto; display: flex; flex-direction: column; }
  .row { all: unset; box-sizing: border-box; display: flex; align-items: center; gap: 10px; height: var(--row-h-lg); padding: 0 10px; border-radius: var(--r-sm); color: var(--text); font-size: var(--fs-md); cursor: pointer; }
  .row.on { background: var(--tint-sel); }
  .row :global(svg) { flex: none; color: var(--muted); }
  .title { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .detail { flex: none; max-width: 40%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--muted); font-size: var(--fs-xs); }
  kbd { flex: none; font: var(--fs-xs) var(--mono); color: var(--muted); }
  .none { margin: 8px 10px; color: var(--muted); font-size: var(--fs-sm); }
</style>
