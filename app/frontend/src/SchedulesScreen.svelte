<script lang="ts">
  import ScreenPage from './ScreenPage.svelte'
  import EmptyState from './EmptyState.svelte'
  import Select from './Select.svelte'
  import TextInput from './TextInput.svelte'
  import Button from './Button.svelte'
  import Toggle from './Toggle.svelte'
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { SchedulesService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import { axisClock } from './lib/until'
  import { scheduleStatus } from './lib/schedulerow'
  import Icon from './Icon.svelte'
  import { copy } from './lib/clip'
  import { triggerCurl } from './lib/triggercurl'
  import type { CronPreview, ScheduleView, SchedulesView } from './lib/types'
  import { cronPreviewLine } from './lib/cronpreview'
  import { debounced } from './lib/debounce'

  let view = $state<SchedulesView>({ runner: { state: 'stopped', notes: [], upcoming: [], triggerAddr: '' }, schedules: [], flows: [], environments: [], triggerAddr: '', hasToken: false })
  let editing = $state<{ old: string; s: ScheduleView } | null>(null)
  // The line under the cron field: what it means and when it fires next, or why it cannot be used. The answer of an
  // older line never replaces the answer of a newer one.
  let preview = $state<CronPreview | null>(null)
  let previewSeq = 0
  const askPreview = debounced(async (cron: string, tz: string) => {
    const mine = ++previewSeq
    const p = (await SchedulesService.PreviewCron(cron, tz)) as unknown as CronPreview
    if (mine === previewSeq) preview = p
  }, 250)
  $effect(() => {
    if (editing) askPreview(editing.s.cron, editing.s.tz)
    else preview = null
  })
  let problem = $state('')
  // The name is a file name and a word in the trigger's address, so it cannot hold a space; say so while it is typed.
  const nameBad = $derived(!!editing && /[^\p{L}\p{N}._-]/u.test(editing.s.name))
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

  const blank = (): ScheduleView => ({ name: '', flow: '', collection: '', cron: '@daily', env: '', inputs: '', tz: '', disabled: false, onFailure: '', onSuccess: '', flowLabel: '', when: '', nextAt: '', fires: [] })
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

  // A new token ends the old one at once: whatever calls the trigger with it starts failing. The user is asked first.
  function askToken() {
    if (!view.hasToken) return void makeToken()
    ui.confirm('Make a new token?', 'The current token stops working at once. Anything that calls the trigger with it must be given the new one.', makeToken, 'Make a new token')
  }
  const firstFlow = $derived(view.flows[0]?.flow ?? '')
  const curlLine = $derived(token ? triggerCurl(view.triggerAddr, firstFlow, token) : '')

  async function makeToken() {
    try {
      token = (await SchedulesService.NewToken()) as string
    } catch (e) {
      ui.fail(e)
    }
  }
</script>

<ScreenPage>
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
  {#if !running && !busy && view.schedules.length}<p class="err" role="status">The schedules are not firing: the switch above is off. Turn it on to run them while this window is open.</p>{/if}
  {#if view.runner.error}<p class="err">{view.runner.error}</p>{/if}
  {#each view.runner.notes as n}<p class="muted">{n}</p>{/each}

  <div class="list">
    {#each view.schedules as s (s.name)}
      {@const status = scheduleStatus(s, running, now)}
      <div class="row" class:off={s.disabled}>
        <div class="what">
          <strong>{s.name}</strong>
          <div class="when">{s.when || s.cron}{#if s.when}<code>{s.cron}</code>{/if}{#if s.tz}<code>{s.tz}</code>{/if}</div>
          <div class="muted">{s.flowLabel || s.flow}{#if s.env}{' · env '}<b>{s.env}</b>{/if}</div>
        </div>
        <span class="next" class:dim={!status.firing}>
          <i style:background={status.firing ? hues[view.schedules.indexOf(s) % hues.length] : undefined}></i>{status.text}
        </span>
        <Button onclick={() => edit(s)}>Edit</Button>
        <Button variant="danger" onclick={() => ui.confirm('Remove schedule', `Remove the schedule ${s.name}?`, () => SchedulesService.Remove(s.name))}>Remove</Button>
      </div>
    {:else}
      <EmptyState icon="clock" title="No schedules yet" hint="A schedule runs a flow on a timer, for example a health check every five minutes. Choose + Add schedule, pick a flow and how often." compact />
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
      <label><span>Name</span><TextInput bind:value={editing.s.name} invalid={nameBad} error={nameBad ? 'A name is letters, digits, dot, underscore and dash: use a dash where you would put a space.' : ''} /></label>
      <label><span>Flow</span>
        <Select value={editing.s.collection + '/' + editing.s.flow} onchange={(e) => pickFlow(e.currentTarget.value)}>
          <option value="/" disabled>Choose a flow…</option>
          {#each view.flows as f}<option value={f.collection + '/' + f.flow}>{f.label}</option>{/each}
        </Select>
      </label>
      <label><span>Cron (five fields, or @hourly, @daily …)</span><TextInput bind:value={editing.s.cron} mono /></label>
      {#if preview}{@const line = cronPreviewLine(preview)}{#if line.text}<p class={line.error ? 'err' : 'muted'} role={line.error ? 'alert' : 'status'}>{line.text}</p>{/if}{/if}
      <label><span>Environment (empty: the default)</span>
        <Select bind:value={editing.s.env}><option value="">default</option>{#each view.environments as e}<option value={e}>{e}</option>{/each}</Select>
      </label>
      <label><span>Time zone (IANA name; empty: this machine's)</span><TextInput bind:value={editing.s.tz} placeholder="Europe/Berlin" /></label>
      <label><span>Inputs (name=value, one per line)</span><textarea bind:value={editing.s.inputs} spellcheck="false"></textarea></label>
      <label><span>Tell when a run fails (webhook address, or env:NAME; empty: nobody)</span><TextInput bind:value={editing.s.onFailure} mono placeholder="env:SLACK_HOOK" /></label>
      <label><span>Tell when a run passes (same; empty: nobody)</span><TextInput bind:value={editing.s.onSuccess} mono placeholder="env:SLACK_HOOK" /></label>
      <p class="muted">A webhook address is a credential: env:NAME reads it from an environment variable of the machine that runs <code>flowi serve</code>.</p>
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
  <Button onclick={askToken}>{view.hasToken ? 'Make a new token' : 'Make a token'}</Button>
  {#if token}
    <p class="muted">Copy it now; it is shown once:</p>
    <pre>{token}</pre>
    <div class="tokenacts"><Button small onclick={() => void copy(token, 'the token')}>Copy token</Button><Button small onclick={() => void copy(curlLine, 'the curl line')}>Copy as curl</Button></div>
    <pre>{curlLine}</pre>
  {/if}
</ScreenPage>

<style>
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
  .tokenacts { display: flex; gap: var(--sp-2); }
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
  textarea { min-height: 70px; font: var(--fs-sm) var(--mono); }
  .actions { display: flex; gap: 8px; justify-content: flex-end; }
  .runner { display: flex; align-items: center; gap: 10px; flex: none; }
  .cli { margin: 8px 0; font-size: var(--fs-sm); }
  .cli summary { cursor: default; color: var(--accent); }
</style>
