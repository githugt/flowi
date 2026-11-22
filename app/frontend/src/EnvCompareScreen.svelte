<script lang="ts">
  import ScreenPage from './ScreenPage.svelte'
  // The environments side by side: a row for each variable, a column for each environment. Differing values, missing
  // variables and secrets without a value stand out, so "why does staging act unlike local" is one look, not three
  // editors. A secret shows only whether it holds a value.
  import EmptyState from './EmptyState.svelte'
  import Button from './Button.svelte'
  import Toggle from './Toggle.svelte'
  import Dropdown from './Dropdown.svelte'
  import { EnvironmentService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { app } from './lib/app.svelte'
  import { errorText } from './lib/errors'
  import { LOADING, READY, failed, type LoadState } from './lib/loadstate'
  import { cellLabel, cellText, pickGroup, shownKind, summary, visibleRows } from './lib/envcompare'
  import { compareChoice } from './lib/envgroup.svelte'
  import type { EnvCompareView } from './lib/types'

  let view = $state<EnvCompareView>({ group: '', groups: [], envs: [], rows: [], notes: [] })
  let load = $state<LoadState>(LOADING)
  let onlyIssues = $state(false)

  // One group at a time: the one chosen here or on Manage environments. Until one is chosen the screen picks it (the
  // default group, or the first group with two environments to compare), once the groups are known.
  const group = $derived(compareChoice.group ?? '')

  // Read again whenever the environments change (a save reloads the workspace tree), when the group changes, and on
  // Refresh. An answer that arrives after a newer question is dropped.
  let asked = 0
  async function reload() {
    const mine = ++asked
    const want = group
    try {
      const v = (await EnvironmentService.Compare(want)) as unknown as EnvCompareView
      if (mine !== asked) return
      view = v
      load = READY
      const pick = pickGroup(v.groups, compareChoice.group)
      if (pick !== want || compareChoice.group === null) compareChoice.group = pick
    } catch (e) {
      if (mine === asked) load = failed(e)
    }
  }
  $effect(() => {
    void app.tree?.environments
    void group
    void reload()
  })

  const rows = $derived(visibleRows(view.rows, onlyIssues))
  const groupOptions = $derived(view.groups.map((g) => ({ value: g.name, label: g.label, count: g.count })))
</script>

{#snippet picker()}
  <span class="muted">Group</span>
  <Dropdown options={groupOptions} value={group} onpick={(g) => (compareChoice.group = g)} label="Group" title="The group of environments to compare" />
{/snippet}

<ScreenPage width={null}>
  <p class="note">Every variable of the group's environments side by side. A value that is not the same everywhere is tinted, and where the environments read it as different types (text, number, true or false, JSON) each cell says which. <span class="gap">—</span> means the environment does not define it, and a secret shows only whether it has a value in the vault.</p>
  {#if view.error}<p class="err">{view.error}</p>{/if}
  {#each view.notes as note (note)}<p class="note">{note}</p>{/each}
  {#if load.status === 'error'}
    <p class="err" role="alert">Could not compare the environments: {load.error} <Button small onclick={reload}>Try again</Button></p>
  {:else if load.status === 'ready' && view.envs.length < 2}
    {#if view.groups.length > 1}<div class="bar">{@render picker()}</div>{/if}
    <EmptyState icon="key" title={view.envs.length ? 'There is only one environment in this group' : 'No environment in this group'} hint="Comparing needs at least two. Make another one, or move one here on Manage environments." compact />
  {:else if load.status === 'ready'}
    <div class="bar">
      {#if view.groups.length > 1}{@render picker()}{/if}
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
                <td class="cell {c.state}" class:warn={c.state === 'missing' || c.state === 'secret-unset'} title={cellLabel(view.envs[i], r.name, c, r.kindsDiffer)}>
                  <span class="text">{cellText(c)}</span>{#if shownKind(r, c)}<span class="from kind">as {shownKind(r, c)}</span>{/if}{#if c.state === 'inherited'}<span class="from">from {c.source}</span>{/if}
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
  .differs .cell.set, .differs .cell.inherited, .differs .cell.secret, .differs .cell.secret-unset { background: var(--tint-accent); }
  .cell.inherited .text { font-style: italic; }
  .kind { color: var(--text); }
  .from { display: block; color: var(--muted); font: var(--fs-xs) var(--font, system-ui); }
  .cell.warn, .gap { color: var(--warn); }
  .cell.secret { color: var(--ok); }
  .none { text-align: center; color: var(--muted); padding: var(--sp-6); }
</style>
