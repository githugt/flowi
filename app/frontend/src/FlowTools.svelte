<script lang="ts">
  // The tools of the open flow beyond Run: a matrix of runs, the access check, the drift check, and the sequence diagram
  // of the last run. Each opens a dialog; the checks run the saved flow, so an unsaved flow is refused with a message.
  import { errorText } from './lib/errors'
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import SequenceDiagram from './SequenceDiagram.svelte'
  import type { AccessView, ChecksChoices, DriftView, FlowView, MatrixView, SequenceView } from './lib/types'

  let { view }: { view: FlowView } = $props()
  const node = $derived(view.node)

  type Tool = '' | 'matrix' | 'access' | 'drift' | 'sequence'
  let menu = $state(false)
  let tool = $state<Tool>('')
  let choices = $state<ChecksChoices>({ envs: [], overlays: [], inputs: [], steps: [] })
  let busy = $state(false)
  let error = $state('')

  // matrix
  let byEnv = $state<string[]>([])
  let byOverlay = $state<string[]>([])
  let inputName = $state('')
  let inputValues = $state('')
  let matrix = $state<MatrixView | null>(null)

  // access
  let owner = $state('')
  let others = $state<string[]>([])
  let stepsChosen = $state<string[]>([])
  let access = $state<AccessView | null>(null)

  // drift
  let failNew = $state(false)
  let drift = $state<DriftView | null>(null)

  // sequence
  let sequence = $state<SequenceView | null>(null)
  let copied = $state(false)

  async function open(t: Tool) {
    menu = false
    error = ''
    tool = t
    matrix = access = drift = sequence = null
    try {
      choices = (await FlowService.CheckChoices(node)) as unknown as ChecksChoices
      if (t === 'sequence') await draw()
    } catch (e) {
      error = errorText(e)
    }
  }

  const close = () => (tool = '')

  async function run<T>(f: () => Promise<T>, set: (v: T) => void) {
    busy = true
    error = ''
    try {
      set(await f())
    } catch (e) {
      error = errorText(e)
    } finally {
      busy = false
    }
  }

  function toggle(list: string[], v: string): string[] {
    return list.includes(v) ? list.filter((x) => x !== v) : [...list, v]
  }

  const dims = $derived.by(() => {
    const out: { name: string; values: string[] }[] = []
    if (byEnv.length) out.push({ name: 'env', values: byEnv })
    if (byOverlay.length) out.push({ name: 'overlay', values: byOverlay })
    const vals = inputValues.split(',').map((v) => v.trim()).filter(Boolean)
    if (inputName && vals.length) out.push({ name: `input.${inputName}`, values: vals })
    return out
  })
  const cellCount = $derived(dims.reduce((n, d) => n * d.values.length, 1))

  const runMatrix = () => run(() => FlowService.RunMatrix(node, dims, {}), (v) => (matrix = v as unknown as MatrixView))
  const runAccess = () => run(() => FlowService.CheckAccess(node, owner, others, stepsChosen, {}), (v) => (access = v as unknown as AccessView))
  const runDrift = () => run(() => FlowService.CheckDrift(node, '', failNew ? ['new', 'missing', 'changed'] : ['missing', 'changed']), (v) => (drift = v as unknown as DriftView))
  const draw = () => run(() => FlowService.Sequence(node), (v) => (sequence = v as unknown as SequenceView))

  async function copy() {
    if (!sequence) return
    try {
      await navigator.clipboard.writeText(sequence.mermaid)
      copied = true
      setTimeout(() => (copied = false), 1500)
    } catch {
      error = 'The browser did not allow copying: select the text and copy it.'
    }
  }

  // The grid is rows × columns when two things vary; any other number is a list.
  const grid = $derived.by(() => {
    if (!matrix || matrix.dims.length !== 2) return null
    const [rows, cols] = matrix.dims
    return { rows: rows.values, cols: cols.values, at: (r: number, c: number) => matrix!.cells[r * cols.values.length + c] }
  })
  const mark = (s: string) => ({ passed: '✓', failed: '✗', error: '!', cancelled: '–' })[s] ?? '?'
</script>

<svelte:window onpointerdown={() => (menu = false)} />

<div class="tools">
  <button class="more" aria-haspopup="menu" aria-expanded={menu} onpointerdown={(e) => e.stopPropagation()} onclick={() => (menu = !menu)}>Checks ▾</button>
  {#if menu}
    <div class="menu" role="menu" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
      <button role="menuitem" onclick={() => open('matrix')}>Run a matrix…</button>
      <button role="menuitem" onclick={() => open('access')}>Check access…</button>
      <button role="menuitem" onclick={() => open('drift')}>Check drift…</button>
      <hr />
      <button role="menuitem" onclick={() => open('sequence')}>Sequence diagram of the last run…</button>
    </div>
  {/if}
</div>

{#if tool}
  <div class="scrim" role="presentation" onpointerdown={close}>
    <div class="dialog" role="dialog" aria-label="Flow checks" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
      {#if tool === 'matrix'}
        <h3>Run a matrix</h3>
        <p class="note">Runs the saved flow once for every combination you pick, one after the other, and shows a grid of passes and failures. Pick at least one thing to vary.</p>
        <div class="group"><strong>Environments</strong>
          {#each choices.envs as e}<label class="opt"><input type="checkbox" checked={byEnv.includes(e)} onchange={() => (byEnv = toggle(byEnv, e))} /> {e}</label>{/each}
          {#if !choices.envs.length}<span class="muted">none yet</span>{/if}
        </div>
        <div class="group"><strong>Overlays</strong> <span class="muted">laid over every step that names none</span>
          {#each choices.overlays as o}<label class="opt"><input type="checkbox" checked={byOverlay.includes(o)} onchange={() => (byOverlay = toggle(byOverlay, o))} /> {o}</label>{/each}
          {#if !choices.overlays.length}<span class="muted">this collection has none</span>{/if}
        </div>
        {#if choices.inputs.length}
          <div class="group"><strong>An input</strong>
            <select bind:value={inputName}><option value="">none</option>{#each choices.inputs as i}<option value={i}>{i}</option>{/each}</select>
            {#if inputName}<input placeholder="values, separated by commas" bind:value={inputValues} spellcheck="false" />{/if}
          </div>
        {/if}
        <div class="actions">
          <span class="muted">{dims.length ? `${cellCount} run${cellCount > 1 ? 's' : ''}` : ''}</span>
          <button class="primary" disabled={busy || !dims.length || cellCount > 100} onclick={runMatrix}>{busy ? 'Running…' : 'Run'}</button>
          <button onclick={close}>Close</button>
        </div>
        {#if matrix}
          <p class="result">{matrix.passed} of {matrix.cells.length} passed</p>
          {#if grid}
            <table class="grid">
              <thead><tr><th>{matrix!.dims[0].name} ↓ · {matrix!.dims[1].name} →</th>{#each grid.cols as c}<th>{c}</th>{/each}</tr></thead>
              <tbody>
                {#each grid.rows as r, ri}
                  <tr><th>{r}</th>{#each grid.cols as _, ci}{@const cell = grid.at(ri, ci)}<td class={cell.status} title={cell.why ?? ''}>{mark(cell.status)}</td>{/each}</tr>
                {/each}
              </tbody>
            </table>
          {:else}
            {#each matrix.cells as c}<div class="cell"><span class={c.status}>{mark(c.status)}</span> {c.labels.join(' · ')}</div>{/each}
          {/if}
          {#each matrix.cells.filter((c) => c.status !== 'passed' && c.why) as c}<p class="why"><strong>{c.labels.join(' · ')}</strong>: {c.why}</p>{/each}
        {/if}

      {:else if tool === 'access'}
        <h3>Check access</h3>
        <p class="note">Runs the flow as the <strong>owner</strong>, then tries its steps as every other identity, standing on what the owner made. A step another identity can reach is a broken access rule. An identity is a flowi overlay that makes the request that user's (it sets the Authorization header, a cookie or an API key).</p>
        <div class="group"><strong>Owner</strong>
          <select bind:value={owner}><option value="">choose…</option>{#each choices.overlays as o}<option value={o}>{o}</option>{/each}</select>
        </div>
        <div class="group"><strong>Try as</strong>
          {#each choices.overlays.filter((o) => o !== owner) as o}<label class="opt"><input type="checkbox" checked={others.includes(o)} onchange={() => (others = toggle(others, o))} /> {o}</label>{/each}
          {#if !choices.overlays.length}<span class="muted">this collection has no overlays: save a request edit as a flowi overlay first</span>{/if}
        </div>
        <div class="group"><strong>Steps</strong> <span class="muted">none chosen: the ones that read what an earlier step made</span>
          {#each choices.steps as s}<label class="opt"><input type="checkbox" checked={stepsChosen.includes(s)} onchange={() => (stepsChosen = toggle(stepsChosen, s))} /> {s}</label>{/each}
        </div>
        <div class="actions">
          <button class="primary" disabled={busy || !owner || !others.length} onclick={runAccess}>{busy ? 'Running…' : 'Check'}</button>
          <button onclick={close}>Close</button>
        </div>
        {#if access}
          <table class="grid">
            <thead><tr><th>step</th><th>{access.owner} (owner)</th>{#each access.steps[0]?.attempts ?? [] as a}<th>{a.identity}</th>{/each}</tr></thead>
            <tbody>
              {#each access.steps as s}
                <tr><th>{s.id}</th><td>{s.owner}</td>{#each s.attempts as a}<td class={a.verdict} title={a.verdict}>{a.verdict === 'denied' ? '✓' : a.verdict === 'allowed' ? '✗' : '?'} {a.status} {a.verdict}</td>{/each}</tr>
              {/each}
            </tbody>
          </table>
          <p class={access.exposed ? 'bad' : 'ok'}>{access.exposed ? 'Another identity reached what the owner made: the access rule is broken for the steps marked ✗.' : 'No other identity reached what the owner made.'}</p>
        {/if}

      {:else if tool === 'drift'}
        <h3>Check drift</h3>
        <p class="note">Runs the flow without learning and compares every response with the schema stored for it: fields the schema does not know (<strong>new</strong>), required fields that are gone (<strong>missing</strong>) and fields of another type (<strong>changed</strong>). The stored schemas are not touched, so the next check measures against the same baseline.</p>
        <label class="opt"><input type="checkbox" bind:checked={failNew} /> A new field also fails the check (missing and changed always do)</label>
        <div class="actions">
          <button class="primary" disabled={busy} onclick={runDrift}>{busy ? 'Running…' : 'Run and compare'}</button>
          <button onclick={close}>Close</button>
        </div>
        {#if drift}
          <p class="muted">The run {drift.runStatus}.</p>
          {#each drift.requests as q}
            <div class="drift"><strong>{q.request}</strong> <span class="muted">version {q.version} · {q.status}</span>
              {#if q.noSchema}<div class="muted">No schema is stored for this response yet. Send it with schema learning on, then check again.</div>
              {:else if !q.differences.length}<div class="ok">no drift</div>{/if}
              {#each q.differences as d}<div class="diff" class:fails={d.fails}>{d.fails ? '✗' : '!'} <strong>{d.kind}</strong> <code>{d.path}</code> <span class="muted">{d.detail ?? ''}</span></div>{/each}
            </div>
          {/each}
          <p class={drift.failing ? 'bad' : 'ok'}>{drift.failing ? 'This check fails.' : 'This check passes.'}</p>
        {/if}

      {:else if tool === 'sequence'}
        <h3>Sequence diagram of the last run</h3>
        {#if sequence}
          <p class="note">Flowi and one lifeline for each host it called, an arrow out and one back for each attempt with its status and time; a crossed or red arrow is a refusal or an error. Only host and path are drawn, never the query.</p>
          <div class="diagram"><SequenceDiagram seq={sequence} /></div>
          <div class="actions">
            <button onclick={copy}>{copied ? 'Copied' : 'Copy as Mermaid'}</button>
            <button onclick={close}>Close</button>
          </div>
        {:else}
          <div class="actions"><button onclick={close}>Close</button></div>
        {/if}
      {/if}
      {#if error}<p class="bad">{error}</p>{/if}
    </div>
  </div>
{/if}

<style>
  .tools { position: relative; }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 12px; font: inherit; }
  button:hover:not(:disabled) { border-color: var(--accent); }
  button:disabled { opacity: 0.4; }
  .primary { background: var(--act-send); color: var(--on-act); border-color: var(--act-send); font-weight: 600; }
  .menu { position: absolute; top: 100%; right: 0; z-index: 30; min-width: 250px; display: flex; flex-direction: column; padding: 4px; background: var(--panel); border: 1px solid var(--line); border-radius: 10px; box-shadow: var(--shadow); }
  .menu button { border: 0; text-align: left; padding: 6px 10px; }
  hr { border: 0; border-top: 1px solid var(--line); width: 100%; margin: 4px 0; }
  .scrim { position: fixed; inset: 0; z-index: 40; background: rgba(0, 0, 0, 0.45); display: flex; align-items: center; justify-content: center; }
  .dialog { width: min(760px, 94vw); max-height: 88vh; overflow: auto; background: var(--panel); border: 1px solid var(--line); border-radius: 14px; padding: 16px 20px; display: flex; flex-direction: column; gap: 8px; box-shadow: var(--shadow); }
  h3 { margin: 0; }
  .note, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .group { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 14px; font-size: var(--fs-sm); }
  .group > strong { min-width: 90px; }
  .opt { display: flex; align-items: center; gap: 4px; font-size: var(--fs-sm); }
  select, input:not([type]) { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 8px; font: inherit; }
  .actions { display: flex; align-items: center; justify-content: flex-end; gap: 8px; margin-top: 4px; }
  .actions .muted { margin-right: auto; }
  .result { margin: 4px 0 0; font-weight: 600; }
  table.grid { border-collapse: collapse; font-size: var(--fs-sm); margin: 4px 0; }
  table.grid th, table.grid td { border: 1px solid var(--line); padding: 4px 12px; text-align: center; }
  table.grid th { color: var(--muted); font-weight: 600; }
  .passed, .denied { color: var(--ok); }
  .failed, .allowed, .bad { color: var(--fail); }
  .error, .unclear { color: var(--warn); }
  .cancelled { color: var(--muted); }
  .ok { color: var(--ok); margin: 2px 0; }
  .cell { font-size: var(--fs-sm); }
  .why { margin: 0; font-size: var(--fs-sm); color: var(--muted); }
  .drift { border-bottom: 1px solid var(--line); padding: 4px 0; font-size: var(--fs-sm); }
  .diff.fails { color: var(--fail); }
  .diff { padding-left: 14px; }
  code { font: var(--fs-xs) var(--mono); }
  .diagram { overflow: auto; border: 1px solid var(--line); border-radius: var(--r-md); padding: 6px; background: var(--bg); }
</style>
