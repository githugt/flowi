<script lang="ts">
  // What opens under a card when one of its rows is clicked: the same controls the side inspector has for that part
  // of the step, with nothing else. One row is open at a time; the card's step is the selected one.
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { FlowView, Pair, ValueRow } from './lib/types'
  import PairsEditor from './PairsEditor.svelte'
  import ValuesEditor from './ValuesEditor.svelte'
  import ParamFields from './ParamFields.svelte'
  import OutcomesEditor from './OutcomesEditor.svelte'
  import { ui } from './lib/ui.svelte'
  import { errorText } from './lib/errors'
  import TextField from './TextField.svelte'
  import TemplateField from './TemplateField.svelte'
  import { completeFor, vocabularyFor } from './lib/flowfields'

  let { view, row, onclose }: { view: FlowView; row: string; onclose: () => void } = $props()
  const node = $derived(view.node)
  const step = $derived(view.form.step)
  const call = $derived(step?.call)
  const TITLES: Record<string, string> = {
    inputs: 'Inputs', send: 'Message', receive: 'Receive', connection: 'Connection', hooks: 'Hooks', outcomes: 'Outcomes',
    settings: 'Settings', values: 'Values', checks: 'Checks', steps: 'Steps', message: 'Message',
  }
  const text = (field: string, value: string) => FlowService.SetText(node, field, value)
  const failLabels: Record<string, string> = { fail: 'Stop the run', continue: 'Carry on with the next step', goto: 'Go to another step' }
</script>

{#if step}
  <div class="rowedit" role="region" aria-label={TITLES[row] ?? row}>
    <div class="top"><h4>{TITLES[row] ?? row}</h4><button class="close" aria-label="Close" onclick={onclose}>×</button></div>

    {#if row === 'inputs' || row === 'send' || row === 'receive'}
      {#if call}
        <p class="hint">{row === 'inputs' ? 'The request itself (its query, headers and body) is edited in the request.' : 'The message belongs to the request: change it there.'} Values this step passes in:</p>
        <PairsEditor rows={call.with} onchange={(rows: Pair[]) => FlowService.SetPairs(node, 'with', rows)} valueLabel="value (template)" />
        {#if call.kind === 'request'}
          <span class="label">Extra headers for this step</span>
          <PairsEditor rows={call.headers ?? []} onchange={(rows: Pair[]) => FlowService.SetPairs(node, 'headers', rows)} keyLabel="header" valueLabel="value (template)" />
        {/if}
      {:else}
        <ParamFields {view} />
      {/if}
    {:else if row === 'hooks' && call?.kind === 'request'}
      <label class="field">
        <span class="label">Which hooks run with this step</span>
        <select value={call.hooks} onchange={(e) => FlowService.SetHooks(node, e.currentTarget.value)}>
          <option value="all">all of them</option><option value="none">none</option><option value="skip-pre">all but the ones before</option><option value="skip-post">all but the ones after</option>
        </select>
      </label>
    {:else if row === 'outcomes' && call?.kind === 'request'}
      <p class="hint">{call.outcomesOwn ? 'This step has its own outcomes, used in this flow only.' : "The request's outcomes. Change one and this step gets its own list, used in this flow only."} The arrows leaving the card follow them.</p>
      <OutcomesEditor outcomes={call.outcomes} ruleKind={call.ruleKind} resetLabel="Use the request's" hideReset={!call.outcomesOwn}
        onchange={(rows) => FlowService.SetStepOutcomes(node, step.id, rows as never).catch((e) => ui.toast(errorText(e)))}>
        {#snippet footer()}
          <button title="Write this list into the request, so every flow that uses it has these outcomes" disabled={!call.outcomesOwn}
            onclick={() => FlowService.SaveStepOutcomesToRequest(node, step.id).then(() => ui.toast('Saved to the request.')).catch((e) => ui.toast(errorText(e)))}>Save to the request</button>
        {/snippet}
      </OutcomesEditor>
    {:else if row === 'connection' && call?.kind === 'request'}
      <TextField label="Open connection" value={call.open ?? ''} oninput={(v) => text('open', v)} placeholder="name (WebSocket, MQTT 5, NATS)" />
      <TextField label="Use connection" value={call.over ?? ''} oninput={(v) => text('over', v)} placeholder="a name opened by an earlier step" />
      {#if call.over}
        <label class="check"><input type="checkbox" checked={call.close ?? false} onchange={(e) => FlowService.SetClose(node, e.currentTarget.checked)} /> Close it after this step</label>
      {/if}
    {:else if row === 'settings'}
      <TextField label="Timeout (e.g. 5s)" value={step.timeout} oninput={(v) => text('timeout', v)} />
      <TemplateField label="Run when (a test, no braces)" help="flow.when" placeholder="vars.count > 3" value={step.when} oninput={(v) => text('when', v)} suggest={completeFor(node)} vocabulary={vocabularyFor(node, 'flow.when')} />
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
      <p class="hint">Retries and the request's own timeout are settings of the request.</p>
    {:else if row === 'values'}
      {#if step.type === 'set'}
        <ValuesEditor rows={step.values ?? []} onchange={(rows: ValueRow[]) => FlowService.SetValues(node, rows)} />
      {:else}
        <ParamFields {view} />
      {/if}
    {:else}
      <ParamFields {view} />
    {/if}
  </div>
{/if}

<style>
  .rowedit { display: flex; flex-direction: column; gap: 8px; }
  .top { display: flex; align-items: center; justify-content: space-between; }
  h4 { margin: 0; font-size: var(--fs-md); }
  .close { background: none; border: 0; color: var(--muted); font: inherit; font-size: var(--fs-lg); line-height: 1; }
  .close:hover { color: var(--text); }
  .hint { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .field { display: flex; flex-direction: column; gap: 3px; }
  .label { color: var(--muted); font-size: var(--fs-xs); }
  .check { display: flex; align-items: center; gap: 6px; }
  select { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 6px; font: inherit; }
</style>
