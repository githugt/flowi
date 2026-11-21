<script lang="ts">
  import type { FlowView } from './lib/types'
  import Inspector from './Inspector.svelte'
  import DataTab from './DataTab.svelte'
  import RunsTab from './RunsTab.svelte'
  import Splitter from './Splitter.svelte'
  import Tabs from './Tabs.svelte'

  let { view, editing = true, onaddinside }: { view: FlowView; editing?: boolean; onaddinside?: (lane: number, e: MouseEvent) => void } = $props()
  let tab = $state<'settings' | 'data' | 'runs'>('settings')
  let width = $state(320)
</script>

<Splitter name="flow-side" side="right" min={240} max={640} bind:width />
<aside style:width={`${width}px`}>
  <div class="bar"><Tabs label="Inspector" fill items={[{ id: 'settings', label: 'Settings' }, { id: 'data', label: 'Data' }, { id: 'runs', label: 'Runs' }]} active={tab} onpick={(id) => (tab = id as typeof tab)} /></div>
  <div class="body">
    {#if tab === 'settings'}
      <Inspector {view} {editing} {onaddinside} />
    {:else if tab === 'data'}
      <div class="pad"><DataTab {view} /></div>
    {:else}
      <div class="pad"><RunsTab {view} /></div>
    {/if}
  </div>
</aside>

<style>
  aside { flex: none; display: flex; flex-direction: column; border-left: 1px solid var(--line); background: var(--bg); min-height: 0; }
  .bar { flex: none; padding: 0 8px; }
  .body { flex: 1; min-height: 0; overflow: auto; display: flex; flex-direction: column; }
  .pad { padding: 12px; }
</style>
