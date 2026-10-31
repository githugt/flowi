<script lang="ts">
  import type { VarsView } from './lib/types'

  let { vars }: { vars: VarsView } = $props()
  let open = $state<string | null>(null)

  const valueOf = $derived(new Map((vars.values ?? []).map((v) => [v.name, v.value])))
  const groups = $derived(
    [
      { title: 'Variables', names: vars.set, warn: false },
      { title: 'Secrets', names: vars.secrets, warn: false },
      { title: 'Never set', names: vars.unset, warn: true },
    ].filter((g) => g.names.length),
  )
</script>

<svelte:window onpointerdown={() => (open = null)} />

{#if groups.length}
  <div class="bar">
    {#each groups as g}
      <div class="chip">
        <button class:warn={g.warn} onpointerdown={(e) => e.stopPropagation()} onclick={() => (open = open === g.title ? null : g.title)}>
          {g.title} {g.names.length}
        </button>
        {#if open === g.title}
          <div class="pop" role="dialog" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
            {#each g.names as n}
              <div>{n}{#if g.title === 'Variables' && valueOf.has(n)}<span class="val" title="What it held when the last run ended"> = {valueOf.get(n)}</span>{/if}</div>
            {/each}
            {#if g.title === 'Variables'}
              {@const known = new Set(vars.set)}
              {#each (vars.values ?? []).filter((v) => !known.has(v.name)) as v}
                <div>{v.name}<span class="val" title="Set while the flow ran"> = {v.value}</span></div>
              {/each}
            {/if}
          </div>
        {/if}
      </div>
    {/each}
  </div>
{/if}

<style>
  .bar { display: flex; gap: 6px; padding: 6px 10px; border-top: 1px solid var(--line); background: var(--panel); }
  .chip { position: relative; }
  button {
    background: none;
    color: var(--muted);
    border: 1px solid var(--line);
    border-radius: var(--r-pill);
    padding: 2px 10px;
    font: inherit;
    font-size: var(--fs-sm);
  }
  button:hover { border-color: var(--accent); color: var(--text); }
  .warn { color: var(--warn); border-color: color-mix(in srgb, var(--warn) 50%, var(--line)); }
  .val { color: var(--muted); word-break: break-all; }
  .pop {
    position: absolute;
    bottom: 100%;
    left: 0;
    margin-bottom: 6px;
    min-width: 160px;
    max-height: 240px;
    overflow: auto;
    padding: 8px 12px;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: var(--r-lg);
    box-shadow: var(--shadow);
    font: var(--fs-sm) var(--mono);
    z-index: 20;
  }
</style>
