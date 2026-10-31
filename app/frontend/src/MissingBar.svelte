<script lang="ts">
  // Says before a send what it would lack: variables the selected environment
  // does not define. A send still goes (the person may mean it); this only warns.
  import { RequestService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { app } from './lib/app.svelte'
  import { promptNewEnvironment } from './lib/menus'
  import type { MissingView, RequestView } from './lib/types'

  let { view }: { view: RequestView } = $props()
  let missing = $state<MissingView>({ environment: '', vars: [] })

  // Asked again whenever the request or the environments change. An answer
  // that comes after a newer question, or after the editor closed, is dropped:
  // view may be gone by then.
  $effect(() => {
    void view.current
    void app.tree
    let stale = false
    RequestService.Missing(view.node).then((m) => {
      if (!stale) missing = m as unknown as MissingView
    })
    return () => (stale = true)
  })

  const addable = $derived(missing.vars.filter((v) => v.name))
  const label = (v: { expr: string; name: string; secret: boolean }) => (v.name ? v.name : v.expr) + (v.secret ? ' (secret)' : '')
</script>

{#if missing.vars.length}
  <div class="bar" role="status">
    <span class="icon" aria-hidden="true">⚠</span>
    <span class="text">
      {#if missing.environment}
        Not set in <strong>{missing.environment}</strong>:
      {:else if app.tree?.environments.length}
        No environment is selected. This request needs:
      {:else}
        This request needs an environment for:
      {/if}
      {#each missing.vars as v, i}<code class:fn={!v.name}>{label(v)}</code>{i < missing.vars.length - 1 ? ', ' : ''}{/each}
    </span>
    {#if missing.environment && addable.length}
      <button onclick={() => app.addEnvironmentRows(missing.environment, addable)}>Set in {missing.environment}…</button>
    {:else if !missing.environment && !app.tree?.environments.length}
      <button onclick={promptNewEnvironment}>New environment…</button>
    {/if}
  </div>
{/if}

<style>
  .bar { display: flex; align-items: center; gap: var(--sp-2); padding: 5px 12px; font-size: var(--fs-sm); background: color-mix(in srgb, var(--warn) 12%, transparent); border-bottom: 1px solid var(--line); }
  .icon { color: var(--warn); }
  .text { flex: 1; min-width: 0; }
  code { font-family: var(--mono); }
  code.fn { color: var(--fail); }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 2px 10px; font: inherit; white-space: nowrap; }
  button:hover { border-color: var(--accent); }
</style>
