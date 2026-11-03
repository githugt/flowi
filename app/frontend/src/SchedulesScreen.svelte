<script lang="ts">
  import TextInput from './TextInput.svelte'
  import Button from './Button.svelte'
  import Toggle from './Toggle.svelte'
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { SchedulesService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import { axisClock, untilText } from './lib/until'
  import Icon from './Icon.svelte'
  import type { ScheduleView, SchedulesView } from './lib/types'

  let view = $state<SchedulesView>({ runner: { state: 'stopped', notes: [], upcoming: [], triggerAddr: '' }, schedules: [], flows: [], environments: [], triggerAddr: '', hasToken: false })
  let editing = $state<{ old: string; s: ScheduleView } | null>(null)
  let problem = $state('')
  let token = $state('')

  // The firing offsets are minutes from when a view was made, so the strip needs to know how long ago that was.
  let viewAt = $state(Date.now())
  let now = $state(Date.now())
  const SPAN_MINUTES = 24 * 60
  const hues = ['var(--accent)', 'var(--ok)', 'var(--warn)', 'var(--info)', 'var(--data)', 'var(--literal)']

  onMount(() => {
    const tick = setInterval(() => (now = Date.now()), 30_000)
    const stop = live<SchedulesView>('schedules', () => SchedulesService.State(), (v) => ((view = v), (viewAt = Date.now()), (now = viewAt)))
    return () => (clearInterval(tick), stop())
  })

  // Where on the 24-hour strip each firing sits, as a percentage; those that have passed since the view was made drop off.
  function ticks(fires: number[]): number[] {
    const elapsed = (now - viewAt) / 60_000
    return fires.map((m) => ((m - elapsed) / SPAN_MINUTES) * 100).filter((p) => p >= 0 && p <= 100)
  }

  const blank = (): ScheduleView => ({ name: '', flow: '', collection: '', cron: '@daily', env: '', inputs: '', tz: '', disabled: false, flowLabel: '', when: '', nextAt: '', fires: [] })
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

  const busiestFirst = $derived([...view.schedules].sort((a, b) => b.fires.length - a.fires.length))
  const running = $derived(view.runner.state === 'running' || view.runner.state === 'stopping')
  const busy = $derived(view.runner.state === 'starting' || view.runner.state === 'stopping')
  const runnerLabel = $derived({ starting: 'Starting…', stopping: 'Stopping…', running: 'Running' }[view.runner.state as string] ?? 'Stopped')

  async function makeToken() {
    try {
      token = (await SchedulesService.NewToken()) as string
    } catch (e) {
      ui.toast(String(e), 'error')
    }
  }
</script>

<div class="page">
  <header class="head">
    <span class="tile"><Icon name="clock" size={20} /></span>
    <div class="who">
      <h2>Schedules</h2>
      <p class="muted">Run flows on a timer while this window is open, or from <code>flowi serve</code>. They are kept in workspace.yaml.</p>
    </div>
    <div class="runner" class:on={running}>
      <Toggle framed tone="ok" checked={running} disabled={busy || (!running && !view.schedules.length)} aria-label="Fire the schedules"
        title={running ? 'Stop firing the schedules' : view.schedules.length ? 'Fire the schedules while this window is open' : 'Add a schedule first'}
        onchange={(on) => (on ? SchedulesService.StartRunner() : SchedulesService.StopRunner())}>{runnerLabel}</Toggle>
      <Button variant="primary" onclick={() => edit(blank())}>+ Add schedule</Button>
    </div>
  </header>
  {#if view.error}<p class="err">{view.error}</p>{/if}
  {#if view.runner.error}<p class="err">{view.runner.error}</p>{/if}
  {#each view.runner.notes as n}<p class="muted">{n}</p>{/each}

  <div class="list">
    {#each view.schedules as s (s.name)}
      <div class="row" class:off={s.disabled}>
        <div class="what">
          <strong>{s.name}</strong>
          <div class="when">{s.when || s.cron}{#if s.when}<code>{s.cron}</code>{/if}{#if s.tz}<code>{s.tz}</code>{/if}</div>
          <div class="muted">{s.flowLabel || s.flow}{#if s.env}{' · env '}<b>{s.env}</b>{/if}</div>
        </div>
        <span class="next" class:dim={!s.nextAt}>
          <i style:background={s.nextAt ? hues[view.schedules.indexOf(s) % hues.length] : undefined}></i>{s.disabled ? 'Disabled' : s.nextAt ? 'Next run ' + untilText(s.nextAt, now) : 'Never fires'}
        </span>
        <Button onclick={() => edit(s)}>Edit</Button>
        <Button variant="danger" onclick={() => ui.confirm('Remove schedule', `Remove the schedule ${s.name}?`, () => SchedulesService.Remove(s.name))}>Remove</Button>
      </div>
    {:else}
      <p class="muted none">No schedules yet.</p>
    {/each}
  </div>

  {#if view.schedules.some((s) => s.fires.length)}
    <h3>Next 24 hours</h3>
    <div class="strip-card">
      <div class="strip" role="img" aria-label="When each schedule fires in the next 24 hours">
        <!-- The busiest schedule is drawn first so the rare ones stay visible on top of it. -->
        {#each busiestFirst as s (s.name)}
          {#each ticks(s.fires) as p}<i class="tick" class:rare={s.fires.length < 12} style:left={`${p}%`} style:background={hues[view.schedules.indexOf(s) % hues.length]}></i>{/each}
        {/each}
      </div>
      <div class="axis" title="Clock time on this machine">{#each axisClock(now, [0, 6, 12, 18, 24]) as t, i}<span>{i === 0 ? `now ${t}` : t}</span>{/each}</div>
      <div class="legend">{#each view.schedules.filter((s) => s.fires.length) as s (s.name)}<span><i style:background={hues[view.schedules.indexOf(s) % hues.length]}></i>{s.name}{#if s.tz} <small title="Fires by the clock of this zone">{s.tz}</small>{/if}</span>{/each}</div>
    </div>
  {/if}
  {#if running && view.runner.upcoming.length}
    <p class="muted">Next: {#each view.runner.upcoming as u, i}{i ? ' · ' : ''}<strong>{u.name}</strong> {u.at}{/each}</p>
  {/if}

  {#if editing}
    <form class="edit" onsubmit={(e) => { e.preventDefault(); save() }}>
      <h4>{editing.old ? 'Edit schedule' : 'New schedule'}</h4>
      <label><span>Name</span><TextInput bind:value={editing.s.name} /></label>
      <label><span>Flow</span>
        <select value={editing.s.collection + '/' + editing.s.flow} onchange={(e) => pickFlow(e.currentTarget.value)}>
          <option value="/" disabled>Choose a flow…</option>
          {#each view.flows as f}<option value={f.collection + '/' + f.flow}>{f.label}</option>{/each}
        </select>
      </label>
      <label><span>Cron (five fields, or @hourly, @daily …)</span><TextInput bind:value={editing.s.cron} mono /></label>
      <label><span>Environment (empty: the default)</span>
        <select bind:value={editing.s.env}><option value="">default</option>{#each view.environments as e}<option value={e}>{e}</option>{/each}</select>
      </label>
      <label><span>Time zone (IANA name; empty: this machine's)</span><TextInput bind:value={editing.s.tz} placeholder="Europe/Berlin" /></label>
      <label><span>Inputs (name=value, one per line)</span><textarea bind:value={editing.s.inputs} spellcheck="false"></textarea></label>
      <Toggle checked={editing.s.disabled} onchange={(on) => (editing!.s.disabled = on)}>Disabled</Toggle>
      {#if problem}<p class="err">{problem}</p>{/if}
      <div class="actions">
        <Button type="button" onclick={() => (editing = null)}>Cancel</Button>
        <Button variant="primary" type="submit">Save</Button>
      </div>
    </form>
  {/if}

  <details class="cli">
    <summary>Run them without the window</summary>
    <p class="muted">Closing this window stops the schedules above. To keep them running (on a server, or at login), start the same thing from a terminal:</p>
    <pre>flowi serve --no-replay</pre>
    <p class="muted">With a trigger token it also listens for <code>POST /flows/&lt;flow&gt;/run</code>. <code>flowi schedule ls</code> lists the schedules and when each fires next; <code>flowi schedule add</code> and <code>rm</code> change them. Restart <code>flowi serve</code> after changing the schedules.</p>
  </details>

  <h3>HTTP trigger</h3>
  <p class="muted">`flowi serve` listens on <code>{view.triggerAddr || '—'}</code>. Triggers stay off until a token exists.</p>
  <Button onclick={makeToken}>{view.hasToken ? 'Make a new token' : 'Make a token'}</Button>
  {#if token}<p class="muted">Copy it now; it is shown once:</p><pre>{token}</pre>{/if}
</div>

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: 28px 32px 56px; max-width: 880px; margin: 0 auto; width: 100%; box-sizing: border-box; }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 6px; }
  .err { color: var(--fail); }
  h3 { margin: 22px 0 8px; font-size: var(--fs-lg); font-weight: 600; display: flex; justify-content: space-between; align-items: center; }
  h4 { margin: 0; }
  .head { display: flex; align-items: center; gap: 14px; margin-bottom: 18px; }
  .tile { display: grid; place-items: center; width: 40px; height: 40px; flex: none; border-radius: var(--r-lg); background: var(--raised); color: var(--accent); }
  .who { flex: 1; min-width: 0; }
  .who h2 { margin: 0; font-size: var(--fs-xl); font-weight: 600; }
  .who .muted { margin: 2px 0 0; }
  .list { border-radius: var(--r-xl); background: var(--panel); box-shadow: inset 0 0 0 1px var(--line); }
  .row { display: flex; align-items: center; gap: 12px; padding: 14px 18px; }
  .row + .row { border-top: 1px solid var(--line); }
  .row.off { opacity: 0.6; }
  .what { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 3px; }
  .what strong { font-size: var(--fs-md); }
  .when { display: flex; align-items: baseline; gap: 8px; font-size: var(--fs-sm); }
  .when code { color: var(--muted); }
  .next { display: flex; align-items: center; gap: 7px; margin-right: 8px; font-size: var(--fs-sm); color: var(--muted); white-space: nowrap; }
  .next i { width: 7px; height: 7px; border-radius: 50%; background: var(--muted); }
  .next.dim { opacity: 0.7; }
  .none { padding: 14px 18px; margin: 0; }
  .strip-card { padding: 14px 16px; border-radius: var(--r-xl); background: var(--panel); box-shadow: inset 0 0 0 1px var(--line); }
  .strip { position: relative; height: 34px; border-radius: var(--r-md); background: var(--sunk); overflow: hidden; }
  .tick { position: absolute; top: 9px; width: 2px; height: 16px; border-radius: 1px; opacity: 0.8; }
  .tick.rare { top: 3px; width: 3px; height: 28px; opacity: 1; box-shadow: 0 0 0 1px var(--sunk); }
  .axis { display: flex; justify-content: space-between; margin-top: 8px; color: var(--muted); font: var(--fs-xs) var(--mono); }
  .legend { display: flex; gap: 14px; margin-top: 10px; font-size: var(--fs-sm); color: var(--muted); }
  .legend span { display: flex; align-items: center; gap: 6px; }
  .legend small { font: var(--fs-xs) var(--mono); }
  .legend i { width: 8px; height: 8px; border-radius: 2px; }
  code, pre { font: var(--fs-sm) var(--mono); }
  pre { background: var(--bg); border: 1px solid var(--line); border-radius: var(--r-md); padding: 8px; -webkit-user-select: text; user-select: text; }
  .edit { border: 1px solid var(--accent); border-radius: var(--r-lg); padding: 12px; margin: 10px 0; display: flex; flex-direction: column; gap: 8px; }
  label { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  select, textarea { background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: inherit; -webkit-user-select: text; user-select: text; }
  textarea { min-height: 70px; font: var(--fs-sm) var(--mono); }
  .actions { display: flex; gap: 8px; justify-content: flex-end; }
  .runner { display: flex; align-items: center; gap: 10px; flex: none; }
  .cli { margin: 8px 0; font-size: var(--fs-sm); }
  .cli summary { cursor: default; color: var(--accent); }
</style>
