<script lang="ts">
  // The home of everything about environments: what to do with them (the cards), the groups they are sorted into, and
  // every environment with its groups, colour and base, so all of it can be seen and set in one place. A group is a set
  // of environments that belong together, such as the ones for one set of services; an environment can be in several,
  // and Compare environments shows one group at a time. A group needs no environment.
  import ScreenPage from './ScreenPage.svelte'
  import EmptyState from './EmptyState.svelte'
  import Button from './Button.svelte'
  import ColourPicker from './ColourPicker.svelte'
  import Menu, { type MenuItem } from './Menu.svelte'
  import IconButton from './IconButton.svelte'
  import Icon from './Icon.svelte'
  import { EnvironmentService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { app } from './lib/app.svelte'
  import { envSwatch } from './lib/envcolours'
  import { envTools, openCompare } from './lib/envtools'
  import { errorText } from './lib/errors'
  import { LOADING, READY, failed, type LoadState } from './lib/loadstate'
  import { reported } from './lib/reported'
  import { environmentNode } from './lib/treewalk'
  import { ui } from './lib/ui.svelte'
  import { groupMenuItems, groupName, toggledGroups } from './lib/envmanage'
  import { anchorAt, type MenuAt } from './lib/menuanchor'
  import type { EnvManageView, EnvMoveView, ManagedEnv } from './lib/types'

  let view = $state<EnvManageView>({ envs: [], groups: [], colours: [] })
  let load = $state<LoadState>(LOADING)

  // Read again when the environments change (a save reloads the workspace tree) and after each move. An answer that
  // arrives after a newer question is dropped.
  let asked = 0
  async function reload() {
    const mine = ++asked
    try {
      const v = (await EnvironmentService.Manage()) as unknown as EnvManageView
      if (mine === asked) (view = v), (load = READY)
    } catch (e) {
      if (mine === asked) load = failed(e)
    }
  }
  $effect(() => {
    void app.tree?.environments
    void reload()
  })

  const named = $derived(view.groups.filter((g) => g.name !== ''))

  // A change that would delete the comments of a file asks first, naming the environment; the same call with `drop`
  // set goes ahead. A call that fails rejects, so a prompt shows why and keeps what was typed.
  async function perform(call: (drop: boolean) => Promise<unknown>, drop = false): Promise<void> {
    const res = (await call(drop)) as unknown as EnvMoveView
    if (res.refused) {
      ui.confirm(`Rewrite ${res.refused}?`, `The file of ${res.refused} has comments. Changing it rewrites the file without them.`, () => perform(call, true), 'Rewrite')
      return
    }
    await reload()
  }

  const setGroups = (env: ManagedEnv, groups: string[]) => (drop: boolean) => EnvironmentService.MoveToGroups([env.file], groups, drop)

  // The groups of one environment are chips with a × and a menu of ticks: a group is switched on or off with one click,
  // and several can be switched without the menu closing. The menu is built from the environment as Go last said it, so
  // each tick follows the file.
  let menuFor = $state<string | null>(null) // the file of the environment whose menu is open
  let menuAt = $state<MenuAt>({ x: 0, y: 0 })
  const menuEnv = $derived(view.envs.find((e) => e.file === menuFor))
  const menuItems = $derived<MenuItem[]>(menuEnv ? groupMenuItems(view.groups, menuEnv.groups, (g) => changeGroups(menuEnv, toggledGroups(menuEnv.groups, g)), () => newGroupFor(menuEnv)) : [])

  function openGroups(e: MouseEvent, env: ManagedEnv) {
    if (menuFor === env.file) return void (menuFor = null)
    menuAt = anchorAt((e.currentTarget as HTMLElement).getBoundingClientRect(), { flip: true })
    menuFor = env.file
  }
  const changeGroups = (env: ManagedEnv, groups: string[]) => void reported(perform(setGroups(env, groups)), ui.toast)

  function newGroupFor(env: ManagedEnv) {
    menuFor = null
    ui.prompt({ title: `New group for ${env.name}`, label: 'Group name', initial: '', done: (name) => perform(setGroups(env, toggledGroups(env.groups, name.trim()))) })
  }

  const colourTo = (env: ManagedEnv, colour: string) => void reported(perform((drop) => EnvironmentService.ColourEnvironments([env.file], colour, drop)), ui.toast)

  // A group needs no environment: it is made on its own, and environments are moved into it afterwards.
  function newGroup() {
    ui.prompt({ title: 'New group', label: 'Group name', initial: '', done: async (name) => (await EnvironmentService.AddGroup(name.trim()), reload()) })
  }

  function renameGroup(name: string) {
    ui.prompt({ title: `Rename ${name}`, label: 'Group name', initial: name, done: (to) => perform((drop) => EnvironmentService.RenameGroup(name, to.trim(), drop)) })
  }

  function deleteGroup(name: string, count: number) {
    const text = count === 0 ? 'No environment is in it.' : `Its ${count === 1 ? 'environment moves' : `${count} environments move`} to the default group. No environment is deleted.`
    ui.confirm(`Delete the group ${name}?`, text, () => perform((drop) => EnvironmentService.DeleteGroup(name, drop)), 'Delete group')
  }

  function openEditor(name: string) {
    const node = environmentNode(app.tree?.roots, name)
    if (node) void app.open(node)
  }
</script>

<ScreenPage width={980}>
  <p class="note">Environments sorted into groups, such as one set for each set of services. Compare environments shows one group at a time.</p>

  <ul class="tools" aria-label="Things to do with environments">
    {#each envTools as t (t.id)}
      <li class="tool">
        <Button onclick={t.run}><Icon name={t.icon} size={14} /> {t.title}</Button>
        <span class="hint">{t.hint}</span>
      </li>
    {/each}
  </ul>

  {#if load.status === 'error'}
    <p class="err" role="alert">Could not read the environments: {load.error} <Button small onclick={reload}>Try again</Button></p>
  {:else if load.status === 'ready' && !view.envs.length}
    <EmptyState icon="key" title="No environment yet" hint="An environment holds the variables of one target, such as local or staging." compact>
      <Button variant="primary" onclick={envTools[0].run}>New environment</Button>
    </EmptyState>
  {:else if load.status === 'ready'}
    <div class="heading">
      <h2>Groups</h2>
      <Button small onclick={newGroup} title="Make a group, then move environments into it">New group…</Button>
    </div>
    <ul class="groups">
      {#each view.groups as g (g.name)}
        <li class="group">
          <span class="gname">{g.label}</span>
          <span class="muted">{g.count === 0 ? 'no environments' : g.count === 1 ? '1 environment' : `${g.count} environments`}</span>
          <span class="spacer"></span>
          <Button small variant="ghost" disabled={g.count < 2} title={g.count < 2 ? 'Comparing needs at least two environments in the group' : `Compare the environments of ${g.label}`} onclick={() => openCompare(g.name)}>Compare</Button>
          {#if g.name !== ''}
            <Button small variant="ghost" onclick={() => renameGroup(g.name)}>Rename</Button>
            <Button small variant="ghost" danger title="The environments move to the default group" onclick={() => deleteGroup(g.name, g.count)}>Delete</Button>
          {/if}
        </li>
      {/each}
    </ul>
    {#if !named.length}<p class="note">There is only the default group. Make another with New group…, then add environments to it with + Group.</p>{/if}

    <h2>Environments</h2>
    <div class="scroll">
      <table>
        <thead>
          <tr><th scope="col">Environment</th><th scope="col">Colour</th><th scope="col">Groups</th><th scope="col">Builds on</th><th scope="col" class="num">Variables</th></tr>
        </thead>
        <tbody>
          {#each view.envs as e (e.file)}
            <tr>
              <th scope="row" class="name">
                <Button variant="link" onclick={() => openEditor(e.name)} title="Open {e.name}">
                  <span class="dot" class:none={!envSwatch(e.colour)} style:background={envSwatch(e.colour)} aria-hidden="true"></span>{e.name}
                </Button>
                {#if e.protected}<span class="tag" title="Asks before a request that changes data is sent">asks before writes</span>{/if}
                {#if e.warning}<span class="warn">{e.warning}</span>{/if}
              </th>
              <td><ColourPicker colours={view.colours} value={e.colour} onpick={(c) => colourTo(e, c)} label="Colour of {e.name}" /></td>
              <td>
                <div class="chips">
                  {#each e.groups as g (g)}
                    <span class="chip">{groupName(g)}<IconButton icon="close" size={10} label="Remove {e.name} from {groupName(g)}" onclick={() => changeGroups(e, toggledGroups(e.groups, g))} /></span>
                  {:else}
                    <span class="muted">Default</span>
                  {/each}
                  <Button small variant="ghost" aria-haspopup="menu" aria-expanded={menuFor === e.file} title="Choose the groups of {e.name}" onclick={(ev) => openGroups(ev, e)}>+ Group</Button>
                </div>
              </td>
              <td>{e.base || '—'}</td>
              <td class="num">{e.vars}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    {#if menuFor && menuEnv}<Menu at={menuAt} label="Groups of {menuEnv.name}" items={menuItems} onclose={() => (menuFor = null)} />{/if}
  {/if}
</ScreenPage>

<style>
  .note, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 6px; }
  .err { color: var(--fail); font-size: var(--fs-sm); }
  h2 { font-size: var(--fs-md); margin: var(--sp-6) 0 var(--sp-2); }
  .heading { display: flex; align-items: center; gap: var(--sp-3); margin: var(--sp-6) 0 var(--sp-2); }
  .heading h2 { margin: 0; }
  .tools { display: grid; grid-template-columns: repeat(auto-fill, minmax(210px, 1fr)); gap: var(--sp-3); list-style: none; margin: var(--sp-3) 0 0; padding: 0; }
  .tool { display: flex; flex-direction: column; align-items: flex-start; gap: 4px; padding: var(--sp-3); border: 1px solid var(--line); border-radius: var(--r-lg); }
  .tool .hint { color: var(--muted); font-size: var(--fs-xs); }
  .groups { list-style: none; margin: 0; padding: 0; border: 1px solid var(--line); border-radius: var(--r-lg); }
  .group { display: flex; align-items: center; gap: var(--sp-3); padding: var(--sp-2) var(--sp-3); border-bottom: 1px solid var(--line); }
  .group:last-child { border-bottom: 0; }
  .gname { font-weight: 500; }
  .group .muted { margin: 0; }
  .spacer { flex: 1; }
  .scroll { overflow: auto; border: 1px solid var(--line); border-radius: var(--r-lg); }
  table { border-collapse: collapse; width: 100%; font-size: var(--fs-sm); }
  th, td { text-align: left; padding: var(--sp-2) var(--sp-3); border-bottom: 1px solid var(--line); vertical-align: middle; }
  thead th { background: var(--raised); font-weight: 600; }
  tbody tr:last-child > * { border-bottom: 0; }
  .name { font-weight: 500; }
  .num { text-align: right; color: var(--muted); }
  .chips { display: flex; flex-wrap: wrap; align-items: center; gap: var(--sp-1) var(--sp-2); }
  .chip { display: inline-flex; align-items: center; gap: 2px; padding: 0 2px 0 var(--sp-2); border: 1px solid var(--line); border-radius: var(--r-pill); background: var(--raised); font-size: var(--fs-xs); }
  td .muted { margin: 0; }
  .dot { display: inline-block; width: 10px; height: 10px; border-radius: 50%; margin-right: 8px; vertical-align: middle; }
  .dot.none { border: 1px dashed var(--muted); box-sizing: border-box; }
  .tag { margin-left: var(--sp-2); color: var(--muted); font-size: var(--fs-xs); font-weight: 400; }
  .warn { display: block; color: var(--warn); font-size: var(--fs-xs); font-weight: 400; margin-top: 2px; }
</style>
