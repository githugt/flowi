<script lang="ts">
  // What shows beside a matrix while it runs: which cell it is on, and a Cancel that stops it after the cell in hand,
  // keeping the cells that already ran. Mounted only while a matrix runs, so each run starts from nothing.
  import Button from './Button.svelte'
  import { onEvent } from './lib/events'
  import { runningText, type MatrixProgress } from './lib/matrixprogress'

  let { node, oncancel }: { node: string; oncancel: () => void } = $props()

  let progress = $state<MatrixProgress | null>(null)
  let cancelling = $state(false)

  $effect(() => {
    const forNode = node
    return onEvent('matrix', (d: MatrixProgress) => {
      if (d?.node === forNode) progress = d
    })
  })

  function cancel() {
    cancelling = true
    oncancel()
  }
</script>

<span class="running" role="status">{cancelling ? 'Cancelling…' : runningText(progress)}</span>
<Button small disabled={cancelling} onclick={cancel}>Cancel</Button>

<style>
  .running { color: var(--muted); font-variant-numeric: tabular-nums; }
</style>
