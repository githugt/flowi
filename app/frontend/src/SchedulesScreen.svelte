<script lang="ts">
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { SchedulesService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import type { ScheduleView, SchedulesView } from './lib/types'

  let view = $state<SchedulesView>({ runner: { state: 'stopped', notes: [], upcoming: [], triggerAddr: '' }, schedules: [], flows: [], environments: [], triggerAddr: '', hasToken: false })
  let editing = $state<{ old: string; s: ScheduleView } | null>(null)
  let problem = $state('')
  let token = $state('')

  onMount(() => live<SchedulesView>('schedules', () => SchedulesService.State(), (v) => (view = v)))

  const blank = (): ScheduleView => ({ name: '', flow: '', collection: '', cron: '@daily', env: '', inputs: '', tz: '', disabled: false, flowLabel: '' })
  const edit = (s: ScheduleView) => ((editing = { old: s.name, s: { ...s } }), (problem = ''))

  // Picking a flow stores its id and the collection that makes it unambiguous.
  function pickFlow(value: string) {
    const f = view.flows.find((x) => x.collection + '/' + x.flow === value)
    if (f && editing) {
      editing.s.flow = f.flow
      editing.s.collection = f.collection
    }
  }

  async function save() {
    problem = (await SchedulesService.Save(editing!.old, $state.snapshot(editing!.s))) as string
    if (!problem) editing = null
  }

  const running = $derived(view.runner.state === 'running' || view.runner.state === 'stopping')

  async function makeToken() {
    try {
      token = (await SchedulesService.NewToken()) as string
    } catch (e) {
      ui.toast(String(e), 'error')
    }
  }
</script>

<div class="page">
  <p class="note">Schedules are kept in workspace.yaml. They fire only while something runs them: this window (below), or <code>flowi serve</code> without a window.</p>
  {#if view.error}<p class="err">{view.error}</p>{/if}

  <div class="runner" class:on={running}>
    {#if running}
      <button disabled={view.runner.state === 'stopping'} onclick={() => SchedulesService.StopRunner()}>{view.runner.state === 'stopping' ? 'Stopping…' : '■ Stop schedules'}</button>
      <span class="ok">● running while this window is open</span>
    {:else}
      <button class="primary" disabled={view.runner.state === 'starting' || !view.schedules.length} title={view.schedules.length ? 'Fire the schedules while this window is open' : 'Add a schedule first'} onclick={() => SchedulesService.StartRunner()}>{view.runner.state === 'starting' ? 'Starting…' : '▶ Start schedules'}</button>
      <span class="muted">stopped: nothing fires</span>
    {/if}
  </div>
  {#if view.runner.error}<p class="err">{view.runner.error}</p>{/if}
  {#each view.runner.notes as n}<p class="muted">{n}</p>{/each}
  {#if running && view.runner.upcoming.length}
    <div class="muted">Next: {#each view.runner.upcoming as u, i}{i ? ' · ' : ''}<strong>{u.name}</strong> {u.at}{/each}</div>
  {/if}
  <details class="cli">
    <summary>Run them without the window</summary>
    <p class="muted">Closing this window stops the schedules above. To keep them running (on a server, or at login), start the same thing from a terminal:</p>
    <pre>flowi serve --no-replay</pre>
    <p class="muted">With a trigger token it also listens for <code>POST /flows/&lt;flow&gt;/run</code>. <code>flowi schedule ls</code> lists the schedules and when each fires next; <code>flowi schedule add</code> and <code>rm</code> change them. Restart <code>flowi serve</code> after changing the schedules.</p>
  </details>

  <h3>Schedules <button class="add" onclick={() => edit(blank())}>+ New schedule</button></h3>
  {#each view.schedules as s (s.name)}
    <div class="card" class:off={s.disabled}>
      <div class="top"><strong>{s.name}</strong><code>{s.cron}{s.tz ? ' ' + s.tz : ''}</code><span class="spacer"></span>
        <button onclick={() => edit(s)}>Edit</button>
        <button onclick={() => ui.confirm('Remove schedule', `Remove the schedule ${s.name}?`, () => SchedulesService.Remove(s.name))}>Remove</button>
      </div>
      <div class="muted">{s.flowLabel || s.flow}{s.env ? ' · ' + s.env : ''}{s.disabled ? ' · disabled' : ''}</div>
    </div>
  {/each}
  {#if !view.schedules.length}<p class="muted">No schedules yet.</p>{/if}

  {#if editing}
    <form class="edit" onsubmit={(e) => { e.preventDefault(); save() }}>
      <h4>{editing.old ? 'Edit schedule' : 'New schedule'}</h4>
      <label><span>Name</span><input bind:value={editing.s.name} spellcheck="false" /></label>
      <label><span>Flow</span>
        <select value={editing.s.collection + '/' + editing.s.flow} onchange={(e) => pickFlow(e.currentTarget.value)}>
          <option value="/" disabled>Choose a flow…</option>
          {#each view.flows as f}<option value={f.collection + '/' + f.flow}>{f.label}</option>{/each}
        </select>
      </label>
      <label><span>Cron (five fields, or @hourly, @daily …)</span><input bind:value={editing.s.cron} spellcheck="false" /></label>
      <label><span>Environment (empty: the default)</span>
        <select bind:value={editing.s.env}><option value="">default</option>{#each view.environments as e}<option value={e}>{e}</option>{/each}</select>
      </label>
      <label><span>Time zone (IANA name; empty: this machine's)</span><input bind:value={editing.s.tz} spellcheck="false" placeholder="Europe/Berlin" /></label>
      <label><span>Inputs (name=value, one per line)</span><textarea bind:value={editing.s.inputs} spellcheck="false"></textarea></label>
      <label class="check"><input type="checkbox" bind:checked={editing.s.disabled} /> Disabled</label>
      {#if problem}<p class="err">{problem}</p>{/if}
      <div class="actions">
        <button type="button" onclick={() => (editing = null)}>Cancel</button>
        <button class="primary" type="submit">Save</button>
      </div>
    </form>
  {/if}

  <h3>HTTP trigger</h3>
  <p class="muted">`flowi serve` listens on <code>{view.triggerAddr || '—'}</code>. Triggers stay off until a token exists.</p>
  <button onclick={makeToken}>{view.hasToken ? 'Make a new token' : 'Make a token'}</button>
  {#if token}<p class="muted">Copy it now; it is shown once:</p><pre>{token}</pre>{/if}
</div>

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: 16px 24px 48px; max-width: 820px; }
  .note, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 6px; }
  .err { color: var(--fail); }
  h3 { margin: 22px 0 8px; font-size: var(--fs-md); border-bottom: 1px solid var(--line); padding-bottom: 4px; display: flex; justify-content: space-between; align-items: center; }
  h4 { margin: 0; }
  .card { border: 1px solid var(--line); border-radius: var(--r-lg); padding: 8px 12px; margin: 6px 0; }
  .card.off { opacity: 0.6; }
  .top { display: flex; align-items: center; gap: 10px; }
  .spacer { flex: 1; }
  code, pre { font: var(--fs-sm) var(--mono); }
  pre { background: var(--bg); border: 1px solid var(--line); border-radius: var(--r-md); padding: 8px; -webkit-user-select: text; user-select: text; }
  .edit { border: 1px solid var(--accent); border-radius: var(--r-lg); padding: 12px; margin: 10px 0; display: flex; flex-direction: column; gap: 8px; }
  label { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  .check { flex-direction: row; align-items: center; gap: 6px; font-size: var(--fs-sm); color: var(--text); }
  input:not([type]), select, textarea { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: inherit; -webkit-user-select: text; user-select: text; }
  textarea { min-height: 70px; font: var(--fs-sm) var(--mono); }
  .actions { display: flex; gap: 8px; justify-content: flex-end; }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 12px; font: inherit; }
  .add { border: 0; color: var(--accent); }
  .runner { display: flex; align-items: center; gap: 10px; margin: 8px 0; }
  .runner.on button:not(.primary) { border-color: var(--fail); color: var(--fail); }
  .ok { color: var(--ok); }
  .cli { margin: 8px 0; font-size: var(--fs-sm); }
  .cli summary { cursor: default; color: var(--accent); }
  .primary { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
</style>
