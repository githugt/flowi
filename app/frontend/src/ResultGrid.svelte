<script lang="ts">
  // The rows of a dataset result as a table: shared by the response pane and the flow's output dock.
  import Button from './Button.svelte'
  import TextInput from './TextInput.svelte'
  import { cellOf } from './lib/cells'
  import { ariaSort, gridView, nextSort, type Sort } from './lib/gridview'
  import type { TableView } from './lib/types'

  let { table }: { table: TableView } = $props()

  // A header click sorts by that column (again: the other way, a third time: the original order); the box keeps the
  // rows with a cell that contains the word. Both only change what is shown: the # column is the row's place in the
  // result, so it does not move with the sort.
  let sort = $state<Sort | null>(null)
  let query = $state('')
  const shown = $derived(gridView(table.rows, query, sort))
  const arrow = (column: number) => (sort?.column === column ? (sort.direction === 'asc' ? ' ▲' : ' ▼') : '')
</script>

<p class="muted rowsum">
  {table.rowCount} {table.rowCount === 1 ? 'row' : 'rows'}{#if table.truncated}. The store had more: raise “Keep at most … rows” in the Query tab to see them.{/if}
  {#if query.trim()}Showing {shown.length} of {table.rows.length}.{/if}
</p>
{#if table.columns.length}
  {#if table.rows.length > 1}
    <div class="find"><TextInput bind:value={query} placeholder="Filter rows" label="Filter rows" style="width: 16em" /></div>
  {/if}
  <div class="grid">
    <table>
      <caption class="sr-only">{table.rowCount} {table.rowCount === 1 ? 'row' : 'rows'}</caption>
      <thead>
        <tr>
          <th class="idx" scope="col">#</th>
          {#each table.columns as c, ci (ci)}
            <th scope="col" aria-sort={ariaSort(sort, ci)}><Button variant="ghost" small title="Sort by {c}" onclick={() => (sort = nextSort(sort, ci))}>{c}{arrow(ci)}</Button></th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#each shown as i (i)}
          <tr><td class="idx">{i + 1}</td>{#each table.rows[i] as v}{@const cell = cellOf(v)}<td class={cell.kind} title={cell.text.length > 40 ? cell.text : undefined}>{cell.text}</td>{/each}</tr>
        {:else}
          <tr><td class="none" colspan={table.columns.length + 1}>No row contains “{query}”.</td></tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<style>
  .muted { color: var(--muted); font-size: var(--fs-sm); }
  .rowsum { margin: 0 0 6px; }
  .find { margin: 0 0 6px; }
  .none { text-align: center; color: var(--muted); font-style: italic; }
  .grid { overflow: auto; border: 1px solid var(--line); border-radius: var(--r-md); }
  .grid table { border-collapse: collapse; width: 100%; font: var(--fs-sm)/1.5 ui-monospace, Menlo, monospace; -webkit-user-select: text; user-select: text; }
  .grid th, .grid td { padding: 3px 10px; border-bottom: 1px solid var(--line); text-align: left; white-space: nowrap; max-width: 28em; overflow: hidden; text-overflow: ellipsis; }
  .grid th { position: sticky; top: 0; background: var(--bg); color: var(--muted); font-weight: 600; font-size: var(--fs-xs); }
  .grid td.number { text-align: right; color: var(--syn-number); }
  .grid td.bool { color: var(--syn-atom); }
  .grid td.null { color: var(--muted); font-style: italic; }
  .grid .idx { color: var(--muted); text-align: right; width: 1%; }
</style>
