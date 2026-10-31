<script lang="ts">
  // The sections only a dataset request has: the query, and the values it is sent with.
  import { RequestService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import type { DatasetBlock, RequestView } from './lib/types'
  import CodeArea from './CodeArea.svelte'
  import TextField from './TextField.svelte'
  import EmptyState from './EmptyState.svelte'

  let { view, section }: { view: RequestView; section: string } = $props()
  const node = $derived(view.node)
  const block = $derived(view.dataset!.block)
  const params = $derived(block.Params ?? [])
  // Placeholders are $1, $2 … for PostgreSQL; MySQL and MariaDB use ?; SQLite takes either.
  const placeholder = $derived(block.Driver === 'postgres' ? '$1, $2 …' : block.Driver === 'sqlite' ? '? or $1, $2 …' : '?')

  // Writes: the block comes from the latest view, with one change.
  function put(change: (b: DatasetBlock) => void) {
    const b = structuredClone($state.snapshot(block)) as DatasetBlock
    change(b)
    RequestService.SetDataset(node, b as never)
  }
  const setParam = (i: number, text: string) => put((b) => ((b.Params ??= []), (b.Params[i] = text)))
  const addParam = () => put((b) => ((b.Params ??= []), b.Params.push('')))
  const removeParam = (i: number) => put((b) => b.Params?.splice(i, 1))
  const setLimit = (text: string) => put((b) => (b.Limit = Math.max(0, Math.trunc(Number(text)) || 0)))
</script>

{#if section === 'Query'}
  <CodeArea
    value={block.Query}
    lang="sql"
    oninput={(t) => put((b) => (b.Query = t))}
    empty={{ icon: 'body', title: 'No query', hint: 'Click to write the query: one SELECT, WITH, VALUES, SHOW, EXPLAIN or DESCRIBE.' }}
  />
  <p class="muted note">
    Read-only: the query is sent in a read-only transaction, and a statement that changes data is refused. Put values in <strong>Values</strong> and write
    them as <code>{placeholder}</code>: they travel apart from the statement, so a value can never change what it means.
    For the strongest guarantee, connect as a database user that can only read.
  </p>
  <label class="limit">Keep at most
    <input class="mini" inputmode="numeric" placeholder="1000" value={block.Limit || ''} onchange={(e) => setLimit(e.currentTarget.value)} aria-label="Most rows kept" />
    rows
  </label>
{:else if section === 'Values'}
  {#if params.length === 0}
    <EmptyState icon="params" title="No values" hint={`Each value is sent for a placeholder of the query (${placeholder}), in order. Templates such as {{id}} work here.`} compact>
      <button onclick={addParam}>Add value</button>
    </EmptyState>
  {:else}
    {#each params as value, i}
      <div class="row">
        <span class="n" title="The placeholder this value is sent for">{block.Driver === 'postgres' ? '$' + (i + 1) : '#' + (i + 1)}</span>
        <TextField mono {value} placeholder={'{{id}}'} oninput={(t) => setParam(i, t)} />
        <button class="x" aria-label={`Remove value ${i + 1}`} title="Remove" onclick={() => removeParam(i)}>×</button>
      </div>
    {/each}
    <button class="add" onclick={addParam}>Add value</button>
  {/if}
{/if}

<style>
  .note { margin: 8px 0 0; font-size: var(--fs-xs); }
  .limit { display: inline-flex; gap: 6px; align-items: center; margin-top: 8px; color: var(--muted); font-size: var(--fs-xs); }
  .mini { width: 5em; box-sizing: border-box; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 6px; font: inherit; }
  .row { display: grid; grid-template-columns: 2.4em 1fr auto; gap: 6px; align-items: center; margin-bottom: 6px; }
  .n { color: var(--muted); font-family: var(--mono, monospace); font-size: var(--fs-xs); text-align: right; }
  .x, .add { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); font: inherit; padding: 4px 10px; cursor: pointer; }
  .x { padding: 2px 8px; }
</style>
