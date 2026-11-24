<script lang="ts">
  // The bar under the address: what type of request this is (HTTP, gRPC, a message protocol, a database, a data file)
  // with a menu to switch it, the request's own name (optional), and the version.
  // Before: the version picker sat in the address row, the type could not be changed, and a request had nowhere to
  // be given a name.
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { Snippet } from 'svelte'
  import type { DropOption, RequestView } from './lib/types'
  import Dropdown from './Dropdown.svelte'
  import TextInput from './TextInput.svelte'
  import VersionPicker from './VersionPicker.svelte'

  let { view, node, onmanage, onnew, children }: { view: RequestView; node: string; onmanage: () => void; onnew: () => void; children?: Snippet } = $props()

  const options = $derived<DropOption[]>(view.types.map((t) => ({ value: t.id, label: t.label, icon: t.icon })))
</script>

<div class="bar">
  <div class="type">
    <Dropdown {options} value={view.type} icon onpick={(id) => RequestService.SwitchType(node, id)} title="Type of request (the protocol, database or file format is chosen in the address row): switching keeps the checks, headers and documentation and what fits the new type; Discard undoes it." />
  </div>
  <div class="name"><TextInput label="Name of the request" placeholder="Name (optional)" value={view.name} ontext={(t) => RequestService.SetName(node, t)} title="A name for this request. It is kept in the request file when you save." /></div>
  <VersionPicker {view} {node} {onmanage} {onnew} />
  {#if children}<div class="acts">{@render children()}</div>{/if}
</div>

<style>
  /* A quiet bar: the address is what the screen is about, so this one is shorter, smaller and its controls are borderless until
     hovered; a thin line under it separates it from the section tabs, with muted words. Its controls draw an edge only when pointed at or focused. */
  .bar { display: flex; align-items: center; gap: 6px; padding: 0 16px 3px; border-bottom: 1px solid var(--line); font-size: var(--fs-sm); --bar-h: 24px; }
  .name { flex: 1; min-width: 0; }
  .bar :global(.btn), .bar :global(input), .bar :global(.acts button) { min-height: var(--bar-h); height: var(--bar-h); box-sizing: border-box; font-size: var(--fs-sm); background: transparent; border-color: transparent; color: var(--muted); font-weight: 500; }
  .bar :global(.btn:hover), .bar :global(.acts button:hover:not(:disabled)) { background: var(--raised-3); border-color: var(--line); color: var(--text); }
  .bar :global(input) { color: var(--text); }
  .bar :global(input:hover) { border-color: var(--line); }
  .bar :global(input:focus) { border-color: var(--accent); background: var(--panel); }
  .bar :global(input::placeholder) { color: var(--muted); opacity: 0.7; }
  .type :global(.btn svg:first-child) { color: var(--accent); }
  .acts { display: flex; align-items: center; gap: 4px; flex: none; }
  .acts :global(.send) { align-items: center; }
  .bar .acts :global(button.pending) { color: var(--accent); }
</style>
