<script lang="ts">
  import type { FlowView } from './lib/types'
  import Inspector from './Inspector.svelte'
  import DataTab from './DataTab.svelte'
  import RunsTab from './RunsTab.svelte'
  import Splitter from './Splitter.svelte'

  let { view, editing = true }: { view: FlowView; editing?: boolean } = $props()
  let tab = $state<'settings' | 'data' | 'runs'>('settings')
  let width = $state(320)
</script>

<Splitter name="flow-side" side="right" min={240} max={640} bind:width />
<aside style:width={`${width}px`}>
  <div class="tabs" role="tablist">
    {#each [['settings', 'Settings'], ['data', 'Data'], ['runs', 'Runs']] as [id, label]}
      <button role="tab" aria-selected={tab === id} class:active={tab === id} onclick={() => (tab = id as typeof tab)}>{label}</button>
    {/each}
  </div>
  <div class="body">
    {#if tab === 'settings'}
      <Inspector {view} {editing} />
    {:else if tab === 'data'}
      <div class="pad"><DataTab {view} /></div>
    {:else}
      <div class="pad"><RunsTab {view} /></div>
    {/if}
  </div>
</aside>

<style>
  aside { flex: none; display: flex; flex-direction: column; border-left: 1px solid var(--line); background: var(--panel); min-height: 0; }
  .tabs { display: flex; border-bottom: 1px solid var(--line); flex: none; }
  .tabs button {
    flex: 1;
    background: none;
    border: 0;
    border-bottom: 2px solid transparent;
    color: var(--muted);
    font: inherit;
    padding: 8px 0;
  }
  .tabs button.active { color: var(--text); border-bottom-color: var(--accent); }
  .body { flex: 1; min-height: 0; overflow: auto; display: flex; flex-direction: column; }
  .pad { padding: 12px; }
</style>
