<script lang="ts">
  import { confirmDeleteSteps } from './lib/flowDelete'
  import { errorText } from './lib/errors'
  import { FlowService, SystemService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import type { FlowView, Pair } from './lib/types'
  import TextField from './TextField.svelte'
  import type { Item } from './lib/exprcomplete'
  import Hint from './Hint.svelte'
  import TemplateField from './TemplateField.svelte'
  import ExprHelp from './ExprHelp.svelte'
  import PairsEditor from './PairsEditor.svelte'
  import { ui } from './lib/ui.svelte'

  // editing: the flow editor is in Edit, where steps can be deleted.
  let { view, editing = true }: { view: FlowView; editing?: boolean } = $props()
  const node = $derived(view.node)
  const step = $derived(view.form.step)

  // What a step's field can read depends on what the field is.
  function helpFor(type: string, p: { name: string; kind: string }): string {
    if (p.kind === 'expr') return 'flow.check'
    if (type === 'batch' && p.name === 'over') return 'flow.collection'
    if (type === 'transform' || type === 'calc') return 'flow.transform'
    return 'flow.template'
  }
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

  // Examples of the expressions a step's check, until and when take: true or false, no braces.
  const exprHint = 'Examples: nodes.login.status == 200 · vars.count > 3 · response.body.token != empty.'

  // Steps that make sense to draw with a request: not requests themselves, and not containers.
  const attachable = $derived((view.palette ?? []).filter((p) => !['request', 'subflow', 'parallel', 'condition', 'batch', 'run_till'].includes(p.type)))
  function addAttached(e: MouseEvent, after: boolean, side = '') {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    ui.openMenu(r.left, r.bottom + 4, attachable.map((p) => ({ label: p.type, run: () => (side ? FlowService.AddAttachedAt(node, p.type, side) : FlowService.AddAttached(node, p.type, after)) })))
  }
  const complete = (t: string, c: number) => FlowService.Complete(node, t, c) as unknown as Promise<any>
  // What a bare expression (a test, a "run when") can name: the context's roots, functions and keywords,
  // then the real names of this step's surroundings, such as nodes.login.body.token.
  const vocabularyFor = (context: string) => async (): Promise<Item[]> => {
    const [roots, names] = await Promise.all([SystemService.ExpressionVocabulary(context), FlowService.ExpressionNames(node)])
    return [...(names as Item[]), ...(roots as Item[])]
  }

  const failLabels: Record<string, string> = {
    fail: 'Stop the run',
    continue: 'Carry on with the next step',
    goto: 'Go to another step',
  }
</script>

<div class="inspector">
  {#if step}
    <div class="head">
      <h3>{step.type} <span class="id">{step.id}</span></h3>
      {#if editing}
        <button class="del" title="Delete this step" onclick={() => confirmDeleteSteps(node, view.selection.length ? view.selection : [step.id])}>Delete step…</button>
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

    {#if step.intro === 'extract'}<h4>Read a value from the response</h4>{/if}
    {#each step.params as p (p.name)}
      {#if p.complex}
        <div class="field">
          <span class="label">{p.name}</span>
          <em class="muted">(a list or mapping: edit it in the file)</em>
        </div>
      {:else if p.kind === 'bool'}
        <label class="check">
          <input type="checkbox" checked={p.value === 'true'} onchange={(e) => FlowService.SetParam(node, p.name, String(e.currentTarget.checked))} />
          <span>{p.name}{p.required ? ' *' : ''}</span>
        </label>
      {:else if p.kind === 'enum'}
        <label class="field">
          <span class="label">{p.name}{p.required ? ' *' : ''}</span>
          <select value={p.value} onchange={(e) => FlowService.SetParam(node, p.name, e.currentTarget.value)}>
            <option value=""></option>
            {#each p.enum ?? [] as o}<option value={o}>{o}</option>{/each}
          </select>
        </label>
      {:else}
        <TemplateField
          label={p.name + (p.required ? ' *' : '')}
          help={helpFor(step.type, p)}
          placeholder={p.kind === 'expr' ? 'nodes.login.status == 200' : p.doc}
          value={p.value}
          mono={p.kind === 'text' || p.kind === 'expr'}
          oninput={(v) => FlowService.SetParam(node, p.name, v)}
          suggest={complete}
          vocabulary={p.kind === 'expr' ? vocabularyFor('flow.check') : undefined}
        />
        {#if p.kind === 'expr'}<p class="hint">{p.doc || 'True or false, written without braces.'} {exprHint}</p>{/if}
      {/if}
    {/each}

    {#if step.variables}
      <h4>{step.varsHeading === 'extract' ? 'Or set variables from templates' : 'Variables'}<ExprHelp context={step.type === 'calc' ? 'flow.transform' : 'flow.template'} /><Hint>Each row sets a variable to a template, for example token = {'{{nodes.login.body.token}}'}. Later steps read it as {'{{vars.token}}'}.</Hint></h4>
      <PairsEditor rows={step.variables} onchange={pairs('variables')} keyLabel="variable" valueLabel="value (template)" />
    {/if}

    {#if step.type === 'condition'}
      {#if step.cases}
        <h4>If … else if … else<Hint>Tried from the top: the first test that is true runs its steps. When none is, the else lane runs. Drop steps into a lane on the canvas.</Hint></h4>
        {#each step.cases as c, i}
          <div class="lane">
            <TemplateField label={`${c.label} (a test, no braces)`} help="flow.check" placeholder="nodes.login.status == 404" value={c.when ?? ''} oninput={(v) => FlowService.SetCaseWhen(node, i, v)} vocabulary={vocabularyFor('flow.check')} />
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
        <button class="lane named" onclick={() => FlowService.Select(node, u.from)}><span>← <code>{u.name}</code></span><span class="muted">from {u.from}</span></button>
      {/each}
      {#each step.usedBy as u}
        <button class="lane named" onclick={() => FlowService.Select(node, u.to)}><span>→ <code>{u.name}</code></span><span class="muted">used by {u.to}</span></button>
      {/each}
    {/if}

    <details>
      <summary>General</summary>
      <TextField label="Id (Enter to rename)" value={step.id} onenter={rename} />
      <TextField label="Name" value={step.name} oninput={(v) => text('name', v)} />
      <TextField label="Description" value={step.description} oninput={(v) => text('description', v)} />
      <TemplateField label="Run when (expression)" help="flow.when" placeholder="vars.count > 3" value={step.when} oninput={(v) => text('when', v)} suggest={complete} vocabulary={vocabularyFor('flow.when')} />
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
  .head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .del { background: none; border: 1px solid var(--line); border-radius: var(--r-md); color: var(--muted); font: inherit; font-size: var(--fs-sm); padding: 2px 8px; }
  .del:hover { color: var(--fail); border-color: var(--fail); }
  h4 { margin: 8px 0 0; font-size: var(--fs-sm); }
  .id { color: var(--muted); font-weight: 400; margin-left: 6px; }
  .summary, .hint, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .field { display: flex; flex-direction: column; gap: 3px; }
  .label { color: var(--muted); font-size: var(--fs-xs); }
  .check { display: flex; align-items: center; gap: 6px; }
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
