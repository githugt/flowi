<script lang="ts">
  import type { PartView } from './lib/types'
  import Code from './Code.svelte'

  let { parts }: { parts: PartView[] } = $props()
</script>

{#each parts as p}
  <section>
    {#if p.heading}<h5>{p.heading}</h5>{/if}
    {#if p.kind === 'lines'}
      {#each p.lines ?? [] as l}<div class="line">{l}</div>{/each}
    {:else if p.kind === 'table'}
      {#if p.collapsed}
        <details>
          <summary>{p.rows?.length ?? 0} rows</summary>
          <table><tbody>{#each p.rows ?? [] as r}<tr><td class="k">{r.key}</td><td>{r.value}</td></tr>{/each}</tbody></table>
        </details>
      {:else}
        <table><tbody>{#each p.rows ?? [] as r}<tr><td class="k">{r.key}</td><td>{r.value}</td></tr>{/each}</tbody></table>
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
  .k { color: var(--muted); white-space: nowrap; }
</style>
