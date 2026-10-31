<script lang="ts">
  // The Details tab of the footer: what the tab in front is, in plain labelled values (a request's method, URL,
  // version, overlay, auth, environment, hooks and last result; a flow's inputs, steps and last run), where the
  // headers only hint at them. It reads the values again when the request or flow changes or another tab comes forward.
  import { PanelsService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { app } from './lib/app.svelte'
  import type { DetailsView } from './lib/types'

  let d = $state<DetailsView>({ kind: '', title: '', rows: [] })
  // Anything the open tab's view carries that these values come from.
  const stamp = $derived(JSON.stringify([app.active, app.requests[app.active]?.selected, app.requests[app.active]?.flowiOverlay, app.requests[app.active]?.response?.at, app.flows[app.active]?.rev, app.flows[app.active]?.run?.summary]))
  $effect(() => {
    void stamp
    const node = app.active
    PanelsService.Details(node).then((v) => {
      if (node === app.active) d = v as unknown as DetailsView
    })
  })
</script>

{#if d.kind}
  <h4>{d.title} <span class="kind">{d.kind}</span></h4>
  <dl>
    {#each d.rows as r (r.label)}
      <dt>{r.label}</dt>
      <dd>{r.value}</dd>
    {/each}
  </dl>
{:else}
  <p class="muted">Open a request or a flow to see its details here.</p>
{/if}

<style>
  h4 { margin: 0 0 6px; font-size: var(--fs-md); }
  .kind { color: var(--muted); font-weight: 400; font-size: var(--fs-sm); margin-left: 6px; }
  dl { display: grid; grid-template-columns: 120px 1fr; gap: 3px 12px; margin: 0; }
  dt { color: var(--muted); }
  dd { margin: 0; overflow-wrap: anywhere; }
  .muted { color: var(--muted); font-family: inherit; }
</style>
