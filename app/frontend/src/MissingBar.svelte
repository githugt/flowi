<script lang="ts">
  import Banner from './Banner.svelte'
  import Button from './Button.svelte'
  // Says before a send what it would lack: variables the selected environment
  // does not define. A send still goes (the person may mean it); this only warns.
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
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

  // The chip in the tab strip warns only when this request needs an environment that is not chosen. Cleared when
  // the editor goes, so a flow or a screen never inherits the last request's warning.
  $effect(() => {
    app.envWanted = missing.vars.length > 0 && !missing.environment
    return () => (app.envWanted = false)
  })

  const addable = $derived(missing.vars.filter((v) => v.name))
  const label = (v: { expr: string; name: string; secret: boolean }) => (v.name ? v.name : v.expr) + (v.secret ? ' (secret)' : '')
</script>

{#if missing.vars.length}
  <Banner tone="warn" icon="⚠">
      {#if missing.environment}
        Not set in <strong>{missing.environment}</strong>:
      {:else if app.tree?.environments.length}
        No environment is selected. This request needs:
      {:else}
        This request needs an environment for:
      {/if}
      {#each missing.vars as v, i}<code class:fn={!v.name}>{label(v)}</code>{i < missing.vars.length - 1 ? ', ' : ''}{/each}
    {#snippet actions()}
      {#if missing.environment && addable.length}
        <Button small onclick={() => app.addEnvironmentRows(missing.environment, addable)}>Set in {missing.environment}…</Button>
      {:else if !missing.environment && !app.tree?.environments.length}
        <Button small onclick={promptNewEnvironment}>New environment…</Button>
      {/if}
    {/snippet}
  </Banner>
{/if}

<style>
  code { font-family: var(--mono); }
  code.fn { color: var(--fail); }
</style>
