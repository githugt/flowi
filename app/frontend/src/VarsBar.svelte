<script lang="ts">
  import Popover from './Popover.svelte'
  import Button from './Button.svelte'
  import type { VarsView } from './lib/types'

  // onselect shows a step on the canvas: a variable's popover lists the steps that write and read it.
  let { vars, onselect }: { vars: VarsView; onselect?: (step: string) => void } = $props()
  let open = $state<string | null>(null)
  let at = $state({ x: 0, y: 0 })
  // The variable whose writers and readers are listed.
  let shown = $state<string | null>(null)
  const usesOf = $derived(new Map((vars.uses ?? []).map((u) => [u.name, u])))

  const valueOf = $derived(new Map((vars.values ?? []).map((v) => [v.name, v.value])))
  const groups = $derived(
    [
      { title: 'Variables', names: vars.set, warn: false },
      { title: 'Secrets', names: vars.secrets, warn: false },
      { title: 'Never set', names: vars.unset, warn: true },
    ].filter((g) => g.names.length),
  )
</script>

{#if groups.length}
  <div class="bar">
    {#each groups as g}
      <div class="chip">
        <button class:warn={g.warn} aria-haspopup="dialog" aria-expanded={open === g.title} onclick={(e) => { const r = e.currentTarget.getBoundingClientRect(); at = { x: r.left, y: r.top }; open = open === g.title ? null : g.title }}>
          {g.title} {g.names.length}
        </button>
        {#if open === g.title}
          <Popover x={at.x} y={at.y} above width={300} height={240} maxHeight="240px" label={g.title} onclose={() => (open = null)}>
            <div class="list">
            {#each g.names as n}
              <div>
                {#if onselect && g.title !== 'Secrets' && usesOf.has(n)}
                  <Button variant="ghost" small pressed={shown === n} title="Who writes and reads {n}" onclick={() => (shown = shown === n ? null : n)}>{n}</Button>
                {:else}{n}{/if}{#if g.title === 'Variables' && valueOf.has(n)}<span class="val" title="What it held when the last run ended"> = {valueOf.get(n)}</span>{/if}
                {#if shown === n}
                  {@const u = usesOf.get(n)}
                  <div class="uses">
                    <span class="lbl">Written by</span>
                    {#each u?.writers ?? [] as id (id)}<Button variant="ghost" small onclick={() => onselect?.(id)}>{id}</Button>{:else}<span class="val">no step</span>{/each}
                    <span class="lbl">Read by</span>
                    {#each u?.readers ?? [] as id (id)}<Button variant="ghost" small onclick={() => onselect?.(id)}>{id}</Button>{:else}<span class="val">no step</span>{/each}
                  </div>
                {/if}
              </div>
            {/each}
            {#if g.title === 'Variables'}
              {@const known = new Set(vars.set)}
              {#each (vars.values ?? []).filter((v) => !known.has(v.name)) as v}
                <div>{v.name}<span class="val" title="Set while the flow ran"> = {v.value}</span></div>
              {/each}
            {/if}
            </div>
          </Popover>
        {/if}
      </div>
    {/each}
  </div>
{/if}

<style>
  .bar { display: flex; gap: 6px; padding: 6px 10px; border-top: 1px solid var(--line); background: var(--panel); }
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
  .uses { display: flex; flex-wrap: wrap; gap: 2px 4px; align-items: center; padding: 2px 0 6px 10px; font-size: var(--fs-xs); }
  .lbl { color: var(--muted); }
  .list { font: var(--fs-sm) var(--mono); overflow: auto; }
</style>
