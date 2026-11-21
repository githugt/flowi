<script lang="ts">
  import { onMount } from 'svelte'
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import Button from './Button.svelte'
  import { isBlankQuery, tableTitle } from './lib/datatables'
  import type { DataTableView } from './lib/types'
  import { ui } from './lib/ui.svelte'

  // The tables of the data files this request reads, as chips above the statement: a click writes the SELECT that looks
  // at one. Before: the person had to know the table names to write FROM sales. The parent keys this on the server and
  // the format, so a new path reads the files again.
  let { node, query, setQuery }: { node: string; query: string; setQuery: (text: string) => void } = $props()

  let tables = $state<DataTableView[]>([])
  let note = $state('')
  let loaded = $state(false)
  // The path is typed one letter at a time and the parent keys this on it, so the files are read once typing pauses.
  onMount(() => {
    let gone = false
    const timer = setTimeout(async () => {
      try {
        const v = await RequestService.DataTables(node)
        if (gone) return
        tables = (v?.tables ?? []) as DataTableView[]
        note = v?.note ?? ''
      } catch {
        if (!gone) note = 'The tables could not be read.'
      } finally {
        if (!gone) loaded = true
      }
    }, 400)
    return () => {
      gone = true
      clearTimeout(timer)
    }
  })

  function pick(t: DataTableView) {
    if (isBlankQuery(query) || query.trim() === t.select) return setQuery(t.select)
    ui.confirm('Replace the query?', `The query becomes ${t.select}.`, () => setQuery(t.select), 'Replace')
  }
</script>

{#if loaded && (tables.length || note)}
  <div class="tables" role="group" aria-label="Tables">
    <span class="lbl">Tables</span>
    {#each tables as t (t.file + '/' + t.sheet + '/' + t.name)}
      <Button small title={tableTitle(t)} onclick={() => pick(t)}>{t.name}<span class="n"> · {t.rows}</span></Button>
    {/each}
    {#if note}<span class="note">{note}</span>{/if}
  </div>
{/if}

<style>
  .tables { display: flex; flex-wrap: wrap; gap: 4px 6px; align-items: center; padding: 4px 0 8px; }
  .lbl, .note { color: var(--muted); font-size: var(--fs-xs); }
  .n { color: var(--muted); font-weight: 400; }
</style>
