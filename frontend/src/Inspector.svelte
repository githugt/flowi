<script lang="ts">
  import { confirmDeleteSteps } from './lib/flowDelete'
  import { errorText } from './lib/errors'
  import { FlowService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import type { FlowView, Pair, ValueRow } from './lib/types'
  import ValuesEditor from './ValuesEditor.svelte'
  import TextField from './TextField.svelte'
  import ParamFields from './ParamFields.svelte'
  import { completeFor, vocabularyFor } from './lib/flowfields'
  import Hint from './Hint.svelte'
  import TemplateField from './TemplateField.svelte'
  import ExprHelp from './ExprHelp.svelte'
  import PairsEditor from './PairsEditor.svelte'
  import { ui } from './lib/ui.svelte'
  import { lostText, modesOf } from './lib/modes'

  // editing: the flow editor is in Edit, where steps can be deleted.
  let { view, editing = true }: { view: FlowView; editing?: boolean } = $props()
  const node = $derived(view.node)
  const step = $derived(view.form.step)

  // The steps before it that it could wait for besides the one it follows anyway.
  const waitChoices = $derived(step ? step.needChoices.filter((id) => id !== step.previous && !step.needs.includes(id)) : [])
  const flow = $derived(view.form.flow)
  let error = $state('')

  async function text(field: string, value: string) {
    error = ''
    try {
      await FlowService.SetText(node, field, value)
    } catch (e) {
      error = errorText(e)
    }
  }
  const pairs = (field: string) => (rows: Pair[]) => FlowService.SetPairs(node, field, rows)

  async function rename(to: string) {
    error = ''
    try {
      await FlowService.Rename(node, to)
    } catch (e) {
      error = errorText(e)
    }
  }

  // Steps that make sense to draw with a request: not requests themselves, and not containers.
  const attachable = $derived((view.palette ?? []).filter((p) => !['request', 'subflow', 'parallel', 'condition', 'batch', 'run_till'].includes(p.type)))
  function addAttached(e: MouseEvent, after: boolean, side = '') {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    ui.openMenu(r.left, r.bottom + 4, attachable.map((p) => ({ label: p.type, run: () => (side ? FlowService.AddAttachedAt(node, p.type, side) : FlowService.AddAttached(node, p.type, after)) })))
  }
  const complete = $derived(completeFor(node))
  const vocabulary = (context: string) => vocabularyFor(node, context)

  const failLabels: Record<string, string> = {
    fail: 'Stop the run',
    continue: 'Carry on with the next step',
    goto: 'Go to another step',
  }

  const modes = $derived(step ? modesOf(step.type) : [])

  // Changing how a step does its job converts it. What cannot carry over is asked about first; Undo brings it back.
  async function changeMode(node_: string) {
    const to = modes.find((m) => m.node === node_)
    if (!step || !to) return
    const lost = await FlowService.ConvertStep(node, step.id, to.type, to.mode, false)
    const apply = () => FlowService.ConvertStep(node, step.id, to.type, to.mode, true)
    if (lost.length) ui.confirm(`Change to ${to.label.toLowerCase()}?`, lostText(lost), apply, 'Change')
    else await apply()
  }
</script>

<div class="inspector">
  {#if step}
    <div class="head">
      <h3>{step.type} <span class="id">{step.id}</span></h3>
      {#if editing && modes.length > 1}
        <select class="mode" aria-label="How this step does its job" value={step.type} onchange={(e) => changeMode(e.currentTarget.value)}>
          {#each modes as m (m.node)}<option value={m.node}>{m.label}</option>{/each}
        </select>
      {/if}
      <button class="skip" class:on={view.skips.steps.includes(step.id)} aria-pressed={view.skips.steps.includes(step.id)} title="Leave this step out of the next runs (not saved in the flow)" onclick={() => FlowService.ToggleSkip(node, step.id)}>{view.skips.steps.includes(step.id) ? 'Skipped' : 'Skip'}</button>
      {#if editing}
        <button class="del" title="Delete this step" onclick={() => confirmDeleteSteps(node, view.selection.length ? view.selection : [step.id])}>Delete…</button>
      {/if}
    </div>
    <p class="summary">{step.summary}</p>

    {#if step.call}
      <label class="field">
        <span class="label">{step.call.kind === 'request' ? 'Request' : 'Flow'}</span>
        <select value={step.call.ref} onchange={(e) => FlowService.SetCall(node, e.currentTarget.value)}>
          <option value="" disabled>Choose…</option>
          {#each step.call.choices as c}<option value={c.id}>{c.label}</option>{/each}
        </select>
      </label>
      {#if step.call.kind === 'request'}
        <TextField label="Version (empty: resolved)" value={step.call.version ?? ''} oninput={(v) => text('version', v)} />
        {@const stack = step.call.overlay ? step.call.overlay.split('+') : []}
        {@const spare = (step.call.overlays ?? []).filter((o) => !stack.includes(o))}
        <label class="field">
          <span class="label">Flowi overlay</span>
          <select value={stack[0] ?? ''} onchange={(e) => text('overlay', [e.currentTarget.value, ...stack.slice(1)].filter(Boolean).join('+'))} title="Changes laid over the version when this step sends: the request as a VIP customer, a slow one…">
            <option value="">None</option>
            {#each step.call.overlays ?? [] as o}<option value={o}>{o}</option>{/each}
            {#if stack[0] && !(step.call.overlays ?? []).includes(stack[0])}<option value={stack[0]}>{stack[0]} (not found)</option>{/if}
          </select>
        </label>
        {#each stack.slice(1) as n}
          <div class="field inline">
            <span class="label">then</span>
            <span class="grow">{n}{#if !(step.call.overlays ?? []).includes(n)} (not found){/if}</span>
            <button class="add" aria-label={`Take ${n} off`} title={`Take ${n} off`} onclick={() => text('overlay', stack.filter((x) => x !== n).join('+'))}>×</button>
          </div>
        {/each}
        {#if stack.length && spare.length}
          <label class="field">
            <span class="label"></span>
            <select value="" onchange={(e) => { const x = e.currentTarget.value; e.currentTarget.value = ''; if (x) text('overlay', [...stack, x].join('+')) }} aria-label="Stack another flowi overlay" title="Lay another overlay over this one; the later one wins where both set a field">
              <option value="">+ stack another</option>
              {#each spare as o}<option value={o}>{o}</option>{/each}
            </select>
          </label>
        {/if}
        <TextField label="Open connection" value={step.call.open ?? ''} oninput={(v) => text('open', v)} placeholder="name (WebSocket, MQTT 5, NATS)" />
        <TextField label="Run over connection" value={step.call.over ?? ''} oninput={(v) => text('over', v)} placeholder="a name opened by an earlier step" />
        {#if step.call.over}
          <label class="check"><input type="checkbox" checked={step.call.close ?? false} onchange={(e) => FlowService.SetClose(node, e.currentTarget.checked)} /> Close it after this step</label>
        {/if}
      {/if}
      <span class="label">With</span>
      <PairsEditor rows={step.call.with} onchange={pairs('with')} valueLabel="value (template)" />
      {#if step.call.kind === 'request'}
        <label class="field">
          <span class="label">Hooks</span>
          <select value={step.call.hooks} onchange={(e) => FlowService.SetHooks(node, e.currentTarget.value)}>
            {#each ['all', 'none', 'skip-pre', 'skip-post'] as m}<option value={m}>{m}</option>{/each}
          </select>
        </label>
        <span class="label">Extra headers</span>
        <PairsEditor rows={step.call.headers ?? []} onchange={pairs('headers')} keyLabel="header" valueLabel="value (template)" />
      {/if}
    {/if}

    {#if step.type === 'request' || step.type === 'subflow'}
      <h4>Actions shown with this request<Hint>Steps drawn in one block with this {step.type}, before and after it. The run order is the order of the list: moving one here (or dragging it inside the block) changes when it runs.</Hint></h4>
      {#each step.attached as id, i}
        <div class="line">
          <button class="lane named" onclick={() => FlowService.Select(node, id)}>{id}</button>
          <button class="add" title="Move earlier" disabled={i === 0} onclick={() => FlowService.MoveAttachedBy(node, id, -1)}>↑</button>
          <button class="add" title="Move later" disabled={i === step.attached.length - 1} onclick={() => FlowService.MoveAttachedBy(node, id, 1)}>↓</button>
        </div>
      {/each}
      <!-- Above and below are where a step is drawn, not when it runs: they read like a third and fourth
           timing beside Before and After. The rows now say when, and the buttons where. -->
      <div class="line"><span class="when">Runs before</span>
        <button class="add" onpointerdown={(e) => e.stopPropagation()} onclick={(e) => addAttached(e, false)} title="Drawn beside the request, on its left; runs right before it">+ Beside</button>
        <button class="add" onpointerdown={(e) => e.stopPropagation()} onclick={(e) => addAttached(e, false, 'above')} title="Drawn in a stack above the request; runs right before it">+ Stacked above</button>
      </div>
      <div class="line"><span class="when">Runs after</span>
        <button class="add" onpointerdown={(e) => e.stopPropagation()} onclick={(e) => addAttached(e, true)} title="Drawn beside the request, on its right; runs right after it">+ Beside</button>
        <button class="add" onpointerdown={(e) => e.stopPropagation()} onclick={(e) => addAttached(e, true, 'below')} title="Drawn in a stack below the request; runs right after it">+ Stacked below</button>
      </div>
    {:else if step.attachTo.length || step.attach}
      <label class="field">
        <span class="label">Shown with request<Hint>Draws this step in one block with that request (it has to sit right next to it in the list). The run order does not change.</Hint></span>
        <select value={step.attach} onchange={(e) => FlowService.SetAttach(node, e.currentTarget.value)}>
          <option value="">none: a step of the flow</option>
          {#each [...new Set([...step.attachTo, ...(step.attach ? [step.attach] : [])])] as r}<option value={r}>{r}</option>{/each}
        </select>
      </label>
      {#if step.attach}
        <label class="field">
          <span class="label">Drawn</span>
          <select value={step.attachAt} onchange={(e) => FlowService.SetAttachAt(node, e.currentTarget.value)}>
            <option value="">beside the request</option>
            <option value="above">in a stack above the request</option>
            <option value="below">in a stack below the request</option>
          </select>
        </label>
      {/if}
    {/if}

    {#if step.type === 'set'}
      <h4>Values<Hint>Each row writes a variable: from an expression, text as written, a part of the last response, or a generator. Later steps read it as {'{{vars.name}}'}.</Hint></h4>
      <ValuesEditor rows={step.values ?? []} onchange={(rows: ValueRow[]) => FlowService.SetValues(node, rows)} />
    {/if}
    <ParamFields {view} />

    {#if step.variables}
      <h4>Variables<ExprHelp context="flow.template" /><Hint>Each row sets a variable to a template, for example token = {'{{nodes.login.body.token}}'}. Later steps read it as {'{{vars.token}}'}.</Hint></h4>
      <PairsEditor rows={step.variables} onchange={pairs('variables')} keyLabel="variable" valueLabel="value (template)" />
    {/if}

    {#if step.type === 'condition'}
      {#if step.cases}
        <h4>If … else if … else<Hint>Tried from the top: the first test that is true runs its steps. When none is, the else lane runs. Drop steps into a lane on the canvas.</Hint></h4>
        {#each step.cases as c, i}
          <div class="lane">
            <TemplateField label={`${c.label} (a test, no braces)`} help="flow.check" placeholder="nodes.login.status == 404" value={c.when ?? ''} oninput={(v) => FlowService.SetCaseWhen(node, i, v)} vocabulary={vocabulary('flow.check')} />
            <button disabled={!c.empty} title={c.empty ? 'Remove this case' : 'Move its steps out first'} onclick={() => FlowService.RemoveLane(node, i)}>×</button>
          </div>
        {/each}
      {:else}
        <p class="hint">Runs the <strong>if</strong> lane when the test is true, the <strong>else</strong> lane when it is not. Drop steps into a lane on the canvas.</p>
      {/if}
      <button class="add" onclick={() => FlowService.AddLane(node)}>+ Add else if</button>
    {/if}
    {#if step.branches}
      <h4>Branches<Hint>Branches run at the same time. Drop steps into a branch's lane on the canvas.</Hint></h4>
      {#each step.branches as b, i}
        <div class="lane named">
          <span>{b.label}</span>
          <button disabled={!b.empty} title={b.empty ? 'Remove' : 'Move its steps out first'} onclick={() => FlowService.RemoveLane(node, i)}>×</button>
        </div>
      {/each}
      <button class="add" onclick={() => FlowService.AddLane(node)}>Add branch</button>
    {/if}

    <h4>Order<Hint>A step runs after the one before it in its list; that needs nothing extra. Add a wait only when it must also wait for another step.</Hint></h4>
    <p class="hint">
      {#if step.previous}
        Runs after <button class="inline" onclick={() => FlowService.Select(node, step.previous)}>{step.previous}</button>.
      {:else}
        The first step of its list.
      {/if}
    </p>
    {#each step.needs as id}
      <div class="line">
        <button class="lane named" onclick={() => FlowService.Select(node, id)}>also waits for {id}</button>
        <button class="add" title="Stop waiting for {id}" onclick={() => FlowService.RemoveNeed(node, id)}>×</button>
      </div>
    {/each}
    {#if waitChoices.length}
      <select class="wait" value="" onchange={(e) => { FlowService.AddNeed(node, e.currentTarget.value); e.currentTarget.value = '' }}>
        <option value="">+ also wait for…</option>
        {#each waitChoices as id}<option value={id}>{id}</option>{/each}
      </select>
    {/if}

    {#if step.uses.length || step.usedBy.length}
      <h4>Values<Hint>Where this step takes values from, and who takes values from it. The canvas draws these as dotted lines under the cards.</Hint></h4>
      {#each step.uses as u}
        <button class="lane named" onclick={() => FlowService.Select(node, u.from)}><span>← <code>{u.name}</code>{#if u.type} <span class="type" title="What the value is">{u.type}</span>{/if}</span><span class="muted">from {u.from}</span></button>
      {/each}
      {#each step.usedBy as u}
        <button class="lane named" onclick={() => FlowService.Select(node, u.to)}><span>→ <code>{u.name}</code>{#if u.type} <span class="type" title="What the value is">{u.type}</span>{/if}</span><span class="muted">used by {u.to}</span></button>
      {/each}
    {/if}

    <details>
      <summary>General</summary>
      <TextField label="Id (Enter to rename)" value={step.id} onenter={rename} />
      <TextField label="Name" value={step.name} oninput={(v) => text('name', v)} />
      <TextField label="Description" value={step.description} oninput={(v) => text('description', v)} />
      <TemplateField label="Run when (expression)" help="flow.when" placeholder="vars.count > 3" value={step.when} oninput={(v) => text('when', v)} suggest={complete} vocabulary={vocabulary('flow.when')} />
      <TextField label="Timeout (e.g. 5s)" value={step.timeout} oninput={(v) => text('timeout', v)} />
      <label class="check">
        <input type="checkbox" checked={step.concurrent} onchange={(e) => FlowService.SetConcurrent(node, e.currentTarget.checked)} />
        <span>Concurrent (may run alongside earlier nodes)</span>
      </label>
      <label class="field">
        <span class="label">If this step fails</span>
        <select value={step.failMode} onchange={(e) => FlowService.SetFailRoute(node, e.currentTarget.value, step.failTarget)}>
          {#each Object.entries(failLabels) as [m, l]}<option value={m}>{l}</option>{/each}
        </select>
      </label>
      {#if step.failMode === 'goto'}
        <select value={step.failTarget} onchange={(e) => FlowService.SetFailRoute(node, 'goto', e.currentTarget.value)}>
          <option value="" disabled>Choose a step</option>
          {#each step.steps as s}<option value={s}>{s}</option>{/each}
        </select>
      {/if}
    </details>
  {:else}
    <h3>Flow</h3>
    <TextField label="Name" value={flow.name} oninput={(v) => text('flowName', v)} />
    <TextField label="Description" value={flow.description} oninput={(v) => text('flowDescription', v)} />
    <TextField label="Prefer version" value={flow.preferVersion} oninput={(v) => text('preferVersion', v)} />
    <span class="label">Inputs</span>
    <PairsEditor rows={flow.inputs} onchange={pairs('inputs')} valueLabel="default (template)" />
    <span class="label">Outputs (for subflows)</span>
    <PairsEditor rows={flow.outputs} onchange={pairs('outputs')} valueLabel="value (template)" />
    <details>
      <summary>Execution</summary>
      <TextField label="Max concurrency (empty: unlimited)" value={flow.maxConcurrency} oninput={(v) => text('maxConcurrency', v)} />
      <TextField label="Rate limit (e.g. 50/s)" value={flow.rateLimit} oninput={(v) => text('rateLimit', v)} />
      <label class="field">
        <span class="label">On branch error</span>
        <select value={flow.onBranchError} onchange={(e) => FlowService.SetBranchPolicy(node, e.currentTarget.value)}>
          {#each ['', 'fail_fast', 'finish_running', 'continue_all'] as m}<option value={m}>{m}</option>{/each}
        </select>
      </label>
    </details>
  {/if}
  {#if error}<p class="err">{error}</p>{/if}
  {#if view.error}<p class="err">{view.error}</p>{/if}
</div>

<style>
  .inspector {
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  h3 { margin: 0; font-size: var(--fs-md); }
  /* The side panel is narrow: the title, the mode and the buttons wrap onto lines of their own instead of squeezing. */
  .head { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 8px; }
  .head h3 { flex: 1 1 100%; }
  .head button { white-space: nowrap; }
  .del { background: none; border: 1px solid var(--line); border-radius: var(--r-md); color: var(--muted); font: inherit; font-size: var(--fs-sm); padding: 2px 8px; }
  .skip { background: none; border: 1px solid var(--line); border-radius: var(--r-md); color: var(--muted); font: inherit; font-size: var(--fs-sm); padding: 2px 8px; }
  .skip.on { color: var(--text); border-color: var(--muted); background: var(--tint-hover); }
  .del:hover { color: var(--fail); border-color: var(--fail); }
  h4 { margin: 8px 0 0; font-size: var(--fs-sm); }
  .id { color: var(--muted); font-weight: 400; margin-left: 6px; }
  .summary, .hint, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .field { display: flex; flex-direction: column; gap: 3px; }
  .label { color: var(--muted); font-size: var(--fs-xs); }
  .field.inline { flex-direction: row; align-items: center; }
  .field.inline .grow { flex: 1; }
  .check { display: flex; align-items: center; gap: 6px; }
  .mode { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 6px; font: inherit; font-size: var(--fs-sm); }
  .type { color: var(--muted); font-size: var(--fs-xs); border: 1px solid var(--line); border-radius: var(--r-pill); padding: 0 6px; }
  select {
    background: var(--bg);
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    padding: 5px 6px;
    font: inherit;
  }
  details { display: flex; flex-direction: column; gap: 8px; }
  summary { cursor: default; font-weight: 600; font-size: var(--fs-sm); margin-bottom: 4px; }
  .lane { display: flex; align-items: flex-end; gap: 4px; }
  .lane :global(.field) { flex: 1; }
  .lane.named { align-items: center; justify-content: space-between; }
  .line { display: flex; gap: 12px; }
  button.lane.named { width: 100%; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 8px; font: inherit; text-align: left; }
  .lane button, .add {
    background: none;
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    color: var(--text);
    font: inherit;
    padding: 4px 8px;
  }
  .lane button:disabled { opacity: 0.35; }
  .add { align-self: flex-start; color: var(--accent); }
  .when { color: var(--muted); font-size: var(--fs-xs); min-width: 72px; align-self: center; }
  button.inline { background: none; border: 0; padding: 0; color: var(--accent); font: inherit; cursor: pointer; }
  select.wait { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 6px; font: inherit; }
  .err { color: var(--fail); margin: 0; font-size: var(--fs-sm); }
</style>
