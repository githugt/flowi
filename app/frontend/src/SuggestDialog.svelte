<script lang="ts">
  import { tick } from 'svelte'
  import Button from './Button.svelte'
  // "Suggest flows": the flows Flowi proposes for a collection from what its requests return, each with the
  // evidence for every link. Accept saves it as a flow and opens it (where it can be edited like any flow);
  // Reject only takes it off this list. Nothing is saved until Accept.
  import { BlueprintsService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import { miniLayout, shapeText, sureText, surestFirst, takeText } from './lib/suggest'
  import type { FlowSuggestion } from './lib/types'

  let list = $state<FlowSuggestion[]>([])
  let loading = $state(false)
  let reviewed = $state(false)

  const ask = $derived(ui.suggest)
  let box = $state<HTMLDivElement>()
  // The dialog takes focus when it opens, so Escape reaches it.
  $effect(() => {
    if (ask) tick().then(() => box?.focus())
  })

  $effect(() => {
    const a = ui.suggest
    if (!a) return
    loading = true
    reviewed = false
    list = []
    BlueprintsService.Suggestions(a.collection).then((got) => {
      list = surestFirst(got as unknown as FlowSuggestion[])
      loading = false
    })
  })

  function close() {
    ui.suggest = null
  }
  function accept(s: FlowSuggestion) {
    if (!ask) return
    BlueprintsService.AcceptSuggestion(ask.collection, s.path)
    reviewed = true
    list = list.filter((x) => x.path !== s.path)
    // The accepted flow opens behind the dialog; with nothing left to review, the dialog only hides it.
    if (!list.length) close()
  }
  function reject(s: FlowSuggestion) {
    reviewed = true
    list = list.filter((x) => x.path !== s.path)
  }
  function keys(e: KeyboardEvent) {
    if (e.key !== 'Escape') return
    e.preventDefault()
    close()
  }
</script>

{#if ask}
  <div class="scrim" role="presentation" onpointerdown={close}>
    <div bind:this={box} class="dialog" role="dialog" aria-label="Suggest flows" tabindex="-1" onpointerdown={(e) => e.stopPropagation()} onkeydown={keys}>
      <h3>Suggested flows</h3>
      <p class="muted">Found from what the responses of this collection return. Nothing is saved until you accept. Values the first request needs (a body field, say) are left for you to fill in.</p>
      {#each list as s (s.path)}
        {@const mini = miniLayout(s)}
        <section class="card" aria-label={s.name}>
          <header>
            <strong>{s.name}</strong>
            <span class="shape">{shapeText(s)}</span>
          </header>
          <svg class="mini" viewBox="0 0 {mini.width} {mini.height}" width={mini.width} height={mini.height} role="img" aria-label="The steps of {s.name}, left to right">
            {#each mini.links as l (l.from + '>' + l.to + l.label)}
              <path d={l.path} class="link" class:weak={l.weak} />
            {/each}
            {#each mini.nodes as n (n.id)}
              <rect x={n.x} y={n.y} width={n.w} height={n.h} rx="6" class="node" />
              <text x={n.x + n.w / 2} y={n.y + n.h / 2} class="verbtext" text-anchor="middle" dominant-baseline="central">{n.label}</text>
            {/each}
          </svg>
          <ol>
            {#each s.steps as step (step.id)}
              <li>
                <span class="verb">{step.method}</span> <code>{step.request}</code>
                {#each step.takes as t (t.param)}
                  <div class="take">{takeText(t)} <span class="sure" class:weak={t.confidence < 0.85}>{sureText(t.confidence)}</span></div>
                {/each}
              </li>
            {/each}
          </ol>
          <footer>
            <Button type="button" onclick={() => reject(s)}>Reject</Button>
            <Button type="button" variant="primary" onclick={() => accept(s)}>Accept and open</Button>
          </footer>
        </section>
      {:else}
        <p class="muted empty">
          {loading
            ? 'Looking…'
            : reviewed
              ? 'Nothing left to review.'
              : 'Nothing to suggest. A suggestion needs a request that makes something (a POST whose response has an id) and another that takes it in its path, with the response known from an import, a pinned example or a saved response.'}
        </p>
      {/each}
      <div class="actions"><Button type="button" onclick={close}>Close</Button></div>
    </div>
  </div>
{/if}

<style>
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 60; }
  .dialog { width: 520px; max-width: calc(100vw - 32px); max-height: 82vh; overflow: auto; display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); outline: none; }
  h3 { margin: 0; }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .empty { padding: 8px 0; }
  .card { display: flex; flex-direction: column; gap: 6px; padding: 10px; border: 1px solid var(--line); border-radius: var(--r-md); background: var(--bg); }
  header { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; }
  .shape { color: var(--muted); font: var(--fs-xs) var(--mono); }
  .mini { max-width: 100%; overflow: visible; align-self: center; }
  .mini .node { fill: var(--panel); stroke: var(--line); }
  .mini .verbtext { fill: var(--text); font: 600 var(--fs-xs) var(--mono); }
  .mini .link { fill: none; stroke: var(--muted); stroke-width: 1.5; }
  .mini .link.weak { stroke: var(--warn, var(--muted)); stroke-dasharray: 4 3; }
  ol { margin: 0; padding-left: 20px; display: flex; flex-direction: column; gap: 4px; }
  .verb { font: var(--fs-xs) var(--mono); font-weight: 600; }
  code { font: var(--fs-xs) var(--mono); }
  .take { color: var(--muted); font-size: var(--fs-xs); padding-left: 2px; }
  .sure { margin-left: 4px; }
  .sure.weak { color: var(--warn, var(--muted)); font-weight: 600; }
  footer, .actions { display: flex; justify-content: flex-end; gap: 8px; }
</style>
