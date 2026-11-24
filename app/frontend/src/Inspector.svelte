<script lang="ts">
  import IconButton from './IconButton.svelte'
  import Field from './Field.svelte'
  import Select from './Select.svelte'
  import Toggle from './Toggle.svelte'
  import Button from './Button.svelte'
  import FoldSection from './FoldSection.svelte'
  import Tabs, { type TabItem } from './Tabs.svelte'
  import { nodeUi, setNodeUi } from './lib/nodeui'
  import { untrack } from 'svelte'
  import { confirmDeleteSteps } from './lib/flowDelete'
  import { errorText } from './lib/errors'
  import InlineName from './InlineName.svelte'
  import LabelsForm from './LabelsForm.svelte'
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { FlowView, Pair, ValueRow } from './lib/types'
  import ValuesEditor from './ValuesEditor.svelte'
  import TextInput from './TextInput.svelte'
  import ParamFields from './ParamFields.svelte'
  import { completeFor, vocabularyFor } from './lib/flowfields'
  import Hint from './Hint.svelte'
  import TemplateField from './TemplateField.svelte'
  import ExprHelp from './ExprHelp.svelte'
  import PairsEditor from './PairsEditor.svelte'
  import OutcomesEditor from './OutcomesEditor.svelte'
  import { ui } from './lib/ui.svelte'
  import { openStepTarget } from './lib/opentarget'
  import { typeLabel } from './lib/card'
  import { lostText, modesOf } from './lib/modes'
  import { subflowNotes } from './lib/subflownotes'

  // editing: the flow editor is in Edit, where steps can be deleted.
  let { view, editing = true, onaddinside }: { view: FlowView; editing?: boolean; onaddinside?: (lane: number, e: MouseEvent) => void } = $props()
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

  const complete = $derived(completeFor(node))
  const vocabulary = (context: string) => vocabularyFor(node, context)

  const failLabels: Record<string, string> = {
    fail: 'Stop the run',
    continue: 'Carry on with the next step',
    goto: 'Go to another step',
  }

  const modes = $derived(step ? modesOf(step.type) : [])

  // The settings of a step are in sections, so a request's twelve fields are not one scroll: what it calls, how it is
  // sent, how it can end, when and in what order it runs, and what it is called. The strip stays in view; a count says
  // how much is set in a section without opening it. The section chosen is kept for the flow, and a step that has no
  // such section (a Set step has no Outcomes) shows the first.
  // Before: a request step's form was one column of Request, Version, Overlay, two connection fields, With, Hooks,
  // Headers, Outcomes, Order and a fold of timing, with the fields used most between the ones used least.
  let chosen = $state(untrack(() => nodeUi(view.node).stepSection))
  $effect(() => setNodeUi(node, { stepSection: chosen }))
  let showConnection = $state(false)
  const asyncRequest = $derived(!!step?.call && (step.call.ruleKind === 'message' || !!step.call.open || !!step.call.over))
  const sections = $derived.by((): TabItem[] => {
    if (!step) return []
    const call = step.call
    const set = (n: number) => (n > 0 ? n : undefined)
    const list: TabItem[] = [{ id: 'main', label: typeLabel(step.type), count: call ? set(call.with.length + (call.headers?.length ?? 0)) : step.values ? set(step.values.length) : undefined }]
    if (call?.kind === 'request') {
      const sent = [call.version, call.overlay, call.open, call.over].filter(Boolean).length + (call.hooks && call.hooks !== 'all' ? 1 : 0)
      list.push({ id: 'send', label: 'Send', count: set(sent) })
      list.push({ id: 'outcomes', label: 'Outcomes', count: call.outcomesOwn ? set(call.outcomes?.length ?? 0) : undefined })
    }
    const run = [step.when, step.timeout, step.retries, step.concurrent, step.failMode !== 'fail', step.needs.length].filter(Boolean).length
    list.push({ id: 'run', label: 'Run', count: set(run) })
    list.push({ id: 'name', label: 'Name', count: set([step.name, step.description].filter(Boolean).length) })
    return list
  })
  const section = $derived(sections.some((x) => x.id === chosen) ? chosen : 'main')

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

{#snippet errors()}
  {#if error}<p class="err">{error}</p>{/if}
  {#if view.error}<p class="err">{view.error}</p>{/if}
{/snippet}

<div class="inspector">
  {#if step}
    <div class="pin">
    <div class="head">
      <h3><span class="kind">{typeLabel(step.type)}</span><span class="stepname">{step.id}</span></h3>
      <!-- A skip is saved in the flow, so only Edit changes it; without Edit the mark is shown, not offered. -->
      {#if editing}
        <button class="skip" class:on={view.skips.steps.includes(step.id)} aria-pressed={view.skips.steps.includes(step.id)} title="Leave this step out of every run of the flow (saved with the flow)" onclick={() => FlowService.ToggleSkip(node, step.id).catch(ui.fail)}>{view.skips.steps.includes(step.id) ? 'Skipped' : 'Skip'}</button>
      {:else if view.skips.steps.includes(step.id)}
        <span class="skip on" title="This step is left out of every run of the flow">Skipped</span>
      {/if}
      {#if editing}
        <Button variant="ghost" small danger title="Delete this step" onclick={() => confirmDeleteSteps(node, view.selection.length ? view.selection : [step.id])}>Delete…</Button>
      {/if}
    </div>
    {#if modes.length > 1}
      <!-- Changing it converts the step, which is an edit: without Edit the choice is shown and cannot be changed. -->
      <span class="mode"><Select label="How this step does its job" disabled={!editing} title={editing ? '' : 'Turn on Edit to change how this step does its job'} value={step.type} onchange={(e) => changeMode(e.currentTarget.value)}>
        {#each modes as m (m.node)}<option value={m.node}>{m.label}</option>{/each}
      </Select></span>
    {/if}
      <Tabs label="Step settings" size="sm" wrap items={sections} active={section} onpick={(id) => (chosen = id)} />
    </div>
    <div class="pane">
      {#if section === 'main'}
      <p class="summary">{step.summary}</p>
      {#if step.call}
      <Field label={`${step.call.kind === 'request' ? 'Request' : 'Flow'}`}>
        <Select value={step.call.ref} onchange={(e) => FlowService.SetCall(node, e.currentTarget.value)}>
          <option value="" disabled>Choose…</option>
          {#each step.call.choices as c}<option value={c.id}>{c.label}</option>{/each}
        </Select>
      </Field>
      {#if step.call.ref}<span class="open-target"><Button variant="link" onclick={() => openStepTarget(node, step.id)}>Open {step.call.kind === 'request' ? 'request' : 'flow'} ↗</Button></span>{/if}
      <span class="label">With</span>
      <PairsEditor rows={step.call.with} onchange={pairs('with')} valueLabel="value (template)" />
      {#if step.call.kind === 'subflow'}
        {@const notes = subflowNotes(step.id, step.call.with.map((p) => p.key), step.call.inputs, step.call.outputs)}
        {#if notes.inputs.length}<p class="hint">Inputs of this flow: {notes.inputs.join(', ')}.</p>{/if}
        {#if notes.outputs.length}<p class="hint">Later steps can read: {#each notes.outputs as o, i}<code>{o}</code>{i < notes.outputs.length - 1 ? ', ' : ''}{/each}.</p>{/if}
        {#if notes.unknown.length}<p class="hint warn">This flow has no input named {notes.unknown.join(', ')}.</p>{/if}
        {#if notes.missing.length}<p class="hint warn">{notes.missing.join(', ')} {notes.missing.length === 1 ? 'has' : 'have'} no default and {notes.missing.length === 1 ? 'is' : 'are'} not set here.</p>{/if}
      {/if}
        {#if step.call.kind === 'request'}
        <span class="label">Extra headers</span>
        <PairsEditor rows={step.call.headers ?? []} onchange={pairs('headers')} keyLabel="header" valueLabel="value (template)" />
        {/if}
      {/if}
      <!-- "Shown with request" is not offered: the canvas draws every step as its own card, so where an action was
           "drawn" around its request changes nothing on screen. -->

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
          <h4>If … else if … else<Hint>Tried from the top: the first test that is true runs its steps. When none is, the else branch runs. Drop steps into a branch on the canvas.</Hint></h4>
          {#each step.cases as c, i}
            <div class="lane">
              {#if onaddinside}<Button small variant="ghost" title={`Add a step to ${c.label}`} onclick={(e) => onaddinside(i, e)}>+ step</Button>{/if}
              <TemplateField label={`${c.label} (a test, no braces)`} help="flow.check" placeholder="nodes.login.status == 404" value={c.when ?? ''} oninput={(v) => FlowService.SetCaseWhen(node, i, v)} vocabulary={vocabulary('flow.check')} />
              <IconButton icon="close" size={12} danger disabled={!c.empty} label="Remove this case" hint={c.empty ? 'Remove this case' : 'Move its steps out first'} onclick={() => FlowService.RemoveLane(node, i)} />
            </div>
          {/each}
        {:else}
          <p class="hint">Runs the <strong>if</strong> branch when the test is true, the <strong>else</strong> branch when it is not. Drop steps into a branch on the canvas.</p>
        {/if}
        {#if onaddinside}
          {#if step.cases}
            <Button small variant="ghost" onclick={(e) => onaddinside(step.cases!.length, e)}>+ Add a step to else</Button>
          {:else}
            <Button small variant="ghost" onclick={(e) => onaddinside(0, e)}>+ Add a step to if</Button>
            <Button small variant="ghost" onclick={(e) => onaddinside(1, e)}>+ Add a step to else</Button>
          {/if}
        {/if}
        <Button variant="link" onclick={() => FlowService.AddLane(node)}>+ Add else if</Button>
      {/if}
      {#if step.type === 'batch' && onaddinside}
        <h4>Steps inside<Hint>The steps that repeat. They run in order each time round.</Hint></h4>
        <Button small variant="ghost" onclick={(e) => onaddinside(0, e)}>+ Add a step inside the loop</Button>
      {/if}
      {#if step.branches}
        <h4>Branches<Hint>Branches run at the same time. Drop steps into a branch on the canvas.</Hint></h4>
        {#each step.branches as b, i}
          <div class="lane named">
            <span>{b.label}</span>
            {#if onaddinside}<Button small variant="ghost" title={`Add a step to ${b.label}`} onclick={(e) => onaddinside(i, e)}>+ step</Button>{/if}
            <IconButton icon="close" size={12} danger disabled={!b.empty} label="Remove this branch" hint={b.empty ? 'Remove' : 'Move its steps out first'} onclick={() => FlowService.RemoveLane(node, i)} />
          </div>
        {/each}
        <Button variant="link" onclick={() => FlowService.AddLane(node)}>Add branch</Button>
      {/if}

      {:else if section === 'send' && step.call?.kind === 'request'}
        <Field label="Version (empty: resolved)"><TextInput value={step.call.version ?? ''} ontext={(v) => text('version', v)} /></Field>
        {@const stack = step.call.overlay ? step.call.overlay.split('+') : []}
        {@const spare = (step.call.overlays ?? []).filter((o) => !stack.includes(o))}
        <Field label="Flowi overlay">
          <Select value={stack[0] ?? ''} onchange={(e) => text('overlay', [e.currentTarget.value, ...stack.slice(1)].filter(Boolean).join('+'))} title="Changes laid over the version when this step sends: the request as a VIP customer, a slow one…">
            <option value="">None</option>
            {#each step.call.overlays ?? [] as o}<option value={o}>{o}</option>{/each}
            {#if stack[0] && !(step.call.overlays ?? []).includes(stack[0])}<option value={stack[0]}>{stack[0]} (not found)</option>{/if}
          </Select>
        </Field>
        {#each stack.slice(1) as n}
          <div class="field inline">
            <span class="label">then</span>
            <span class="grow">{n}{#if !(step.call.overlays ?? []).includes(n)} (not found){/if}</span>
            <IconButton icon="close" size={12} label={`Take ${n} off`} onclick={() => text('overlay', stack.filter((x) => x !== n).join('+'))} />
          </div>
        {/each}
        {#if stack.length && spare.length}
          <Field label="">
            <Select value="" onchange={(e) => { const x = e.currentTarget.value; e.currentTarget.value = ''; if (x) text('overlay', [...stack, x].join('+')) }} label="Stack another flowi overlay" title="Lay another overlay over this one; the later one wins where both set a field">
              <option value="">+ stack another</option>
              {#each spare as o}<option value={o}>{o}</option>{/each}
            </Select>
          </Field>
        {/if}
        <Field label="Hooks">
          <Select value={step.call.hooks} onchange={(e) => FlowService.SetHooks(node, e.currentTarget.value)}>
            {#each ['all', 'none', 'skip-pre', 'skip-post'] as m}<option value={m}>{m}</option>{/each}
          </Select>
        </Field>
        <h4>Connection<Hint>Keep a connection open for later steps, or send over one an earlier step opened. For WebSocket, MQTT 5 and NATS requests.</Hint></h4>
        {#if asyncRequest || showConnection}
          <Field label="Open connection"><TextInput value={step.call.open ?? ''} ontext={(v) => text('open', v)} placeholder="name (WebSocket, MQTT 5, NATS)" /></Field>
          <Field label="Run over connection"><TextInput list="flow-connections" value={step.call.over ?? ''} ontext={(v) => text('over', v)} placeholder="a name opened by an earlier step" /></Field>
          <datalist id="flow-connections">{#each step.call.connections ?? [] as c}<option value={c}></option>{/each}</datalist>
          {#if step.call.over && !(step.call.connections ?? []).includes(step.call.over)}
            <p class="hint warn">No step in this flow opens “{step.call.over}”. It works only if a flow that calls this one opened it.</p>
          {/if}
          {#if step.call.over}
            <Toggle checked={step.call.close ?? false} onchange={(on) => FlowService.SetClose(node, on)}>Close it after this step</Toggle>
          {/if}
        {:else}
          <Button variant="link" onclick={() => (showConnection = true)}>Keep a connection open between steps…</Button>
        {/if}
      {:else if section === 'outcomes' && step.call?.kind === 'request'}
        <h4>Outcomes<Hint>The ways this request ends, which the arrows leaving it follow. {step.call.outcomesOwn ? 'This step has its own list, used in this flow only.' : "These are the request's. Change one and this step gets its own list, used in this flow only."}</Hint></h4>
        <OutcomesEditor outcomes={step.call.outcomes} ruleKind={step.call.ruleKind} resetLabel="Use the request's" hideReset={!step.call.outcomesOwn}
          onchange={(rows) => FlowService.SetStepOutcomes(node, step.id, rows as never).catch(ui.fail)}>
          {#snippet footer()}
            <Button title="Write this list into the request, so every flow that uses it has these outcomes" disabled={!step.call?.outcomesOwn}
              onclick={() => FlowService.SaveStepOutcomesToRequest(node, step.id).then(() => ui.toast('Saved to the request.')).catch(ui.fail)}>Save to the request</Button>
          {/snippet}
        </OutcomesEditor>
      {:else if section === 'run'}
      <h4>Order<Hint>A step runs after the one before it in its list; that needs nothing extra. Add a wait only when it must also wait for another step.</Hint></h4>
      <p class="hint">
        {#if step.previous}
          Runs after <Button variant="link" onclick={() => FlowService.Select(node, step.previous)}>{step.previous}</Button>.
        {:else}
          The first step of its list.
        {/if}
      </p>
      {#each step.needs as id}
        <div class="line">
          <button class="lane named" onclick={() => FlowService.Select(node, id)}>also waits for {id}</button>
          <IconButton icon="close" size={12} label="Stop waiting for {id}" onclick={() => FlowService.RemoveNeed(node, id)} />
        </div>
      {/each}
      {#if waitChoices.length}
        <Select value="" onchange={(e) => { FlowService.AddNeed(node, e.currentTarget.value); e.currentTarget.value = '' }}>
          <option value="">+ also wait for…</option>
          {#each waitChoices as id}<option value={id}>{id}</option>{/each}
        </Select>
      {/if}

        <h4>When &amp; failure</h4>
        <TemplateField label="Run when (expression)" help="flow.when" placeholder="vars.count > 3" value={step.when} oninput={(v) => text('when', v)} suggest={complete} vocabulary={vocabulary('flow.when')} />
        <Field label="Timeout (e.g. 5s)"><TextInput value={step.timeout} ontext={(v) => text('timeout', v)} /></Field>
        {#if step.canRetry}
          <Field label="Retries (0 to 10; a step that passes only after one is reported as flaky)"><TextInput value={step.retries} ontext={(v) => text('retries', v)} /></Field>
          {#if step.retries}<Field label="Wait before each retry (e.g. 2s)"><TextInput value={step.retryWait} ontext={(v) => text('retryWait', v)} /></Field>{/if}
        {/if}
        <Toggle checked={step.concurrent} onchange={(on) => FlowService.SetConcurrent(node, on)}>Concurrent (may run alongside earlier nodes)</Toggle>
        <Field label="If this step fails">
          <Select value={step.failMode} onchange={(e) => FlowService.SetFailRoute(node, e.currentTarget.value, step.failTarget)}>
            {#each Object.entries(failLabels) as [m, l]}<option value={m}>{l}</option>{/each}
          </Select>
        </Field>
        {#if step.failMode === 'goto'}
          <Select value={step.failTarget} onchange={(e) => FlowService.SetFailRoute(node, 'goto', e.currentTarget.value)}>
            <option value="" disabled>Choose a step</option>
            {#each step.steps as s}<option value={s}>{s}</option>{/each}
          </Select>
        {/if}
      {:else if section === 'name'}
        <InlineName label="Id" value={step.id} commit={(to) => FlowService.Rename(node, to)} />
        <Field label="Name"><TextInput value={step.name} ontext={(v) => text('name', v)} /></Field>
        <Field label="Description"><TextInput value={step.description} ontext={(v) => text('description', v)} /></Field>
      {/if}
      {@render errors()}
    </div>
  {:else}
    <div class="pane">
      <h3>Flow</h3>
      <Field label="Name"><TextInput value={flow.name} ontext={(v) => text('flowName', v)} /></Field>
      <Field label="Description"><TextInput value={flow.description} ontext={(v) => text('flowDescription', v)} /></Field>
      <Field label="Prefer version"><TextInput value={flow.preferVersion} ontext={(v) => text('preferVersion', v)} /></Field>
      <FoldSection name="flow-labels" title="Tags and status"><LabelsForm {node} /></FoldSection>
      <span class="label">Inputs</span>
      <PairsEditor rows={flow.inputs} onchange={pairs('inputs')} valueLabel="default (template)" />
      <span class="label">Outputs (for subflows)</span>
      <PairsEditor rows={flow.outputs} onchange={pairs('outputs')} valueLabel="value (template)" />
      <FoldSection name="flow-execution" title="Execution">
        <Field label="Max concurrency (empty: unlimited)"><TextInput value={flow.maxConcurrency} ontext={(v) => text('maxConcurrency', v)} /></Field>
        <Field label="Rate limit (e.g. 50/s)"><TextInput value={flow.rateLimit} ontext={(v) => text('rateLimit', v)} /></Field>
        <Field label="On branch error">
          <Select value={flow.onBranchError} onchange={(e) => FlowService.SetBranchPolicy(node, e.currentTarget.value)}>
            {#each ['', 'fail_fast', 'finish_running', 'continue_all'] as m}<option value={m}>{m}</option>{/each}
          </Select>
        </Field>
      </FoldSection>
      {@render errors()}
    </div>
  {/if}
</div>

<style>
  .open-target { align-self: flex-start; font-size: var(--fs-sm); }
  .inspector { display: flex; flex-direction: column; }
  /* The step's name and the section strip stay in view while the section under them scrolls. */
  .pin { position: sticky; top: 0; display: flex; flex-direction: column; gap: 8px; padding: 14px 16px 0; background: var(--bg); }
  .pane { padding: 12px 16px 20px; display: flex; flex-direction: column; gap: 10px; }
  h3 { margin: 0; font-size: var(--fs-md); }
  /* The step's kind as a pill, its name beside it, and the two things done to it, kept quiet, at the right. */
  .head { display: flex; align-items: center; gap: 8px; min-width: 0; }
  .head h3 { flex: 1; min-width: 0; display: flex; align-items: center; gap: 8px; font-size: var(--fs-md); font-weight: 600; }
  .kind { flex: none; padding: 0 9px; line-height: 20px; border-radius: var(--r-pill); background: var(--tint-accent); color: var(--accent); font-size: var(--fs-xs); font-weight: 600; }
  .stepname { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .head button { white-space: nowrap; }
  .skip { background: none; border: 0; border-radius: var(--r-md); color: var(--muted); font: inherit; font-size: var(--fs-sm); padding: 2px 6px; }
  .skip:hover { color: var(--text); background: var(--tint-hover); }
  .skip.on { color: var(--warn); background: color-mix(in srgb, var(--warn) 14%, transparent); }
  h4 { margin: 8px 0 0; font-size: var(--fs-sm); }
  .summary, .hint { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .field { display: flex; flex-direction: column; gap: 3px; }
  .label { color: var(--muted); font-size: var(--fs-sm); }
  .field.inline { flex-direction: row; align-items: center; }
  .field.inline .grow { flex: 1; }
  .mode { align-self: flex-start; }
  .lane { display: flex; align-items: flex-end; gap: 4px; }
  .lane :global(.field) { flex: 1; }
  .lane.named { align-items: center; justify-content: space-between; }
  .line { display: flex; gap: 12px; }
  button.lane.named { width: 100%; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 8px; font: inherit; text-align: left; }
  .err { color: var(--fail); margin: 0; font-size: var(--fs-sm); }
</style>
