<script lang="ts">
  import { tick } from 'svelte'
  import Button from './Button.svelte'
  // How two environments differ, as a run would see them: what only one has and what the two set differently.
  // A secret's value is never shown (it lives in the vault); its name is listed as hidden.
  import { EnvironmentService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import type { EnvComparisonView } from './lib/types'

  let result = $state<EnvComparisonView | null>(null)
  const ask = $derived(ui.envCompare)
  let box = $state<HTMLDivElement>()
  // The dialog takes focus when it opens, so Escape reaches it.
  $effect(() => {
    if (ask) tick().then(() => box?.focus())
  })

  $effect(() => {
    const a = ui.envCompare
    result = null
    if (!a) return
    EnvironmentService.Compare(a.a, a.b).then((r) => (result = r as unknown as EnvComparisonView))
  })

  const alike = $derived(!!result && !result.error && !result.onlyA.length && !result.onlyB.length && !result.changed.length)

  function close() {
    ui.envCompare = null
  }
  function keys(e: KeyboardEvent) {
    if (e.key !== 'Escape') return
    e.preventDefault()
    close()
  }
</script>

{#if ask}
  <div class="scrim" role="presentation" onpointerdown={close}>
    <div bind:this={box} class="dialog" role="dialog" aria-label="Compare environments" tabindex="-1" onpointerdown={(e) => e.stopPropagation()} onkeydown={keys}>
      <h3>{ask.a} <span class="vs">vs</span> {ask.b}</h3>
      {#if !result}
        <p class="muted">Comparing…</p>
      {:else if result.error}
        <p class="err">{result.error}</p>
      {:else}
        {#if alike}<p class="muted">The same: {result.same} variables, with the same values.</p>{/if}
        {#if result.changed.length}
          <section aria-label="Different values">
            <h4>Different values</h4>
            <table>
              <thead><tr><th></th><th>{ask.a}</th><th>{ask.b}</th></tr></thead>
              <tbody>{#each result.changed as c (c.name)}<tr><td class="name">{c.name}</td><td>{c.a}</td><td>{c.b}</td></tr>{/each}</tbody>
            </table>
          </section>
        {/if}
        {#if result.onlyA.length}
          <section aria-label={`Only in ${ask.a}`}><h4>Only in {ask.a}</h4><p class="names">{result.onlyA.join(', ')}</p></section>
        {/if}
        {#if result.onlyB.length}
          <section aria-label={`Only in ${ask.b}`}><h4>Only in {ask.b}</h4><p class="names">{result.onlyB.join(', ')}</p></section>
        {/if}
        {#if !alike}<p class="muted">{result.same} the same.{#if result.secrets.length}{' '}Secrets, values not shown: {result.secrets.join(', ')}.{/if}</p>{/if}
      {/if}
      <div class="actions"><Button type="button" onclick={close}>Close</Button></div>
    </div>
  </div>
{/if}

<style>
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 60; }
  .dialog { width: 560px; max-width: calc(100vw - 32px); max-height: 82vh; overflow: auto; display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); outline: none; }
  h3 { margin: 0; }
  h4 { margin: 0 0 4px; font-size: var(--fs-sm); }
  .vs { color: var(--muted); font-weight: 400; }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .err { color: var(--fail, var(--text)); margin: 0; }
  table { width: 100%; border-collapse: collapse; font-size: var(--fs-sm); }
  th { text-align: left; color: var(--muted); font-weight: 500; padding: 2px 8px 2px 0; }
  td { padding: 3px 8px 3px 0; font-family: var(--mono); font-size: var(--fs-xs); word-break: break-all; vertical-align: top; }
  td.name { font-weight: 600; }
  .names { margin: 0; font: var(--fs-xs) var(--mono); }
  .actions { display: flex; justify-content: flex-end; }
</style>
