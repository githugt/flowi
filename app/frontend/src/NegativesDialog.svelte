<script lang="ts">
  import { tick } from 'svelte'
  import Button from './Button.svelte'
  // "Generate negative variants": the ways a request can be sent wrong (a required field dropped, a field of the
  // wrong type, an expired token), as a checklist. The ticked ones are saved as flowi overlays; each is then sent
  // like any overlay, or all together in a matrix. What the API should answer is left for you to assert.
  import { BlueprintsService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import type { AcceptedNegatives, NegativesView } from './lib/types'

  const ask = $derived(ui.negatives)
  let box = $state<HTMLDivElement>()
  let view = $state<NegativesView | null>(null)
  let ticked = $state<string[]>([])
  let done = $state<AcceptedNegatives | null>(null)
  let busy = $state(false)

  $effect(() => {
    const a = ui.negatives
    if (!a) return
    view = done = null
    ticked = []
    tick().then(() => box?.focus())
    BlueprintsService.Negatives(a.node).then((v) => {
      view = v as unknown as NegativesView
      ticked = view.variants.map((x) => x.name)
    })
  })

  const toggle = (name: string) => (ticked = ticked.includes(name) ? ticked.filter((n) => n !== name) : [...ticked, name])

  async function save() {
    if (!ask) return
    busy = true
    try {
      done = (await BlueprintsService.AcceptNegatives(ask.node, ticked)) as unknown as AcceptedNegatives
    } finally {
      busy = false
    }
  }
  function close() {
    ui.negatives = null
  }
  function keys(e: KeyboardEvent) {
    if (e.key !== 'Escape') return
    e.preventDefault()
    close()
  }
</script>

{#if ask}
  <div class="scrim" role="presentation" onpointerdown={close}>
    <div bind:this={box} class="dialog" role="dialog" aria-label="Negative variants" tabindex="-1" onpointerdown={(e) => e.stopPropagation()} onkeydown={keys}>
      <h3>Negative variants</h3>
      <p class="muted">Ways to send this request wrong, saved as overlays. A required field comes from the request's schema; with none, every field of the body counts. Add what the API should answer (a 400, say) as an assertion of the overlay.</p>
      {#if !view}
        <p class="muted">Looking…</p>
      {:else if view.error}
        <p class="err">{view.error}</p>
      {:else if !view.variants.length}
        <p class="muted">Nothing to break: the request has no JSON object body and no Authorization header.</p>
      {:else}
        {#each view.variants as v (v.name)}
          <label class="row"><input type="checkbox" checked={ticked.includes(v.name)} onchange={() => toggle(v.name)} /> <code>{v.name}</code> <span class="muted">{v.description}</span></label>
        {/each}
        {#if done}
          <p class="ok">Saved {done.saved.length} overlay{done.saved.length === 1 ? '' : 's'}.{#if done.skipped.length} Left alone, the name is taken: {done.skipped.join(', ')}.{/if}{#if done.error} {done.error}{/if}</p>
        {/if}
        <div class="actions"><Button type="button" variant="primary" disabled={busy || !ticked.length || !!done} onclick={save}>Save {ticked.length} as overlays</Button></div>
      {/if}
      <div class="actions"><Button type="button" onclick={close}>Close</Button></div>
    </div>
  </div>
{/if}

<style>
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 60; }
  .dialog { width: 520px; max-width: calc(100vw - 32px); max-height: 82vh; overflow: auto; display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); outline: none; }
  h3 { margin: 0; }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .ok { margin: 0; }
  .err { color: var(--fail, var(--text)); margin: 0; }
  .row { display: flex; gap: 6px; align-items: baseline; font-size: var(--fs-sm); padding: 2px 0; }
  code { font: var(--fs-xs) var(--mono); }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
</style>
