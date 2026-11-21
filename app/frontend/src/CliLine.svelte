<script lang="ts">
  // The command line that does what the form above it does, with a Copy button. Shown where a setup is tried, so it
  // can move to a script or CI.
  import Button from './Button.svelte'
  import { copy } from './lib/clip'

  let { command }: { command: string } = $props()
</script>

<div class="cli">
  <span class="label">Same thing in a terminal or CI</span>
  <div class="row"><code>{command}</code><Button small onclick={() => copy(command, 'the command')}>Copy</Button></div>
</div>

<style>
  .cli { display: flex; flex-direction: column; gap: 3px; margin: 8px 0; }
  .label { color: var(--muted); font-size: var(--fs-xs); }
  .row { display: flex; align-items: center; gap: 8px; }
  code { flex: 1; min-width: 0; padding: 5px 8px; border: 1px solid var(--line); border-radius: var(--r-md); background: var(--sunk); font: var(--fs-sm) var(--mono, ui-monospace, monospace); overflow-x: auto; white-space: nowrap; -webkit-user-select: text; user-select: text; }
</style>
