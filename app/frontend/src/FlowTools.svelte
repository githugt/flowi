<script lang="ts">
  import Checkbox from './Checkbox.svelte'
  import Menu, { type MenuItem } from './Menu.svelte'
  import { anchorAt, type MenuAt } from './lib/menuanchor'
  import Modal from './Modal.svelte'
  // The tools of the open flow beyond Run: a matrix of runs, the access check, the drift check, and the sequence diagram
  // of the last run. Each opens a dialog; the checks run the saved flow, so an unsaved flow is refused with a message.
  import Button from './Button.svelte'
  import Hint from './Hint.svelte'
  import { copy as copyText } from './lib/clip'
  import { acceptLabel, canAccept, pending, settledNote } from './lib/driftaccept'
  import { glossary } from './lib/glossary'
  import { verdictText } from './lib/accessverdict'
  import { checkedText, choicesOf, isStale } from './lib/accessage'
  import { errorText } from './lib/errors'
  import { matrixNote } from './lib/matrixlimit'
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import MatrixResult from './MatrixResult.svelte'
  import MatrixRunning from './MatrixRunning.svelte'
  import Select from './Select.svelte'
  import SequenceDiagram from './SequenceDiagram.svelte'
  import Toggle from './Toggle.svelte'
  import Icon from './Icon.svelte'
  import type { AccessView, ChecksChoices, DriftView, FlowView, MatrixView, SequenceView } from './lib/types'

  let { view }: { view: FlowView } = $props()
  const node = $derived(view.node)

  type Tool = '' | 'matrix' | 'access' | 'drift' | 'sequence'
  let menu = $state(false)
  let menuAt = $state<MenuAt>({ x: 0, y: 0 })
  const menuItems: MenuItem[] = [
    { label: 'Run a matrix…', onselect: () => open('matrix') },
    { label: 'Check access…', onselect: () => open('access') },
    { label: 'Check drift…', onselect: () => open('drift') },
    { label: 'Sequence diagram of the last run…', onselect: () => open('sequence'), separated: true },
  ]
  let tool = $state<Tool>('')
  const TITLES: Record<string, string> = { matrix: 'Run a matrix', access: 'Check access', drift: 'Check drift', sequence: 'Sequence diagram of the last run' }
  const toolTitle = $derived(TITLES[tool ?? ''] ?? 'Flow checks')
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
  let quiet = $state(false) // also draw the steps that call nothing: delays, sets, assertions
  let copied = $state(false)

  // The last check of this flow comes back with its form, so closing the dialog or restarting the app does not lose it.
  async function restoreAccess() {
    const kept = (await FlowService.LastAccess(node)) as unknown as AccessView
    if (!kept.owner) return
    access = kept
    const c = choicesOf(kept)
    owner = c.owner
    others = c.others
  }

  async function open(t: Tool) {
    menu = false
    error = ''
    tool = t
    matrix = access = sequence = null // a drift check stays, so closing the dialog does not lose it
    try {
      choices = (await FlowService.CheckChoices(node)) as unknown as ChecksChoices
      if (t === 'access') await restoreAccess()
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
  const note = $derived(matrixNote(dims, 'run'))

  const runMatrix = () => run(() => FlowService.RunMatrix(node, dims, {}), (v) => (matrix = v as unknown as MatrixView))
  const runAccess = () => run(() => FlowService.CheckAccess(node, owner, others, stepsChosen, {}), (v) => (access = v as unknown as AccessView))
  const runDrift = () => run(() => FlowService.CheckDrift(node, '', failNew ? ['new', 'missing', 'changed'] : ['missing', 'changed']), (v) => (drift = v as unknown as DriftView))
  const accept = (requests: string[]) => run(() => FlowService.AcceptDrift(node, requests), (v) => (drift = v as unknown as DriftView))
  const draw = () => run(() => FlowService.Sequence(node, quiet), (v) => (sequence = v as unknown as SequenceView))

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

</script>

<div class="tools">
  <button class="more" aria-haspopup="menu" aria-expanded={menu} onclick={(e) => { menuAt = anchorAt(e.currentTarget.getBoundingClientRect(), { end: true }); menu = !menu }}>Checks <Icon name="chevd" size={12} /></button>
  {#if menu}<Menu at={menuAt} minWidth={250} label="Checks" items={menuItems} onclose={() => (menu = false)} />{/if}
</div>

{#if tool}
  <Modal title={toolTitle} width="min(760px, 94vw)" maxHeight="88vh" scroll onclose={close}>
    {#snippet badge()}{#if tool === 'drift'}<Hint>{glossary.drift}</Hint>{/if}{/snippet}
    {#if tool === 'matrix'}
      <p class="note">Runs the saved flow once for every combination you pick, one after the other, and shows a grid of passes and failures. Pick at least one thing to vary.</p>
      <div class="group"><strong>Environments</strong>
        {#each choices.envs as e}<span class="opt"><Checkbox checked={byEnv.includes(e)} onchange={() => (byEnv = toggle(byEnv, e))}>{e}</Checkbox></span>{/each}
        {#if !choices.envs.length}<span class="muted">none yet</span>{/if}
      </div>
      <div class="group"><strong>Overlays</strong> <span class="muted">laid over every step that names none</span>
        {#each choices.overlays as o}<span class="opt"><Checkbox checked={byOverlay.includes(o)} onchange={() => (byOverlay = toggle(byOverlay, o))}>{o}</Checkbox></span>{/each}
        {#if !choices.overlays.length}<span class="muted">this collection has none</span>{/if}
      </div>
      {#if choices.inputs.length}
        <div class="group"><strong>An input</strong>
          <Select bind:value={inputName}><option value="">none</option>{#each choices.inputs as i}<option value={i}>{i}</option>{/each}</Select>
          {#if inputName}<input placeholder="values, separated by commas" bind:value={inputValues} spellcheck="false" />{/if}
        </div>
      {/if}
      <div class="actions">
        {#if busy}
          <MatrixRunning {node} oncancel={() => FlowService.CancelMatrix(node)} />
        {:else}
          <span class={note.over ? 'bad' : 'muted'} role={note.over ? 'alert' : undefined}>{note.text}</span>
        {/if}
        <Button variant="primary" disabled={busy || !dims.length || note.over} onclick={runMatrix}>{busy ? 'Running…' : 'Run'}</Button>
        <Button onclick={close}>Close</Button>
      </div>
      {#if matrix}<MatrixResult {matrix} />
      {/if}

    {:else if tool === 'access'}
      <p class="note">Runs the flow as the <strong>owner</strong>, then tries its steps as every other identity, standing on what the owner made. A step another identity can reach is a broken access rule. An identity is a flowi overlay that makes the request that user's (it sets the Authorization header, a cookie or an API key).</p>
      <div class="group"><strong>Owner</strong>
        <Select bind:value={owner}><option value="">choose…</option>{#each choices.overlays as o}<option value={o}>{o}</option>{/each}</Select>
      </div>
      <div class="group"><strong>Try as</strong>
        {#each choices.overlays.filter((o) => o !== owner) as o}<span class="opt"><Checkbox checked={others.includes(o)} onchange={() => (others = toggle(others, o))}>{o}</Checkbox></span>{/each}
        {#if !choices.overlays.length}<span class="muted">this collection has no overlays: save a request edit as a flowi overlay first</span>{/if}
      </div>
      <div class="group"><strong>Steps</strong> <span class="muted">none chosen: the ones that read what an earlier step made</span>
        {#each choices.steps as s}<span class="opt"><Checkbox checked={stepsChosen.includes(s)} onchange={() => (stepsChosen = toggle(stepsChosen, s))}>{s}</Checkbox></span>{/each}
      </div>
      <div class="actions">
        <Button variant="primary" disabled={busy || !owner || !others.length} onclick={runAccess}>{busy ? 'Running…' : 'Check'}</Button>
        <Button onclick={close}>Close</Button>
      </div>
      {#if access}
        <table class="grid">
          <caption class="sr-only">Whether each step could be reached by another identity</caption>
          <thead><tr><th scope="col">step</th><th scope="col">{access.owner} (owner)</th>{#each access.steps[0]?.attempts ?? [] as a}<th scope="col">{a.identity}</th>{/each}</tr></thead>
          <tbody>
            {#each access.steps as s}
              <tr><th scope="row">{s.id}</th><td>{s.owner}</td>{#each s.attempts as a}{@const v = verdictText(a.verdict, a.status)}<td class={a.verdict} title={v.title}>{v.mark} {v.text}</td>{/each}</tr>
            {/each}
          </tbody>
        </table>
        {#if access.checked}<p class="muted" role="status">{checkedText(access.checked, new Date())}{#if isStale(access.checked, new Date())}. This is old: the server or the flow may have changed since. Check again.{/if}</p>{/if}
        <p class={access.exposed ? 'bad' : 'ok'}>{access.exposed ? 'Another identity reached what the owner made: the access rule is broken for the steps marked ✗.' : 'No other identity reached what the owner made.'}</p>
      {/if}

    {:else if tool === 'drift'}
      <p class="note">Runs the flow without learning and compares every response with the schema stored for it: fields the schema does not know (<strong>new</strong>), required fields that are gone (<strong>missing</strong>) and fields of another type (<strong>changed</strong>). The stored schemas are not touched until you press Accept on a request, so the next check measures against the same baseline.</p>
      <span class="opt"><Checkbox bind:checked={failNew}>A new field also fails the check (missing and changed always do)</Checkbox></span>
      <div class="actions">
        <Button variant="primary" disabled={busy} onclick={runDrift}>{busy ? 'Running…' : 'Run and compare'}</Button>
        <Button onclick={close}>Close</Button>
      </div>
      {#if drift}
        <p class="muted">The run {drift.runStatus}.</p>
        {#each drift.requests as q}
          <div class="drift"><strong>{q.request}</strong> <span class="muted">version {q.version} · {q.status}</span>
            {#if q.accepted}<div class="ok">accepted: the stored schema matches this response now</div>
            {:else if q.noSchema}<div class="muted">No schema is stored for this response yet: learn one from this response.</div>
            {:else if !q.differences.length}<div class="ok">no drift</div>{/if}
            {#each q.differences as d}<div class="diff" class:fails={d.fails}>{d.fails ? '✗' : '!'} <strong>{d.kind}</strong> <code>{d.path}</code> <span class="muted">{d.detail ?? ''}</span></div>{/each}
            {#if canAccept(q)}<div class="rowact"><Button small disabled={busy} title="Bring the stored schema up to date with this response; nothing is run again" onclick={() => accept([q.request])}>{acceptLabel(q)}</Button></div>{/if}
          </div>
        {/each}
        <p class={drift.failing ? 'bad' : 'ok'}>{drift.failing ? 'This check fails.' : 'This check passes.'}</p>
        {#if settledNote(drift)}<p class="muted" role="status">{settledNote(drift)}</p>{/if}
        {#if drift.note}<p class="muted" role="status">{drift.note}</p>{/if}
        <div class="actions">
          {#if pending(drift).length > 1}<Button disabled={busy} title="Accept what the check saw for every request listed" onclick={() => accept([])}>Accept all ({pending(drift).length})</Button>{/if}
          <Button title="The command that runs this check from a script or CI" onclick={() => copyText(drift?.ci ?? '', 'the command')}>Copy the CI command</Button>
        </div>
        <code class="ci">{drift.ci}</code>
      {/if}

    {:else if tool === 'sequence'}
      {#if sequence}
        <p class="note">Flowi and one lifeline for each host it called, an arrow out and one back for each attempt with its status and time; a crossed or red arrow is a refusal or an error. Only host and path are drawn, never the query.</p>
        <Toggle checked={quiet} onchange={(on) => { quiet = on; draw() }}>Show steps that call nothing (delays, sets, assertions)</Toggle>
        <div class="diagram"><SequenceDiagram seq={sequence} /></div>
        <div class="actions">
          <Button onclick={copy}>{copied ? 'Copied' : 'Copy as Mermaid'}</Button>
          <Button onclick={close}>Close</Button>
        </div>
      {:else}
        <div class="actions"><Button onclick={close}>Close</Button></div>
      {/if}
    {/if}
    {#if error}<p class="bad">{error}</p>{/if}
  </Modal>
{/if}

<style>
  .tools { position: relative; }
  button { background: var(--raised); color: var(--text); border: 0; box-shadow: inset 0 0 0 1px var(--line); border-radius: var(--r-md); padding: 0 12px; min-height: var(--ctl-h); font: inherit; font-weight: 500; }
  button:hover:not(:disabled) { background: var(--raised-3); }
  button:disabled { opacity: var(--op-disabled); }
  .primary { background: var(--send-fill); color: var(--on-send); border: 0; font-weight: 600; box-shadow: var(--send-edge), 0 4px 14px var(--send-glow); }
  .note, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .group { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 14px; font-size: var(--fs-sm); }
  .group > strong { min-width: 90px; }
  .opt { display: flex; align-items: center; gap: 4px; font-size: var(--fs-sm); }
  input:not([type]) { padding: 3px 8px; }
  .actions { display: flex; align-items: center; justify-content: flex-end; gap: 8px; margin-top: 4px; }
  .actions .muted { margin-right: auto; }
  table.grid { border-collapse: collapse; font-size: var(--fs-sm); margin: 4px 0; }
  table.grid th, table.grid td { border: 1px solid var(--line); padding: 4px 12px; text-align: center; }
  table.grid th { color: var(--muted); font-weight: 600; }
  .passed, .denied { color: var(--ok); }
  .failed, .allowed, .bad { color: var(--fail); }
  .error, .unclear { color: var(--warn); }
  .cancelled { color: var(--muted); }
  .ok { color: var(--ok); margin: 2px 0; }
  .rowact { margin-top: 4px; }
  .ci { display: block; margin-top: 4px; font: var(--fs-xs) var(--mono); color: var(--muted); overflow-wrap: anywhere; }
  .drift { border-bottom: 1px solid var(--line); padding: 4px 0; font-size: var(--fs-sm); }
  .diff.fails { color: var(--fail); }
  .diff { padding-left: 14px; }
  code { font: var(--fs-xs) var(--mono); }
  .diagram { overflow: auto; border: 1px solid var(--line); border-radius: var(--r-md); padding: 6px; background: var(--bg); }
</style>
