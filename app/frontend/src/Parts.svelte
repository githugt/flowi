<script lang="ts">
  import type { PartView } from './lib/types'
  import Code from './Code.svelte'
  import Button from './Button.svelte'
  import { sensitiveValue } from './lib/sensitive'
  import { copy } from './lib/clip'
  import { filterHeaders, headerText, isSetCookie, parseSetCookie } from './lib/headerview'

  let { parts }: { parts: PartView[] } = $props()
  // The filter of each headers table, by heading: shown only when a table is long enough to need one.
  let queries = $state<Record<string, string>>({})
  const filterFrom = 8
</script>

{#snippet rows(p: PartView)}
  {@const all = p.rows ?? []}
  {@const shown = filterHeaders(all, queries[p.heading] ?? '')}
  {#if p.heading === 'Headers' && all.length > 1}
    <div class="tools">
      {#if all.length >= filterFrom}<input type="search" placeholder="Filter headers" aria-label="Filter headers" bind:value={queries[p.heading]} />{/if}
      <Button small onclick={() => copy(headerText(shown), 'the headers')}>Copy</Button>
    </div>
  {/if}
  <table>
    <tbody>
      {#each shown as r}
        <tr>
          <td class="k">{r.key}</td>
          <td class:sensitive={sensitiveValue(r.key, r.value)}>
            {#if isSetCookie(r)}
              {@const c = parseSetCookie(r.value)}
              {#if c.name}<strong>{c.name}</strong> = {c.value}{:else}{c.value}{/if}
              {#each c.attrs as a}<span class="attr">{a}</span>{/each}
            {:else}{r.value}{/if}
          </td>
        </tr>
      {/each}
      {#if shown.length === 0}<tr><td class="k" colspan="2">No header matches.</td></tr>{/if}
    </tbody>
  </table>
{/snippet}

{#each parts as p}
  <section>
    {#if p.heading}<h5>{p.heading}</h5>{/if}
    {#if p.kind === 'lines'}
      {#each p.lines ?? [] as l}<div class="line">{l}</div>{/each}
    {:else if p.kind === 'table'}
      {#if p.collapsed}
        <details>
          <summary>{p.rows?.length ?? 0} {p.rows?.length === 1 ? 'header' : 'headers'}</summary>
          {@render rows(p)}
        </details>
      {:else}
        {@render rows(p)}
      {/if}
    {:else}
      <Code text={p.code ?? ''} />
    {/if}
  </section>
{/each}

<style>
  section { margin-bottom: 8px; }
  h5 { margin: 0 0 3px; color: var(--muted); font-size: var(--fs-xs); font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; }
  .line { font-family: var(--mono); font-size: var(--fs-sm); word-break: break-all; }
  table { border-collapse: collapse; width: 100%; font-size: var(--fs-sm); }
  td { padding: 2px 6px 2px 0; vertical-align: top; word-break: break-all; }
  .tools { display: flex; gap: 6px; align-items: center; margin: 4px 0; }
  .tools input { flex: 1; min-width: 0; }
  .attr { display: inline-block; margin: 0 4px 0 6px; padding: 0 6px; border: 1px solid var(--line); border-radius: var(--r-sm, 4px); color: var(--muted); font-size: var(--fs-xs); }
  .k { color: var(--muted); white-space: nowrap; }
</style>
