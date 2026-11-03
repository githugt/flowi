<script lang="ts">
  // Cmd/Ctrl+Shift+P: every action of the window's menu by name, filtered as you type. Enter runs the highlighted one,
  // the arrows move, Escape or a click outside closes it. Jump to… (Cmd/Ctrl+K) stays the way to reach a request.
  import { tick } from 'svelte'
  import { app } from './lib/app.svelte'
  import { commandList, filterCommands } from './lib/commands'
  import SearchField from './SearchField.svelte'
  import Icon from './Icon.svelte'

  let query = $state('')
  let index = $state(0)
  let field = $state<HTMLInputElement>()
  let list = $state<HTMLDivElement>()
  const commands = $derived(app.paletteOpen ? commandList((a) => app.onMenu(a), app.paletteContext()) : [])
  const shown = $derived(filterCommands(commands, query))

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
    else if (e.key === 'ArrowDown') index = Math.min(index + 1, shown.length - 1)
    else if (e.key === 'ArrowUp') index = Math.max(index - 1, 0)
    else if (e.key === 'Enter') runAt(index)
    else return
    e.preventDefault()
    tick().then(() => list?.querySelector('.on')?.scrollIntoView({ block: 'nearest' }))
  }
</script>

{#if app.paletteOpen}
  <div class="scrim" role="presentation" onpointerdown={close}>
    <div class="palette" role="dialog" aria-label="Command palette" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
      <SearchField bind:value={query} bind:el={field} placeholder="Type a command…" label="Command" onkeydown={keys} />
      <div class="list" role="listbox" bind:this={list}>
        {#each shown as c, i (c.id)}
          <button class="row" class:on={i === index} role="option" aria-selected={i === index} onpointermove={() => (index = i)} onclick={() => runAt(i)}>
            <Icon name={c.icon} size={14} /><span class="title">{c.title}</span>{#if c.keys}<kbd>{c.keys}</kbd>{/if}
          </button>
        {:else}
          <p class="none">No command matches.</p>
        {/each}
      </div>
    </div>
  </div>
{/if}

<style>
  .scrim { position: fixed; inset: 0; z-index: 60; display: flex; justify-content: center; align-items: flex-start; padding-top: 14vh; background: rgb(0 0 0 / 0.35); }
  .palette { width: min(520px, calc(100vw - 32px)); display: flex; flex-direction: column; gap: 6px; padding: 8px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg, 10px); box-shadow: 0 18px 48px rgb(0 0 0 / 0.5); }
  .list { max-height: 340px; overflow: auto; display: flex; flex-direction: column; }
  .row { all: unset; box-sizing: border-box; display: flex; align-items: center; gap: 10px; height: var(--row-h-lg); padding: 0 10px; border-radius: var(--r-sm); color: var(--text); font-size: var(--fs-md); cursor: pointer; }
  .row.on { background: var(--tint-sel); }
  .row :global(svg) { flex: none; color: var(--muted); }
  .title { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  kbd { flex: none; font: var(--fs-xs) var(--mono); color: var(--muted); }
  .none { margin: 8px 10px; color: var(--muted); font-size: var(--fs-sm); }
</style>
