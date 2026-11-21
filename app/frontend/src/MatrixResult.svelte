<script lang="ts">
  // The outcome of a matrix run: how many passed, a grid when two things vary (a list otherwise), and why each cell that
  // did not pass did not. Shared by the flow's matrix and the request's.
  import { gridComplete, stoppedNote } from './lib/matrixprogress'
  import type { MatrixView } from './lib/types'

  let { matrix }: { matrix: MatrixView } = $props()

  // The grid is rows × columns when two things vary and every cell ran; any other case is a list, so a matrix that was
  // cancelled shows what ran without holes in a grid.
  const grid = $derived.by(() => {
    if (!gridComplete(matrix)) return null
    const [rows, cols] = matrix.dims
    return { rows: rows.values, cols: cols.values, at: (r: number, c: number) => matrix.cells[r * cols.values.length + c] }
  })
  const mark = (s: string) => ({ passed: '✓', failed: '✗', error: '!', cancelled: '–' })[s] ?? '?'
</script>

<p class="result">{matrix.passed} of {matrix.cells.length} passed</p>
{#if matrix.cancelled}<p class="why" role="status">{stoppedNote(matrix)}</p>{/if}
{#if grid}
  <table class="grid">
    <caption class="sr-only">{matrix.passed} of {matrix.cells.length} passed: {matrix.dims[0].name} down, {matrix.dims[1].name} across</caption>
    <thead><tr><th scope="col">{matrix.dims[0].name} ↓ · {matrix.dims[1].name} →</th>{#each grid.cols as c}<th scope="col">{c}</th>{/each}</tr></thead>
    <tbody>
      {#each grid.rows as r, ri}
        <tr><th scope="row">{r}</th>{#each grid.cols as _, ci}{@const cell = grid.at(ri, ci)}<td class={cell.status} title={cell.why ?? ''}><span class="sr-only">{cell.status}: </span>{mark(cell.status)}{cell.detail ? ' ' + cell.detail : ''}</td>{/each}</tr>
      {/each}
    </tbody>
  </table>
{:else}
  {#each matrix.cells as c}<div class="cell"><span class={c.status}>{mark(c.status)}</span> {c.labels.join(' · ')}{c.detail ? ' · ' + c.detail : ''}</div>{/each}
{/if}
{#each matrix.cells.filter((c) => c.status !== 'passed' && c.why) as c}<p class="why"><strong>{c.labels.join(' · ')}</strong>: {c.why}</p>{/each}

<style>
  .result { margin: 4px 0 0; font-weight: 600; }
  table.grid { border-collapse: collapse; font-size: var(--fs-sm); margin: 4px 0; }
  table.grid th, table.grid td { border: 1px solid var(--line); padding: 4px 12px; text-align: center; }
  table.grid th { color: var(--muted); font-weight: 600; }
  .passed { color: var(--ok); }
  .failed { color: var(--fail); }
  .error { color: var(--warn); }
  .cancelled { color: var(--muted); }
  .why { margin: 2px 0; font-size: var(--fs-sm); color: var(--muted); }
</style>
