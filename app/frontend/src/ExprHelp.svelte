<script lang="ts">
  // A "?" that opens what an expression field can read: the names, the
  // operators and functions, and examples. What it shows depends on where the
  // field is (a flow step, a request, a message request, an "only if" filter).
  import { SystemService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { ExprHelpView } from './lib/types'

  let { context }: { context: string } = $props()

  let open = $state(false)
  let help = $state<ExprHelpView | null>(null)
  let tab = $state<'names' | 'tools' | 'generators'>('names')
  let at = $state({ x: 0, y: 0 })
  let pop = $state<HTMLDivElement>()

  async function toggle(e: MouseEvent) {
    e.preventDefault()
    e.stopPropagation()
    if (open) return void (open = false)
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    help = (await SystemService.ExpressionHelp(context)) as unknown as ExprHelpView
    tab = 'names'
    at = { x: r.left, y: r.bottom + 6 }
    open = true
  }

  // Kept inside the window: moved left when it would run off the right edge, up when off the bottom.
  // It is measured, not assumed: the old 448 left out its padding, so in the side panel, near the right
  // edge, its last column was cut off.
  const left = $derived(Math.max(8, Math.min(at.x, window.innerWidth - (pop?.offsetWidth ?? 448) - 8)))
  const top = $derived(Math.max(8, Math.min(at.y, window.innerHeight - (pop?.offsetHeight ?? 420) - 8)))
  const rows = $derived(help ? help[tab] : [])
</script>

<svelte:window onpointerdown={() => (open = false)} onkeydown={(e) => e.key === 'Escape' && (open = false)} />

<button type="button" class="q" title="What can I write here?" aria-label="Help with this expression" aria-expanded={open} onpointerdown={(e) => e.stopPropagation()} onclick={toggle}>?</button>

{#if open && help}
  <div class="pop" bind:this={pop} role="dialog" tabindex="-1" style:left={`${left}px`} style:top={`${top}px`} onpointerdown={(e) => e.stopPropagation()}>
    <h4>{help.title}</h4>
    <p class="intro">{help.intro}</p>
    <div class="tabs" role="tablist">
      <button role="tab" aria-selected={tab === 'names'} class:on={tab === 'names'} onclick={() => (tab = 'names')}>Names</button>
      <button role="tab" aria-selected={tab === 'tools'} class:on={tab === 'tools'} onclick={() => (tab = 'tools')}>Operators and functions</button>
      <button role="tab" aria-selected={tab === 'generators'} class:on={tab === 'generators'} onclick={() => (tab = 'generators')}>$generators</button>
    </div>
    <div class="rows">
      {#each rows as r}
        <div class="row">
          <code class="name">{r.name}</code>
          <span class="what">{r.what}</span>
          {#if r.example}<code class="ex">{r.example}</code>{/if}
        </div>
      {/each}
    </div>
    {#if help.examples.length}
      <h5>Examples</h5>
      {#each help.examples as e}<code class="ex block">{e}</code>{/each}
    {/if}
  </div>
{/if}

<style>
  .q { width: 16px; height: 16px; padding: 0; margin-left: 6px; border-radius: 50%; border: 1px solid var(--line); background: transparent; color: var(--muted); font: 600 10px/1 inherit; cursor: pointer; vertical-align: middle; }
  .q:hover, .q[aria-expanded='true'] { color: var(--accent); border-color: var(--accent); }
  .pop { position: fixed; z-index: 70; box-sizing: border-box; width: min(440px, calc(100vw - 16px)); max-height: min(70vh, 560px); overflow: auto; padding: 12px 14px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); box-shadow: 0 10px 30px rgb(0 0 0 / 0.4); font-size: var(--fs-sm); color: var(--text); }
  h4 { margin: 0 0 4px; font-size: var(--fs-md); }
  h5 { margin: 10px 0 4px; font-size: var(--fs-xs); color: var(--muted); text-transform: uppercase; letter-spacing: 0.05em; }
  .intro { margin: 0 0 8px; color: var(--muted); line-height: 1.45; }
  .tabs { display: flex; gap: 2px; border-bottom: 1px solid var(--line); margin-bottom: 6px; }
  .tabs button { background: none; border: 0; border-bottom: 2px solid transparent; color: var(--muted); font: inherit; padding: 3px 8px; cursor: pointer; }
  .tabs button.on { color: var(--text); border-bottom-color: var(--accent); }
  .row { display: grid; grid-template-columns: 1fr; gap: 2px; padding: 5px 0; border-bottom: 1px solid color-mix(in srgb, var(--line) 50%, transparent); }
  .name { color: var(--accent); font: var(--fs-sm) var(--mono); }
  .what { line-height: 1.4; }
  .ex { font: var(--fs-xs) var(--mono); color: var(--muted); background: var(--bg); border-radius: var(--r-sm); padding: 1px 5px; width: fit-content; -webkit-user-select: text; user-select: text; }
  .ex.block { display: block; margin: 3px 0; }
</style>
