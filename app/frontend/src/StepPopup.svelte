<script lang="ts">
  // The popup that adds a step: a search over the ten kinds, each with a line saying what it is for. Type to
  // narrow, the arrow keys move, Enter adds the highlighted one, Escape closes. It only picks; the editor adds.
  import Icon from './Icon.svelte'
  import { colourToken, typeIcon } from './lib/card'
  import { filterSteps, groupSteps, nextIndex, stepAt } from './lib/popup'
  import type { StepChoice } from './lib/types'

  let { steps, x, y, onpick, onclose }: { steps: StepChoice[]; x: number; y: number; onpick: (s: StepChoice) => void; onclose: () => void } = $props()

  let query = $state('')
  let at = $state(0)
  let search = $state<HTMLInputElement>()
  const shown = $derived(filterSteps(steps, query))
  const groups = $derived(groupSteps(shown))
  const flat = $derived(shown)

  $effect(() => {
    search?.focus()
  })
  $effect(() => {
    void query
    at = 0
  })

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') return onclose()
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      at = nextIndex(at, e.key === 'ArrowDown' ? 1 : -1, flat.length)
    }
    if (e.key === 'Enter') {
      e.preventDefault()
      const s = stepAt(groups, at)
      if (s) onpick(s)
    }
  }

  const indexOf = (s: StepChoice) => flat.indexOf(s)
</script>

<svelte:window {onkeydown} />
<!-- A click anywhere outside closes it, as a menu does. -->
<div class="scrim" role="presentation" onpointerdown={onclose}></div>
<div class="popup" style:left={`${Math.min(x, window.innerWidth - 380)}px`} style:top={`${Math.min(y + 8, window.innerHeight - 440)}px`} role="dialog" aria-label="Add a step">
  <input bind:this={search} bind:value={query} placeholder="Search steps" aria-label="Search steps" spellcheck="false" autocomplete="off" />
  <div class="list" role="listbox" aria-label="Steps">
    {#each groups as g (g.name)}
      <h4>{g.name}</h4>
      {#each g.items as s (s.name)}
        {@const i = indexOf(s)}
        <button class="item" class:on={i === at} role="option" aria-selected={i === at} onpointermove={() => (at = i)} onclick={() => onpick(s)}>
          <span class="tile" style:background={`var(${colourToken(s.type)})`}><Icon name={typeIcon(s.type)} size={18} /></span>
          <span class="text"><span class="name">{s.name}</span><span class="desc">{s.description}</span></span>
        </button>
      {/each}
    {:else}
      <p class="none">No step matches “{query}”.</p>
    {/each}
  </div>
</div>

<style>
  .scrim { position: fixed; inset: 0; z-index: 29; }
  .popup { position: fixed; z-index: 30; width: 360px; max-height: 430px; box-sizing: border-box; display: flex; flex-direction: column; gap: 8px; padding: 10px; background: var(--panel); border: 1px solid var(--line); border-radius: 14px; box-shadow: var(--shadow); }
  input { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 7px 10px; font: inherit; }
  input:focus { border-color: var(--accent); outline: none; }
  .list { overflow: auto; display: flex; flex-direction: column; gap: 2px; }
  h4 { margin: 8px 4px 2px; color: var(--muted); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: 0.06em; }
  .item { display: flex; align-items: center; gap: 10px; text-align: left; background: transparent; color: var(--text); border: 0; border-radius: 10px; padding: 6px 8px; font: inherit; }
  .item.on { background: var(--tint-hover); }
  .tile { flex: none; width: 30px; height: 30px; border-radius: 8px; display: grid; place-items: center; color: var(--text); }
  .text { display: flex; flex-direction: column; min-width: 0; }
  .name { font-weight: 600; font-size: var(--fs-md); }
  .desc { color: var(--muted); font-size: var(--fs-sm); }
  .none { color: var(--muted); margin: 8px 4px; }
</style>
