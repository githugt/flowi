<script lang="ts">
  import ScreenPage from './ScreenPage.svelte'
  // The environments side by side: a row for each variable, a column for each environment. Differing values, missing
  // variables and secrets without a value stand out, so "why does staging act unlike local" is one look, not three
  // editors. A secret shows only whether it holds a value.
  import EmptyState from './EmptyState.svelte'
  import Button from './Button.svelte'
  import Toggle from './Toggle.svelte'
  import { EnvironmentService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { app } from './lib/app.svelte'
  import { errorText } from './lib/errors'
  import { LOADING, READY, failed, type LoadState } from './lib/loadstate'
  import { cellLabel, cellText, summary, visibleRows } from './lib/envcompare'
  import type { EnvCompareView } from './lib/types'

  let view = $state<EnvCompareView>({ envs: [], rows: [] })
  let load = $state<LoadState>(LOADING)
  let onlyIssues = $state(false)

  // Read again whenever the environments change (a save reloads the workspace tree), and on Refresh. An answer that
  // arrives after a newer question is dropped.
  let asked = 0
  async function reload() {
    const mine = ++asked
    try {
      const v = (await EnvironmentService.Compare()) as unknown as EnvCompareView
      if (mine === asked) (view = v), (load = READY)
    } catch (e) {
      if (mine === asked) load = failed(e)
    }
  }
  $effect(() => {
    void app.tree?.environments
    void reload()
  })

  const rows = $derived(visibleRows(view.rows, onlyIssues))
</script>

<ScreenPage width={null}>
  <p class="note">Every environment's variables side by side. A value that is not the same everywhere is tinted, <span class="gap">—</span> means the environment does not define it, and a secret shows only whether it has a value in the vault.</p>
  {#if view.error}<p class="err">{view.error}</p>{/if}
  {#if load.status === 'error'}
    <p class="err" role="alert">Could not compare the environments: {load.error} <Button small onclick={reload}>Try again</Button></p>
  {:else if load.status === 'ready' && view.envs.length < 2}
    <EmptyState icon="key" title={view.envs.length ? 'There is only one environment' : 'No environment yet'} hint="Comparing needs at least two. Make another one from the environment chip in the tab strip." compact />
  {:else if load.status === 'ready'}
    <div class="bar">
      <Toggle checked={onlyIssues} onchange={(on) => (onlyIssues = on)}>Only what differs or has a gap</Toggle>
      <span class="spacer"></span>
      <span class="muted" role="status">{summary(view.rows)}</span>
      <Button small onclick={reload}>Refresh</Button>
    </div>
    <div class="scroll">
      <table>
        <thead>
          <tr>
            <th scope="col" class="name">Variable</th>
            {#each view.envs as env (env)}<th scope="col">{env}</th>{/each}
          </tr>
        </thead>
        <tbody>
          {#each rows as r (r.name)}
            <tr class:differs={r.differs}>
              <th scope="row" class="name"><code>{r.name}</code></th>
              {#each r.cells as c, i (view.envs[i])}
                <td class="cell {c.state}" class:warn={c.state === 'missing' || c.state === 'secret-unset'} title={cellLabel(view.envs[i], r.name, c)}>
                  <span class="text">{cellText(c)}</span>{#if c.kind}<span class="from">as {c.kind}</span>{/if}{#if c.state === 'inherited'}<span class="from">from {c.source}</span>{/if}
                </td>
              {/each}
            </tr>
          {:else}
            <tr><td class="none" colspan={view.envs.length + 1}>Nothing differs and nothing is missing.</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</ScreenPage>

<style>
  .note, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 6px; }
  .err { color: var(--fail); font-size: var(--fs-sm); }
  .bar { display: flex; align-items: center; gap: var(--sp-3); margin: var(--sp-3) 0 var(--sp-2); }
  .spacer { flex: 1; }
  .scroll { overflow: auto; border: 1px solid var(--line); border-radius: var(--r-lg); }
  table { border-collapse: collapse; width: 100%; font-size: var(--fs-sm); }
  th, td { text-align: left; padding: var(--sp-2) var(--sp-3); border-bottom: 1px solid var(--line); vertical-align: top; }
  thead th { position: sticky; top: 0; background: var(--raised); font-weight: 600; }
  tbody tr:last-child > * { border-bottom: 0; }
  .name { font-weight: 500; white-space: nowrap; }
  .cell { font-family: var(--mono); overflow-wrap: anywhere; max-width: 28ch; }
  .differs .cell.set, .differs .cell.inherited { background: var(--tint-accent); }
  .cell.inherited .text { font-style: italic; }
  .from { display: block; color: var(--muted); font: var(--fs-xs) var(--font, system-ui); }
  .cell.warn, .gap { color: var(--warn); }
  .cell.secret { color: var(--ok); }
  .none { text-align: center; color: var(--muted); padding: var(--sp-6); }
</style>
